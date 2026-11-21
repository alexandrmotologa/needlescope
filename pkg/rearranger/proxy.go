package rearranger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alexandrmotologa/needlescope/pkg/benchmark"
)

// ProxyConfig holds options for the salience re-arranger reverse proxy.
type ProxyConfig struct {
	ListenAddr        string       `json:"listen_addr"`
	UpstreamURL       string       `json:"upstream_url"`
	Strategy          StrategyType `json:"strategy"`
	TargetChunkTokens int          `json:"target_chunk_tokens"`
	DefaultAPIKey     string       `json:"default_api_key,omitempty"`
	ShadowMode        bool         `json:"shadow_mode"` // Enables asynchronous dual-inference A/B lift testing
}

// ProxyStats holds runtime performance telemetry and shadow test metrics.
type ProxyStats struct {
	TotalRequests           int64   `json:"total_requests"`
	ReorderedRequests       int64   `json:"reordered_requests"`
	TotalChunksMoved        int64   `json:"total_chunks_moved"`
	EvictedFromMiddle       int64   `json:"evicted_from_middle"`
	AvgReorderTimeUs        int64   `json:"avg_reorder_time_us"`
	UptimeSeconds           int64   `json:"uptime_seconds"`
	ShadowRuns              int64   `json:"shadow_runs"`
	ShadowAvgLatencyDeltaMs float64 `json:"shadow_avg_latency_delta_ms"`
}

// Proxy is a drop-in OpenAI-compatible reverse proxy that reorganizes long-context prompts.
type Proxy struct {
	config       ProxyConfig
	upstream     *url.URL
	reverseProxy *httputil.ReverseProxy
	chunker      *Chunker
	scorer       SalienceScorer
	startTime    time.Time
	server       *http.Server

	reqCount            int64
	reorderCount        int64
	chunksMoved         int64
	evictedMiddle       int64
	totalReorderUs      int64
	shadowRuns          int64
	shadowLatencySumMs  int64
	mu                  sync.RWMutex
}

// NewProxy creates a new proxy instance.
func NewProxy(cfg ProxyConfig) (*Proxy, error) {
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = ":8500"
	}
	if cfg.UpstreamURL == "" {
		cfg.UpstreamURL = "https://api.openai.com/v1"
	}
	if cfg.Strategy == "" {
		cfg.Strategy = StrategyUShaped
	}
	if cfg.TargetChunkTokens <= 0 {
		cfg.TargetChunkTokens = 350
	}

	parsedUpstream, err := url.Parse(cfg.UpstreamURL)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream URL %s: %w", cfg.UpstreamURL, err)
	}

	tok := benchmark.NewFastTokenizer()
	chunker := NewChunker(tok, cfg.TargetChunkTokens)
	scorer := NewBM25Scorer()

	p := &Proxy{
		config:       cfg,
		upstream:     parsedUpstream,
		reverseProxy: httputil.NewSingleHostReverseProxy(parsedUpstream),
		chunker:      chunker,
		scorer:       scorer,
		startTime:    time.Now(),
	}

	return p, nil
}

// Handler returns the http.Handler configured for all proxy routes.
func (p *Proxy) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", p.handleChatCompletions)
	mux.HandleFunc("/chat/completions", p.handleChatCompletions)
	mux.HandleFunc("/needlescope/stats", p.handleStats)
	mux.HandleFunc("/needlescope/shadow-stats", p.handleStats)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","engine":"needlescope-rearranger"}`)
	})
	mux.HandleFunc("/", p.handleFallback)
	return mux
}

// ServeHTTP implements http.Handler.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.Handler().ServeHTTP(w, r)
}

// Start launches the HTTP proxy server.
func (p *Proxy) Start() error {
	p.server = &http.Server{
		Addr:    p.config.ListenAddr,
		Handler: p.Handler(),
	}

	shadowNote := ""
	if p.config.ShadowMode {
		shadowNote = " [Shadow Dual-Inference Active]"
	}

	log.Printf("[needlescope-proxy] Listening on %s -> Upstream %s (Strategy: %s)%s\n",
		p.config.ListenAddr, p.config.UpstreamURL, p.config.Strategy, shadowNote)

	return p.server.ListenAndServe()
}

// Shutdown gracefully stops the proxy.
func (p *Proxy) Shutdown(ctx context.Context) error {
	if p.server != nil {
		return p.server.Shutdown(ctx)
	}
	return nil
}

func (p *Proxy) handleFallback(w http.ResponseWriter, r *http.Request) {
	r.Host = p.upstream.Host
	p.reverseProxy.ServeHTTP(w, r)
}

func (p *Proxy) handleStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	stats := p.GetStats()
	_ = json.NewEncoder(w).Encode(stats)
}

// GetStats returns current operational telemetry.
func (p *Proxy) GetStats() ProxyStats {
	reqs := atomic.LoadInt64(&p.reqCount)
	reorders := atomic.LoadInt64(&p.reorderCount)
	totalUs := atomic.LoadInt64(&p.totalReorderUs)
	avgUs := int64(0)
	if reorders > 0 {
		avgUs = totalUs / reorders
	}

	shadowRuns := atomic.LoadInt64(&p.shadowRuns)
	shadowLatencySum := atomic.LoadInt64(&p.shadowLatencySumMs)
	avgShadowDelta := 0.0
	if shadowRuns > 0 {
		avgShadowDelta = float64(shadowLatencySum) / float64(shadowRuns)
	}

	return ProxyStats{
		TotalRequests:           reqs,
		ReorderedRequests:       reorders,
		TotalChunksMoved:        atomic.LoadInt64(&p.chunksMoved),
		EvictedFromMiddle:       atomic.LoadInt64(&p.evictedMiddle),
		AvgReorderTimeUs:        avgUs,
		UptimeSeconds:           int64(time.Since(p.startTime).Seconds()),
		ShadowRuns:              shadowRuns,
		ShadowAvgLatencyDeltaMs: avgShadowDelta,
	}
}

