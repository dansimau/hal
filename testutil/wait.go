package testutil

import (
	"strings"
	"testing"
	"time"
)

const (
	pollInterval = 100 * time.Millisecond
	waitTimeout  = 3 * time.Second
)

// WaitFor waits for the given function to return true.
func WaitFor(t *testing.T, callbackFn func() bool, msg ...string) {
	t.Helper()

	timeout := time.After(waitTimeout)

	for {
		select {
		case <-timeout:
			t.Fatal(strings.Join(msg, " "))
		default:
			if callbackFn() {
				return
			}

			time.Sleep(pollInterval)
		}
	}
}
