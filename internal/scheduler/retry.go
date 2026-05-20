package scheduler

import (
	"math/rand"
	"time"
)

type RetryJitter struct {
	Min time.Duration
	Max time.Duration
}

func (r RetryJitter) Delay(rng *rand.Rand) time.Duration {
	if r.Min <= 0 && r.Max <= 0 {
		return 0
	}
	if r.Max <= r.Min {
		return r.Min
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	span := int64(r.Max - r.Min)
	return r.Min + time.Duration(rng.Int63n(span+1))
}
