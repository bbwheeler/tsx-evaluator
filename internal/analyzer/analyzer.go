package analyzer

import (
	"context"
	"log/slog"

	"github.com/example/tsx-evaluator/internal/db"
	"github.com/example/tsx-evaluator/internal/finance"
)

// Analyze fetches real financial data for symbol and returns a ScoreSet.
// The Financials field holds the Piotroski-based financial health score in [-1, 1].
// Sentiment, Leadership, and TypeSentiment remain 0 (not yet implemented).
func Analyze(ctx context.Context, client *finance.Client, symbol string, log *slog.Logger) *db.ScoreSet {
	ev := finance.NewEvaluator(client, log)
	financialScore := ev.Evaluate(ctx, symbol)

	return &db.ScoreSet{
		Symbol:        symbol,
		Financials:    financialScore,
		Sentiment:     0,
		Leadership:    0,
		TypeSentiment: 0,
	}
}
