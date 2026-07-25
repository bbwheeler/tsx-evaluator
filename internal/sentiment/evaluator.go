package sentiment

import (
	"context"
	"log/slog"
	"sync"
)

// Evaluator scores a company's sentiment using news and social media data.
type Evaluator struct {
	llmClient    *LLMClient
	yahooClient  *YahooRSSClient
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
		yahooClient:  NewYahooRSSClient(),
		googleClient: NewGoogleNewsClient("en-US"),
		log:          log,
	}
}

// Evaluate fetches sentiment data for a symbol and returns a score in [-1, 1].
// 1.0 = extremely positive sentiment, -1.0 = extremely negative sentiment.
// Returns 0.0 (neutral) when data is insufficient or analysis fails.
func (e *Evaluator) Evaluate(ctx context.Context, symbol string) float64 {
	// Fetch articles from all sources concurrently
	articles := e.fetchAllArticles(ctx, symbol)

	if len(articles) == 0 {
		e.log.Warn("no articles found for sentiment analysis", "symbol", symbol)
		return 0
	}

	e.log.Info("fetched articles for sentiment",
		"symbol", symbol,
		"count", len(articles))

	// Format articles for LLM
	articlesText := FormatArticlesForLLM(articles)

	// Send to LLM for analysis
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

// fetchAllArticles fetches articles from all sources concurrently.
func (e *Evaluator) fetchAllArticles(ctx context.Context, symbol string) []Article {
	type sourceResult struct {
		articles []Article
		err      error
	}

	var wg sync.WaitGroup
	results := make(chan sourceResult, 2)

	// Yahoo Finance RSS
	wg.Add(1)
	go func() {
		defer wg.Done()
		articles, err := e.yahooClient.FetchArticles(ctx, symbol, 10)
		results <- sourceResult{articles, err}
	}()

	// Google News RSS
	wg.Add(1)
	go func() {
		defer wg.Done()
		articles, err := e.googleClient.FetchArticles(ctx, symbol, 15)
		results <- sourceResult{articles, err}
	}()

	// Wait for all goroutines to finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect all articles
	var allArticles []Article
	for result := range results {
		if result.err != nil {
			e.log.Warn("failed to fetch articles",
				"error", result.err)
			continue
		}
		allArticles = append(allArticles, result.articles...)
	}

	return allArticles
}
