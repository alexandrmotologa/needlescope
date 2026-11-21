package benchmark

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// TrialResult records the outcome of a single needle evaluation at (length, depth).
type TrialResult struct {
	CoordinateID  string     `json:"coordinate_id"`
	ContextLength int        `json:"context_length"`
	Depth         float64    `json:"depth"`
	TrialIndex    int        `json:"trial_index"`
	NeedleType    NeedleType `json:"needle_type"`
	Success       bool       `json:"success"`
	Score         float64    `json:"score"` // 0.0 - 1.0 (F1 or exact match)
	Expected      string     `json:"expected"`
	Actual        string     `json:"actual"`
	LatencyMs     int64      `json:"latency_ms"`
	PromptTokens  int        `json:"prompt_tokens"`
	Timestamp     time.Time  `json:"timestamp"`
}

// BenchmarkConfig holds the configuration for a 2D grid evaluation sweep.
type BenchmarkConfig struct {
	Provider            string        `json:"provider"`
	Model               string        `json:"model"`
	Endpoint            string        `json:"endpoint"`
	APIKey              string        `json:"api_key"`
	ContextLengths      []int         `json:"context_lengths"`
	Depths              []float64     `json:"depths"`
	TrialsPerCoordinate int           `json:"trials_per_coordinate"`
	Type                NeedleType    `json:"type"`
	Domain              CorpusDomain  `json:"domain"`
	Concurrency         int           `json:"concurrency"`
	RateLimitDelay      time.Duration `json:"rate_limit_delay"`
}

// BenchmarkProgressEvent reports incremental run state to CLI and web cockpit SSE.
type BenchmarkProgressEvent struct {
	Type          string       `json:"type"` // "start", "progress", "trial_complete", "finished", "error"
	TotalTrials   int          `json:"total_trials"`
	Completed     int          `json:"completed"`
	CurrentLength int          `json:"current_length,omitempty"`
	CurrentDepth  float64      `json:"current_depth,omitempty"`
	LatestTrial   *TrialResult `json:"latest_trial,omitempty"`
	Message       string       `json:"message,omitempty"`
}

// Runner orchestrates the execution of 2D grid needle retrieval stress tests.
type Runner struct {
	config     BenchmarkConfig
	client     LLMClient
	tokenizer  Tokenizer
	needleGen  *NeedleGenerator
	corpusGen  *CorpusGenerator
	listeners  []chan BenchmarkProgressEvent
	mu         sync.RWMutex
	cancelFunc context.CancelFunc
	isRunning  bool
}

// NewRunner creates a new benchmark runner instance.
func NewRunner(cfg BenchmarkConfig, client LLMClient) *Runner {
	tok := NewFastTokenizer()
	if cfg.TrialsPerCoordinate <= 0 {
		cfg.TrialsPerCoordinate = 1
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 2
	}
	if len(cfg.ContextLengths) == 0 {
		cfg.ContextLengths = []int{8192, 16384, 32768, 65536, 131072}
	}
	if len(cfg.Depths) == 0 {
		cfg.Depths = []float64{0.0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0}
	}
	if cfg.Type == "" {
		cfg.Type = TypeSingleNeedle
	}

	return &Runner{
		config:    cfg,
		client:    client,
		tokenizer: tok,
		needleGen: NewNeedleGenerator(),
		corpusGen: NewCorpusGenerator(tok),
	}
}

// Subscribe returns a channel that receives benchmark progress events.
func (r *Runner) Subscribe() chan BenchmarkProgressEvent {
	r.mu.Lock()
	defer r.mu.Unlock()

	ch := make(chan BenchmarkProgressEvent, 100)
	r.listeners = append(r.listeners, ch)
	return ch
}

// broadcast dispatches an event to all active subscribers.
func (r *Runner) broadcast(ev BenchmarkProgressEvent) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, ch := range r.listeners {
		select {
		case ch <- ev:
		default:
			// avoid blocking on slow consumers
		}
	}
}

// IsRunning returns whether an evaluation sweep is currently active.
func (r *Runner) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.isRunning
}

