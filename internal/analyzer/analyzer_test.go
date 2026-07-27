package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/tsx-evaluator/internal/finance"
	"github.com/example/tsx-evaluator/internal/leadership"
	"github.com/example/tsx-evaluator/internal/sentiment"
	"github.com/example/tsx-evaluator/internal/typesentiment"
)

func timeseriesResponse(data map[string][]tsEntry) map[string]any {
	var results []any
	for k, v := range data {
		results = append(results, map[string]any{
			"meta": map[string]any{
				"symbol": []string{"TEST.TO"},
				"type":   []string{k},
			},
			k: v,
		})
	}
	return map[string]any{
		"timeseries": map[string]any{
			"result": results,
		},
	}
}

type tsEntry struct {
	AsOfDate      string `json:"asOfDate"`
	PeriodType    string `json:"periodType"`
	ReportedValue struct {
		Raw float64 `json:"raw"`
	} `json:"reportedValue"`
}

func tsEntryVal(date string, val float64) tsEntry {
	e := tsEntry{AsOfDate: date, PeriodType: "12M"}
	e.ReportedValue.Raw = val
	return e
}

func mockYahooTimeserver(handler func(symbol string) map[string][]tsEntry) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		symbol := r.URL.Query().Get("symbol")
		if symbol == "" {
			symbol = "TEST.TO"
		}
		data := handler(symbol)
		resp := timeseriesResponse(data)
		json.NewEncoder(w).Encode(resp)
	}))
}

func mockProfileServer(handler func(symbol string) map[string]any) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		symbol := r.URL.Query().Get("symbol")
		if symbol == "" {
			symbol = "TEST.TO"
		}
		json.NewEncoder(w).Encode(handler(symbol))
	}))
}

func newMockFinanceAndProfileServers(t *testing.T) (*httptest.Server, *httptest.Server) {
	t.Helper()

	financeServer := mockYahooTimeserver(func(symbol string) map[string][]tsEntry {
		return map[string][]tsEntry{
			"annualTotalRevenue":    {tsEntryVal("2024-12-31", 1e9), tsEntryVal("2023-12-31", 800e6)},
			"annualGrossProfit":     {tsEntryVal("2024-12-31", 400e6), tsEntryVal("2023-12-31", 300e6)},
			"annualOperatingIncome": {tsEntryVal("2024-12-31", 200e6)},
			"annualNetIncome":       {tsEntryVal("2024-12-31", 100e6), tsEntryVal("2023-12-31", 80e6)},
			"annualTotalAssets":     {tsEntryVal("2024-12-31", 2e9), tsEntryVal("2023-12-31", 1.8e9)},
			"annualCurrentAssets":   {tsEntryVal("2024-12-31", 1e9), tsEntryVal("2023-12-31", 900e6)},
			"annualCurrentLiabilities": {tsEntryVal("2024-12-31", 500e6), tsEntryVal("2023-12-31", 550e6)},
			"annualStockholdersEquity": {tsEntryVal("2024-12-31", 1.5e9), tsEntryVal("2023-12-31", 1.2e9)},
			"annualCommonStock":     {tsEntryVal("2024-12-31", 10e6), tsEntryVal("2023-12-31", 10e6)},
			"annualTotalDebt":       {tsEntryVal("2024-12-31", 200e6)},
			"annualLongTermDebt":    {tsEntryVal("2024-12-31", 100e6), tsEntryVal("2023-12-31", 200e6)},
		}
	})

	profileServer := mockProfileServer(func(symbol string) map[string]any {
		return map[string]any{
			"quoteSummary": map[string]any{
				"result": []map[string]any{},
			},
		}
	})

	return financeServer, profileServer
}

func newTypeSentEvForTest(profileURL string) *typesentiment.Evaluator {
	llmClient := sentiment.NewLLMClient("http://localhost:11434", "llama3", 60)
	return typesentiment.NewEvaluatorForTest(profileURL, llmClient, slog.Default())
}

