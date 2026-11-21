package cockpit

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alexandrmotologa/needlescope/pkg/benchmark"
	"github.com/alexandrmotologa/needlescope/pkg/profiler"
	"github.com/alexandrmotologa/needlescope/pkg/rearranger"
	"github.com/alexandrmotologa/needlescope/pkg/report"
)

//go:embed web/*
var webFS embed.FS

// ServerConfig configures the cockpit HTTP server.
type ServerConfig struct {
	Port          int    `json:"port"`
	Host          string `json:"host"`
	ReportPath    string `json:"report_path,omitempty"`
	ProxyUpstream string `json:"proxy_upstream,omitempty"`
}

// Server serves the interactive 2D Heatmap Cockpit and live benchmark controls.
type Server struct {
	config     ServerConfig
	httpServer *http.Server
	report     *profiler.MatrixReport
	runner     *benchmark.Runner
	chunker    *rearranger.Chunker
	scorer     rearranger.SalienceScorer
	mu         sync.RWMutex

	// SSE subscriber registry
	clients   map[chan benchmark.BenchmarkProgressEvent]bool
	clientsMu sync.Mutex
}

// NewServer initializes the cockpit server with default or loaded report.
func NewServer(cfg ServerConfig, rep *profiler.MatrixReport) *Server {
	if cfg.Port <= 0 {
		cfg.Port = 8585
	}
	if cfg.Host == "" {
		cfg.Host = "0.0.0.0"
	}

	tok := benchmark.NewFastTokenizer()
	chunker := rearranger.NewChunker(tok, 350)
	scorer := rearranger.NewBM25Scorer()

	// If no report provided, generate a baseline demo profile
	if rep == nil {
		rep = generateDefaultReport()
	}

	return &Server{
		config:  cfg,
		report:  rep,
		chunker: chunker,
		scorer:  scorer,
		clients: make(map[chan benchmark.BenchmarkProgressEvent]bool),
	}
}

// Start runs the HTTP cockpit server.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	mux := http.NewServeMux()

	// Static web assets
	subFS, err := fs.Sub(webFS, "web")
	if err != nil {
		return fmt.Errorf("failed to load embedded web assets: %w", err)
	}
	fileServer := http.FileServer(http.FS(subFS))
	mux.Handle("/", fileServer)

	// API routes
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/matrix", s.handleMatrix)
	mux.HandleFunc("/api/benchmark/start", s.handleBenchmarkStart)
	mux.HandleFunc("/api/benchmark/stop", s.handleBenchmarkStop)
	mux.HandleFunc("/api/benchmark/stream", s.handleBenchmarkStream)
	mux.HandleFunc("/api/rearrange", s.handleRearrange)
	mux.HandleFunc("/api/export/json", s.handleExportJSON)
	mux.HandleFunc("/api/export/csv", s.handleExportCSV)

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	log.Printf("[needlescope-cockpit] Cockpit UI available at http://localhost:%d\n", s.config.Port)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the cockpit server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	running := false
	if s.runner != nil {
		running = s.runner.IsRunning()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "active",
		"port":       s.config.Port,
		"model":      s.report.Model,
		"provider":   s.report.Provider,
		"running":    running,
		"total_runs": s.report.TotalTrials,
	})
}

func (s *Server) handleMatrix(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.report)
}

