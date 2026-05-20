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
	MinRetryDelay            time.Duration `toml:"min_retry_delay"`
	MaxRetryDelay            time.Duration `toml:"max_retry_delay"`
	CO2Interval              time.Duration `toml:"co2_interval"`
	EnvironmentInterval      time.Duration `toml:"environment_interval"`
	RadonInterval            time.Duration `toml:"radon_interval"`
	StaleAfter               time.Duration `toml:"stale_after"`
	BLEDiscoveryTimeout      time.Duration `toml:"ble_discovery_timeout"`
	BLEConnectTimeout        time.Duration `toml:"ble_connect_timeout"`
	BLEServicesTimeout       time.Duration `toml:"ble_services_timeout"`
	BLEReadTimeout           time.Duration `toml:"ble_read_timeout"`
	BLEDisconnectTimeout     time.Duration `toml:"ble_disconnect_timeout"`
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
		DatabasePath:             "/mnt/pihole-usb/airthings/airthings.db",
		ListenAddress:            "0.0.0.0:8080",
		FrontendEnabled:          true,
		FrontendDir:              "web/dist",
		LogLevel:                 "info",
		PollInterval:             time.Minute,
		MinRetryDelay:            45 * time.Second,
		MaxRetryDelay:            90 * time.Second,
		CO2Interval:              time.Minute,
		EnvironmentInterval:      5 * time.Minute,
		RadonInterval:            time.Hour,
		StaleAfter:               10 * time.Minute,
		BLEDiscoveryTimeout:      20 * time.Second,
		BLEConnectTimeout:        20 * time.Second,
		BLEServicesTimeout:       15 * time.Second,
		BLEReadTimeout:           5 * time.Second,
		BLEDisconnectTimeout:     5 * time.Second,
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
	if c.MinRetryDelay <= 0 || c.MaxRetryDelay <= 0 {
		return errors.New("retry delays must be positive")
	}
	if c.MinRetryDelay > c.MaxRetryDelay {
		return errors.New("min_retry_delay cannot be greater than max_retry_delay")
	}
	if c.CO2Interval <= 0 || c.EnvironmentInterval <= 0 || c.RadonInterval <= 0 {
		return errors.New("sampling intervals must be positive")
	}
	if c.StaleAfter <= 0 {
		return errors.New("stale_after must be positive")
	}
	if c.BLEDiscoveryTimeout <= 0 || c.BLEConnectTimeout <= 0 || c.BLEServicesTimeout <= 0 || c.BLEReadTimeout <= 0 || c.BLEDisconnectTimeout <= 0 {
		return errors.New("BLE timeouts must be positive")
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
