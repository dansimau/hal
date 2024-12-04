package perf

import (
	"time"
)

type TimerFuncCallback func(timeTaken time.Duration)

func Timer(fn TimerFuncCallback) func() {
	start := time.Now()

	return func() {
		fn(time.Since(start))
	}
}
