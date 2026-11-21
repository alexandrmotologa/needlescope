package test

import (
	"bytes"
	"testing"
	"time"

	"github.com/alexandrmotologa/needlescope/pkg/benchmark"
	"github.com/alexandrmotologa/needlescope/pkg/profiler"
	"github.com/alexandrmotologa/needlescope/pkg/report"
)

func TestFitUCurve(t *testing.T) {
	// Synthetic U-curve: high at edges (1.0), low at middle (0.4)
	depths := []float64{0.0, 0.2, 0.4, 0.5, 0.6, 0.8, 1.0}
	recalls := []float64{1.0, 0.85, 0.50, 0.40, 0.50, 0.85, 1.0}

	fit := profiler.FitUCurve(depths, recalls)

	if fit.Alpha <= 0 {
		t.Fatalf("expected positive alpha for U-curve, got %f", fit.Alpha)
	}
	if fit.Gamma > 0.60 {
		t.Fatalf("expected middle vertex gamma <= 0.60, got %f", fit.Gamma)
	}
	if fit.RSquared < 0.80 {
		t.Fatalf("expected high R^2 (>0.80) for symmetric U-curve, got %f", fit.RSquared)
	}

	blindSpot := profiler.DetectBlindSpot(depths, recalls, 0.70)
	if !blindSpot.Detected {
		t.Fatalf("expected blind spot detection for recall < 0.70")
	}
	if blindSpot.MinDepth > 0.4 || blindSpot.MaxDepth < 0.6 {
		t.Fatalf("expected blind spot around center [0.4, 0.6], got [%f, %f]", blindSpot.MinDepth, blindSpot.MaxDepth)
	}
}

func TestAggregate_And_Reports(t *testing.T) {
	results := []benchmark.TrialResult{
		{ContextLength: 8192, Depth: 0.0, Success: true, Score: 1.0, LatencyMs: 12, Timestamp: time.Now()},
		{ContextLength: 8192, Depth: 0.5, Success: false, Score: 0.0, LatencyMs: 15, Timestamp: time.Now()},
		{ContextLength: 8192, Depth: 1.0, Success: true, Score: 1.0, LatencyMs: 11, Timestamp: time.Now()},
		{ContextLength: 16384, Depth: 0.0, Success: true, Score: 1.0, LatencyMs: 20, Timestamp: time.Now()},
		{ContextLength: 16384, Depth: 0.5, Success: false, Score: 0.0, LatencyMs: 25, Timestamp: time.Now()},
		{ContextLength: 16384, Depth: 1.0, Success: true, Score: 1.0, LatencyMs: 18, Timestamp: time.Now()},
	}

	rep := profiler.Aggregate(results, "test-model", "mock")

	if rep.OverallRecall != 4.0/6.0 {
		t.Fatalf("expected overall recall 0.666, got %f", rep.OverallRecall)
	}
	if rep.PrimacyRecall != 1.0 {
		t.Fatalf("expected primacy recall 1.0, got %f", rep.PrimacyRecall)
	}
	if rep.MiddleRecall != 0.0 {
		t.Fatalf("expected middle recall 0.0, got %f", rep.MiddleRecall)
	}
	if rep.DegradationDelta != 1.0 {
		t.Fatalf("expected degradation delta 1.0, got %f", rep.DegradationDelta)
	}

	// Verify terminal output formatting
	var buf bytes.Buffer
	report.PrintTerminal(rep, &buf)
	if buf.Len() == 0 {
		t.Fatalf("expected non-empty terminal report output")
	}

	// Verify CSV export
	var csvBuf bytes.Buffer
	if err := report.ExportCSV(rep, &csvBuf); err != nil {
		t.Fatalf("csv export error: %v", err)
	}
	if !bytes.Contains(csvBuf.Bytes(), []byte("context_length,depth")) {
		t.Fatalf("csv header missing")
	}

	// Verify JSON export
	var jsonBuf bytes.Buffer
	if err := report.ExportJSON(rep, &jsonBuf); err != nil {
		t.Fatalf("json export error: %v", err)
	}
	if !bytes.Contains(jsonBuf.Bytes(), []byte("test-model")) {
		t.Fatalf("json body missing model name")
	}
}