// Stop cancels any currently executing benchmark run.
func (r *Runner) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cancelFunc != nil {
		r.cancelFunc()
		r.cancelFunc = nil
	}
	r.isRunning = false
}

// Run executes the 2D benchmark grid sweep and returns all trial results.
func (r *Runner) Run(parentCtx context.Context) ([]TrialResult, error) {
	r.mu.Lock()
	if r.isRunning {
		r.mu.Unlock()
		return nil, fmt.Errorf("a benchmark is already running")
	}
	ctx, cancel := context.WithCancel(parentCtx)
	r.cancelFunc = cancel
	r.isRunning = true
	r.mu.Unlock()

	defer func() {
		r.mu.Lock()
		r.isRunning = false
		r.cancelFunc = nil
		r.mu.Unlock()
	}()

	totalTrials := len(r.config.ContextLengths) * len(r.config.Depths) * r.config.TrialsPerCoordinate
	r.broadcast(BenchmarkProgressEvent{
		Type:        "start",
		TotalTrials: totalTrials,
		Completed:   0,
		Message:     fmt.Sprintf("Starting evaluation sweep: %d total trials", totalTrials),
	})

	type workItem struct {
		length     int
		depth      float64
		trialIndex int
	}

	var workItems []workItem
	for _, length := range r.config.ContextLengths {
		for _, depth := range r.config.Depths {
			for t := 0; t < r.config.TrialsPerCoordinate; t++ {
				workItems = append(workItems, workItem{
					length:     length,
					depth:      depth,
					trialIndex: t,
				})
			}
		}
	}

	results := make([]TrialResult, len(workItems))
	var completedCount int64
	var countMu sync.Mutex

	// Worker pool pattern with concurrency control
	jobs := make(chan int, len(workItems))
	for i := range workItems {
		jobs <- i
	}
	close(jobs)

	var wg sync.WaitGroup
	workerCount := r.config.Concurrency
	if workerCount > len(workItems) {
		workerCount = len(workItems)
	}

	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				item := workItems[idx]
				res := r.executeTrial(ctx, item.length, item.depth, item.trialIndex)
				results[idx] = res

				countMu.Lock()
				completedCount++
				currentDone := int(completedCount)
				countMu.Unlock()

				r.broadcast(BenchmarkProgressEvent{
					Type:          "trial_complete",
					TotalTrials:   totalTrials,
					Completed:     currentDone,
					CurrentLength: item.length,
					CurrentDepth:  item.depth,
					LatestTrial:   &res,
				})

				if r.config.RateLimitDelay > 0 {
					time.Sleep(r.config.RateLimitDelay)
				}
			}
		}()
	}

	wg.Wait()

	if ctx.Err() != nil {
		r.broadcast(BenchmarkProgressEvent{
			Type:    "error",
			Message: "Benchmark cancelled by user or context timeout",
		})
		return nil, ctx.Err()
	}

	r.broadcast(BenchmarkProgressEvent{
		Type:        "finished",
		TotalTrials: totalTrials,
		Completed:   totalTrials,
		Message:     "Benchmark sweep completed successfully",
	})

	return results, nil
}

