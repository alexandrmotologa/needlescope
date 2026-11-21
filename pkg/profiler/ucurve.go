package profiler

import (
	"math"
)

// UCurveFitResult contains regression parameters for the attention degradation parabola.
type UCurveFitResult struct {
	Alpha       float64 `json:"alpha"`        // Curvature coefficient: alpha * (d - 0.5)^2 + gamma
	Gamma       float64 `json:"gamma"`        // Vertex recall at middle depth d = 0.5
	RSquared    float64 `json:"r_squared"`    // Goodness of fit (0.0 to 1.0)
	Beta        float64 `json:"beta"`         // Peak attenuation factor (1.0 - gamma)
	IsAttenuated bool   `json:"is_attenuated"` // True if significant U-curve degradation detected
}

// BlindSpotWindow defines the contiguous depth interval exhibiting severe recall degradation.
type BlindSpotWindow struct {
	Detected   bool    `json:"detected"`
	MinDepth   float64 `json:"min_depth"`   // Lower bound (e.g. 0.25)
	MaxDepth   float64 `json:"max_depth"`   // Upper bound (e.g. 0.75)
	Width      float64 `json:"width"`       // MaxDepth - MinDepth
	MinRecall  float64 `json:"min_recall"`  // Lowest observed recall inside window
	Threshold  float64 `json:"threshold"`   // Evaluation threshold applied (e.g. 0.80)
}

// FitUCurve calculates parabolic regression parameters: R(d) = alpha * (d - 0.5)^2 + gamma.
func FitUCurve(depths []float64, recalls []float64) UCurveFitResult {
	n := len(depths)
	if n < 3 || n != len(recalls) {
		return UCurveFitResult{Alpha: 0, Gamma: 1.0, RSquared: 0, Beta: 0, IsAttenuated: false}
	}

	// Transform x_i = (d_i - 0.5)^2
	x := make([]float64, n)
	y := make([]float64, n)
	sumX := 0.0
	sumY := 0.0

	for i := 0; i < n; i++ {
		x[i] = math.Pow(depths[i]-0.5, 2)
		y[i] = recalls[i]
		sumX += x[i]
		sumY += y[i]
	}

	meanX := sumX / float64(n)
	meanY := sumY / float64(n)

	num := 0.0
	den := 0.0
	for i := 0; i < n; i++ {
		dx := x[i] - meanX
		dy := y[i] - meanY
		num += dx * dy
		den += dx * dx
	}

	alpha := 0.0
	if den > 1e-9 {
		alpha = num / den
	}
	gamma := meanY - alpha*meanX

	// Ensure boundary validity
	if gamma < 0 {
		gamma = 0
	}
	if gamma > 1.0 {
		gamma = 1.0
	}

	// Calculate R-squared
	ssTot := 0.0
	ssRes := 0.0
	for i := 0; i < n; i++ {
		pred := alpha*x[i] + gamma
		ssTot += math.Pow(y[i]-meanY, 2)
		ssRes += math.Pow(y[i]-pred, 2)
	}

	rSquared := 0.0
	if ssTot > 1e-9 {
		rSquared = math.Max(0.0, 1.0-(ssRes/ssTot))
	}

	beta := math.Max(0.0, 1.0-gamma)
	isAttenuated := alpha > 0.3 && beta > 0.15

	return UCurveFitResult{
		Alpha:        alpha,
		Gamma:        gamma,
		RSquared:     rSquared,
		Beta:         beta,
		IsAttenuated: isAttenuated,
	}
}

// DetectBlindSpot finds the depth interval around the center where recall dips below threshold.
func DetectBlindSpot(depths []float64, recalls []float64, threshold float64) BlindSpotWindow {
	if threshold <= 0 {
		threshold = 0.80
	}

	minDepth := -1.0
	maxDepth := -1.0
	lowestRecall := 1.0
	detected := false

	for i := 0; i < len(depths) && i < len(recalls); i++ {
		d := depths[i]
		r := recalls[i]

		if r < threshold {
			detected = true
			if minDepth < 0 || d < minDepth {
				minDepth = d
			}
			if d > maxDepth {
				maxDepth = d
			}
			if r < lowestRecall {
				lowestRecall = r
			}
		}
	}

	if !detected {
		return BlindSpotWindow{
			Detected:  false,
			Threshold: threshold,
		}
	}

	return BlindSpotWindow{
		Detected:  true,
		MinDepth:  minDepth,
		MaxDepth:  maxDepth,
		Width:     maxDepth - minDepth,
		MinRecall: lowestRecall,
		Threshold: threshold,
	}
}