func TestAnalyze_ReturnsRealScore(t *testing.T) {
	financeServer, profileServer := newMockFinanceAndProfileServers(t)
	defer financeServer.Close()
	defer profileServer.Close()

	client := finance.NewClientWithBaseURL(financeServer.URL)
	llmClient := sentiment.NewLLMClient("http://localhost:11434", "llama3", 60)
	sentEv := sentiment.NewEvaluator(llmClient, slog.Default())
	leadEv := leadership.NewEvaluator(&mockExecutiveClient{}, slog.Default())
	typeSentEv := newTypeSentEvForTest(profileServer.URL)
	scores := Analyze(context.Background(), client, sentEv, leadEv, typeSentEv, "TEST.TO", slog.Default())

	if scores.Symbol != "TEST.TO" {
		t.Errorf("symbol: got %q, want %q", scores.Symbol, "TEST.TO")
	}
	if scores.Financials < -1 || scores.Financials > 1 {
		t.Errorf("financials out of range: %v", scores.Financials)
	}
	t.Logf("financial score: %.3f", scores.Financials)
}

func TestAnalyze_TypeSentimentReturnsScore(t *testing.T) {
	financeServer := mockYahooTimeserver(func(symbol string) map[string][]tsEntry {
		return map[string][]tsEntry{
			"annualTotalRevenue":       {tsEntryVal("2024-12-31", 1e9)},
			"annualGrossProfit":        {tsEntryVal("2024-12-31", 400e6)},
			"annualNetIncome":          {tsEntryVal("2024-12-31", 100e6)},
			"annualTotalAssets":        {tsEntryVal("2024-12-31", 2e9)},
			"annualCurrentAssets":      {tsEntryVal("2024-12-31", 1e9)},
			"annualCurrentLiabilities": {tsEntryVal("2024-12-31", 500e6)},
			"annualStockholdersEquity": {tsEntryVal("2024-12-31", 1.5e9)},
			"annualCommonStock":        {tsEntryVal("2024-12-31", 10e6)},
		}
	})
	defer financeServer.Close()

	profileServer := mockProfileServer(func(symbol string) map[string]any {
		return map[string]any{
			"quoteSummary": map[string]any{
				"result": []map[string]any{
					{
						"assetProfile": map[string]any{
							"companyName":    "Tech Corp",
							"sector":         "Technology",
							"industry":       "Software",
							"description":    "A tech company",
							"fullTimeEmployees": 1000,
						},
					},
				},
			},
		}
	})
	defer profileServer.Close()

	client := finance.NewClientWithBaseURL(financeServer.URL)
	llmClient := sentiment.NewLLMClient("http://localhost:11434", "llama3", 60)
	sentEv := sentiment.NewEvaluator(llmClient, slog.Default())
	leadEv := leadership.NewEvaluator(&mockExecutiveClient{}, slog.Default())
	typeSentEv := newTypeSentEvForTest(profileServer.URL)
	scores := Analyze(context.Background(), client, sentEv, leadEv, typeSentEv, "TECH.TO", slog.Default())

	if scores.TypeSentiment < -1 || scores.TypeSentiment > 1 {
		t.Errorf("type_sentiment out of range: %v", scores.TypeSentiment)
	}
	t.Logf("type_sentiment score: %.3f", scores.TypeSentiment)
}

