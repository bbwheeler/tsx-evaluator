package finance

import (
	"context"
	"log/slog"
)

// Evaluator scores a company's financial health using the Piotroski F-Score
// and supplementary metrics fetched from the FMP API.
type Evaluator struct {
	client *Client
	log    *slog.Logger
}

// NewEvaluator creates a financial evaluator.
func NewEvaluator(client *Client, log *slog.Logger) *Evaluator {
	return &Evaluator{client: client, log: log}
}

// Evaluate fetches financial data for symbol and returns a score in [-1, 1].
// 1.0 = excellent financial health, -1.0 = terrible/likely distressed.
// Returns 0.0 (neutral) when data is insufficient.
func (e *Evaluator) Evaluate(ctx context.Context, symbol string) float64 {
	currentIS, priorIS, currentBS, priorBS, err := e.fetchFinancials(ctx, symbol)
	if err != nil {
		e.log.Warn("fetch financials", "symbol", symbol, "error", err)
		return 0
	}
	if currentIS == nil || currentBS == nil {
		e.log.Warn("no financial data", "symbol", symbol)
		return 0
	}

	piotroskiScore, _ := CalculatePiotroski(currentIS, priorIS, currentBS, priorBS)
	piotroskiNorm := (float64(piotroskiScore) - 4.5) / 4.5
	piotroskiNorm = clamp(piotroskiNorm, -1, 1)

	customScore := e.customMetricsScore(currentIS, currentBS, priorIS, priorBS)

	// 60% Piotroski, 40% custom health metrics
	final := 0.6*piotroskiNorm + 0.4*customScore
	return clamp(final, -1, 1)
}

func (e *Evaluator) fetchFinancials(ctx context.Context, symbol string) (
	currentIS, priorIS *IncomeStatement,
	currentBS, priorBS *BalanceSheet,
	err error,
) {
	is, err := e.client.GetIncomeStatement(ctx, symbol, 2)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	bs, err := e.client.GetBalanceSheet(ctx, symbol, 2)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	switch len(is) {
	case 0:
		return nil, nil, nil, nil, nil
	case 1:
		currentIS = &is[0]
	default:
		currentIS = &is[0]
		priorIS = &is[1]
	}

	switch len(bs) {
	case 0:
		return currentIS, priorIS, nil, nil, nil
	case 1:
		currentBS = &bs[0]
	default:
		currentBS = &bs[0]
		priorBS = &bs[1]
	}

	return currentIS, priorIS, currentBS, priorBS, nil
}

// customMetricsScore evaluates debt-to-equity, current ratio, revenue growth,
// and net margin, returning a combined score in [-1, 1].
func (e *Evaluator) customMetricsScore(
	currentIS *IncomeStatement, currentBS *BalanceSheet,
	priorIS *IncomeStatement, priorBS *BalanceSheet,
) float64 {
	total := 0.0
	count := 0.0

	// Debt-to-Equity (lower is better)
	if currentBS.TotalStockholdersEquity > 0 {
		de := currentBS.TotalDebt / currentBS.TotalStockholdersEquity
		deScore := 1.0 - 2.0*clamp01(de/3.0) // DE of 0 → 1.0, DE of 1.5 → 0, DE of 3+ → -1.0
		total += deScore
		count++
	}

	// Current ratio (higher is better, capped at 4.0)
	if currentBS.TotalCurrentLiabilities > 0 {
		cr := currentBS.TotalCurrentAssets / currentBS.TotalCurrentLiabilities
		crScore := 2.0*clamp01(cr/3.0) - 1.0 // CR of 0 → -1, CR of 1.5 → 0, CR of 3+ → 1
		total += crScore
		count++
	}

	// Revenue growth (vs prior year)
	if priorIS != nil && priorIS.Revenue > 0 {
		growth := (currentIS.Revenue - priorIS.Revenue) / priorIS.Revenue
		growthScore := clamp(growth*2, -1, 1) // 50% growth → 1, -50% → -1
		total += growthScore
		count++
	}

	// Net margin
	if currentIS.Revenue > 0 {
		margin := currentIS.NetIncome / currentIS.Revenue
		marginScore := clamp(margin*5, -1, 1) // 20% margin → 1, -20% → -1
		total += marginScore
		count++
	}

	if count == 0 {
		return 0
	}
	return clamp(total/count, -1, 1)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clamp01(v float64) float64 {
	return clamp(v, 0, 1)
}
