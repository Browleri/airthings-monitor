package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"log/slog"

	"github.com/browler/airthings-monitor/internal/config"
	"github.com/browler/airthings-monitor/internal/db"
	"github.com/browler/airthings-monitor/internal/scheduler"
)

type fakeStore struct{}

func (fakeStore) Current(ctx context.Context) (db.Current, error) {
	co2 := 710
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	return db.Current{CO2PPM: &co2, LastReadAt: &now}, nil
}

func (fakeStore) Ping(ctx context.Context) error {
	return nil
}

func (fakeStore) Readings(ctx context.Context, metric string, since time.Time) ([]db.MetricPoint, error) {
	return []db.MetricPoint{{RecordedAt: since.Add(time.Minute), Value: 710}}, nil
}

func (fakeStore) Summary(ctx context.Context, since time.Time) ([]db.MetricSummary, error) {
	return []db.MetricSummary{{Metric: "co2", Count: 1, Min: 710, Max: 710, Avg: 710}}, nil
}

type fakeStatus struct {
	status scheduler.Status
}

func (f fakeStatus) Status() scheduler.Status {
	return f.status
}

func TestCurrentHandler(t *testing.T) {
	now := time.Now().UTC()
	server := New(fakeStore{}, fakeStatus{status: scheduler.Status{LastSuccessAt: &now}}, slog.Default(), Options{
		SensorAddress: "D8:71:4D:AA:78:34",
		DatabasePath:  "/mnt/usb/airthings/airthings.db",
		StaleAfter:    3 * time.Minute,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/current", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["co2_ppm"].(float64) != 710 {
		t.Fatalf("co2_ppm = %v, want 710", body["co2_ppm"])
	}
}

func TestReadingsRejectsUnknownRange(t *testing.T) {
	server := New(fakeStore{}, fakeStatus{}, slog.Default(), Options{StaleAfter: time.Minute})
	req := httptest.NewRequest(http.MethodGet, "/api/readings?metric=co2&range=2h", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestStatusHandlerResponseShape(t *testing.T) {
	lastSuccess := time.Now().UTC().Add(-time.Minute)
	lastErrorAt := time.Now().UTC().Add(-30 * time.Second)
	retryDelay := 55 * time.Second
	server := New(fakeStore{}, fakeStatus{status: scheduler.Status{
		LastSuccessAt:       &lastSuccess,
		LastErrorAt:         &lastErrorAt,
		LastError:           "le-connection-abort-by-local",
		LastErrorKind:       "sensor",
		LastRetryDelay:      &retryDelay,
		ConsecutiveFailures: 1,
	}}, slog.Default(), Options{
		SensorAddress: "D8:71:4D:AA:78:34",
		DatabasePath:  "/mnt/pihole-usb/airthings/airthings.db",
		StaleAfter:    10 * time.Minute,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["sensor_stale"].(bool) {
		t.Fatalf("sensor_stale = true, want false")
	}
	if body["database_ok"].(bool) != true {
		t.Fatalf("database_ok = %v, want true", body["database_ok"])
	}
	if body["bluetooth_ok"].(bool) != false {
		t.Fatalf("bluetooth_ok = %v, want false", body["bluetooth_ok"])
	}
	if body["last_error"] != "le-connection-abort-by-local" {
		t.Fatalf("last_error = %v", body["last_error"])
	}
	if _, ok := body["last_successful_read"]; !ok {
		t.Fatalf("last_successful_read missing from response")
	}
	if _, ok := body["last_error_at"]; !ok {
		t.Fatalf("last_error_at missing from response")
	}
	if body["last_retry_delay_seconds"].(float64) != 55 {
		t.Fatalf("last_retry_delay_seconds = %v, want 55", body["last_retry_delay_seconds"])
	}
}

func TestThresholdsHandler(t *testing.T) {
	max := 1000.0
	server := New(fakeStore{}, fakeStatus{}, slog.Default(), Options{
		Thresholds: config.Thresholds{
			"co2": {{Level: "good", Max: &max}},
		},
		StaleAfter: time.Minute,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/thresholds", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var body struct {
		Metrics map[string][]config.ThresholdBand `json:"metrics"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := body.Metrics["co2"][0].Level; got != "good" {
		t.Fatalf("co2 level = %q, want good", got)
	}
	if body.Metrics["co2"][0].Max == nil || *body.Metrics["co2"][0].Max != 1000 {
		t.Fatalf("co2 max = %v, want 1000", body.Metrics["co2"][0].Max)
	}
}

func TestSensorStale(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-9 * time.Minute)
	old := now.Add(-11 * time.Minute)

	if sensorStale(&recent, 10*time.Minute, now) {
		t.Fatalf("recent reading marked stale")
	}
	if !sensorStale(&old, 10*time.Minute, now) {
		t.Fatalf("old reading not marked stale")
	}
	if !sensorStale(nil, 10*time.Minute, now) {
		t.Fatalf("nil last success should be stale")
	}
}
