package analyzer

import (
	"hash/fnv"
	"math"

	"github.com/example/tsx-evaluator/internal/db"
)

// Analyze produces dummy evaluation scores for a company.
// Scores are deterministic per symbol (hash-based) so re-evaluations of
// the same symbol produce consistent results. Replace this with real
// analysis logic when ready.
func Analyze(symbol string) *db.ScoreSet {
	h := fnv.New32a()
	h.Write([]byte(symbol))
	seed := h.Sum32()

	return &db.ScoreSet{
		Symbol:        symbol,
		Financials:    scoreFromSeed(seed, 0),
		Sentiment:     scoreFromSeed(seed, 1),
		Leadership:    scoreFromSeed(seed, 2),
		TypeSentiment: scoreFromSeed(seed, 3),
	}
}

// scoreFromSeed returns a value in [-1, 1] derived from the seed and a
// per-metric offset so each metric produces a different value.
func scoreFromSeed(seed uint32, offset int) float64 {
	v := float64((seed>>uint(offset*8))&0xFF) / 255.0
	return math.Round((v*2-1)*1000) / 1000
}
