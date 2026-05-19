package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	SensorAddress            string        `toml:"sensor_address"`
	SensorMode               string        `toml:"sensor_mode"`
	DatabasePath             string        `toml:"database_path"`
	ListenAddress            string        `toml:"listen_address"`
	FrontendEnabled          bool          `toml:"frontend_enabled"`
	FrontendDir              string        `toml:"frontend_dir"`
	LogLevel                 string        `toml:"log_level"`
	PollInterval             time.Duration `toml:"poll_interval"`
	CO2Interval              time.Duration `toml:"co2_interval"`
	EnvironmentInterval      time.Duration `toml:"environment_interval"`
	RadonInterval            time.Duration `toml:"radon_interval"`
	StaleAfter               time.Duration `toml:"stale_after"`
	SQLiteJournalMode        string        `toml:"sqlite_journal_mode"`
	SQLiteSynchronous        string        `toml:"sqlite_synchronous"`
	SQLiteBusyTimeout        time.Duration `toml:"sqlite_busy_timeout"`
	RetentionDays            int           `toml:"retention_days"`
	RetentionCleanupInterval time.Duration `toml:"retention_cleanup_interval"`
}

func Defaults() Config {
	return Config{
		SensorAddress:            "D8:71:4D:AA:78:34",
		SensorMode:               "ble",
		DatabasePath:             "/mnt/usb/airthings/airthings.db",
		ListenAddress:            "0.0.0.0:8080",
		FrontendEnabled:          true,
		FrontendDir:              "web/dist",
		LogLevel:                 "info",
		PollInterval:             time.Minute,
		CO2Interval:              time.Minute,
		EnvironmentInterval:      5 * time.Minute,
		RadonInterval:            time.Hour,
		StaleAfter:               3 * time.Minute,
		SQLiteJournalMode:        "WAL",
		SQLiteSynchronous:        "NORMAL",
		SQLiteBusyTimeout:        5 * time.Second,
		RetentionDays:            400,
		RetentionCleanupInterval: 24 * time.Hour,
	}
}

func Load(path string) (Config, error) {
	cfg := Defaults()
	if path != "" {
		if _, err := toml.DecodeFile(path, &cfg); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return Config{}, err
			}
			return Config{}, fmt.Errorf("config file %q does not exist", path)
		}
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.SensorAddress) == "" {
		return errors.New("sensor_address is required")
	}
	switch c.SensorMode {
	case "ble", "mock":
	default:
		return fmt.Errorf("sensor_mode must be ble or mock, got %q", c.SensorMode)
	}
	if strings.TrimSpace(c.DatabasePath) == "" {
		return errors.New("database_path is required")
	}
	if strings.TrimSpace(c.ListenAddress) == "" {
		return errors.New("listen_address is required")
	}
	if c.PollInterval <= 0 {
		return errors.New("poll_interval must be positive")
	}
	if c.CO2Interval <= 0 || c.EnvironmentInterval <= 0 || c.RadonInterval <= 0 {
		return errors.New("sampling intervals must be positive")
	}
	if c.SQLiteBusyTimeout <= 0 {
		return errors.New("sqlite_busy_timeout must be positive")
	}
	if c.RetentionDays < 0 {
		return errors.New("retention_days cannot be negative")
	}
	if c.RetentionCleanupInterval <= 0 {
		return errors.New("retention_cleanup_interval must be positive")
	}

	journal := strings.ToUpper(c.SQLiteJournalMode)
	if journal != "WAL" {
		return fmt.Errorf("sqlite_journal_mode must be WAL for this service, got %q", c.SQLiteJournalMode)
	}
	sync := strings.ToUpper(c.SQLiteSynchronous)
	if sync != "NORMAL" && sync != "FULL" && sync != "OFF" {
		return fmt.Errorf("sqlite_synchronous must be NORMAL, FULL, or OFF, got %q", c.SQLiteSynchronous)
	}
	return nil
}
