package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/browler/airthings-monitor/internal/config"
	"github.com/browler/airthings-monitor/internal/scheduler"
)

const initialMigration = `
CREATE TABLE IF NOT EXISTS readings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    recorded_at TEXT NOT NULL,
    co2_ppm INTEGER NULL,
    voc_ppb INTEGER NULL,
    temperature_c REAL NULL,
    humidity_percent REAL NULL,
    pressure_hpa REAL NULL,
    radon_short_bqm3 INTEGER NULL,
    radon_long_bqm3 INTEGER NULL,
    raw_payload BLOB NULL
);

CREATE INDEX IF NOT EXISTS idx_readings_recorded_at ON readings(recorded_at);

CREATE INDEX IF NOT EXISTS idx_readings_co2_time ON readings(recorded_at) WHERE co2_ppm IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_readings_voc_time ON readings(recorded_at) WHERE voc_ppb IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_readings_temperature_time ON readings(recorded_at) WHERE temperature_c IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_readings_humidity_time ON readings(recorded_at) WHERE humidity_percent IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_readings_pressure_time ON readings(recorded_at) WHERE pressure_hpa IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_readings_radon_short_time ON readings(recorded_at) WHERE radon_short_bqm3 IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_readings_radon_long_time ON readings(recorded_at) WHERE radon_long_bqm3 IS NOT NULL;
`

type Store struct {
	db *sql.DB
}

type MetricPoint struct {
	RecordedAt time.Time `json:"recorded_at"`
	Value      float64   `json:"value"`
}

