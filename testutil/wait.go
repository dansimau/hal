package testutil

import (
	"testing"
	"time"
)

const (
	pollInterval = 100 * time.Millisecond
	waitTimeout  = 3 * time.Second
)

// WaitFor waits for the given function to return true.
func WaitFor(t *testing.T, name string, callbackFn func() bool, onFailed func()) {
	t.Helper()

	timeout := time.After(waitTimeout)

	for {
		select {
		case <-timeout:
			t.Errorf("assertion failed: %s", name)
			onFailed()
			t.FailNow()
		default:
			if callbackFn() {
				return
			}

			time.Sleep(pollInterval)
		}
	}
}
