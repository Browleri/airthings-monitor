package scheduler

import (
	"math/rand"
	"testing"
	"time"
)

func TestRetryJitterDelayWithinBounds(t *testing.T) {
	jitter := RetryJitter{Min: 45 * time.Second, Max: 90 * time.Second}
	rng := rand.New(rand.NewSource(1))

	for i := 0; i < 1000; i++ {
		delay := jitter.Delay(rng)
		if delay < jitter.Min || delay > jitter.Max {
			t.Fatalf("delay = %s, want between %s and %s", delay, jitter.Min, jitter.Max)
		}
	}
}

func TestRetryJitterDelayUsesMinWhenRangeCollapsed(t *testing.T) {
	delay := RetryJitter{Min: time.Minute, Max: time.Minute}.Delay(rand.New(rand.NewSource(1)))
	if delay != time.Minute {
		t.Fatalf("delay = %s, want 1m", delay)
	}
}
