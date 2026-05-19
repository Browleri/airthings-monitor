package scheduler

import (
	"time"

	"github.com/browler/airthings-monitor/internal/airthings"
)

type Intervals struct {
	CO2         time.Duration
	Environment time.Duration
	Radon       time.Duration
}

type Decision struct {
	CO2         bool
	Environment bool
	Radon       bool
}

type Sampler struct {
	intervals Intervals
	lastCO2   time.Time
	lastEnv   time.Time
	lastRadon time.Time
}

func NewSampler(intervals Intervals) *Sampler {
	return &Sampler{intervals: intervals}
}

func (s *Sampler) Decide(now time.Time) Decision {
	return Decision{
		CO2:         due(s.lastCO2, now, s.intervals.CO2),
		Environment: due(s.lastEnv, now, s.intervals.Environment),
		Radon:       due(s.lastRadon, now, s.intervals.Radon),
	}
}

func (s *Sampler) MarkStored(now time.Time, d Decision) {
	if d.CO2 {
		s.lastCO2 = now
	}
	if d.Environment {
		s.lastEnv = now
	}
	if d.Radon {
		s.lastRadon = now
	}
}

func due(last, now time.Time, interval time.Duration) bool {
	return last.IsZero() || !now.Before(last.Add(interval))
}

type SampledReading struct {
	RecordedAt       time.Time
	CO2PPM           *int
	VOCppb           *int
	TemperatureC     *float64
	HumidityPercent  *float64
	PressureHPa      *float64
	RadonShortBqm3   *int
	RadonLongBqm3    *int
	RawPayload       []byte
	SamplingDecision Decision
}

func ApplyDecision(reading airthings.Reading, d Decision) SampledReading {
	out := SampledReading{
		RecordedAt:       reading.RecordedAt,
		RawPayload:       append([]byte(nil), reading.RawPayload...),
		SamplingDecision: d,
	}
	if d.CO2 {
		out.CO2PPM = ptr(reading.CO2PPM)
	}
	if d.Environment {
		out.VOCppb = ptr(reading.VOCppb)
		out.TemperatureC = ptr(reading.TemperatureC)
		out.HumidityPercent = ptr(reading.HumidityPercent)
		out.PressureHPa = ptr(reading.PressureHPa)
	}
	if d.Radon {
		out.RadonShortBqm3 = ptr(reading.RadonShortBqm3)
		out.RadonLongBqm3 = ptr(reading.RadonLongBqm3)
	}
	return out
}

func ptr[T any](v T) *T {
	return &v
}