func TestAnalyze_LeadershipReturnsScore(t *testing.T) {
	financeServer := mockYahooTimeserver(func(symbol string) map[string][]tsEntry {
		return map[string][]tsEntry{
			"annualTotalRevenue":       {tsEntryVal("2024-12-31", 1e9)},
			"annualGrossProfit":        {tsEntryVal("2024-12-31", 400e6)},
			"annualNetIncome":          {tsEntryVal("2024-12-31", 100e6)},
			"annualTotalAssets":        {tsEntryVal("2024-12-31", 2e9)},
			"annualCurrentAssets":      {tsEntryVal("2024-12-31", 1e9)},
			"annualCurrentLiabilities": {tsEntryVal("2024-12-31", 500e6)},
			"annualStockholdersEquity": {tsEntryVal("2024-12-31", 1.5e9)},
			"annualCommonStock":        {tsEntryVal("2024-12-31", 10e6)},
		}
	})
	defer financeServer.Close()

	profileServer := mockProfileServer(func(symbol string) map[string]any {
		return map[string]any{"quoteSummary": map[string]any{"result": []map[string]any{}}}
	})
	defer profileServer.Close()

	client := finance.NewClientWithBaseURL(financeServer.URL)
	llmClient := sentiment.NewLLMClient("http://localhost:11434", "llama3", 60)
	sentEv := sentiment.NewEvaluator(llmClient, slog.Default())
	leadEv := leadership.NewEvaluator(&mockExecutiveClient{
		executives: []leadership.Executive{
			{Name: "Jane Doe", Title: "Chief Executive Officer", YearOfBirth: 1970},
			{Name: "John Smith", Title: "Chief Financial Officer", YearOfBirth: 1977},
		},
	}, slog.Default())
	typeSentEv := newTypeSentEvForTest(profileServer.URL)
	scores := Analyze(context.Background(), client, sentEv, leadEv, typeSentEv, "LEAD.TO", slog.Default())

	if scores.Leadership < -1 || scores.Leadership > 1 {
		t.Errorf("leadership out of range: %v", scores.Leadership)
	}
	t.Logf("leadership score: %.3f", scores.Leadership)
}

func TestAnalyze_NoData(t *testing.T) {
	server := mockYahooTimeserver(func(symbol string) map[string][]tsEntry {
		return map[string][]tsEntry{}
	})
	defer server.Close()

	client := finance.NewClientWithBaseURL(server.URL)
	llmClient := sentiment.NewLLMClient("http://localhost:11434", "llama3", 60)
	sentEv := sentiment.NewEvaluator(llmClient, slog.Default())
	leadEv := leadership.NewEvaluator(&mockExecutiveClient{}, slog.Default())

	profileServer := mockProfileServer(func(symbol string) map[string]any {
		return map[string]any{"quoteSummary": map[string]any{"result": []map[string]any{}}}
	})
	defer profileServer.Close()

	typeSentEv := newTypeSentEvForTest(profileServer.URL)
	scores := Analyze(context.Background(), client, sentEv, leadEv, typeSentEv, "MISSING.TO", slog.Default())

	if scores.Financials != 0 {
		t.Errorf("expected 0 for missing data, got %v", scores.Financials)
	}
}

func TestAnalyze_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := finance.NewClientWithBaseURL(server.URL)
	llmClient := sentiment.NewLLMClient("http://localhost:11434", "llama3", 60)
	sentEv := sentiment.NewEvaluator(llmClient, slog.Default())
	leadEv := leadership.NewEvaluator(&mockExecutiveClient{}, slog.Default())

	profileServer := mockProfileServer(func(symbol string) map[string]any {
		return map[string]any{"quoteSummary": map[string]any{"result": []map[string]any{}}}
	})
	defer profileServer.Close()

	typeSentEv := newTypeSentEvForTest(profileServer.URL)
	scores := Analyze(context.Background(), client, sentEv, leadEv, typeSentEv, "ERR.TO", slog.Default())

	if scores.Financials != 0 {
		t.Errorf("expected 0 for API error, got %v", scores.Financials)
	}
}

func TestAnalyze_Deterministic(t *testing.T) {
	financeServer, profileServer := newMockFinanceAndProfileServers(t)
	defer financeServer.Close()
	defer profileServer.Close()

	client := finance.NewClientWithBaseURL(financeServer.URL)
	llmClient := sentiment.NewLLMClient("http://localhost:11434", "llama3", 60)
	sentEv := sentiment.NewEvaluator(llmClient, slog.Default())
	leadEv := leadership.NewEvaluator(&mockExecutiveClient{}, slog.Default())
	typeSentEv := newTypeSentEvForTest(profileServer.URL)
	s1 := Analyze(context.Background(), client, sentEv, leadEv, typeSentEv, "TEST.TO", slog.Default())
	s2 := Analyze(context.Background(), client, sentEv, leadEv, typeSentEv, "TEST.TO", slog.Default())

	if s1.Financials != s2.Financials {
		t.Errorf("non-deterministic: %v vs %v", s1.Financials, s2.Financials)
	}
}