// executeTrial runs an isolated trial at (length, depth).
func (r *Runner) executeTrial(ctx context.Context, length int, depth float64, trialIdx int) TrialResult {
	coordID := fmt.Sprintf("%d_%d_%d", length, int(depth*100), trialIdx)

	// 1. Generate base background context
	baseContext := r.corpusGen.Generate(length, r.config.Domain)

	var prompt string
	var expectedAnswer string
	var hopDepths []float64

	switch r.config.Type {
	case TypeMultiHop:
		hops := []float64{math.Max(0.05, depth-0.3), depth, math.Min(0.95, depth+0.3)}
		spec := r.needleGen.GenerateMultiHop(hops)
		expectedAnswer = spec.FinalAnswer
		hopDepths = hops

		// Inject all hops into context
		curContext := baseContext
		for _, h := range spec.Hops {
			curContext, _ = r.tokenizer.InsertAtDepth(curContext, h.Fact, h.Depth)
		}
		prompt = fmt.Sprintf("CONTEXT DOCUMENTS:\n\n%s\n\nQUERY:\n%s\n\nAnswer concisely with the exact value.", curContext, spec.FinalQuery)

	case TypeMultiNeedle:
		needles, query, ans := r.needleGen.GenerateMulti([]float64{depth})
		expectedAnswer = ans
		curContext := baseContext
		for _, n := range needles {
			curContext, _ = r.tokenizer.InsertAtDepth(curContext, n.Fact, n.Depth)
		}
		prompt = fmt.Sprintf("CONTEXT DOCUMENTS:\n\n%s\n\nQUERY:\n%s\n\nAnswer concisely with the exact values.", curContext, query)

	case TypeSingleNeedle:
		fallthrough
	default:
		needle := r.needleGen.GenerateSingle(depth)
		expectedAnswer = needle.Answer
		curContext, _ := r.tokenizer.InsertAtDepth(baseContext, needle.Fact, depth)

		// Optionally inject 1 distractor at opposite depth
		if len(needle.Distractors) > 0 {
			oppositeDepth := 1.0 - depth
			curContext, _ = r.tokenizer.InsertAtDepth(curContext, needle.Distractors[0], oppositeDepth)
		}

		prompt = fmt.Sprintf("CONTEXT DOCUMENTS:\n\n%s\n\nQUERY:\n%s\n\nAnswer concisely with the exact code or passkey.", curContext, needle.Query)
	}

	req := CompletionRequest{
		Model:          r.config.Model,
		Prompt:         prompt,
		SystemPrompt:   "You are an accurate corporate intelligence retrieval assistant. Answer factually based only on the provided context.",
		ExpectedAnswer: expectedAnswer,
		NeedleDepth:    depth,
		ContextTokens:  length,
		NeedleType:     r.config.Type,
		HopDepths:      hopDepths,
	}

	resp, err := r.client.Complete(ctx, req)
	if err != nil {
		return TrialResult{
			CoordinateID:  coordID,
			ContextLength: length,
			Depth:         depth,
			TrialIndex:    trialIdx,
			NeedleType:    r.config.Type,
			Success:       false,
			Score:         0.0,
			Expected:      expectedAnswer,
			Actual:        fmt.Sprintf("ERROR: %v", err),
			LatencyMs:     0,
			PromptTokens:  length,
			Timestamp:     time.Now(),
		}
	}

	// Grade response using normalized token matching
	score := GradeAnswer(expectedAnswer, resp.Content)
	success := score >= 0.70

	return TrialResult{
		CoordinateID:  coordID,
		ContextLength: length,
		Depth:         depth,
		TrialIndex:    trialIdx,
		NeedleType:    r.config.Type,
		Success:       success,
		Score:         score,
		Expected:      expectedAnswer,
		Actual:        resp.Content,
		LatencyMs:     resp.LatencyMs,
		PromptTokens:  resp.PromptTokens,
		Timestamp:     time.Now(),
	}
}

// GradeAnswer evaluates whether the expected answer is present in the actual response.
// Uses substring presence and token overlap F1 scoring.
func GradeAnswer(expected, actual string) float64 {
	expClean := strings.TrimSpace(strings.ToLower(expected))
	actClean := strings.TrimSpace(strings.ToLower(actual))

	if expClean == "" {
		return 1.0
	}

	// Exact substring containment guarantees 1.0
	if strings.Contains(actClean, expClean) {
		return 1.0
	}

	// Token overlap F1 score
	expTokens := strings.Fields(expClean)
	actTokens := strings.Fields(actClean)

	if len(expTokens) == 0 || len(actTokens) == 0 {
		return 0.0
	}

	actMap := make(map[string]int)
	for _, t := range actTokens {
		actMap[t]++
	}

	common := 0
	for _, t := range expTokens {
		if actMap[t] > 0 {
			common++
			actMap[t]--
		}
	}

	if common == 0 {
		return 0.0
	}

	precision := float64(common) / float64(len(actTokens))
	recall := float64(common) / float64(len(expTokens))

	// Return harmonic mean (F1) weighted toward recall
	return (2 * precision * recall) / (precision + recall)
}
