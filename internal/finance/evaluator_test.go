package finance

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func tsVal(date string, val float64) tsEntry {
	e := tsEntry{AsOfDate: date, PeriodType: "12M"}
	e.ReportedValue.Raw = val
	return e
}

func makeTimeseriesResponse(entries map[string][]tsEntry) map[string]any {
	result := map[string]any{
		"timeseries": map[string]any{
			"result": []any{entries},
		},
	}
	return result
}

func TestClient_GetIncomeStatement(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(makeTimeseriesResponse(map[string][]tsEntry{
			"annualTotalRevenue": {tsVal("2025-12-31", 3.5e9), tsVal("2024-12-31", 2.8e9)},
			"annualGrossProfit":  {tsVal("2025-12-31", 1.8e9), tsVal("2024-12-31", 1.4e9)},
			"annualNetIncome":    {tsVal("2025-12-31", 200e6), tsVal("2024-12-31", 150e6)},
		}))
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.URL)
	stmts, err := client.GetIncomeStatement(context.Background(), "SHOP.TO", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(stmts))
	}
	if stmts[0].Revenue != 3.5e9 {
		t.Errorf("revenue: got %v, want 3.5e9", stmts[0].Revenue)
	}
}

func TestClient_GetBalanceSheet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(makeTimeseriesResponse(map[string][]tsEntry{
			"annualTotalAssets":                      {tsVal("2025-10-31", 2e12)},
			"annualCurrentAssets":                    {tsVal("2025-10-31", 500e9)},
			"annualTotalLiabilitiesNetMinorityInterest": {tsVal("2025-10-31", 1.8e12)},
			"annualCurrentLiabilities":               {tsVal("2025-10-31", 400e9)},
			"annualStockholdersEquity":               {tsVal("2025-10-31", 200e9)},
			"annualLongTermDebt":                     {tsVal("2025-10-31", 100e9)},
		}))
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.URL)
	sheets, err := client.GetBalanceSheet(context.Background(), "RY.TO", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sheets) != 1 {
		t.Fatalf("expected 1 sheet, got %d", len(sheets))
	}
	if sheets[0].TotalAssets != 2e12 {
		t.Errorf("total assets: got %v, want 2e12", sheets[0].TotalAssets)
	}
}

func TestClient_GetCashFlowStatement(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(makeTimeseriesResponse(map[string][]tsEntry{
			"annualOperatingCashFlow": {tsVal("2025-10-31", 50e9)},
		}))
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.URL)
	sheets, err := client.GetCashFlowStatement(context.Background(), "TD.TO", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sheets) != 1 {
		t.Fatalf("expected 1 sheet, got %d", len(sheets))
	}
	if sheets[0].OperatingCashFlow != 50e9 {
		t.Errorf("operating cash flow: got %v, want 50e9", sheets[0].OperatingCashFlow)
	}
}

func TestClient_EmptyTimeseries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"timeseries": map[string]any{"result": []map[string]any{}},
		})
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.URL)
	stmts, err := client.GetIncomeStatement(context.Background(), "NOPE.TO", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stmts) != 0 {
		t.Errorf("expected empty result, got %d", len(stmts))
	}
}

func TestClient_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.URL)
	_, err := client.GetIncomeStatement(context.Background(), "SHOP.TO", 1)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestEvaluator_Evaluate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(makeTimeseriesResponse(map[string][]tsEntry{
			"annualTotalRevenue":    {tsVal("2024-12-31", 1e9), tsVal("2023-12-31", 800e6)},
			"annualGrossProfit":     {tsVal("2024-12-31", 400e6), tsVal("2023-12-31", 300e6)},
			"annualNetIncome":       {tsVal("2024-12-31", 100e6), tsVal("2023-12-31", 80e6)},
			"annualTotalAssets":     {tsVal("2024-12-31", 2e9), tsVal("2023-12-31", 1.8e9)},
			"annualCurrentAssets":   {tsVal("2024-12-31", 1e9), tsVal("2023-12-31", 900e6)},
			"annualCurrentLiabilities": {tsVal("2024-12-31", 500e6), tsVal("2023-12-31", 550e6)},
			"annualStockholdersEquity": {tsVal("2024-12-31", 1.5e9), tsVal("2023-12-31", 1.2e9)},
			"annualCommonStock":     {tsVal("2024-12-31", 10e6), tsVal("2023-12-31", 10e6)},
			"annualTotalDebt":       {tsVal("2024-12-31", 200e6)},
			"annualLongTermDebt":    {tsVal("2024-12-31", 100e6), tsVal("2023-12-31", 200e6)},
		}))
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.URL)
	ev := NewEvaluator(client, slog.Default())
	score := ev.Evaluate(context.Background(), "TEST")

	if score < -1 || score > 1 {
		t.Errorf("score out of range: %v", score)
	}
	t.Logf("financial score for TEST: %.3f", score)
}

func TestEvaluator_NoData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"timeseries": map[string]any{"result": []map[string]any{}},
		})
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.URL)
	ev := NewEvaluator(client, slog.Default())
	score := ev.Evaluate(context.Background(), "MISSING")

	if score != 0 {
		t.Errorf("expected 0 for missing data, got %v", score)
	}
}

func TestEvaluator_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.URL)
	ev := NewEvaluator(client, slog.Default())
	score := ev.Evaluate(context.Background(), "ERR")

	if score != 0 {
		t.Errorf("expected 0 for API error, got %v", score)
	}
}

func TestEvaluator_ScoreBounds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(makeTimeseriesResponse(map[string][]tsEntry{
			"annualTotalRevenue":       {tsVal("2024-12-31", 1e12)},
			"annualGrossProfit":        {tsVal("2024-12-31", 800e9)},
			"annualNetIncome":          {tsVal("2024-12-31", 500e9)},
			"annualTotalAssets":        {tsVal("2024-12-31", 1e12)},
			"annualCurrentAssets":      {tsVal("2024-12-31", 500e9)},
			"annualCurrentLiabilities": {tsVal("2024-12-31", 1e9)},
			"annualStockholdersEquity": {tsVal("2024-12-31", 1e12)},
		}))
	}))
	defer server.Close()

	client := NewClientWithBaseURL(server.URL)
	ev := NewEvaluator(client, slog.Default())
	score := ev.Evaluate(context.Background(), "EXTREME")

	if score < -1 || score > 1 {
		t.Errorf("score out of bounds: %v", score)
	}
	t.Logf("extreme score: %.3f", score)
}
