package hassws

import (
	"sync"
	"testing"
	"time"

	"gotest.tools/v3/assert"
)

func TestNewClientReadTimeoutDefaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		pingInterval     time.Duration
		readTimeout      time.Duration
		wantPingInterval time.Duration
		wantReadTimeout  time.Duration
	}{
		{
			name:             "defaults applied when unset",
			wantPingInterval: defaultPingInterval,
			wantReadTimeout:  2 * defaultPingInterval,
		},
		{
			name:             "read timeout derived from custom ping interval",
			pingInterval:     90 * time.Second,
			wantPingInterval: 90 * time.Second,
			wantReadTimeout:  180 * time.Second,
		},
		{
			name:             "explicit read timeout above ping interval is kept",
			pingInterval:     30 * time.Second,
			readTimeout:      45 * time.Second,
			wantPingInterval: 30 * time.Second,
			wantReadTimeout:  45 * time.Second,
		},
		{
			name:             "read timeout not larger than ping interval is bumped",
			pingInterval:     30 * time.Second,
			readTimeout:      20 * time.Second,
			wantPingInterval: 30 * time.Second,
			wantReadTimeout:  60 * time.Second,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			c := NewClient(ClientConfig{
				PingInterval: tc.pingInterval,
				ReadTimeout:  tc.readTimeout,
			})

			assert.Equal(t, tc.wantPingInterval, c.cfg.PingInterval)
			assert.Equal(t, tc.wantReadTimeout, c.cfg.ReadTimeout)
		})
	}
}

// TestFrameQueueOrderAndUnbounded verifies that frameQueue preserves FIFO order,
// accepts more items than any fixed buffer without a consumer (unbounded), and
// drains fully before reporting closed.
func TestFrameQueueOrderAndUnbounded(t *testing.T) {
	t.Parallel()

	q := newFrameQueue()

	const n = 1000

	// Push far more than any fixed buffer with no consumer running; push must
	// never block.
	for i := range n {
		q.push([]byte{byte(i), byte(i >> 8)})
	}

	q.close()

	// Everything pushed comes back out in order.
	for i := range n {
		b, ok := q.pop()
		assert.Equal(t, true, ok)
		assert.Equal(t, byte(i), b[0])
		assert.Equal(t, byte(i>>8), b[1])
	}

	// Once drained and closed, pop reports no more frames.
	_, ok := q.pop()
	assert.Equal(t, false, ok)
}

// TestRunOrderedHandlerDoesNotBlockProducerOnSlowHandler verifies the property
// that keeps the shared read loop alive: a slow (parked) handler must not stop
// frames from being drained off the input channel. If handler execution were
// coupled to draining (as it is when a single goroutine both receives and
// invokes the handler), a handler blocked in a synchronous CallService would
// stall the producer - here the read loop - and deadlock it.
func TestRunOrderedHandlerDoesNotBlockProducerOnSlowHandler(t *testing.T) {
	t.Parallel()

	// Unbuffered: a send only succeeds if runOrderedHandler's drainer receives.
	ch := make(chan []byte)
	release := make(chan struct{})

	var (
		mu        sync.Mutex
		processed []byte
		parked    sync.Once
	)

	go runOrderedHandler(ch, func(b []byte) {
		// Park on the very first frame, simulating a handler blocked in a
		// synchronous CallService.
		firstFrame := false
		parked.Do(func() { firstFrame = true })

		if firstFrame {
			<-release
		}

		mu.Lock()
		processed = append(processed, b[0])
		mu.Unlock()
	})

	const n = 1000

	// Send while the handler is parked. Each send must complete promptly because
	// the drainer keeps moving frames off ch even though the handler is blocked.
	for i := range n {
		select {
		case ch <- []byte{byte(i)}:
		case <-time.After(2 * time.Second):
			close(release)
			t.Fatalf("producer blocked sending frame %d: draining is coupled to the slow handler", i)
		}
	}

	// Unblock the handler and let the queue drain.
	close(release)
	close(ch)

	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		done := len(processed) == n
		mu.Unlock()

		if done {
			break
		}

		select {
		case <-deadline:
			t.Fatal("handler did not process all frames")
		case <-time.After(time.Millisecond):
		}
	}

	// Frames were processed in the order they were sent.
	mu.Lock()
	defer mu.Unlock()

	for i := range n {
		assert.Equal(t, byte(i), processed[i])
	}
}
