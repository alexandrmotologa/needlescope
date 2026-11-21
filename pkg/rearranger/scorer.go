package rearranger

import (
	"math"
	"strings"
	"unicode"
)

// SalienceScorer computes relevance scores for document chunks relative to a query.
type SalienceScorer interface {
	ScoreChunks(chunks []Chunk, query string) []Chunk
}

// BM25Scorer implements an ultra-fast in-memory BM25 ranker with phrase boosting.
type BM25Scorer struct {
	k1 float64
	b  float64
}

// NewBM25Scorer creates a new BM25 scorer with standard hyper-parameters (k1=1.5, b=0.75).
func NewBM25Scorer() *BM25Scorer {
	return &BM25Scorer{
		k1: 1.5,
		b:  0.75,
	}
}

// ScoreChunks calculates BM25 salience scores for each chunk against the query.
func (s *BM25Scorer) ScoreChunks(chunks []Chunk, query string) []Chunk {
	if len(chunks) == 0 {
		return chunks
	}

	queryTerms := tokenizeTerms(query)
	if len(queryTerms) == 0 {
		// No query terms; assign neutral uniform scores
		for i := range chunks {
			chunks[i].SalienceScore = 0.5
		}
		return chunks
	}

	n := float64(len(chunks))
	docTermFreqs := make([]map[string]int, len(chunks))
	docLengths := make([]int, len(chunks))
	dfMap := make(map[string]int)
	totalLen := 0

	for i, ch := range chunks {
		terms := tokenizeTerms(ch.Content)
		docLengths[i] = len(terms)
		totalLen += len(terms)

		tf := make(map[string]int)
		for _, t := range terms {
			tf[t]++
		}
		docTermFreqs[i] = tf

		for t := range tf {
			dfMap[t]++
		}
	}

	avgdl := float64(totalLen) / n
	if avgdl == 0 {
		avgdl = 1.0
	}

	// Compute IDF for query terms
	idfMap := make(map[string]float64)
	for _, qt := range queryTerms {
		df := float64(dfMap[qt])
		// Okapi BM25 standard smoothed IDF
		idf := math.Log(1.0 + (n-df+0.5)/(df+0.5))
		if idf < 0 {
			idf = 0.05
		}
		idfMap[qt] = idf
	}

	cleanQuery := strings.ToLower(strings.TrimSpace(query))

	for i := range chunks {
		score := 0.0
		tfMap := docTermFreqs[i]
		docLen := float64(docLengths[i])

		for _, qt := range queryTerms {
			tf := float64(tfMap[qt])
			if tf == 0 {
				continue
			}

			idf := idfMap[qt]
			num := tf * (s.k1 + 1.0)
			den := tf + s.k1*(1.0-s.b+s.b*(docLen/avgdl))
			score += idf * (num / den)
		}

		// Exact query containment boost
		lowerContent := strings.ToLower(chunks[i].Content)
		if cleanQuery != "" && strings.Contains(lowerContent, cleanQuery) {
			score += 10.0
		}

		chunks[i].SalienceScore = score
	}

	return chunks
}

// tokenizeTerms splits text into lowercase alphanumeric tokens.
func tokenizeTerms(text string) []string {
	var terms []string
	var cur strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			cur.WriteRune(unicode.ToLower(r))
		} else {
			if cur.Len() > 0 {
				word := cur.String()
				if !isCommonStopword(word) {
					terms = append(terms, word)
				}
				cur.Reset()
			}
		}
	}

	if cur.Len() > 0 {
		word := cur.String()
		if !isCommonStopword(word) {
			terms = append(terms, word)
		}
	}

	return terms
}

var stopwords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "in": true,
	"on": true, "at": true, "to": true, "of": true, "for": true, "with": true,
	"is": true, "are": true, "was": true, "were": true, "it": true, "that": true,
	"this": true, "from": true, "by": true, "as": true, "be": true, "have": true,
}

func isCommonStopword(w string) bool {
	return stopwords[w] || len(w) < 2
}
