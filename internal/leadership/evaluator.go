package leadership

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

// Evaluator scores a company's leadership quality using executive data.
type Evaluator struct {
	fmpClient *FMPClient
	log       *slog.Logger
}

// NewEvaluator creates a leadership evaluator.
func NewEvaluator(fmpClient *FMPClient, log *slog.Logger) *Evaluator {
	return &Evaluator{fmpClient: fmpClient, log: log}
}

// Evaluate fetches executive data for a symbol and returns a score in [-1, 1].
// 1.0 = excellent leadership (long tenure, stable team), -1.0 = poor leadership.
// Returns 0.0 (neutral) when data is insufficient.
func (e *Evaluator) Evaluate(ctx context.Context, symbol string) float64 {
	data, err := e.fetchLeadershipData(ctx, symbol)
	if err != nil {
		e.log.Warn("failed to fetch leadership data", "symbol", symbol, "error", err)
		return 0
	}

	if len(data.Executives) == 0 {
		e.log.Warn("no executive data found", "symbol", symbol)
		return 0
	}

	tenureScore := e.calculateTenureScore(data)
	stabilityScore := e.calculateStabilityScore(data)

	// 60% tenure, 40% stability (insider sentiment skipped for TSX/Canadian stocks)
	final := 0.6*tenureScore + 0.4*stabilityScore

	e.log.Info("leadership analysis complete",
		"symbol", symbol,
		"avg_tenure", data.AvgTenure,
		"tenure_score", tenureScore,
		"stability_score", stabilityScore,
		"final_score", final)

	return clamp(final, -1, 1)
}

func (e *Evaluator) fetchLeadershipData(ctx context.Context, symbol string) (*LeadershipData, error) {
	executives, err := e.fmpClient.GetExecutives(ctx, symbol)
	if err != nil {
		return nil, err
	}

	if len(executives) == 0 {
		return &LeadershipData{}, nil
	}

	data := &LeadershipData{
		Executives: executives,
	}

	currentYear := time.Now().Year()
	var tenureSum float64
	var tenureCount int

	for i := range executives {
		exec := &executives[i]

		// Identify CEO
		titleLower := strings.ToLower(exec.Title)
		if strings.Contains(titleLower, "chief executive") || strings.Contains(titleLower, "ceo") {
			data.CEO = exec
		}

		// Identify CFO
		if strings.Contains(titleLower, "chief financial") || strings.Contains(titleLower, "cfo") {
			data.CFO = exec
		}

		if exec.Since != "" {
			sinceYear, err := strconv.Atoi(exec.Since)
			if err == nil && sinceYear > 0 && sinceYear <= currentYear {
				tenure := float64(currentYear - sinceYear)
				tenureSum += tenure
				tenureCount++
			}
		}
	}

	if tenureCount > 0 {
		data.AvgTenure = tenureSum / float64(tenureCount)
	}

	return data, nil
}

// calculateTenureScore evaluates the average tenure of executives.
// Returns a score in [-1, 1].
func (e *Evaluator) calculateTenureScore(data *LeadershipData) float64 {
	if data.AvgTenure <= 0 {
		return 0
	}

	// Scoring thresholds:
	// < 2 years: -0.5 to -1.0 (new team, risky)
	// 2-5 years: 0.0 to 0.5 (established)
	// 5+ years: 0.5 to 1.0 (stable, experienced)
	switch {
	case data.AvgTenure < 2:
		// Linear from -1.0 (at 0 years) to -0.5 (at 2 years)
		return -1.0 + 0.5*(data.AvgTenure/2.0)
	case data.AvgTenure < 5:
		// Linear from 0.0 (at 2 years) to 0.5 (at 5 years)
		return 0.5 * ((data.AvgTenure - 2.0) / 3.0)
	default:
		// Linear from 0.5 (at 5 years) to 1.0 (at 10+ years)
		score := 0.5 + 0.5*((data.AvgTenure-5.0)/5.0)
		return clamp(score, 0.5, 1.0)
	}
}

// calculateStabilityScore evaluates CEO and CFO tenure.
// Returns a score in [-1, 1].
func (e *Evaluator) calculateStabilityScore(data *LeadershipData) float64 {
	currentYear := time.Now().Year()
	var scores []float64

	if data.CEO != nil && data.CEO.Since != "" {
		sinceYear, err := strconv.Atoi(data.CEO.Since)
		if err == nil && sinceYear > 0 && sinceYear <= currentYear {
			ceoTenure := float64(currentYear - sinceYear)
			scores = append(scores, e.stabilityFromTenure(ceoTenure))
		}
	}

	if data.CFO != nil && data.CFO.Since != "" {
		sinceYear, err := strconv.Atoi(data.CFO.Since)
		if err == nil && sinceYear > 0 && sinceYear <= currentYear {
			cfoTenure := float64(currentYear - sinceYear)
			scores = append(scores, e.stabilityFromTenure(cfoTenure))
		}
	}

	if len(scores) == 0 {
		return 0
	}

	total := 0.0
	for _, s := range scores {
		total += s
	}
	return total / float64(len(scores))
}

// stabilityFromTenure converts tenure years to a stability score.
// < 1 year: -1.0 (very new)
// 1-3 years: -0.5 to 0.0
// 3-7 years: 0.0 to 0.5
// 7+ years: 0.5 to 1.0
func (e *Evaluator) stabilityFromTenure(tenure float64) float64 {
	switch {
	case tenure < 1:
		return -1.0
	case tenure < 3:
		// Linear from -0.5 (at 1 year) to 0.0 (at 3 years)
		return -0.5 + 0.5*((tenure-1.0)/2.0)
	case tenure < 7:
		// Linear from 0.0 (at 3 years) to 0.5 (at 7 years)
		return 0.5 * ((tenure - 3.0) / 4.0)
	default:
		// Linear from 0.5 (at 7 years) to 1.0 (at 12+ years)
		score := 0.5 + 0.5*((tenure-7.0)/5.0)
		return clamp(score, 0.5, 1.0)
	}
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
