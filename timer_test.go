package hal_test

import (
	"testing"
	"time"

	"github.com/dansimau/hal"
)

func TestTimer(t *testing.T) {
	t.Parallel()

	timerRan := false

	var timer hal.Timer

	timer.Start(func() {
		timerRan = true
	}, 100*time.Millisecond)

	time.Sleep(200 * time.Millisecond)

	if !timerRan {
		t.Error("Timer did not run")
	}
}
