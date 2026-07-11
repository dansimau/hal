package store

import (
	"math"
	"slices"
)

// HistogramGamma is the ratio between consecutive histogram bucket boundaries.
// Bucket i covers the range (gamma^(i-1), gamma^i], so the representative value
// of any bucket is within ~2% of every sample it contains. This is the DDSketch
// bucketing scheme: because the boundaries are fixed for all time (independent of
// the data), per-minute histograms can be summed exactly, and a quantile computed
// over any set of merged histograms is a true quantile of the underlying samples,
// accurate to within that relative error.
//
// gamma = (1+a)/(1-a) with a = 0.02 gives a relative accuracy of ~2%.
const HistogramGamma = 1.0408163265306123

var histogramLogGamma = math.Log(HistogramGamma)

// HistogramBucket returns the histogram bucket index for a value. Values below 1
// are clamped to bucket 0 (they are not meaningful for the nanosecond timings we
// record).
func HistogramBucket(value int64) int32 {
	if value < 1 {
		return 0
	}

	return int32(math.Ceil(math.Log(float64(value)) / histogramLogGamma))
}

// HistogramValue returns a representative value for a bucket index: the midpoint
// of the bucket's range, which is within HistogramGamma-relative error of every
// value the bucket contains.
func HistogramValue(bucket int32) int64 {
	return int64(2 * math.Pow(HistogramGamma, float64(bucket)) / (HistogramGamma + 1))
}

// MergeHistograms sums a set of per-bucket histograms into a single histogram.
func MergeHistograms(histograms ...map[int32]int64) map[int32]int64 {
	merged := make(map[int32]int64)
	for _, h := range histograms {
		for bucket, count := range h {
			merged[bucket] += count
		}
	}

	return merged
}

// HistogramQuantile returns the q-th quantile (0 < q <= 1) of the samples
// summarised by a merged histogram, using the nearest-rank method. The result is
// accurate to within HistogramGamma relative error of the true quantile.
func HistogramQuantile(counts map[int32]int64, q float64) int64 {
	var total int64
	for _, c := range counts {
		total += c
	}

	if total == 0 {
		return 0
	}

	buckets := make([]int32, 0, len(counts))
	for b := range counts {
		buckets = append(buckets, b)
	}
	slices.Sort(buckets)

	rank := int64(math.Ceil(float64(total) * q))

	var cumulative int64
	for _, b := range buckets {
		cumulative += counts[b]
		if cumulative >= rank {
			return HistogramValue(b)
		}
	}

	return HistogramValue(buckets[len(buckets)-1])
}
