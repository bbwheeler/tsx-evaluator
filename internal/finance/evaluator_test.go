package finance

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_GetIncomeStatement(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("symbol") != "SHOP.TO" {
			t.Errorf("expected symbol=SHOP.TO, got %q", r.URL.Query().Get("symbol"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]IncomeStatement{
			{
				Symbol: "SHOP.TO", Date: "2025-12-31", Period: "FY",
				Revenue: 3.5e9, NetIncome: 200e6, GrossProfit: 1.8e9,
			},
			{
				Symbol: "SHOP.TO", Date: "2024-12-31", Period: "FY",
				Revenue: 2.8e9, NetIncome: 150e6, GrossProfit: 1.4e9,
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
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
		json.NewEncoder(w).Encode([]BalanceSheet{
			{
				Symbol: "RY.TO", Date: "2025-10-31", Period: "FY",
				TotalAssets: 2e12, TotalCurrentAssets: 500e9,
				TotalLiabilities: 1.8e12, TotalCurrentLiabilities: 400e9,
				TotalStockholdersEquity: 200e9, LongTermDebt: 100e9,
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
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
		json.NewEncoder(w).Encode([]CashFlowStatement{
			{Symbol: "TD.TO", Date: "2025-10-31", Period: "FY", OperatingCashFlow: 50e9},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
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

func TestClient_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	stmts, err := client.GetIncomeStatement(context.Background(), "MISSING", 1)
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

	client := NewClient(server.URL, "test-key")
	_, err := client.GetIncomeStatement(context.Background(), "SHOP.TO", 1)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestClient_EmptyArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	stmts, err := client.GetIncomeStatement(context.Background(), "NOPE.TO", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stmts) != 0 {
		t.Errorf("expected empty result, got %d", len(stmts))
	}
}

func TestClient_IncludesAPIKey(t *testing.T) {
	var capturedKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedKey = r.URL.Query().Get("apikey")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "my-secret-key")
	_, _ = client.GetIncomeStatement(context.Background(), "TEST", 1)

	if capturedKey != "my-secret-key" {
		t.Errorf("API key: got %q, want %q", capturedKey, "my-secret-key")
	}
}

func TestEvaluator_Evaluate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		case contains(path, "income-statement"):
			json.NewEncoder(w).Encode([]IncomeStatement{
				{Symbol: "TEST", Revenue: 1e9, NetIncome: 100e6, GrossProfit: 400e6},
				{Symbol: "TEST", Revenue: 800e6, NetIncome: 80e6, GrossProfit: 300e6},
			})
		case contains(path, "balance-sheet-statement"):
			json.NewEncoder(w).Encode([]BalanceSheet{
				{
					Symbol: "TEST", TotalAssets: 2e9, TotalCurrentAssets: 1e9,
					TotalCurrentLiabilities: 500e6, TotalDebt: 200e6,
					LongTermDebt: 100e6, TotalStockholdersEquity: 1.5e9, CommonStock: 10e6,
				},
				{
					Symbol: "TEST", TotalAssets: 1.8e9, TotalCurrentAssets: 900e6,
					TotalCurrentLiabilities: 550e6, TotalDebt: 300e6,
					LongTermDebt: 200e6, TotalStockholdersEquity: 1.2e9, CommonStock: 10e6,
				},
			})
		default:
			w.Write([]byte("[]"))
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
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
		w.Write([]byte("[]"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
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

	client := NewClient(server.URL, "test-key")
	ev := NewEvaluator(client, slog.Default())
	score := ev.Evaluate(context.Background(), "ERR")

	if score != 0 {
		t.Errorf("expected 0 for API error, got %v", score)
	}
}

func TestEvaluator_ScoreBounds(t *testing.T) {
	// Test with extreme values that should still clamp to [-1, 1]
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		case contains(path, "income-statement"):
			json.NewEncoder(w).Encode([]IncomeStatement{
				{Symbol: "EXTREME", Revenue: 1e12, NetIncome: 500e9, GrossProfit: 800e9},
			})
		case contains(path, "balance-sheet-statement"):
			json.NewEncoder(w).Encode([]BalanceSheet{
				{Symbol: "EXTREME", TotalAssets: 1e12, TotalCurrentAssets: 500e9,
					TotalCurrentLiabilities: 1e9, TotalDebt: 0, TotalStockholdersEquity: 1e12},
			})
		default:
			w.Write([]byte("[]"))
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	ev := NewEvaluator(client, slog.Default())
	score := ev.Evaluate(context.Background(), "EXTREME")

	if score < -1 || score > 1 {
		t.Errorf("score out of bounds: %v", score)
	}
	t.Logf("extreme score: %.3f", score)
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
