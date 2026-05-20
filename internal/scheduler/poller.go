package scheduler

import (
	"context"
	"log/slog"
	"math/rand"
	"sync"
	"time"

	"github.com/browler/airthings-monitor/internal/airthings"
)

type Writer interface {
	InsertReading(ctx context.Context, reading SampledReading) error
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

type Status struct {
	LastSuccessAt       *time.Time     `json:"last_success_at"`
	LastAttemptAt       *time.Time     `json:"last_attempt_at"`
	LastErrorAt         *time.Time     `json:"last_error_at,omitempty"`
	LastError           string         `json:"last_error,omitempty"`
	LastErrorKind       string         `json:"last_error_kind,omitempty"`
	LastRetryDelay      *time.Duration `json:"-"`
	ConsecutiveFailures int            `json:"consecutive_failures"`
}

type Poller struct {
	client       airthings.Client
	writer       Writer
	sampler      *Sampler
	pollEvery    time.Duration
	retryJitter  RetryJitter
	retention    time.Duration
	cleanupEvery time.Duration
	logger       *slog.Logger

	mu     sync.RWMutex
	status Status
	rand   *rand.Rand
}

type PollerConfig struct {
	PollEvery    time.Duration
	RetryJitter  RetryJitter
	Intervals    Intervals
	Retention    time.Duration
	CleanupEvery time.Duration
}

func NewPoller(client airthings.Client, writer Writer, cfg PollerConfig, logger *slog.Logger) *Poller {
	if cfg.RetryJitter.Min <= 0 {
		cfg.RetryJitter.Min = cfg.PollEvery
	}
	if cfg.RetryJitter.Max <= 0 {
		cfg.RetryJitter.Max = cfg.RetryJitter.Min
	}
	return &Poller{
		client:       client,
		writer:       writer,
		sampler:      NewSampler(cfg.Intervals),
		pollEvery:    cfg.PollEvery,
		retryJitter:  cfg.RetryJitter,
		retention:    cfg.Retention,
		cleanupEvery: cfg.CleanupEvery,
		logger:       logger,
		rand:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (p *Poller) Run(ctx context.Context) {
	nextDelay := p.poll(ctx)
	pollTimer := time.NewTimer(nextDelay)
	defer pollTimer.Stop()

	cleanupTicker := time.NewTicker(p.cleanupEvery)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pollTimer.C:
			nextDelay = p.poll(ctx)
			pollTimer.Reset(nextDelay)
		case <-cleanupTicker.C:
			p.cleanup(ctx)
		}
	}
}

func (p *Poller) Status() Status {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.status
}

func (p *Poller) poll(ctx context.Context) time.Duration {
	now := time.Now().UTC()
	p.setAttempt(now)

	reading, err := p.client.Read(ctx)
	if err != nil {
		return p.recordFailure("sensor read failed", "sensor", err)
	}
	reading.RecordedAt = now

	decision := p.sampler.Decide(now)
	sampled := ApplyDecision(reading, decision)
	if err := p.writer.InsertReading(ctx, sampled); err != nil {
		return p.recordFailure("database insert failed", "database", err)
	}

	p.sampler.MarkStored(now, decision)
	p.recordSuccess(now)
	p.logger.Info("stored reading", "co2", decision.CO2, "environment", decision.Environment, "radon", decision.Radon)
	return p.pollEvery
}

func (p *Poller) cleanup(ctx context.Context) {
	if p.retention <= 0 {
		return
	}
	cutoff := time.Now().UTC().Add(-p.retention)
	deleted, err := p.writer.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		p.logger.Error("retention cleanup failed", "error", err)
		return
	}
	if deleted > 0 {
		p.logger.Info("retention cleanup complete", "deleted_rows", deleted)
	}
}

func (p *Poller) setAttempt(now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status.LastAttemptAt = &now
}

func (p *Poller) recordFailure(msg string, kind string, err error) time.Duration {
	retryDelay := p.nextRetryDelay()
	now := time.Now().UTC()
	p.mu.Lock()
	p.status.LastError = err.Error()
	p.status.LastErrorAt = &now
	p.status.LastErrorKind = kind
	p.status.LastRetryDelay = &retryDelay
	p.status.ConsecutiveFailures++
	p.mu.Unlock()
	p.logger.Error(msg, "error", err, "kind", kind, "retry_after", retryDelay)
	return retryDelay
}

func (p *Poller) recordSuccess(now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status.LastSuccessAt = &now
	p.status.LastError = ""
	p.status.LastErrorAt = nil
	p.status.LastErrorKind = ""
	p.status.LastRetryDelay = nil
	p.status.ConsecutiveFailures = 0
}

func (p *Poller) nextRetryDelay() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.retryJitter.Delay(p.rand)
}
