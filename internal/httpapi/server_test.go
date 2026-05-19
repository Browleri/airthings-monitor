package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"log/slog"

	"github.com/browler/airthings-monitor/internal/db"
	"github.com/browler/airthings-monitor/internal/scheduler"
)

type fakeStore struct{}

func (fakeStore) Current(ctx context.Context) (db.Current, error) {
	co2 := 710
	now := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	return db.Current{CO2PPM: &co2, LastReadAt: &now}, nil
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
