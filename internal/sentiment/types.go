package sentiment

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Article represents a news article or post from any source.
type Article struct {
	Title     string
	Snippet   string
	Source    string
	URL       string
	Published time.Time
	Score     int // reserved for future use (e.g. social engagement metrics)
}

// SentimentResult holds the LLM's sentiment analysis output.
type SentimentResult struct {
	Score      float64 `json:"score"`
	Reasoning  string  `json:"reasoning"`
	Confidence float64 `json:"confidence"`
}

// SourceClient defines the interface for fetching articles.
type SourceClient interface {
	FetchArticles(ctx context.Context, symbol string, limit int) ([]Article, error)
}

// FormatArticlesForLLM formats articles into a string for the LLM prompt.
func FormatArticlesForLLM(articles []Article) string {
	var sb strings.Builder
	for i, a := range articles {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, a.Source, a.Title))
		if a.Snippet != "" && a.Snippet != a.Title {
			snippet := a.Snippet
			if len(snippet) > 200 {
				snippet = snippet[:200] + "..."
			}
			sb.WriteString(fmt.Sprintf("   %s\n", snippet))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
