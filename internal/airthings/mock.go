package airthings

import (
	"context"
	"math"
	"time"
)

type MockClient struct {
	start time.Time
}

func NewMockClient() *MockClient {
	return &MockClient{start: time.Now()}
}

func (c *MockClient) Read(ctx context.Context) (Reading, error) {
	select {
	case <-ctx.Done():
		return Reading{}, ctx.Err()
	default:
	}

	elapsed := time.Since(c.start).Minutes()
	reading := Reading{
		HumidityPercent: 48 + 4*math.Sin(elapsed/10),
		RadonShortBqm3:  12 + int(4*math.Sin(elapsed/60)),
		RadonLongBqm3:   15,
		TemperatureC:    23.5 + 0.8*math.Sin(elapsed/25),
		PressureHPa:     1011 + 3*math.Sin(elapsed/90),
		CO2PPM:          650 + int(80*math.Sin(elapsed/7)),
		VOCppb:          120 + int(25*math.Sin(elapsed/13)),
		RawPayload:      []byte("mock"),
	}
	return reading, nil
}
