package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/browler/airthings-monitor/internal/airthings"
)

type Writer interface {
	InsertReading(ctx context.Context, reading SampledReading) error
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

type Status struct {
	LastSuccessAt       *time.Time `json:"last_success_at"`
	LastAttemptAt       *time.Time `json:"last_attempt_at"`
	LastError           string     `json:"last_error,omitempty"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
}

type Poller struct {
	client       airthings.Client
	writer       Writer
	sampler      *Sampler
	pollEvery    time.Duration
	retention    time.Duration
	cleanupEvery time.Duration
	logger       *slog.Logger

	mu     sync.RWMutex
	status Status
}

type PollerConfig struct {
	PollEvery    time.Duration
	Intervals    Intervals
	Retention    time.Duration
	CleanupEvery time.Duration
}

func NewPoller(client airthings.Client, writer Writer, cfg PollerConfig, logger *slog.Logger) *Poller {
	return &Poller{
		client:       client,
		writer:       writer,
		sampler:      NewSampler(cfg.Intervals),
		pollEvery:    cfg.PollEvery,
		retention:    cfg.Retention,
		cleanupEvery: cfg.CleanupEvery,
		logger:       logger,
	}
}

func (p *Poller) Run(ctx context.Context) {
	p.poll(ctx)

	pollTicker := time.NewTicker(p.pollEvery)
	defer pollTicker.Stop()

	cleanupTicker := time.NewTicker(p.cleanupEvery)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pollTicker.C:
			p.poll(ctx)
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

func (p *Poller) poll(ctx context.Context) {
	now := time.Now().UTC()
	p.setAttempt(now)

	reading, err := p.client.Read(ctx)
	if err != nil {
		p.recordFailure("sensor read failed", err)
		return
	}
	reading.RecordedAt = now

	decision := p.sampler.Decide(now)
	sampled := ApplyDecision(reading, decision)
	if err := p.writer.InsertReading(ctx, sampled); err != nil {
		p.recordFailure("database insert failed", err)
		return
	}

	p.sampler.MarkStored(now, decision)
	p.recordSuccess(now)
	p.logger.Info("stored reading", "co2", decision.CO2, "environment", decision.Environment, "radon", decision.Radon)
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

func (p *Poller) recordFailure(msg string, err error) {
	p.mu.Lock()
	p.status.LastError = err.Error()
	p.status.ConsecutiveFailures++
	p.mu.Unlock()
	p.logger.Error(msg, "error", err, "retry_after", p.pollEvery)
}

func (p *Poller) recordSuccess(now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.status.LastSuccessAt = &now
	p.status.LastError = ""
	p.status.ConsecutiveFailures = 0
}
