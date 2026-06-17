package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/browler/airthings-monitor/internal/airthings"
	"github.com/browler/airthings-monitor/internal/config"
	"github.com/browler/airthings-monitor/internal/db"
	"github.com/browler/airthings-monitor/internal/httpapi"
	"github.com/browler/airthings-monitor/internal/scheduler"
)

func main() {
	configPath := flag.String("config", "/etc/airthings-monitor/config.toml", "path to TOML config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.LogLevel)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := db.Open(ctx, cfg)
	if err != nil {
		logger.Error("open database", "path", cfg.DatabasePath, "error", err)
		os.Exit(1)
	}
	defer store.Close()

	var sensor airthings.Client
	switch cfg.SensorMode {
	case "mock":
		logger.Warn("using mock sensor client")
		sensor = airthings.NewMockClient()
	default:
		sensor = airthings.NewBLEClient(cfg.SensorAddress, airthings.BLEOptions{
			DiscoveryTimeout:  cfg.BLEDiscoveryTimeout,
			ConnectTimeout:    cfg.BLEConnectTimeout,
			ServicesTimeout:   cfg.BLEServicesTimeout,
			ReadTimeout:       cfg.BLEReadTimeout,
			DisconnectTimeout: cfg.BLEDisconnectTimeout,
		}, logger)
	}

	retention := time.Duration(0)
	if cfg.RetentionDays > 0 {
		retention = time.Duration(cfg.RetentionDays) * 24 * time.Hour
	}
	poller := scheduler.NewPoller(sensor, store, scheduler.PollerConfig{
		PollEvery: cfg.PollInterval,
		RetryJitter: scheduler.RetryJitter{
			Min: cfg.MinRetryDelay,
			Max: cfg.MaxRetryDelay,
		},
		Intervals: scheduler.Intervals{
			CO2:         cfg.CO2Interval,
			Environment: cfg.EnvironmentInterval,
			Radon:       cfg.RadonInterval,
		},
		Retention:    retention,
		CleanupEvery: cfg.RetentionCleanupInterval,
	}, logger)

	api := httpapi.New(store, poller, logger, httpapi.Options{
		SensorAddress:   cfg.SensorAddress,
		DatabasePath:    cfg.DatabasePath,
		StaleAfter:      cfg.StaleAfter,
		Thresholds:      cfg.Thresholds,
		FrontendEnabled: cfg.FrontendEnabled,
		FrontendDir:     cfg.FrontendDir,
	})

	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           api,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go poller.Run(ctx)
	go func() {
		logger.Info("starting HTTP server", "listen_address", cfg.ListenAddress)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown failed", "error", err)
	}
	logger.Info("airthings monitor stopped")
}

func newLogger(level string) *slog.Logger {
	var slogLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slogLevel}))
}
