package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/browler/airthings-monitor/internal/db"
	"github.com/browler/airthings-monitor/internal/scheduler"
)

type Store interface {
	Current(ctx context.Context) (db.Current, error)
	Ping(ctx context.Context) error
	Readings(ctx context.Context, metric string, since time.Time) ([]db.MetricPoint, error)
	Summary(ctx context.Context, since time.Time) ([]db.MetricSummary, error)
}

type StatusProvider interface {
	Status() scheduler.Status
}

type Server struct {
	store           Store
	statusProvider  StatusProvider
	logger          *slog.Logger
	sensorAddress   string
	databasePath    string
	staleAfter      time.Duration
	frontendEnabled bool
	frontendDir     string
	mux             *http.ServeMux
}

type Options struct {
	SensorAddress   string
	DatabasePath    string
	StaleAfter      time.Duration
	FrontendEnabled bool
	FrontendDir     string
}

func New(store Store, statusProvider StatusProvider, logger *slog.Logger, opts Options) *Server {
	s := &Server{
		store:           store,
		statusProvider:  statusProvider,
		logger:          logger,
		sensorAddress:   opts.SensorAddress,
		databasePath:    opts.DatabasePath,
		staleAfter:      opts.StaleAfter,
		frontendEnabled: opts.FrontendEnabled,
		frontendDir:     opts.FrontendDir,
		mux:             http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.healthz)
	s.mux.HandleFunc("GET /api/current", s.current)
	s.mux.HandleFunc("GET /api/readings", s.readings)
	s.mux.HandleFunc("GET /api/summary", s.summary)
	s.mux.HandleFunc("GET /api/status", s.status)
	s.mux.HandleFunc("/", s.frontend)
}

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) current(w http.ResponseWriter, r *http.Request) {
	current, err := s.store.Current(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	status := s.statusProvider.Status()
	stale := sensorStale(status.LastSuccessAt, s.staleAfter, time.Now())
	writeJSON(w, http.StatusOK, map[string]any{
		"co2_ppm":              current.CO2PPM,
		"voc_ppb":              current.VOCppb,
		"temperature_c":        current.TemperatureC,
		"humidity_percent":     current.HumidityPercent,
		"pressure_hpa":         current.PressureHPa,
		"radon_short_bqm3":     current.RadonShortBqm3,
		"radon_long_bqm3":      current.RadonLongBqm3,
		"last_read_at":         current.LastReadAt,
		"last_successful_read": status.LastSuccessAt,
		"sensor_stale":         stale,
	})
}

func (s *Server) readings(w http.ResponseWriter, r *http.Request) {
	metric := r.URL.Query().Get("metric")
	if metric == "" {
		metric = "co2"
	}
	since, err := rangeStart(r.URL.Query().Get("range"), time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	points, err := s.store.Readings(r.Context(), metric, since)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"metric": metric,
		"range":  normalizedRange(r.URL.Query().Get("range")),
		"points": points,
	})
}

func (s *Server) summary(w http.ResponseWriter, r *http.Request) {
	since, err := rangeStart(r.URL.Query().Get("range"), time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	summaries, err := s.store.Summary(r.Context(), since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"range":   normalizedRange(r.URL.Query().Get("range")),
		"metrics": summaries,
	})
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	status := s.statusProvider.Status()
	databaseOK := true
	databaseErr := ""
	if err := s.store.Ping(r.Context()); err != nil {
		databaseOK = false
		databaseErr = err.Error()
	}
	stale := sensorStale(status.LastSuccessAt, s.staleAfter, time.Now())
	bluetoothOK := status.LastErrorKind != "sensor"
	var retryDelaySeconds *int
	if status.LastRetryDelay != nil {
		seconds := int(status.LastRetryDelay.Seconds())
		retryDelaySeconds = &seconds
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                       true,
		"sensor_stale":             stale,
		"stale":                    stale,
		"sensor_address":           s.sensorAddress,
		"database_path":            s.databasePath,
		"stale_after_seconds":      int(s.staleAfter.Seconds()),
		"last_successful_read":     status.LastSuccessAt,
		"last_success_at":          status.LastSuccessAt,
		"last_attempt_at":          status.LastAttemptAt,
		"last_error":               status.LastError,
		"last_error_at":            status.LastErrorAt,
		"last_error_kind":          status.LastErrorKind,
		"last_retry_delay_seconds": retryDelaySeconds,
		"consecutive_failures":     status.ConsecutiveFailures,
		"database_ok":              databaseOK,
		"database_error":           databaseErr,
		"bluetooth_ok":             bluetoothOK,
	})
}

func sensorStale(lastSuccess *time.Time, threshold time.Duration, now time.Time) bool {
	if lastSuccess == nil {
		return true
	}
	return now.Sub(*lastSuccess) > threshold
}

func (s *Server) frontend(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	if !s.frontendEnabled {
		http.NotFound(w, r)
		return
	}
	if s.frontendDir == "" {
		http.Error(w, "frontend directory is not configured", http.StatusServiceUnavailable)
		return
	}

	path := filepath.Join(s.frontendDir, filepath.Clean(r.URL.Path))
	info, err := os.Stat(path)
	if err == nil && !info.IsDir() {
		http.ServeFile(w, r, path)
		return
	}

	index := filepath.Join(s.frontendDir, "index.html")
	if _, err := os.Stat(index); err != nil {
		http.Error(w, "frontend is not built; run make web-build", http.StatusServiceUnavailable)
		return
	}
	http.ServeFile(w, r, index)
}

func rangeStart(raw string, now time.Time) (time.Time, error) {
	switch normalizedRange(raw) {
	case "1h":
		return now.Add(-time.Hour), nil
	case "24h":
		return now.Add(-24 * time.Hour), nil
	case "7d":
		return now.Add(-7 * 24 * time.Hour), nil
	case "30d":
		return now.Add(-30 * 24 * time.Hour), nil
	default:
		return time.Time{}, errors.New("range must be one of 1h, 24h, 7d, 30d")
	}
}

func normalizedRange(raw string) string {
	if raw == "" {
		return "24h"
	}
	return raw
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
