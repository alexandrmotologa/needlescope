package benchmark

import (
	"fmt"
	"strings"
)

// ModelPreset defines standard evaluation bounds tailored for specific frontier models.
type ModelPreset struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	DefaultModel   string   `json:"default_model"`
	ContextLengths []int    `json:"context_lengths"`
	DepthSteps     int      `json:"depth_steps"`
	DefaultDomain  CorpusDomain `json:"default_domain"`
	TargetChunkTok int      `json:"target_chunk_tokens"`
}

var Presets = map[string]ModelPreset{
	"gpt-4o": {
		Name:           "gpt-4o",
		Description:    "OpenAI GPT-4o context sweep (8k to 128k tokens)",
		DefaultModel:   "gpt-4o",
		ContextLengths: []int{8192, 16384, 32768, 65536, 98304, 131072},
		DepthSteps:     10,
		DefaultDomain:  DomainFinance,
		TargetChunkTok: 350,
	},
	"claude-3-5-sonnet": {
		Name:           "claude-3-5-sonnet",
		Description:    "Anthropic Claude 3.5 Sonnet context sweep (16k to 200k tokens)",
		DefaultModel:   "claude-3-5-sonnet-20241022",
		ContextLengths: []int{16384, 32768, 65536, 100000, 150000, 200000},
		DepthSteps:     10,
		DefaultDomain:  DomainTechnical,
		TargetChunkTok: 400,
	},
	"llama-3.1-70b": {
		Name:           "llama-3.1-70b",
		Description:    "Meta LLaMA 3.1 70B Instruct sweep (8k to 128k tokens)",
		DefaultModel:   "meta-llama/Llama-3.1-70B-Instruct",
		ContextLengths: []int{8192, 16384, 32768, 65536, 98304, 131072},
		DepthSteps:     10,
		DefaultDomain:  DomainFinance,
		TargetChunkTok: 350,
	},
	"gemini-1-5-pro": {
		Name:           "gemini-1-5-pro",
		Description:    "Google Gemini 1.5 Pro massive-context sweep (64k to 1M tokens)",
		DefaultModel:   "gemini-1.5-pro",
		ContextLengths: []int{65536, 131072, 262144, 524288, 1000000},
		DepthSteps:     10,
		DefaultDomain:  DomainTechnical,
		TargetChunkTok: 500,
	},
	"qwen-2.5-72b": {
		Name:           "qwen-2.5-72b",
		Description:    "Alibaba Qwen 2.5 72B Instruct context sweep (8k to 128k tokens)",
		DefaultModel:   "Qwen/Qwen2.5-72B-Instruct",
		ContextLengths: []int{8192, 16384, 32768, 65536, 98304, 131072},
		DepthSteps:     10,
		DefaultDomain:  DomainLegal,
		TargetChunkTok: 350,
	},
}

// GetPreset resolves a preset name (case-insensitive) or returns error.
func GetPreset(name string) (ModelPreset, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	preset, exists := Presets[key]
	if !exists {
		var available []string
		for k := range Presets {
			available = append(available, k)
		}
		return ModelPreset{}, fmt.Errorf("unknown preset %q (available: %s)", name, strings.Join(available, ", "))
	}
	return preset, nil
}
