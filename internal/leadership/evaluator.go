package leadership

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

type ExecutiveClient interface {
	GetExecutives(ctx context.Context, symbol string) ([]Executive, error)
}

// Evaluator scores a company's leadership quality using executive data.
type Evaluator struct {
	client ExecutiveClient
	log    *slog.Logger
}

// NewEvaluator creates a leadership evaluator.
func NewEvaluator(client ExecutiveClient, log *slog.Logger) *Evaluator {
	return &Evaluator{client: client, log: log}
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
	executives, err := e.client.GetExecutives(ctx, symbol)
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

	for i := range executives {
		exec := &executives[i]

		titleLower := strings.ToLower(exec.Title)
		if strings.Contains(titleLower, "chief executive") || strings.Contains(titleLower, "ceo") {
			data.CEO = exec
		}
		if strings.Contains(titleLower, "chief financial") || strings.Contains(titleLower, "cfo") {
			data.CFO = exec
		}

		if exec.YearOfBirth > 0 && exec.Age == 0 {
			exec.Age = currentYear - exec.YearOfBirth
		}
	}

	return data, nil
}

// calculateTenureScore evaluates the average tenure of executives.
// Returns 0 when tenure data is unavailable (e.g. from free data sources).
func (e *Evaluator) calculateTenureScore(data *LeadershipData) float64 {
	if data.AvgTenure <= 0 {
		return 0
	}

	switch {
	case data.AvgTenure < 2:
		return -1.0 + 0.5*(data.AvgTenure/2.0)
	case data.AvgTenure < 5:
		return 0.5 * ((data.AvgTenure - 2.0) / 3.0)
	default:
		score := 0.5 + 0.5*((data.AvgTenure-5.0)/5.0)
		return clamp(score, 0.5, 1.0)
	}
}

// calculateStabilityScore evaluates CEO and CFO career stage.
// Returns 0 when age data is unavailable.
func (e *Evaluator) calculateStabilityScore(data *LeadershipData) float64 {
	currentYear := time.Now().Year()
	var scores []float64

	if data.CEO != nil && data.CEO.YearOfBirth > 0 {
		ceoAge := float64(currentYear - data.CEO.YearOfBirth)
		scores = append(scores, stabilityFromAge(ceoAge))
	}

	if data.CFO != nil && data.CFO.YearOfBirth > 0 {
		cfoAge := float64(currentYear - data.CFO.YearOfBirth)
		scores = append(scores, stabilityFromAge(cfoAge))
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

// stabilityFromAge converts executive age to a stability score.
// Mid-career (45-60) scores highest; very young or near-retirement scores lower.
func stabilityFromAge(age float64) float64 {
	switch {
	case age < 35:
		return -0.5
	case age < 45:
		return -0.5 + 0.5*((age-35)/10.0)
	case age < 60:
		return 0.5
	case age < 70:
		return 0.5 - 0.5*((age-60)/10.0)
	default:
		return -0.5
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
