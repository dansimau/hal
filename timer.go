package hal

import "time"

type Timer struct {
	timer *time.Timer
}

func (t *Timer) Cancel() {
	if t.timer != nil {
		t.timer.Stop()
	}
}

func (t *Timer) Start(callback func(), d time.Duration) {
	t.Cancel()
	t.timer = time.AfterFunc(d, callback)
}