type MetricSummary struct {
	Metric string  `json:"metric"`
	Count  int     `json:"count"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Avg    float64 `json:"avg"`
}

type Current struct {
	CO2PPM          *int       `json:"co2_ppm"`
	VOCppb          *int       `json:"voc_ppb"`
	TemperatureC    *float64   `json:"temperature_c"`
	HumidityPercent *float64   `json:"humidity_percent"`
	PressureHPa     *float64   `json:"pressure_hpa"`
	RadonShortBqm3  *int       `json:"radon_short_bqm3"`
	RadonLongBqm3   *int       `json:"radon_long_bqm3"`
	LastReadAt      *time.Time `json:"last_read_at"`
}

type MetricInfo struct {
	Column string
	Label  string
}

var Metrics = map[string]MetricInfo{
	"co2":         {Column: "co2_ppm", Label: "CO2 ppm"},
	"voc":         {Column: "voc_ppb", Label: "VOC ppb"},
	"temperature": {Column: "temperature_c", Label: "Temperature C"},
	"humidity":    {Column: "humidity_percent", Label: "Humidity percent"},
	"pressure":    {Column: "pressure_hpa", Label: "Pressure hPa"},
	"radon_short": {Column: "radon_short_bqm3", Label: "Radon short Bq/m3"},
	"radon_long":  {Column: "radon_long_bqm3", Label: "Radon long Bq/m3"},
}

var MetricOrder = []string{"co2", "voc", "temperature", "humidity", "pressure", "radon_short", "radon_long"}

func Open(ctx context.Context, cfg config.Config) (*Store, error) {
	if err := validateParent(cfg.DatabasePath); err != nil {
		return nil, err
	}

	values := url.Values{}
	values.Set("_journal_mode", strings.ToUpper(cfg.SQLiteJournalMode))
	values.Set("_synchronous", strings.ToUpper(cfg.SQLiteSynchronous))
	values.Set("_busy_timeout", fmt.Sprintf("%d", cfg.SQLiteBusyTimeout.Milliseconds()))
	dsn := fmt.Sprintf("file:%s?%s", cfg.DatabasePath, values.Encode())

	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)

	store := &Store{db: sqlDB}
	if err := store.configure(ctx, cfg); err != nil {
		sqlDB.Close()
		return nil, err
	}
	if err := store.Migrate(ctx); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return store, nil
}

func validateParent(path string) error {
	parent := filepath.Dir(path)
	info, err := os.Stat(parent)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("database parent directory %q does not exist; create it on the USB drive before starting", parent)
		}
		return fmt.Errorf("stat database parent directory %q: %w", parent, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("database parent path %q is not a directory", parent)
	}
	return nil
}

func (s *Store) configure(ctx context.Context, cfg config.Config) error {
	pragmas := []string{
		"PRAGMA journal_mode=" + strings.ToUpper(cfg.SQLiteJournalMode),
		"PRAGMA synchronous=" + strings.ToUpper(cfg.SQLiteSynchronous),
		fmt.Sprintf("PRAGMA busy_timeout=%d", cfg.SQLiteBusyTimeout.Milliseconds()),
		"PRAGMA foreign_keys=ON",
	}
	for _, pragma := range pragmas {
		if _, err := s.db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("set %s: %w", pragma, err)
		}
	}
	return nil
}

func (s *Store) Migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, initialMigration)
	return err
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Store) InsertReading(ctx context.Context, r scheduler.SampledReading) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO readings (
	recorded_at, co2_ppm, voc_ppb, temperature_c, humidity_percent, pressure_hpa,
	radon_short_bqm3, radon_long_bqm3, raw_payload
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.RecordedAt.UTC().Format(time.RFC3339Nano),
		nullableInt(r.CO2PPM),
		nullableInt(r.VOCppb),
		nullableFloat(r.TemperatureC),
		nullableFloat(r.HumidityPercent),
		nullableFloat(r.PressureHPa),
		nullableInt(r.RadonShortBqm3),
		nullableInt(r.RadonLongBqm3),
		r.RawPayload,
	)
	return err
}

func (s *Store) Current(ctx context.Context) (Current, error) {
	var out Current
	if t, ok, err := s.latestReadTime(ctx); err != nil {
		return Current{}, err
	} else if ok {
		out.LastReadAt = &t
	}

	out.CO2PPM = latestInt(ctx, s.db, "co2_ppm")
	out.VOCppb = latestInt(ctx, s.db, "voc_ppb")
	out.TemperatureC = latestFloat(ctx, s.db, "temperature_c")
	out.HumidityPercent = latestFloat(ctx, s.db, "humidity_percent")
	out.PressureHPa = latestFloat(ctx, s.db, "pressure_hpa")
	out.RadonShortBqm3 = latestInt(ctx, s.db, "radon_short_bqm3")
	out.RadonLongBqm3 = latestInt(ctx, s.db, "radon_long_bqm3")
	return out, nil
}

func (s *Store) Readings(ctx context.Context, metric string, since time.Time) ([]MetricPoint, error) {
	info, ok := Metrics[metric]
	if !ok {
		return nil, fmt.Errorf("unknown metric %q", metric)
	}
	query := fmt.Sprintf(`
SELECT recorded_at, %s
FROM readings
WHERE recorded_at >= ? AND %s IS NOT NULL
ORDER BY recorded_at ASC`, info.Column, info.Column)
	rows, err := s.db.QueryContext(ctx, query, since.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []MetricPoint
	for rows.Next() {
		var rawTime string
		var value float64
		if err := rows.Scan(&rawTime, &value); err != nil {
			return nil, err
		}
		recordedAt, err := time.Parse(time.RFC3339Nano, rawTime)
		if err != nil {
			return nil, fmt.Errorf("parse recorded_at %q: %w", rawTime, err)
		}
		points = append(points, MetricPoint{RecordedAt: recordedAt, Value: value})
	}
	return points, rows.Err()
}

func (s *Store) Summary(ctx context.Context, since time.Time) ([]MetricSummary, error) {
	out := make([]MetricSummary, 0, len(Metrics))
	for _, metric := range MetricOrder {
		info := Metrics[metric]
		query := fmt.Sprintf(`
SELECT COUNT(%s), MIN(%s), MAX(%s), AVG(%s)
FROM readings
WHERE recorded_at >= ? AND %s IS NOT NULL`, info.Column, info.Column, info.Column, info.Column, info.Column)
		var count int
		var min, max, avg sql.NullFloat64
		err := s.db.QueryRowContext(ctx, query, since.UTC().Format(time.RFC3339Nano)).Scan(&count, &min, &max, &avg)
		if err != nil {
			return nil, err
		}
		summary := MetricSummary{Metric: metric, Count: count}
		if count > 0 {
			summary.Min = min.Float64
			summary.Max = max.Float64
			summary.Avg = avg.Float64
		}
		out = append(out, summary)
	}
	return out, nil
}

func (s *Store) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM readings WHERE recorded_at < ?", cutoff.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) latestReadTime(ctx context.Context) (time.Time, bool, error) {
	var raw sql.NullString
	if err := s.db.QueryRowContext(ctx, "SELECT MAX(recorded_at) FROM readings").Scan(&raw); err != nil {
		return time.Time{}, false, err
	}
	if !raw.Valid {
		return time.Time{}, false, nil
	}
	t, err := time.Parse(time.RFC3339Nano, raw.String)
	if err != nil {
		return time.Time{}, false, err
	}
	return t, true, nil
}

func latestInt(ctx context.Context, database *sql.DB, column string) *int {
	query := fmt.Sprintf("SELECT %s FROM readings WHERE %s IS NOT NULL ORDER BY recorded_at DESC LIMIT 1", column, column)
	var value int
	if err := database.QueryRowContext(ctx, query).Scan(&value); err != nil {
		return nil
	}
	return &value
}

func latestFloat(ctx context.Context, database *sql.DB, column string) *float64 {
	query := fmt.Sprintf("SELECT %s FROM readings WHERE %s IS NOT NULL ORDER BY recorded_at DESC LIMIT 1", column, column)
	var value float64
	if err := database.QueryRowContext(ctx, query).Scan(&value); err != nil {
		return nil
	}
	return &value
}

func nullableInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableFloat(v *float64) any {
	if v == nil {
		return nil
	}
	return *v
}
