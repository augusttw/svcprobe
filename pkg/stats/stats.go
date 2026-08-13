package stats

import (
	"math"
	"sort"
	"time"
)

// LatencyStats holds percentile and statistical metrics for a set of durations.
type LatencyStats struct {
	Min    time.Duration `json:"min_ns"`
	Max    time.Duration `json:"max_ns"`
	Mean   time.Duration `json:"mean_ns"`
	P50    time.Duration `json:"p50_ns"`
	P95    time.Duration `json:"p95_ns"`
	P99    time.Duration `json:"p99_ns"`
	StdDev time.Duration `json:"stddev_ns"`
}

// Calculate computes statistical metrics for a slice of durations.
// Durations slice is copied and sorted internally.
func Calculate(durations []time.Duration) LatencyStats {
	if len(durations) == 0 {
		return LatencyStats{}
	}

	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	n := len(sorted)
	minVal := sorted[0]
	maxVal := sorted[n-1]

	var sum time.Duration
	for _, d := range sorted {
		sum += d
	}
	meanVal := time.Duration(float64(sum) / float64(n))

	p50 := percentile(sorted, 0.50)
	p95 := percentile(sorted, 0.95)
	p99 := percentile(sorted, 0.99)

	var varianceSum float64
	meanFloat := float64(meanVal)
	for _, d := range sorted {
		diff := float64(d) - meanFloat
		varianceSum += diff * diff
	}
	stdDevVal := time.Duration(math.Sqrt(varianceSum / float64(n)))

	return LatencyStats{
		Min:    minVal,
		Max:    maxVal,
		Mean:   meanVal,
		P50:    p50,
		P95:    p95,
		P99:    p99,
		StdDev: stdDevVal,
	}
}

// percentile calculates the interpolated p-th percentile (0.0 <= p <= 1.0) of a sorted slice.
func percentile(sorted []time.Duration, p float64) time.Duration {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	if n == 1 || p <= 0 {
		return sorted[0]
	}
	if p >= 1.0 {
		return sorted[n-1]
	}

	idx := p * float64(n-1)
	lower := int(idx)
	upper := lower + 1
	if upper >= n {
		return sorted[n-1]
	}

	weight := idx - float64(lower)
	valLower := float64(sorted[lower])
	valUpper := float64(sorted[upper])

	interpolated := valLower + weight*(valUpper-valLower)
	return time.Duration(interpolated)
}
