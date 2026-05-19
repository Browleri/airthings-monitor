package airthings

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	MeasurementsCharacteristicUUID = "b42e2a68-ade7-11e4-89d3-123b93f75cba"
	ExpectedPayloadLength          = 20
)

var ErrPayloadTooShort = errors.New("airthings payload too short")

type Reading struct {
	RecordedAt      time.Time `json:"recorded_at"`
	HumidityPercent float64   `json:"humidity_percent"`
	RadonShortBqm3  int       `json:"radon_short_bqm3"`
	RadonLongBqm3   int       `json:"radon_long_bqm3"`
	TemperatureC    float64   `json:"temperature_c"`
	PressureHPa     float64   `json:"pressure_hpa"`
	CO2PPM          int       `json:"co2_ppm"`
	VOCppb          int       `json:"voc_ppb"`
	RawPayload      []byte    `json:"-"`
}

type Client interface {
	Read(ctx context.Context) (Reading, error)
}

// DecodeWavePlusPayload decodes the 20-byte current measurements payload from
// an Airthings Wave Plus. The format is little-endian and returns all current
// metrics in a single BLE characteristic read.
func DecodeWavePlusPayload(payload []byte) (Reading, error) {
	if len(payload) < ExpectedPayloadLength {
		return Reading{}, fmt.Errorf("%w: got %d bytes, need %d", ErrPayloadTooShort, len(payload), ExpectedPayloadLength)
	}

	return Reading{
		HumidityPercent: float64(payload[1]) / 2,
		RadonShortBqm3:  int(binary.LittleEndian.Uint16(payload[4:6])),
		RadonLongBqm3:   int(binary.LittleEndian.Uint16(payload[6:8])),
		TemperatureC:    float64(binary.LittleEndian.Uint16(payload[8:10])) / 100,
		PressureHPa:     float64(binary.LittleEndian.Uint16(payload[10:12])) / 50,
		CO2PPM:          int(binary.LittleEndian.Uint16(payload[12:14])),
		VOCppb:          int(binary.LittleEndian.Uint16(payload[14:16])),
		RawPayload:      append([]byte(nil), payload...),
	}, nil
}

func DecodeHexPayload(raw string) (Reading, error) {
	payload, err := hex.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return Reading{}, err
	}
	return DecodeWavePlusPayload(payload)
}
