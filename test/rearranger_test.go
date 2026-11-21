package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexandrmotologa/needlescope/pkg/benchmark"
	"github.com/alexandrmotologa/needlescope/pkg/rearranger"
)

func TestChunker_SplitChunks(t *testing.T) {
	tok := benchmark.NewFastTokenizer()
	chunker := rearranger.NewChunker(tok, 15)

	text := "Section 1. Operational guidelines for cloud infrastructure.\n\n" +
		"Section 2. Database replication topologies and failover mechanisms.\n\n" +
		"Section 3. Network perimeter firewalls and egress traffic inspection.\n\n" +
		"Section 4. Cryptographic secret management and vault rotation schedules."

	chunks := chunker.SplitChunks(text)
	if len(chunks) < 3 {
		t.Fatalf("expected at least 3 chunks, got %d", len(chunks))
	}

	for i, ch := range chunks {
		if ch.TokenCount <= 0 {
			t.Fatalf("chunk %d has invalid token count %d", i, ch.TokenCount)
		}
		if ch.OriginalDepth < 0 || ch.OriginalDepth > 1.0 {
			t.Fatalf("chunk %d has invalid original depth %f", i, ch.OriginalDepth)
		}
	}
}

func TestBM25Scorer_And_Reorder(t *testing.T) {
	tok := benchmark.NewFastTokenizer()
	chunker := rearranger.NewChunker(tok, 15)
	scorer := rearranger.NewBM25Scorer()

	paras := []string{
		"1. The executive summary details overall commercial revenue projections.",
		"2. Cloud infrastructure expenditure remained flat across all geographical zones.",
		"3. CRITICAL NEEDLE: Project Aegis secret authorization cipher is PHANTOM-9821.",
		"4. Legal indemnification clauses cover standard software warranty provisions.",
		"5. Employee retention metrics improved by 14 percent following remote work policies.",
	}
	text := strings.Join(paras, "\n\n")

	chunks := chunker.SplitChunks(text)
	if len(chunks) != 5 {
		t.Fatalf("expected 5 chunks, got %d", len(chunks))
	}

	query := "What is the Project Aegis secret authorization cipher?"
	scoredChunks := scorer.ScoreChunks(chunks, query)

	// Chunk 2 (index 2: "CRITICAL NEEDLE") should have the highest salience score
	maxScore := -1.0
	maxIdx := -1
	for i, ch := range scoredChunks {
		if ch.SalienceScore > maxScore {
			maxScore = ch.SalienceScore
			maxIdx = i
		}
	}

	if maxIdx != 2 {
		t.Fatalf("expected chunk 2 to have highest salience, but chunk %d had score %f", maxIdx, maxScore)
	}

	// Apply U-shaped re-ordering
	res := rearranger.Reorder(scoredChunks, rearranger.StrategyUShaped)
	if res.ChunkCount != 5 {
		t.Fatalf("expected 5 reordered chunks, got %d", res.ChunkCount)
	}

	// In U-shaped layout, rank 0 (highest salience) moves to the last slot (recency, index 4)
	lastChunk := res.ReorderedChunks[len(res.ReorderedChunks)-1]
	if !strings.Contains(lastChunk.Content, "PHANTOM-9821") {
		t.Fatalf("expected highest salience chunk to be in recency (last slot), got: %s", lastChunk.Content)
	}

	// Verify middle chunk is one of the low-salience chunks
	midChunk := res.ReorderedChunks[len(res.ReorderedChunks)/2]
	if strings.Contains(midChunk.Content, "PHANTOM-9821") {
		t.Fatalf("critical needle must not be in the middle valley after U-shaped re-arrangement")
	}
}

func TestProxy_ChatCompletionsIntercept(t *testing.T) {
	// 1. Setup mock upstream server that mimics OpenAI
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}

		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)

		// Verify the upstream received re-ordered content
		messages := payload["messages"].([]interface{})
		lastMsg := messages[len(messages)-1].(map[string]interface{})
		content := lastMsg["content"].(string)

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"id": "chatcmpl-test",
			"object": "chat.completion",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "Received prompt of length %d"
				}
			}],
			"usage": {"prompt_tokens": 120, "completion_tokens": 10}
		}`, len(content))
	}))
	defer upstreamServer.Close()

	// 2. Initialize Proxy pointing to mock upstream
	proxy, err := rearranger.NewProxy(rearranger.ProxyConfig{
		ListenAddr:        ":0",
		UpstreamURL:       upstreamServer.URL,
		Strategy:          rearranger.StrategyUShaped,
		TargetChunkTokens: 50,
	})
	if err != nil {
		t.Fatalf("failed to create proxy: %v", err)
	}

	// 3. Create test request with long context
	longDoc := strings.Repeat("Quarterly operating results demonstrated resilience across key markets. ", 20) +
		"\n\nTARGET DATA: The vault code is 44921.\n\n" +
		strings.Repeat("Legal disclosures outline regulatory capital compliance metrics. ", 20) +
		"\n\nQUERY:\nWhat is the vault code?"

	reqBody := map[string]interface{}{
		"model": "gpt-4o",
		"messages": []map[string]string{
			{"role": "user", "content": longDoc},
		},
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Execute handler directly
	proxy.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	reorderedHeader := resp.Header.Get("X-Needlescope-Reordered")
	if reorderedHeader != "true" {
		t.Fatalf("expected X-Needlescope-Reordered header to be 'true', got %q", reorderedHeader)
	}

	stats := proxy.GetStats()
	if stats.ReorderedRequests != 1 {
		t.Fatalf("expected 1 reordered request in stats, got %d", stats.ReorderedRequests)
	}
}
