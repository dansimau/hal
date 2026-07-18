package hal_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/dansimau/hal"
)

func TestTimer(t *testing.T) {
	t.Parallel()

	var timerRan atomic.Bool

	var timer hal.Timer

	timer.Start(func() {
		timerRan.Store(true)
	}, 100*time.Millisecond)

	time.Sleep(200 * time.Millisecond)

	if !timerRan.Load() {
		t.Error("Timer did not run")
	}
}
