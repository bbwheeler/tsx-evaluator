package sentiment

import (
	"context"
	"log/slog"
)

// Evaluator scores a company's sentiment using news and social media data.
type Evaluator struct {
	llmClient    *LLMClient
	googleClient *GoogleNewsClient
	log          *slog.Logger
}

// NewEvaluator creates a sentiment evaluator.
func NewEvaluator(
	llmClient *LLMClient,
	log *slog.Logger,
) *Evaluator {
	return &Evaluator{
		llmClient:    llmClient,
		googleClient: NewGoogleNewsClient("en-US"),
		log:          log,
	}
}

// Evaluate fetches sentiment data for a symbol and returns a score in [-1, 1].
// 1.0 = extremely positive sentiment, -1.0 = extremely negative sentiment.
// Returns 0.0 (neutral) when data is insufficient or analysis fails.
func (e *Evaluator) Evaluate(ctx context.Context, symbol string) float64 {
	articles := e.fetchAllArticles(ctx, symbol)

	if len(articles) == 0 {
		e.log.Warn("no articles found for sentiment analysis", "symbol", symbol)
		return 0
	}

	e.log.Info("fetched articles for sentiment",
		"symbol", symbol,
		"count", len(articles))

	articlesText := FormatArticlesForLLM(articles)

	result, err := e.llmClient.AnalyzeSentiment(ctx, symbol, articlesText)
	if err != nil {
		e.log.Error("llm sentiment analysis failed",
			"symbol", symbol,
			"error", err)
		return 0
	}

	e.log.Info("sentiment analysis complete",
		"symbol", symbol,
		"score", result.Score,
		"confidence", result.Confidence,
		"reasoning", result.Reasoning)

	return result.Score
}

func (e *Evaluator) fetchAllArticles(ctx context.Context, symbol string) []Article {
	articles, err := e.googleClient.FetchArticles(ctx, symbol, 20)
	if err != nil {
		e.log.Warn("failed to fetch articles",
			"error", err)
		return nil
	}
	return articles
}
