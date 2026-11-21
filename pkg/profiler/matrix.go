package profiler

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/alexandrmotologa/needlescope/pkg/benchmark"
)

// TrialSample contains compact outcome data for UI inspector cards.
type TrialSample struct {
	Expected  string `json:"expected"`
	Actual    string `json:"actual"`
	Success   bool   `json:"success"`
	LatencyMs int64  `json:"latency_ms"`
}

// MatrixCell represents one coordinate in the 2D (Length x Depth) matrix.
type MatrixCell struct {
	ContextLength    int           `json:"context_length"`
	Depth            float64       `json:"depth"`
	TotalTrials      int           `json:"total_trials"`
	SuccessfulTrials int           `json:"successful_trials"`
	RecallRate       float64       `json:"recall_rate"`
	AverageScore     float64       `json:"average_score"`
	MeanLatencyMs    float64       `json:"mean_latency_ms"`
	P95LatencyMs     float64       `json:"p95_latency_ms"`
	Samples          []TrialSample `json:"samples,omitempty"`
}

// MatrixReport represents the complete aggregated 2D attention degradation profile.
type MatrixReport struct {
	ID                string                  `json:"id"`
	Model             string                  `json:"model"`
	Provider          string                  `json:"provider"`
	Timestamp         time.Time               `json:"timestamp"`
	TotalTrials       int                     `json:"total_trials"`
	OverallRecall     float64                 `json:"overall_recall"`
	PrimacyRecall     float64                 `json:"primacy_recall"`    // depth <= 0.15
	RecencyRecall     float64                 `json:"recency_recall"`    // depth >= 0.85
	MiddleRecall      float64                 `json:"middle_recall"`     // 0.35 <= depth <= 0.65
	DegradationDelta  float64                 `json:"degradation_delta"` // PrimacyRecencyAvg - MiddleRecall
	ContextLengths    []int                   `json:"context_lengths"`
	Depths            []float64               `json:"depths"`
	Cells             map[string]MatrixCell   `json:"cells"` // key: "len_depth"
	LengthUCurves     map[int]UCurveFitResult `json:"length_u_curves"`
	GlobalUCurve      UCurveFitResult         `json:"global_u_curve"`
	CriticalBlindSpot BlindSpotWindow         `json:"critical_blind_spot"`
}

// CellKey creates the standard lookup key for coordinate (length, depth).
func CellKey(length int, depth float64) string {
	return fmt.Sprintf("%d_%.2f", length, depth)
}

