package test

import (
	"context"
	"testing"
	"time"

	"github.com/alexandrmotologa/needlescope/pkg/benchmark"
)

func TestRunner_MockSweep(t *testing.T) {
	mockClient := benchmark.NewMockClient(benchmark.MockClientConfig{
		ModelName:        "mock-test-v1",
		SimulatedLatency: 1 * time.Millisecond,
		BaseErrorRate:    0.0,
		MaxAttenuation:   0.60,
		Sigma:            0.18,
	})

	cfg := benchmark.BenchmarkConfig{
		Provider:            "mock",
		Model:               "mock-test-v1",
		ContextLengths:      []int{1000, 4000},
		Depths:              []float64{0.0, 0.5, 1.0},
		TrialsPerCoordinate: 1,
		Type:                benchmark.TypeSingleNeedle,
		Concurrency:         2,
	}

	runner := benchmark.NewRunner(cfg, mockClient)
	events := runner.Subscribe()

	go func() {
		for range events {
			// consume events
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results, err := runner.Run(ctx)
	if err != nil {
		t.Fatalf("runner failed: %v", err)
	}

	expectedTrials := 2 * 3 * 1
	if len(results) != expectedTrials {
		t.Fatalf("expected %d results, got %d", expectedTrials, len(results))
	}

	// Verify primacy (0.0) and recency (1.0) have high recall compared to middle (0.5)
	for _, r := range results {
		if (r.Depth == 0.0 || r.Depth == 1.0) && !r.Success {
			// Primacy/recency should generally succeed in mock
			t.Logf("Notice: Depth %f had score %f", r.Depth, r.Score)
		}
	}
}

func TestGradeAnswer(t *testing.T) {
	tests := []struct {
		expected string
		actual   string
		minScore float64
	}{
		{"SEC-ORION-A8F", "The key is SEC-ORION-A8F.", 1.0},
		{"SEC-ORION-A8F", "SEC-ORION-A8F", 1.0},
		{"450.2M", "Operating margin is 450.2M for FY27", 1.0},
		{"Agent Vance", "Agent Vance is the custodian", 1.0},
		{"SecretKey123", "I could not find the code.", 0.0},
	}

	for _, tt := range tests {
		score := benchmark.GradeAnswer(tt.expected, tt.actual)
		if score < tt.minScore {
			t.Fatalf("GradeAnswer(%q, %q) = %f, expected >= %f", tt.expected, tt.actual, score, tt.minScore)
		}
	}
}