func (s *Server) handleBenchmarkStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req benchmark.BenchmarkConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid benchmark config payload", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	if s.runner != nil && s.runner.IsRunning() {
		s.mu.Unlock()
		http.Error(w, "A benchmark run is already in progress", http.StatusConflict)
		return
	}

	var client benchmark.LLMClient
	if req.Provider == "openai" {
		client = benchmark.NewOpenAIClient(req.Endpoint, req.APIKey, req.Model)
	} else if req.Provider == "anthropic" {
		client = benchmark.NewAnthropicClient(req.APIKey, req.Model)
	} else {
		client = benchmark.NewMockClient(benchmark.MockClientConfig{
			ModelName:        req.Model,
			SimulatedLatency: 15 * time.Millisecond,
		})
	}

	s.runner = benchmark.NewRunner(req, client)
	eventsCh := s.runner.Subscribe()
	s.mu.Unlock()

	// Forward runner events to all active SSE subscribers
	go func() {
		for ev := range eventsCh {
			s.broadcastEvent(ev)
		}
	}()

	// Execute in background
	go func() {
		results, err := s.runner.Run(context.Background())
		if err == nil && len(results) > 0 {
			newReport := profiler.Aggregate(results, req.Model, req.Provider)
			s.mu.Lock()
			s.report = newReport
			s.mu.Unlock()
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func (s *Server) handleBenchmarkStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.Lock()
	if s.runner != nil {
		s.runner.Stop()
	}
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

func (s *Server) handleBenchmarkStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	eventCh := make(chan benchmark.BenchmarkProgressEvent, 50)
	s.clientsMu.Lock()
	s.clients[eventCh] = true
	s.clientsMu.Unlock()

	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, eventCh)
		close(eventCh)
		s.clientsMu.Unlock()
	}()

	notify := r.Context().Done()

	for {
		select {
		case <-notify:
			return
		case ev, open := <-eventCh:
			if !open {
				return
			}
			data, _ := json.Marshal(ev)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (s *Server) broadcastEvent(ev benchmark.BenchmarkProgressEvent) {
	s.clientsMu.Lock()
	defer s.clientsMu.Unlock()

	for ch := range s.clients {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (s *Server) handleRearrange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Text              string                  `json:"text"`
		Query             string                  `json:"query"`
		Strategy          rearranger.StrategyType `json:"strategy"`
		TargetChunkTokens int                     `json:"target_chunk_tokens"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid rearrange request payload", http.StatusBadRequest)
		return
	}

	chunker := s.chunker
	if req.TargetChunkTokens > 0 {
		chunker = rearranger.NewChunker(benchmark.NewFastTokenizer(), req.TargetChunkTokens)
	}

	chunks := chunker.SplitChunks(req.Text)
	scoredChunks := s.scorer.ScoreChunks(chunks, req.Query)

	strat := req.Strategy
	if strat == "" {
		strat = rearranger.StrategyUShaped
	}

	res := rearranger.Reorder(scoredChunks, strat)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) handleExportJSON(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=needlescope_report_%s.json", s.report.ID))
	_ = report.ExportJSON(s.report, w)
}

func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=needlescope_matrix_%s.csv", s.report.ID))
	_ = report.ExportCSV(s.report, w)
}

// generateDefaultReport creates an initial demonstration profile for immediate visualization.
func generateDefaultReport() *profiler.MatrixReport {
	mockClient := benchmark.NewMockClient(benchmark.MockClientConfig{
		ModelName:      "meta-llama/Llama-3.1-70B-Instruct (Baseline Sweep)",
		MaxAttenuation: 0.55,
		Sigma:          0.20,
	})

	lengths := []int{8192, 16384, 32768, 65536, 131072, 262144}
	depths := []float64{0.0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0}

	cfg := benchmark.BenchmarkConfig{
		Provider:            "mock",
		Model:               "meta-llama/Llama-3.1-70B-Instruct (Baseline Sweep)",
		ContextLengths:      lengths,
		Depths:              depths,
		TrialsPerCoordinate: 2,
		Type:                benchmark.TypeSingleNeedle,
		Concurrency:         4,
	}

	runner := benchmark.NewRunner(cfg, mockClient)
	results, _ := runner.Run(context.Background())
	return profiler.Aggregate(results, cfg.Model, "mock")
}

// ParsePort helper for CLI flag parsing.
func ParsePort(s string) int {
	p, err := strconv.Atoi(strings.TrimPrefix(s, ":"))
	if err != nil || p <= 0 {
		return 8585
	}
	return p
}
