package rearranger

import (
	"regexp"
	"strings"

	"github.com/alexandrmotologa/needlescope/pkg/benchmark"
)

// Chunk represents an atomic semantic document block.
type Chunk struct {
	ID            int     `json:"id"`
	OriginalIndex int     `json:"original_index"`
	NewIndex      int     `json:"new_index"`
	Content       string  `json:"content"`
	TokenCount    int     `json:"token_count"`
	SalienceScore float64 `json:"salience_score"`
	OriginalDepth float64 `json:"original_depth"`
	NewDepth      float64 `json:"new_depth"`
}

// Chunker segments raw context documents into coherent semantic chunks.
type Chunker struct {
	tokenizer      benchmark.Tokenizer
	targetChunkTok int
}

// NewChunker creates a new chunker instance.
func NewChunker(tok benchmark.Tokenizer, targetChunkTokens int) *Chunker {
	if tok == nil {
		tok = benchmark.NewFastTokenizer()
	}
	if targetChunkTokens <= 0 {
		targetChunkTokens = 350
	}
	return &Chunker{
		tokenizer:      tok,
		targetChunkTok: targetChunkTokens,
	}
}

var xmlDocRegex = regexp.MustCompile(`(?s)<(?:doc|document|context)[^>]*>(.*?)</(?:doc|document|context)>`)
var mdHeaderRegex = regexp.MustCompile(`(?m)^(#{1,4}\s+.*)`)

// SplitChunks divides a context body into structured chunks preserving paragraph and section boundaries.
func (c *Chunker) SplitChunks(text string) []Chunk {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	// 1. Try XML/HTML document tags if present
	xmlMatches := xmlDocRegex.FindAllStringIndex(text, -1)
	if len(xmlMatches) >= 2 {
		var chunks []Chunk
		for i, match := range xmlMatches {
			part := strings.TrimSpace(text[match[0]:match[1]])
			if part != "" {
				chunks = append(chunks, Chunk{
					ID:            i,
					OriginalIndex: i,
					Content:       part,
					TokenCount:    c.tokenizer.CountTokens(part),
				})
			}
		}
		c.computeOriginalDepths(chunks)
		return chunks
	}

	// 2. Try splitting by double newline paragraphs
	rawParas := strings.Split(text, "\n\n")
	var paras []string
	for _, p := range rawParas {
		p = strings.TrimSpace(p)
		if p != "" {
			paras = append(paras, p)
		}
	}

	if len(paras) <= 1 {
		// Single block; split by sentences if long
		return []Chunk{
			{
				ID:            0,
				OriginalIndex: 0,
				Content:       text,
				TokenCount:    c.tokenizer.CountTokens(text),
				OriginalDepth: 0.5,
			},
		}
	}

	// Group paragraphs to match target chunk token count without splitting mid-paragraph
	var chunks []Chunk
	var curBuf strings.Builder
	curTokens := 0
	chunkIdx := 0

	for _, p := range paras {
		pTokens := c.tokenizer.CountTokens(p)

		if curTokens > 0 && curTokens+pTokens > c.targetChunkTok {
			// Flush current chunk
			chunks = append(chunks, Chunk{
				ID:            chunkIdx,
				OriginalIndex: chunkIdx,
				Content:       strings.TrimSpace(curBuf.String()),
				TokenCount:    curTokens,
			})
			chunkIdx++
			curBuf.Reset()
			curTokens = 0
		}

		if curBuf.Len() > 0 {
			curBuf.WriteString("\n\n")
		}
		curBuf.WriteString(p)
		curTokens += pTokens
	}

	if curBuf.Len() > 0 {
		chunks = append(chunks, Chunk{
			ID:            chunkIdx,
			OriginalIndex: chunkIdx,
			Content:       strings.TrimSpace(curBuf.String()),
			TokenCount:    curTokens,
		})
	}

	c.computeOriginalDepths(chunks)
	return chunks
}

func (c *Chunker) computeOriginalDepths(chunks []Chunk) {
	if len(chunks) == 0 {
		return
	}
	totalTokens := 0
	for _, ch := range chunks {
		totalTokens += ch.TokenCount
	}

	running := 0
	for i := range chunks {
		mid := float64(running) + float64(chunks[i].TokenCount)/2.0
		if totalTokens > 0 {
			chunks[i].OriginalDepth = mid / float64(totalTokens)
		} else {
			chunks[i].OriginalDepth = float64(i) / float64(len(chunks))
		}
		running += chunks[i].TokenCount
	}
}

// ExtractPromptContext separates prompt instructions, context body, and the terminal user query.
func ExtractPromptContext(prompt string) (prefix string, contextBody string, query string) {
	// Look for standard context tags
	lower := strings.ToLower(prompt)

	// Check if prompt has "QUERY:" or "QUESTION:" delimiter
	queryDelims := []string{"\nquery:\n", "\nquery:", "\nquestion:\n", "\nquestion:", "\ninstruction:\n"}
	queryIdx := -1
	delimLen := 0

	for _, d := range queryDelims {
		idx := strings.Index(lower, d)
		if idx != -1 {
			queryIdx = idx
			delimLen = len(d)
			break
		}
	}

	if queryIdx != -1 {
		contextBody = strings.TrimSpace(prompt[:queryIdx])
		query = strings.TrimSpace(prompt[queryIdx+delimLen:])
		return "", contextBody, query
	}

	// Fallback: Check for "CONTEXT DOCUMENTS:" prefix
	contextPrefixes := []string{"context documents:\n\n", "context:\n\n", "documents:\n\n"}
	for _, cp := range contextPrefixes {
		if strings.HasPrefix(lower, cp) {
			prefix = prompt[:len(cp)]
			contextBody = prompt[len(cp):]
			return prefix, contextBody, ""
		}
	}

	return "", prompt, ""
}
