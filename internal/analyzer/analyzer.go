package analyzer

import (
	"context"
	"log/slog"

	"github.com/example/tsx-evaluator/internal/db"
	"github.com/example/tsx-evaluator/internal/finance"
	"github.com/example/tsx-evaluator/internal/leadership"
	"github.com/example/tsx-evaluator/internal/sentiment"
	"github.com/example/tsx-evaluator/internal/typesentiment"
)

// Analyze fetches real financial data for symbol and returns a ScoreSet.
// Each field holds a score in [-1, 1]:
//   - Financials: Piotroski-based financial health score
//   - Sentiment: LLM-analyzed sentiment from news and social media
//   - Leadership: Executive tenure and stability score
//   - TypeSentiment: Sector/industry outlook sentiment score
func Analyze(ctx context.Context, financeClient *finance.Client, sentimentEv *sentiment.Evaluator, leadershipEv *leadership.Evaluator, typeSentEv *typesentiment.Evaluator, symbol string, log *slog.Logger) *db.ScoreSet {
	// Financial health score
	finEv := finance.NewEvaluator(financeClient, log)
	financialScore := finEv.Evaluate(ctx, symbol)

	// Sentiment score
	sentimentScore := sentimentEv.Evaluate(ctx, symbol)

	// Leadership score
	leadershipScore := leadershipEv.Evaluate(ctx, symbol)

	// Type sentiment score
	typeSentimentScore := typeSentEv.Evaluate(ctx, symbol)

	return &db.ScoreSet{
		Symbol:        symbol,
		Financials:    financialScore,
		Sentiment:     sentimentScore,
		Leadership:    leadershipScore,
		TypeSentiment: typeSentimentScore,
	}
}
