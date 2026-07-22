package analyzer

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/tsx-evaluator/internal/finance"
)

func newMockFMPServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		case contains(path, "income-statement"):
			json.NewEncoder(w).Encode([]finance.IncomeStatement{
				{Symbol: "TEST.TO", Revenue: 1e9, NetIncome: 100e6, GrossProfit: 400e6, GrossProfitRatio: 0.4},
				{Symbol: "TEST.TO", Revenue: 800e6, NetIncome: 80e6, GrossProfit: 300e6, GrossProfitRatio: 0.375},
			})
		case contains(path, "balance-sheet-statement"):
			json.NewEncoder(w).Encode([]finance.BalanceSheet{
				{Symbol: "TEST.TO", TotalAssets: 2e9, TotalCurrentAssets: 1e9,
					TotalCurrentLiabilities: 500e6, TotalDebt: 200e6,
					LongTermDebt: 100e6, TotalStockholdersEquity: 1.5e9, CommonStock: 10e6},
				{Symbol: "TEST.TO", TotalAssets: 1.8e9, TotalCurrentAssets: 900e6,
					TotalCurrentLiabilities: 550e6, TotalDebt: 300e6,
					LongTermDebt: 200e6, TotalStockholdersEquity: 1.2e9, CommonStock: 10e6},
			})
		default:
			w.Write([]byte("[]"))
		}
	}))
}

func TestAnalyze_ReturnsRealScore(t *testing.T) {
	server := newMockFMPServer(t)
	defer server.Close()

	client := finance.NewClient(server.URL, "test-key")
	scores := Analyze(context.Background(), client, "TEST.TO", slog.Default())

	if scores.Symbol != "TEST.TO" {
		t.Errorf("symbol: got %q, want %q", scores.Symbol, "TEST.TO")
	}
	if scores.Financials < -1 || scores.Financials > 1 {
		t.Errorf("financials out of range: %v", scores.Financials)
	}
	t.Logf("financial score: %.3f", scores.Financials)
}

func TestAnalyze_SentimentFieldsZero(t *testing.T) {
	server := newMockFMPServer(t)
	defer server.Close()

	client := finance.NewClient(server.URL, "test-key")
	scores := Analyze(context.Background(), client, "TEST.TO", slog.Default())

	if scores.Sentiment != 0 {
		t.Errorf("sentiment: got %v, want 0", scores.Sentiment)
	}
	if scores.Leadership != 0 {
		t.Errorf("leadership: got %v, want 0", scores.Leadership)
	}
	if scores.TypeSentiment != 0 {
		t.Errorf("type_sentiment: got %v, want 0", scores.TypeSentiment)
	}
}

func TestAnalyze_NoData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	}))
	defer server.Close()

	client := finance.NewClient(server.URL, "test-key")
	scores := Analyze(context.Background(), client, "MISSING.TO", slog.Default())

	if scores.Financials != 0 {
		t.Errorf("expected 0 for missing data, got %v", scores.Financials)
	}
}

func TestAnalyze_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := finance.NewClient(server.URL, "test-key")
	scores := Analyze(context.Background(), client, "ERR.TO", slog.Default())

	if scores.Financials != 0 {
		t.Errorf("expected 0 for API error, got %v", scores.Financials)
	}
}

func TestAnalyze_Deterministic(t *testing.T) {
	server := newMockFMPServer(t)
	defer server.Close()

	client := finance.NewClient(server.URL, "test-key")
	s1 := Analyze(context.Background(), client, "TEST.TO", slog.Default())
	s2 := Analyze(context.Background(), client, "TEST.TO", slog.Default())

	if s1.Financials != s2.Financials {
		t.Errorf("non-deterministic: %v vs %v", s1.Financials, s2.Financials)
	}
}

func TestAnalyze_ScoreBounds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		case contains(path, "income-statement"):
			json.NewEncoder(w).Encode([]finance.IncomeStatement{
				{Symbol: "EXTREME.TO", Revenue: 1e12, NetIncome: 500e9, GrossProfit: 800e9},
			})
		case contains(path, "balance-sheet-statement"):
			json.NewEncoder(w).Encode([]finance.BalanceSheet{
				{Symbol: "EXTREME.TO", TotalAssets: 1e12, TotalCurrentAssets: 500e9,
					TotalCurrentLiabilities: 1e9, TotalDebt: 0, TotalStockholdersEquity: 1e12},
			})
		default:
			w.Write([]byte("[]"))
		}
	}))
	defer server.Close()

	client := finance.NewClient(server.URL, "test-key")
	scores := Analyze(context.Background(), client, "EXTREME.TO", slog.Default())

	if scores.Financials < -1 || scores.Financials > 1 {
		t.Errorf("score out of bounds: %v", scores.Financials)
	}
}

func BenchmarkAnalyze(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		case contains(path, "income-statement"):
			json.NewEncoder(w).Encode([]finance.IncomeStatement{
				{Symbol: "BENCH.TO", Revenue: 1e9, NetIncome: 100e6, GrossProfit: 400e6},
			})
		case contains(path, "balance-sheet-statement"):
			json.NewEncoder(w).Encode([]finance.BalanceSheet{
				{Symbol: "BENCH.TO", TotalAssets: 2e9, TotalCurrentAssets: 1e9,
					TotalCurrentLiabilities: 500e6, TotalStockholdersEquity: 1.5e9},
			})
		default:
			w.Write([]byte("[]"))
		}
	}))
	defer server.Close()

	client := finance.NewClient(server.URL, "test-key")
	log := slog.Default()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Analyze(ctx, client, "BENCH.TO", log)
	}
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
