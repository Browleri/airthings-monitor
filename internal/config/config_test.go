package config

import "testing"

func TestExampleConfigLoads(t *testing.T) {
	cfg, err := Load("../../config.example.toml")
	if err != nil {
		t.Fatalf("load example config: %v", err)
	}
	if len(cfg.Thresholds["co2"]) != 3 {
		t.Fatalf("co2 bands = %d, want 3", len(cfg.Thresholds["co2"]))
	}
}

func TestDefaultThresholdsValidate(t *testing.T) {
	cfg := Defaults()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config did not validate: %v", err)
	}
	if _, ok := cfg.Thresholds["pressure"]; ok {
		t.Fatalf("pressure should not have default quality bands")
	}
	if len(cfg.Thresholds["co2"]) != 3 {
		t.Fatalf("co2 bands = %d, want 3", len(cfg.Thresholds["co2"]))
	}
}

func TestThresholdValidationRejectsUnknownMetric(t *testing.T) {
	cfg := Defaults()
	cfg.Thresholds = Thresholds{
		"pressure": {{Level: "good", Min: floatPtr(1000), Max: floatPtr(1030)}},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected unsupported metric to fail validation")
	}
}

func TestThresholdValidationRejectsInvalidBand(t *testing.T) {
	cfg := Defaults()
	cfg.Thresholds = Thresholds{
		"co2": {{Level: "warning", Max: floatPtr(1000)}},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected invalid threshold level to fail validation")
	}
}

func TestThresholdValidationRejectsInvertedRange(t *testing.T) {
	cfg := Defaults()
	cfg.Thresholds = Thresholds{
		"co2": {{Level: "bad", Min: floatPtr(1500), Max: floatPtr(1000)}},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected inverted threshold range to fail validation")
	}
}
