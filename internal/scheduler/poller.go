package scheduler

import (
	"context"
	"fmt"
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

// Notifier delivers a push notification for a threshold transition.
type Notifier interface {
	Notify(ctx context.Context, title, message, priority, tags string) error
}

type alertState struct {
	level          string
	lastNotifiedAt time.Time
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

	alertEval      func(metric string, value float64) string
	notifier       Notifier
	notifyCooldown time.Duration
	alertStates    map[string]*alertState

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

	// AlertEval maps a metric name and value to a threshold level ("good",
	// "bad", "critical"). Nil disables notifications.
	AlertEval      func(metric string, value float64) string
	Notifier       Notifier
	NotifyCooldown time.Duration
}

func NewPoller(client airthings.Client, writer Writer, cfg PollerConfig, logger *slog.Logger) *Poller {
	if cfg.RetryJitter.Min <= 0 {
		cfg.RetryJitter.Min = cfg.PollEvery
	}
	if cfg.RetryJitter.Max <= 0 {
		cfg.RetryJitter.Max = cfg.RetryJitter.Min
	}
	p := &Poller{
		client:         client,
		writer:         writer,
		sampler:        NewSampler(cfg.Intervals),
		pollEvery:      cfg.PollEvery,
		retryJitter:    cfg.RetryJitter,
		retention:      cfg.Retention,
		cleanupEvery:   cfg.CleanupEvery,
		logger:         logger,
		rand:           rand.New(rand.NewSource(time.Now().UnixNano())),
		alertEval:      cfg.AlertEval,
		notifier:       cfg.Notifier,
		notifyCooldown: cfg.NotifyCooldown,
	}
	if cfg.AlertEval != nil && cfg.Notifier != nil {
		p.alertStates = make(map[string]*alertState)
	}
	return p
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
	p.checkAlerts(ctx, reading)
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

func (p *Poller) checkAlerts(ctx context.Context, reading airthings.Reading) {
	if p.alertStates == nil {
		return
	}
	values := map[string]float64{
		"co2":         float64(reading.CO2PPM),
		"voc":         float64(reading.VOCppb),
		"temperature": reading.TemperatureC,
		"humidity":    reading.HumidityPercent,
		"radon_short": float64(reading.RadonShortBqm3),
		"radon_long":  float64(reading.RadonLongBqm3),
	}
	for metric, value := range values {
		level := p.alertEval(metric, value)
		state, exists := p.alertStates[metric]
		if !exists {
			p.alertStates[metric] = &alertState{level: level}
			continue
		}
		if level == state.level {
			continue
		}
		escalating := levelScore(level) > levelScore(state.level)
		cooldownPassed := p.notifyCooldown <= 0 || time.Since(state.lastNotifiedAt) >= p.notifyCooldown
		if escalating || cooldownPassed {
			title, message, priority, tags := buildAlertMessage(metric, value, level)
			if err := p.notifier.Notify(ctx, title, message, priority, tags); err != nil {
				p.logger.Warn("notification failed", "metric", metric, "error", err)
			} else {
				state.lastNotifiedAt = time.Now()
				p.logger.Info("notification sent", "metric", metric, "level", level)
			}
		}
		state.level = level
	}
}

func levelScore(level string) int {
	switch level {
	case "critical":
		return 2
	case "bad":
		return 1
	default:
		return 0
	}
}

var alertMetricMeta = map[string]struct{ label, unit, format string }{
	"co2":         {"CO2", "ppm", "%.0f"},
	"voc":         {"VOC", "ppb", "%.0f"},
	"temperature": {"Temperature", "°C", "%.1f"},
	"humidity":    {"Humidity", "%", "%.1f"},
	"radon_short": {"Radon short-term", "Bq/m³", "%.0f"},
	"radon_long":  {"Radon long-term", "Bq/m³", "%.0f"},
}

func buildAlertMessage(metric string, value float64, level string) (title, message, priority, tags string) {
	meta, ok := alertMetricMeta[metric]
	if !ok {
		return metric + " alert", fmt.Sprintf("%.1f", value), "default", ""
	}
	formatted := fmt.Sprintf(meta.format, value)
	switch level {
	case "critical":
		title = fmt.Sprintf("%s — Critical", meta.label)
		priority = "high"
		tags = "rotating_light"
	case "bad":
		title = fmt.Sprintf("%s — Elevated", meta.label)
		priority = "default"
		tags = "warning"
	default:
		title = fmt.Sprintf("%s — Recovered", meta.label)
		priority = "low"
		tags = "white_check_mark"
	}
	message = fmt.Sprintf("%s is now %s %s", meta.label, formatted, meta.unit)
	return
}
