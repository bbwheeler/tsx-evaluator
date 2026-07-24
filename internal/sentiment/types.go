package sentiment

import (
	"context"
	"time"
)

// Article represents a news article or post from any source.
type Article struct {
	Title     string
	Snippet   string
	Source    string
	URL       string
	Published time.Time
	Score     int // upvotes/comments for Reddit, 0 for news
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
