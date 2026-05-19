package scheduler

import (
	"testing"
	"time"
)

func TestSamplerInitialTickSamplesAllGroups(t *testing.T) {
	s := NewSampler(Intervals{
		CO2:         time.Minute,
		Environment: 5 * time.Minute,
		Radon:       time.Hour,
	})

	d := s.Decide(time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC))
	if !d.CO2 || !d.Environment || !d.Radon {
		t.Fatalf("initial decision = %+v, want all groups", d)
	}
}

func TestSamplerRespectsIntervals(t *testing.T) {
	s := NewSampler(Intervals{
		CO2:         time.Minute,
		Environment: 5 * time.Minute,
		Radon:       time.Hour,
	})
	start := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	first := s.Decide(start)
	s.MarkStored(start, first)

	d := s.Decide(start.Add(time.Minute))
	if !d.CO2 || d.Environment || d.Radon {
		t.Fatalf("one minute decision = %+v, want only co2", d)
	}

	d = s.Decide(start.Add(5 * time.Minute))
	if !d.CO2 || !d.Environment || d.Radon {
		t.Fatalf("five minute decision = %+v, want co2 and environment", d)
	}

	d = s.Decide(start.Add(time.Hour))
	if !d.CO2 || !d.Environment || !d.Radon {
		t.Fatalf("hour decision = %+v, want all groups", d)
	}
}
