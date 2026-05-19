package airthings

import (
	"errors"
	"testing"
)

func TestDecodeWavePlusPayloadFixture(t *testing.T) {
	reading, err := DecodeHexPayload("01645bff0e00110068098bc5c6027b0000c0a302")
	if err != nil {
		t.Fatalf("DecodeHexPayload returned error: %v", err)
	}

	if reading.HumidityPercent != 50.0 {
		t.Fatalf("humidity = %v, want 50.0", reading.HumidityPercent)
	}
	if reading.RadonShortBqm3 != 14 {
		t.Fatalf("radon short = %v, want 14", reading.RadonShortBqm3)
	}
	if reading.RadonLongBqm3 != 17 {
		t.Fatalf("radon long = %v, want 17", reading.RadonLongBqm3)
	}
	if reading.TemperatureC != 24.08 {
		t.Fatalf("temperature = %v, want 24.08", reading.TemperatureC)
	}
	if reading.PressureHPa != 1011.42 {
		t.Fatalf("pressure = %v, want 1011.42", reading.PressureHPa)
	}
	if reading.CO2PPM != 710 {
		t.Fatalf("co2 = %v, want 710", reading.CO2PPM)
	}
	if reading.VOCppb != 123 {
		t.Fatalf("voc = %v, want 123", reading.VOCppb)
	}
}

func TestDecodeWavePlusPayloadTooShort(t *testing.T) {
	_, err := DecodeWavePlusPayload([]byte{0x01, 0x64})
	if !errors.Is(err, ErrPayloadTooShort) {
		t.Fatalf("error = %v, want ErrPayloadTooShort", err)
	}
}
