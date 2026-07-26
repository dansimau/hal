package perf_test

import (
	"testing"
	"time"

	"github.com/dansimau/hal/perf"
)

func TestTimer(t *testing.T) {
	t.Parallel()

	var got time.Duration

	stop := perf.Timer(func(timeTaken time.Duration) {
		got = timeTaken
	})

	time.Sleep(5 * time.Millisecond)
	stop()

	if got <= 0 {
		t.Errorf("expected a positive elapsed duration, got %v", got)
	}
}
