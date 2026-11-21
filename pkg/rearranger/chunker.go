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
	IsProtected   bool    `json:"is_protected,omitempty"` // True if protected by Boundary Shield (table, code block)
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
var fencedCodeRegex = regexp.MustCompile("(?s)```[\\w]*\\n.*?```")

// SplitChunks divides a context body into structured chunks preserving paragraph, table, and code boundaries.
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

	// 2. Extract atomic semantic units using Boundary Shield (tables, code blocks, paragraphs)
	units := c.extractAtomicUnits(text)

	if len(units) <= 1 {
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

	// Group atomic units to match target chunk token count without splitting mid-unit
	var chunks []Chunk
	var curBuf strings.Builder
	curTokens := 0
	chunkIdx := 0
	chunkHasProtected := false

	for _, unit := range units {
		unitTokens := c.tokenizer.CountTokens(unit.content)

		// If current buffer already has content and adding this unit exceeds target tokens, flush
		if curTokens > 0 && curTokens+unitTokens > c.targetChunkTok {
			chunks = append(chunks, Chunk{
				ID:            chunkIdx,
				OriginalIndex: chunkIdx,
				Content:       strings.TrimSpace(curBuf.String()),
				TokenCount:    curTokens,
				IsProtected:   chunkHasProtected,
			})
			chunkIdx++
			curBuf.Reset()
			curTokens = 0
			chunkHasProtected = false
		}

		if curBuf.Len() > 0 {
			curBuf.WriteString("\n\n")
		}
		curBuf.WriteString(unit.content)
		curTokens += unitTokens
		if unit.isProtected {
			chunkHasProtected = true
		}
	}

	if curBuf.Len() > 0 {
		chunks = append(chunks, Chunk{
			ID:            chunkIdx,
			OriginalIndex: chunkIdx,
			Content:       strings.TrimSpace(curBuf.String()),
			TokenCount:    curTokens,
			IsProtected:   chunkHasProtected,
		})
	}

	c.computeOriginalDepths(chunks)
	return chunks
}

type atomicUnit struct {
	content     string
	isProtected bool // table or code block
}

// extractAtomicUnits parses text into structured blocks, shielding tables and code blocks from being split.
func (c *Chunker) extractAtomicUnits(text string) []atomicUnit {
	lines := strings.Split(text, "\n")
	var units []atomicUnit

	var curBlock strings.Builder
	inCodeBlock := false
	inTable := false

	flushBlock := func(isProtected bool) {
		str := strings.TrimSpace(curBlock.String())
		if str != "" {
			units = append(units, atomicUnit{content: str, isProtected: isProtected})
		}
		curBlock.Reset()
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check fenced code block boundary
		if strings.HasPrefix(trimmed, "```") {
			if !inCodeBlock {
				// Flush any preceding standard text
				if inTable {
					flushBlock(true)
					inTable = false
				} else {
					flushBlock(false)
				}
				inCodeBlock = true
				curBlock.WriteString(line)
				curBlock.WriteString("\n")
				continue
			} else {
				// Close code block
				curBlock.WriteString(line)
				flushBlock(true)
				inCodeBlock = false
				continue
			}
		}

		if inCodeBlock {
			curBlock.WriteString(line)
			curBlock.WriteString("\n")
			continue
		}

		// Check Markdown table line (| ... |)
		isTableLine := strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|")
		if isTableLine {
			if !inTable {
				flushBlock(false)
				inTable = true
			}
			curBlock.WriteString(line)
			curBlock.WriteString("\n")
			continue
		} else if inTable {
			// End of table
			flushBlock(true)
			inTable = false
		}

		// Check empty line paragraph separation
		if trimmed == "" {
			flushBlock(false)
			continue
		}

		if curBlock.Len() > 0 {
			curBlock.WriteString("\n")
		}
		curBlock.WriteString(line)
	}

	if inCodeBlock || inTable {
		flushBlock(true)
	} else {
		flushBlock(false)
	}

	return units
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
	lower := strings.ToLower(prompt)

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
