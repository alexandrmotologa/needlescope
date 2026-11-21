package benchmark

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

// CompletionRequest encapsulates prompt and generation parameters.
type CompletionRequest struct {
	Model          string     `json:"model"`
	Prompt         string     `json:"prompt"`
	SystemPrompt   string     `json:"system_prompt,omitempty"`
	MaxTokens      int        `json:"max_tokens"`
	Temperature    float64    `json:"temperature"`
	ExpectedAnswer string     `json:"expected_answer,omitempty"`
	NeedleDepth    float64    `json:"needle_depth,omitempty"`
	ContextTokens  int        `json:"context_tokens,omitempty"`
	NeedleType     NeedleType `json:"needle_type,omitempty"`
	HopDepths      []float64  `json:"hop_depths,omitempty"`
}

// CompletionResponse returns generated content and performance metrics.
type CompletionResponse struct {
	Content          string `json:"content"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	LatencyMs        int64  `json:"latency_ms"`
}

// LLMClient defines the interface for model inference dispatch.
type LLMClient interface {
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
	Name() string
}

// --- Mock Client ---

// MockClientConfig configures synthetic attention degradation parameters.
type MockClientConfig struct {
	ModelName        string
	SimulatedLatency time.Duration
	BaseErrorRate    float64 // error rate at primacy/recency (e.g. 0.02)
	MaxAttenuation   float64 // attenuation at depth 0.5 (e.g. 0.60 -> 40% recall)
	Sigma            float64 // width of bell curve dip (e.g. 0.18)
}

// MockClient simulates realistic long-context attention degradation and multi-hop failures.
type MockClient struct {
	config MockClientConfig
	rng    *rand.Rand
}

// NewMockClient creates a new mock inference client.
func NewMockClient(cfg MockClientConfig) *MockClient {
	if cfg.ModelName == "" {
		cfg.ModelName = "mock-evaluator-v1"
	}
	if cfg.SimulatedLatency == 0 {
		cfg.SimulatedLatency = 8 * time.Millisecond
	}
	if cfg.BaseErrorRate <= 0 {
		cfg.BaseErrorRate = 0.02
	}
	if cfg.MaxAttenuation <= 0 {
		cfg.MaxAttenuation = 0.58
	}
	if cfg.Sigma <= 0 {
		cfg.Sigma = 0.20
	}

	return &MockClient{
		config: cfg,
		rng:    rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (m *MockClient) Name() string {
	return m.config.ModelName
}

func (m *MockClient) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()

	select {
	case <-time.After(m.config.SimulatedLatency):
	case <-ctx.Done():
		return CompletionResponse{}, ctx.Err()
	}

	// Calculate empirical recall probability based on context length and depth
	depth := req.NeedleDepth
	if depth < 0 {
		depth = 0.5
	}

	// Longer contexts increase middle attenuation:
	// At 8k tokens: minimal attenuation. At 128k+: full attenuation.
	lengthFactor := 1.0
	if req.ContextTokens > 0 {
		lengthFactor = math.Min(1.0, float64(req.ContextTokens)/65536.0)
	}

	// Gaussian dip at depth 0.5: R(d) = 1.0 - beta * exp(- (d - 0.5)^2 / (2 * sigma^2))
	dip := m.config.MaxAttenuation * lengthFactor * math.Exp(-math.Pow(depth-0.5, 2)/(2*math.Pow(m.config.Sigma, 2)))
	recallProb := math.Max(0.05, 1.0-m.config.BaseErrorRate-dip)

	// Multi-hop penalty: if multiple hops are separated across the attention valley, penalize recall
	if len(req.HopDepths) >= 2 {
		span := math.Abs(req.HopDepths[len(req.HopDepths)-1] - req.HopDepths[0])
		hopPenalty := 1.0 - math.Exp(-1.5*span)
		recallProb *= math.Max(0.10, 1.0-hopPenalty*lengthFactor)
	}

	success := m.rng.Float64() < recallProb

	var content string
	if success && req.ExpectedAnswer != "" {
		content = fmt.Sprintf("Based on the provided documents, the verified answer is: %s.", req.ExpectedAnswer)
	} else {
		failureResponses := []string{
			"The requested key or cipher is not explicitly detailed in the provided documentation.",
			"According to the records, the parameter remains unspecified or redacted.",
			"I could not locate the exact figure in the provided context.",
		}
		content = failureResponses[m.rng.Intn(len(failureResponses))]
	}

	elapsed := time.Since(start).Milliseconds()
	return CompletionResponse{
		Content:          content,
		PromptTokens:     req.ContextTokens + 32,
		CompletionTokens: 24,
		LatencyMs:        elapsed,
	}, nil
}

// --- OpenAI Client ---

// OpenAIClient connects to OpenAI-compatible REST endpoints.
type OpenAIClient struct {
	endpoint string
	apiKey   string
	model    string
	httpCli  *http.Client
}

// NewOpenAIClient initializes a client for OpenAI, vLLM, Ollama, or LM Studio.
func NewOpenAIClient(endpoint, apiKey, model string) *OpenAIClient {
	if endpoint == "" {
		endpoint = "https://api.openai.com/v1"
	}
	endpoint = strings.TrimSuffix(endpoint, "/")

	return &OpenAIClient{
		endpoint: endpoint,
		apiKey:   apiKey,
		model:    model,
		httpCli: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (c *OpenAIClient) Name() string {
	return fmt.Sprintf("openai:%s", c.model)
}

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatPayload struct {
	Model       string              `json:"model"`
	Messages    []openAIChatMessage `json:"messages"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Temperature float64             `json:"temperature"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func (c *OpenAIClient) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()

	var messages []openAIChatMessage
	if req.SystemPrompt != "" {
		messages = append(messages, openAIChatMessage{Role: "system", Content: req.SystemPrompt})
	}
	messages = append(messages, openAIChatMessage{Role: "user", Content: req.Prompt})

	model := c.model
	if req.Model != "" {
		model = req.Model
	}

	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 64
	}

	bodyData, err := json.Marshal(openAIChatPayload{
		Model:       model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
	})
	if err != nil {
		return CompletionResponse{}, err
	}

	url := fmt.Sprintf("%s/chat/completions", c.endpoint)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyData))
	if err != nil {
		return CompletionResponse{}, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpCli.Do(httpReq)
	if err != nil {
		return CompletionResponse{}, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return CompletionResponse{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return CompletionResponse{}, fmt.Errorf("upstream API returned %d: %s", resp.StatusCode, string(respBytes))
	}

	var parsed openAIChatResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return CompletionResponse{}, fmt.Errorf("failed to decode response: %w", err)
	}

	content := ""
	if len(parsed.Choices) > 0 {
		content = parsed.Choices[0].Message.Content
	}

	return CompletionResponse{
		Content:          content,
		PromptTokens:     parsed.Usage.PromptTokens,
		CompletionTokens: parsed.Usage.CompletionTokens,
		LatencyMs:        time.Since(start).Milliseconds(),
	}, nil
}

// --- Anthropic Client ---

// AnthropicClient connects to Anthropic /v1/messages endpoint.
type AnthropicClient struct {
	endpoint string
	apiKey   string
	model    string
	httpCli  *http.Client
}

// NewAnthropicClient initializes an Anthropic client.
func NewAnthropicClient(apiKey, model string) *AnthropicClient {
	return &AnthropicClient{
		endpoint: "https://api.anthropic.com/v1",
		apiKey:   apiKey,
		model:    model,
		httpCli: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (c *AnthropicClient) Name() string {
	return fmt.Sprintf("anthropic:%s", c.model)
}

func (c *AnthropicClient) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()

	payload := map[string]interface{}{
		"model":      c.model,
		"max_tokens": 128,
		"messages": []map[string]string{
			{"role": "user", "content": req.Prompt},
		},
	}
	if req.SystemPrompt != "" {
		payload["system"] = req.SystemPrompt
	}

	bodyData, err := json.Marshal(payload)
	if err != nil {
		return CompletionResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/messages", bytes.NewReader(bodyData))
	if err != nil {
		return CompletionResponse{}, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpCli.Do(httpReq)
	if err != nil {
		return CompletionResponse{}, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return CompletionResponse{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return CompletionResponse{}, fmt.Errorf("anthropic returned %d: %s", resp.StatusCode, string(respBytes))
	}

	var parsed struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return CompletionResponse{}, err
	}

	content := ""
	if len(parsed.Content) > 0 {
		content = parsed.Content[0].Text
	}

	return CompletionResponse{
		Content:          content,
		PromptTokens:     parsed.Usage.InputTokens,
		CompletionTokens: parsed.Usage.OutputTokens,
		LatencyMs:        time.Since(start).Milliseconds(),
	}, nil
}
