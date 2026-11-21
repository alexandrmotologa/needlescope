package test

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/alexandrmotologa/needlescope/pkg/benchmark"
	"github.com/alexandrmotologa/needlescope/pkg/rearranger"
)

func TestMemoryFootprintUnder20MB(t *testing.T) {
	// Force GC before measurement
	runtime.GC()
	var mStart runtime.MemStats
	runtime.ReadMemStats(&mStart)

	// Run heavy 128k context generator and chunker
	tok := benchmark.NewFastTokenizer()
	corpusGen := benchmark.NewCorpusGenerator(tok)
	largeContext := corpusGen.Generate(128000, benchmark.DomainFinance)

	chunker := rearranger.NewChunker(tok, 400)
	chunks := chunker.SplitChunks(largeContext)

	scorer := rearranger.NewBM25Scorer()
	scored := scorer.ScoreChunks(chunks, "What is the capital expenditure on server topologies?")

	res := rearranger.Reorder(scored, rearranger.StrategyUShaped)

	if res.ChunkCount < 50 {
		t.Fatalf("expected > 50 chunks for 128k context, got %d", res.ChunkCount)
	}

	runtime.GC()
	var mEnd runtime.MemStats
	runtime.ReadMemStats(&mEnd)

	// Calculate heap allocated memory in MB
	heapAllocMB := float64(mEnd.Alloc) / (1024 * 1024)
	t.Logf("Heap Allocated Memory during 128k processing: %.2f MB", heapAllocMB)

	if heapAllocMB > 20.0 {
		t.Fatalf("Memory footprint %.2f MB exceeds 20 MB limit", heapAllocMB)
	}
}

func TestBenchmarkRunnerMemory_128k(t *testing.T) {
	runtime.GC()

	mockClient := benchmark.NewMockClient(benchmark.MockClientConfig{
		ModelName:        "mock-mem-test",
		SimulatedLatency: 100 * time.Microsecond,
	})

	cfg := benchmark.BenchmarkConfig{
		Provider:            "mock",
		Model:               "mock-mem-test",
		ContextLengths:      []int{32768, 65536, 131072},
		Depths:              []float64{0.0, 0.5, 1.0},
		TrialsPerCoordinate: 1,
		Type:                benchmark.TypeSingleNeedle,
		Concurrency:         2,
	}

	runner := benchmark.NewRunner(cfg, mockClient)
	_, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	heapAllocMB := float64(m.Alloc) / (1024 * 1024)
	t.Logf("Runner heap allocated memory after 131k sweep: %.2f MB", heapAllocMB)

	if heapAllocMB > 20.0 {
		t.Fatalf("Memory footprint %.2f MB exceeds 20 MB limit", heapAllocMB)
	}
}