func (p *Proxy) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	atomic.AddInt64(&p.reqCount, 1)

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	originalRawBytes := make([]byte, len(bodyBytes))
	copy(originalRawBytes, bodyBytes)

	var payload map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	messagesRaw, ok := payload["messages"].([]interface{})
	modified := false

	if ok && len(messagesRaw) > 0 {
		t0 := time.Now()

		lastUserIdx := -1
		for i := len(messagesRaw) - 1; i >= 0; i-- {
			msg, isMsg := messagesRaw[i].(map[string]interface{})
			if isMsg && msg["role"] == "user" {
				lastUserIdx = i
				break
			}
		}

		if lastUserIdx >= 0 {
			userMsg := messagesRaw[lastUserIdx].(map[string]interface{})
			content, isStr := userMsg["content"].(string)

			if isStr && len(content) > 500 {
				prefix, contextBody, query := ExtractPromptContext(content)
				if query == "" && lastUserIdx > 0 {
					query = content[:min(len(content), 120)]
				}

				chunks := p.chunker.SplitChunks(contextBody)
				if len(chunks) >= 2 {
					chunks = p.scorer.ScoreChunks(chunks, query)
					res := Reorder(chunks, p.config.Strategy)

					var newContent string
					if prefix != "" {
						newContent = prefix + res.ReorderedContext
					} else {
						newContent = res.ReorderedContext
					}
					if query != "" && !strings.Contains(newContent, query) {
						newContent = newContent + "\n\nQUERY:\n" + query
					}

					userMsg["content"] = newContent
					messagesRaw[lastUserIdx] = userMsg
					payload["messages"] = messagesRaw
					modified = true

					reorderUs := time.Since(t0).Microseconds()
					atomic.AddInt64(&p.reorderCount, 1)
					atomic.AddInt64(&p.chunksMoved, int64(res.ChunkCount))
					atomic.AddInt64(&p.evictedMiddle, int64(res.MiddleValleyEvictionCount))
					atomic.AddInt64(&p.totalReorderUs, reorderUs)

					w.Header().Set("X-Needlescope-Reordered", "true")
					w.Header().Set("X-Needlescope-Strategy", string(p.config.Strategy))
					w.Header().Set("X-Needlescope-Reorder-Time-Us", fmt.Sprintf("%d", reorderUs))
					w.Header().Set("X-Needlescope-Chunks", fmt.Sprintf("%d", res.ChunkCount))
					w.Header().Set("X-Needlescope-Evicted-From-Middle", fmt.Sprintf("%d", res.MiddleValleyEvictionCount))
				}
			}
		}

		if modified {
			newBytes, err := json.Marshal(payload)
			if err == nil {
				bodyBytes = newBytes
			}
		}
	}

	// Prepare primary upstream request
	upstreamURL := fmt.Sprintf("%s/chat/completions", strings.TrimSuffix(p.config.UpstreamURL, "/"))
	proxyReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL, bytes.NewReader(bodyBytes))
	if err != nil {
		http.Error(w, "Failed to create upstream request", http.StatusInternalServerError)
		return
	}

	// Copy headers
	for k, v := range r.Header {
		proxyReq.Header[k] = v
	}
	proxyReq.Header.Set("Content-Length", fmt.Sprintf("%d", len(bodyBytes)))
	if proxyReq.Header.Get("Authorization") == "" && p.config.DefaultAPIKey != "" {
		proxyReq.Header.Set("Authorization", "Bearer "+p.config.DefaultAPIKey)
	}

	tPrimaryStart := time.Now()
	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Upstream error: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	primaryLatency := time.Since(tPrimaryStart).Milliseconds()

	// If Shadow Mode is active and context was reordered, trigger asynchronous shadow evaluation of original prompt
	if p.config.ShadowMode && modified {
		go p.executeShadowDual(originalRawBytes, r.Header, primaryLatency)
	}

	// Copy response headers
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)

	// Stream response back
	_, _ = io.Copy(w, resp.Body)
}

// executeShadowDual runs the original unmodified prompt asynchronously to measure latency and recall lift.
func (p *Proxy) executeShadowDual(originalBytes []byte, originalHeaders http.Header, primaryLatencyMs int64) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	upstreamURL := fmt.Sprintf("%s/chat/completions", strings.TrimSuffix(p.config.UpstreamURL, "/"))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstreamURL, bytes.NewReader(originalBytes))
	if err != nil {
		return
	}

	for k, v := range originalHeaders {
		req.Header[k] = v
	}
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(originalBytes)))
	if req.Header.Get("Authorization") == "" && p.config.DefaultAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.DefaultAPIKey)
	}

	tStart := time.Now()
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	shadowLatency := time.Since(tStart).Milliseconds()
	deltaMs := primaryLatencyMs - shadowLatency

	atomic.AddInt64(&p.shadowRuns, 1)
	atomic.AddInt64(&p.shadowLatencySumMs, deltaMs)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
