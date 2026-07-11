package store_test

import (
	"math"
	"testing"

	"github.com/dansimau/hal/store"
	"gotest.tools/v3/assert"
)

func TestHistogramQuantileAccuracy(t *testing.T) {
	t.Parallel()

	// A uniform distribution of 1..10000. The true p99 (nearest-rank) is 9900.
	counts := make(map[int32]int64)
	for i := int64(1); i <= 10000; i++ {
		counts[store.HistogramBucket(i)]++
	}

	got := store.HistogramQuantile(counts, 0.99)

	// The result must be within the bucketing's relative accuracy of the true
	// value (allowing a little slack for the nearest-rank bucket boundary).
	relErr := math.Abs(float64(got)-9900) / 9900
	assert.Assert(t, relErr <= 0.05, "p99 = %d, relative error %.3f", got, relErr)
}

func TestHistogramQuantileMergesExactly(t *testing.T) {
	t.Parallel()

	// Splitting the same samples across separate per-minute histograms and merging
	// them must yield the same quantile as summarising them all at once.
	whole := make(map[int32]int64)
	minuteA := make(map[int32]int64)
	minuteB := make(map[int32]int64)
	for i := int64(1); i <= 10000; i++ {
		b := store.HistogramBucket(i)
		whole[b]++
		if i%2 == 0 {
			minuteA[b]++
		} else {
			minuteB[b]++
		}
	}

	merged := store.MergeHistograms(minuteA, minuteB)

	assert.Equal(t, store.HistogramQuantile(merged, 0.99), store.HistogramQuantile(whole, 0.99))
	assert.Equal(t, store.HistogramQuantile(merged, 0.50), store.HistogramQuantile(whole, 0.50))
}

func TestHistogramQuantileEmpty(t *testing.T) {
	t.Parallel()

	assert.Equal(t, store.HistogramQuantile(map[int32]int64{}, 0.99), int64(0))
}
