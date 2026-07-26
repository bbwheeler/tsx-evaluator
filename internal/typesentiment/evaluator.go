package typesentiment

import (
	"context"
	"log/slog"
	"strings"

	"github.com/example/tsx-evaluator/internal/sentiment"
)

// Evaluator scores the sentiment of a company's sector/industry.
// It fetches the company profile to determine sector/industry, then performs
// sentiment analysis on sector-level news using the same tools as stock sentiment.
type Evaluator struct {
	profileClient *ProfileClient
	llmClient     *sentiment.LLMClient
	googleClient  *sentiment.GoogleNewsClient
	log           *slog.Logger
}

func NewEvaluator(
	profileClient *ProfileClient,
	llmClient *sentiment.LLMClient,
	log *slog.Logger,
) *Evaluator {
	return &Evaluator{
		profileClient: profileClient,
		llmClient:     llmClient,
		googleClient:  sentiment.NewGoogleNewsClient("en-US"),
		log:           log,
	}
}

// Evaluate fetches the company's sector/industry and returns a sentiment score
// for that sector in [-1, 1].
// 1.0 = extremely bullish sector outlook, -1.0 = extremely bearish.
// Returns 0.0 (neutral) when data is insufficient.
func (e *Evaluator) Evaluate(ctx context.Context, symbol string) float64 {
	profile, err := e.profileClient.GetProfile(ctx, symbol)
	if err != nil {
		e.log.Warn("failed to fetch company profile for type sentiment",
			"symbol", symbol, "error", err)
		return 0
	}

	sector := strings.TrimSpace(profile.Sector)
	industry := strings.TrimSpace(profile.Industry)

	if sector == "" && industry == "" {
		e.log.Warn("no sector or industry data for type sentiment",
			"symbol", symbol)
		return 0
	}

	e.log.Info("company profile loaded for type sentiment",
		"symbol", symbol,
		"sector", sector,
		"industry", industry)

	queries := e.buildSearchQueries(sector, industry)
	articles := e.fetchSectorArticles(ctx, queries)

	if len(articles) == 0 {
		e.log.Warn("no sector articles found for type sentiment",
			"symbol", symbol,
			"sector", sector,
			"industry", industry)
		return 0
	}

	e.log.Info("fetched sector articles for type sentiment",
		"symbol", symbol,
		"sector", sector,
		"industry", industry,
		"article_count", len(articles))

	articlesText := sentiment.FormatArticlesForLLM(articles)
	score, reasoning, confidence := e.analyzeSectorSentiment(ctx, sector, industry, articlesText)

	e.log.Info("type sentiment analysis complete",
		"symbol", symbol,
		"sector", sector,
		"industry", industry,
		"score", score,
		"confidence", confidence,
		"reasoning", reasoning)

	return score
}

func (e *Evaluator) buildSearchQueries(sector, industry string) []string {
	var queries []string

	if sector != "" {
		queries = append(queries, sector+" sector stocks outlook")
		queries = append(queries, sector+" stocks market trend")
	}
	if industry != "" && industry != sector {
		queries = append(queries, industry+" stocks outlook")
	}

	// Fallback if we only have one
	if len(queries) == 0 {
		if sector != "" {
			queries = append(queries, sector+" stocks")
		}
		if industry != "" {
			queries = append(queries, industry+" stocks")
		}
	}

	return queries
}

func (e *Evaluator) fetchSectorArticles(ctx context.Context, queries []string) []sentiment.Article {
	type result struct {
		articles []sentiment.Article
		err      error
	}

	ch := make(chan result, len(queries))

	for _, q := range queries {
		go func(query string) {
			articles, err := e.googleClient.FetchArticles(ctx, query, 10)
			ch <- result{articles, err}
		}(q)
	}

	var all []sentiment.Article
	for i := 0; i < len(queries); i++ {
		r := <-ch
		if r.err != nil {
			e.log.Warn("failed to fetch sector articles",
				"error", r.err)
			continue
		}
		all = append(all, r.articles...)
	}

	return all
}

func (e *Evaluator) analyzeSectorSentiment(ctx context.Context, sector, industry, articlesText string) (float64, string, float64) {
	typeName := sector
	if industry != "" && industry != sector {
		typeName = industry + " (in the " + sector + " sector)"
	}

	prompt := `You are a sector/industry analyst. Analyze the overall sentiment and outlook for the ` + typeName + ` industry/sector based on these recent news articles and market commentary.

Focus on:
- Overall sector growth prospects and macro trends
- Regulatory environment and policy impacts
- Supply/demand dynamics for this sector
- Competitive positioning of this sector relative to others
- Any major catalysts or headwinds affecting the sector

Articles and commentary:
` + articlesText + `
Respond ONLY with a JSON object in this exact format:
{"score": <number>, "reasoning": "<brief explanation>", "confidence": <number>}

Score must be between -1.0 and 1.0:
- -1.0 = extremely bearish sector outlook (major decline, regulatory crackdown, obsolescence risk)
- -0.5 = bearish outlook (slowing growth, headwinds, competitive pressure)
- 0.0 = neutral/mixed outlook
- 0.5 = bullish outlook (strong growth, favorable trends, tailwinds)
- 1.0 = extremely bullish sector outlook (boom conditions, massive growth, favorable regulation)

Confidence must be between 0.0 and 1.0 based on how much data you had to work with.

Do not include any text before or after the JSON.`

	result, err := e.llmClient.AnalyzeSectorSentiment(ctx, prompt)
	if err != nil {
		e.log.Error("LLM sector sentiment analysis failed",
			"sector", sector,
			"error", err)
		return 0, "", 0
	}

	return result.Score, result.Reasoning, result.Confidence
}
