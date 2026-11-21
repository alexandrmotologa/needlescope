package profiler

import (
	"math"
	"sort"
)

// SummaryStats holds statistical aggregation figures.
type SummaryStats struct {
	Count  int     `json:"count"`
	Mean   float64 `json:"mean"`
	Median float64 `json:"median"`
	StdDev float64 `json:"std_dev"`
	StdErr float64 `json:"std_err"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	P95    float64 `json:"p95"`
	P99    float64 `json:"p99"`
}

// ComputeStats calculates descriptive statistics over a float64 slice.
func ComputeStats(values []float64) SummaryStats {
	if len(values) == 0 {
		return SummaryStats{}
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	sum := 0.0
	for _, v := range sorted {
		sum += v
	}
	mean := sum / float64(len(sorted))

	varianceSum := 0.0
	for _, v := range sorted {
		varianceSum += math.Pow(v-mean, 2)
	}

	stdDev := 0.0
	if len(sorted) > 1 {
		stdDev = math.Sqrt(varianceSum / float64(len(sorted)-1))
	}
	stdErr := stdDev / math.Sqrt(float64(len(sorted)))

	median := percentileSorted(sorted, 0.50)
	p95 := percentileSorted(sorted, 0.95)
	p99 := percentileSorted(sorted, 0.99)

	return SummaryStats{
		Count:  len(sorted),
		Mean:   mean,
		Median: median,
		StdDev: stdDev,
		StdErr: stdErr,
		Min:    sorted[0],
		Max:    sorted[len(sorted)-1],
		P95:    p95,
		P99:    p99,
	}
}

// percentileSorted extracts the p-th percentile from an already sorted slice.
func percentileSorted(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if p <= 0 {
		return sorted[0]
	}
	if p >= 1 {
		return sorted[len(sorted)-1]
	}

	idx := p * float64(len(sorted)-1)
	lower := int(math.Floor(idx))
	upper := int(math.Ceil(idx))

	if lower == upper {
		return sorted[lower]
	}

	weight := idx - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}