// Aggregate compiles raw trial results into a structured 2D MatrixReport.
func Aggregate(results []benchmark.TrialResult, model string, provider string) *MatrixReport {
	if len(results) == 0 {
		return &MatrixReport{
			ID:            fmt.Sprintf("rep_%d", time.Now().Unix()),
			Model:         model,
			Provider:      provider,
			Timestamp:     time.Now(),
			Cells:         make(map[string]MatrixCell),
			LengthUCurves: make(map[int]UCurveFitResult),
		}
	}

	lenSet := make(map[int]bool)
	depthSet := make(map[float64]bool)

	type cellAccumulator struct {
		length    int
		depth     float64
		total     int
		successes int
		scores    []float64
		latencies []float64
		samples   []TrialSample
	}

	accumulators := make(map[string]*cellAccumulator)

	for _, r := range results {
		lenSet[r.ContextLength] = true
		depthSet[r.Depth] = true
		key := CellKey(r.ContextLength, r.Depth)

		acc, exists := accumulators[key]
		if !exists {
			acc = &cellAccumulator{
				length: r.ContextLength,
				depth:  r.Depth,
			}
			accumulators[key] = acc
		}

		acc.total++
		if r.Success {
			acc.successes++
		}
		acc.scores = append(acc.scores, r.Score)
		acc.latencies = append(acc.latencies, float64(r.LatencyMs))

		if len(acc.samples) < 3 {
			acc.samples = append(acc.samples, TrialSample{
				Expected:  r.Expected,
				Actual:    r.Actual,
				Success:   r.Success,
				LatencyMs: r.LatencyMs,
			})
		}
	}

	// Sort lengths and depths
	var sortedLengths []int
	for l := range lenSet {
		sortedLengths = append(sortedLengths, l)
	}
	sort.Ints(sortedLengths)

	var sortedDepths []float64
	for d := range depthSet {
		sortedDepths = append(sortedDepths, d)
	}
	sort.Float64s(sortedDepths)

	cells := make(map[string]MatrixCell)
	totalSuccess := 0
	totalTrials := 0

	primacySuccess := 0
	primacyTotal := 0
	recencySuccess := 0
	recencyTotal := 0
	middleSuccess := 0
	middleTotal := 0

	for k, acc := range accumulators {
		recall := 0.0
		if acc.total > 0 {
			recall = float64(acc.successes) / float64(acc.total)
		}

		avgScore := 0.0
		for _, s := range acc.scores {
			avgScore += s
		}
		if len(acc.scores) > 0 {
			avgScore /= float64(len(acc.scores))
		}

		latStats := ComputeStats(acc.latencies)

		cells[k] = MatrixCell{
			ContextLength:    acc.length,
			Depth:            acc.depth,
			TotalTrials:      acc.total,
			SuccessfulTrials: acc.successes,
			RecallRate:       recall,
			AverageScore:     avgScore,
			MeanLatencyMs:    latStats.Mean,
			P95LatencyMs:     latStats.P95,
			Samples:          acc.samples,
		}

		totalSuccess += acc.successes
		totalTrials += acc.total

		if acc.depth <= 0.15 {
			primacySuccess += acc.successes
			primacyTotal += acc.total
		} else if acc.depth >= 0.85 {
			recencySuccess += acc.successes
			recencyTotal += acc.total
		} else if acc.depth >= 0.35 && acc.depth <= 0.65 {
			middleSuccess += acc.successes
			middleTotal += acc.total
		}
	}

	overallRecall := 0.0
	if totalTrials > 0 {
		overallRecall = float64(totalSuccess) / float64(totalTrials)
	}

	primacyRecall := 0.0
	if primacyTotal > 0 {
		primacyRecall = float64(primacySuccess) / float64(primacyTotal)
	}

	recencyRecall := 0.0
	if recencyTotal > 0 {
		recencyRecall = float64(recencySuccess) / float64(recencyTotal)
	}

	middleRecall := 0.0
	if middleTotal > 0 {
		middleRecall = float64(middleSuccess) / float64(middleTotal)
	}

	edgeAvg := (primacyRecall + recencyRecall) / 2.0
	degradationDelta := math.Max(0.0, edgeAvg-middleRecall)

	// Calculate per-length U-Curves
	lengthUCurves := make(map[int]UCurveFitResult)
	for _, l := range sortedLengths {
		var dVals []float64
		var rVals []float64
		for _, d := range sortedDepths {
			if c, ok := cells[CellKey(l, d)]; ok {
				dVals = append(dVals, d)
				rVals = append(rVals, c.RecallRate)
			}
		}
		lengthUCurves[l] = FitUCurve(dVals, rVals)
	}

	// Calculate global depth recall curve
	var globalDepths []float64
	var globalRecalls []float64
	for _, d := range sortedDepths {
		dSuccess := 0
		dTotal := 0
		for _, l := range sortedLengths {
			if c, ok := cells[CellKey(l, d)]; ok {
				dSuccess += c.SuccessfulTrials
				dTotal += c.TotalTrials
			}
		}
		if dTotal > 0 {
			globalDepths = append(globalDepths, d)
			globalRecalls = append(globalRecalls, float64(dSuccess)/float64(dTotal))
		}
	}

	globalUCurve := FitUCurve(globalDepths, globalRecalls)
	blindSpot := DetectBlindSpot(globalDepths, globalRecalls, 0.80)

	return &MatrixReport{
		ID:                fmt.Sprintf("rep_%d", time.Now().Unix()),
		Model:             model,
		Provider:          provider,
		Timestamp:         time.Now(),
		TotalTrials:       totalTrials,
		OverallRecall:     overallRecall,
		PrimacyRecall:     primacyRecall,
		RecencyRecall:     recencyRecall,
		MiddleRecall:      middleRecall,
		DegradationDelta:  degradationDelta,
		ContextLengths:    sortedLengths,
		Depths:            sortedDepths,
		Cells:             cells,
		LengthUCurves:     lengthUCurves,
		GlobalUCurve:      globalUCurve,
		CriticalBlindSpot: blindSpot,
	}
}
