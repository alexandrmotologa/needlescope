package profiler

import (
	"fmt"
)

// GateConfig holds SLA thresholds for CI/CD automated attention assertions.
type GateConfig struct {
	MinMiddleRecall      float64 `json:"min_middle_recall"`       // e.g. 0.80 (80%)
	MaxDegradationDelta  float64 `json:"max_degradation_delta"`   // e.g. 0.20 (20%)
	MinOverallRecall     float64 `json:"min_overall_recall"`      // e.g. 0.85 (85%)
	MaxBlindSpotWidth    float64 `json:"max_blind_spot_width"`    // e.g. 0.15 (15% depth width)
}

// GateAssertion reports the evaluation of a single quality rule.
type GateAssertion struct {
	Rule        string  `json:"rule"`
	TargetValue float64 `json:"target_value"`
	ActualValue float64 `json:"actual_value"`
	Passed      bool    `json:"passed"`
	Message     string  `json:"message"`
}

// GateResult encapsulates the pass/fail determination of an attention gate check.
type GateResult struct {
	Passed     bool            `json:"passed"`
	Model      string          `json:"model"`
	Assertions []GateAssertion `json:"assertions"`
}

// EvaluateGate tests a MatrixReport against SLA thresholds.
func EvaluateGate(report *MatrixReport, cfg GateConfig) GateResult {
	if report == nil {
		return GateResult{
			Passed: false,
			Assertions: []GateAssertion{
				{Rule: "Report Validity", Passed: false, Message: "Report is nil or invalid"},
			},
		}
	}

	var assertions []GateAssertion
	overallPass := true

	// Rule 1: Min Middle Recall
	if cfg.MinMiddleRecall > 0 {
		passed := report.MiddleRecall >= cfg.MinMiddleRecall
		if !passed {
			overallPass = false
		}
		assertions = append(assertions, GateAssertion{
			Rule:        "Minimum Middle Valley Recall",
			TargetValue: cfg.MinMiddleRecall,
			ActualValue: report.MiddleRecall,
			Passed:      passed,
			Message: fmt.Sprintf("Middle recall %.1f%% %s required threshold %.1f%%",
				report.MiddleRecall*100, passFailWord(passed, ">=", "<"), cfg.MinMiddleRecall*100),
		})
	}

	// Rule 2: Max Degradation Delta
	if cfg.MaxDegradationDelta > 0 {
		passed := report.DegradationDelta <= cfg.MaxDegradationDelta
		if !passed {
			overallPass = false
		}
		assertions = append(assertions, GateAssertion{
			Rule:        "Maximum Degradation Delta",
			TargetValue: cfg.MaxDegradationDelta,
			ActualValue: report.DegradationDelta,
			Passed:      passed,
			Message: fmt.Sprintf("Degradation delta %.1f%% %s threshold limit %.1f%%",
				report.DegradationDelta*100, passFailWord(passed, "<=", ">"), cfg.MaxDegradationDelta*100),
		})
	}

	// Rule 3: Min Overall Recall
	if cfg.MinOverallRecall > 0 {
		passed := report.OverallRecall >= cfg.MinOverallRecall
		if !passed {
			overallPass = false
		}
		assertions = append(assertions, GateAssertion{
			Rule:        "Minimum Overall Recall",
			TargetValue: cfg.MinOverallRecall,
			ActualValue: report.OverallRecall,
			Passed:      passed,
			Message: fmt.Sprintf("Overall recall %.1f%% %s required threshold %.1f%%",
				report.OverallRecall*100, passFailWord(passed, ">=", "<"), cfg.MinOverallRecall*100),
		})
	}

	// Rule 4: Max Blind Spot Width
	if cfg.MaxBlindSpotWidth > 0 {
		actualWidth := 0.0
		if report.CriticalBlindSpot.Detected {
			actualWidth = report.CriticalBlindSpot.Width
		}
		passed := actualWidth <= cfg.MaxBlindSpotWidth
		if !passed {
			overallPass = false
		}
		assertions = append(assertions, GateAssertion{
			Rule:        "Maximum Blind Spot Width",
			TargetValue: cfg.MaxBlindSpotWidth,
			ActualValue: actualWidth,
			Passed:      passed,
			Message: fmt.Sprintf("Blind spot width %.1f%% %s threshold limit %.1f%%",
				actualWidth*100, passFailWord(passed, "<=", ">"), cfg.MaxBlindSpotWidth*100),
		})
	}

	return GateResult{
		Passed:     overallPass,
		Model:      report.Model,
		Assertions: assertions,
	}
}

func passFailWord(passed bool, passSym, failSym string) string {
	if passed {
		return passSym
	}
	return failSym
}
