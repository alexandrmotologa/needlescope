package test

import (
	"strings"
	"testing"

	"github.com/alexandrmotologa/needlescope/pkg/benchmark"
)

func TestFastTokenizer_CountTokens(t *testing.T) {
	tok := benchmark.NewFastTokenizer()

	if count := tok.CountTokens(""); count != 0 {
		t.Fatalf("expected 0 tokens for empty string, got %d", count)
	}

	text := "The quick brown fox jumps over the lazy dog."
	count := tok.CountTokens(text)
	if count < 8 || count > 14 {
		t.Fatalf("expected 8-14 tokens for pangram, got %d", count)
	}
}

func TestFastTokenizer_InsertAtDepth(t *testing.T) {
	tok := benchmark.NewFastTokenizer()
	context := strings.Repeat("Financial disclosures indicate sustained quarter over quarter operating margin expansion. ", 100)
	needle := "TARGET NEEDLE: The vault passcode is 98721."

	tests := []struct {
		depth float64
	}{
		{0.0},
		{0.25},
		{0.50},
		{0.75},
		{1.0},
	}

	for _, tt := range tests {
		res, pos := tok.InsertAtDepth(context, needle, tt.depth)
		if !strings.Contains(res, needle) {
			t.Fatalf("needle not found in context for depth %f", tt.depth)
		}
		totalTokens := tok.CountTokens(res)
		actualRatio := float64(pos) / float64(totalTokens)
		diff := actualRatio - tt.depth
		if diff < -0.15 || diff > 0.15 {
			t.Fatalf("depth %f resulted in actual ratio %f (diff too high: %f)", tt.depth, actualRatio, diff)
		}
	}
}

func TestFastTokenizer_TruncateToTokens(t *testing.T) {
	tok := benchmark.NewFastTokenizer()
	context := strings.Repeat("Enterprise telemetry indicates latency variance. ", 200)

	truncated := tok.TruncateToTokens(context, 50)
	count := tok.CountTokens(truncated)
	if count > 55 {
		t.Fatalf("truncated text token count %d exceeds target limit 50 significantly", count)
	}
}