func TestAnalyze_ScoreBounds(t *testing.T) {
	server := mockYahooTimeserver(func(symbol string) map[string][]tsEntry {
		return map[string][]tsEntry{
			"annualTotalRevenue":       {tsEntryVal("2024-12-31", 1e12)},
			"annualGrossProfit":        {tsEntryVal("2024-12-31", 800e9)},
			"annualNetIncome":          {tsEntryVal("2024-12-31", 500e9)},
			"annualTotalAssets":        {tsEntryVal("2024-12-31", 1e12)},
			"annualCurrentAssets":      {tsEntryVal("2024-12-31", 500e9)},
			"annualCurrentLiabilities": {tsEntryVal("2024-12-31", 1e9)},
			"annualStockholdersEquity": {tsEntryVal("2024-12-31", 1e12)},
		}
	})
	defer server.Close()

	client := finance.NewClientWithBaseURL(server.URL)
	llmClient := sentiment.NewLLMClient("http://localhost:11434", "llama3", 60)
	sentEv := sentiment.NewEvaluator(llmClient, slog.Default())
	leadEv := leadership.NewEvaluator(&mockExecutiveClient{}, slog.Default())

	profileServer := mockProfileServer(func(symbol string) map[string]any {
		return map[string]any{"quoteSummary": map[string]any{"result": []map[string]any{}}}
	})
	defer profileServer.Close()

	typeSentEv := newTypeSentEvForTest(profileServer.URL)
	scores := Analyze(context.Background(), client, sentEv, leadEv, typeSentEv, "EXTREME.TO", slog.Default())

	if scores.Financials < -1 || scores.Financials > 1 {
		t.Errorf("score out of bounds: %v", scores.Financials)
	}
}

func BenchmarkAnalyze(b *testing.B) {
	server := mockYahooTimeserver(func(symbol string) map[string][]tsEntry {
		return map[string][]tsEntry{
			"annualTotalRevenue":       {tsEntryVal("2024-12-31", 1e9)},
			"annualGrossProfit":        {tsEntryVal("2024-12-31", 400e9)},
			"annualNetIncome":          {tsEntryVal("2024-12-31", 100e6)},
			"annualTotalAssets":        {tsEntryVal("2024-12-31", 2e9)},
			"annualCurrentAssets":      {tsEntryVal("2024-12-31", 1e9)},
			"annualCurrentLiabilities": {tsEntryVal("2024-12-31", 500e6)},
			"annualStockholdersEquity": {tsEntryVal("2024-12-31", 1.5e9)},
		}
	})
	defer server.Close()

	profileServer := mockProfileServer(func(symbol string) map[string]any {
		return map[string]any{"quoteSummary": map[string]any{"result": []map[string]any{}}}
	})
	defer profileServer.Close()

	client := finance.NewClientWithBaseURL(server.URL)
	llmClient := sentiment.NewLLMClient("http://localhost:11434", "llama3", 60)
	sentEv := sentiment.NewEvaluator(llmClient, slog.Default())
	leadEv := leadership.NewEvaluator(&mockExecutiveClient{}, slog.Default())
	typeSentEv := newTypeSentEvForTest(profileServer.URL)
	log := slog.Default()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Analyze(ctx, client, sentEv, leadEv, typeSentEv, fmt.Sprintf("BENCH%d.TO", i), log)
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

type mockExecutiveClient struct {
	executives []leadership.Executive
	err        error
}

func (m *mockExecutiveClient) GetExecutives(_ context.Context, _ string) ([]leadership.Executive, error) {
	return m.executives, m.err
}
