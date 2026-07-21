package db

import (
	"strings"
	"testing"
)

func TestBuildCompositeExpr_AllNonZero(t *testing.T) {
	got := BuildCompositeExpr(0.3, 0.2, 0.4, 0.1)
	want := "(0.300000*financials + 0.200000*sentiment + 0.400000*leadership + 0.100000*type_sentiment)"
	if got != want {
		t.Errorf("BuildCompositeExpr:\n  got  %q\n  want %q", got, want)
	}
}

func TestBuildCompositeExpr_AllZero(t *testing.T) {
	got := BuildCompositeExpr(0, 0, 0, 0)
	want := "(0.000000*financials + 0.000000*sentiment + 0.000000*leadership + 0.000000*type_sentiment)"
	if got != want {
		t.Errorf("BuildCompositeExpr:\n  got  %q\n  want %q", got, want)
	}
}

func TestBuildCompositeExpr_ContainsAllColumns(t *testing.T) {
	got := BuildCompositeExpr(1, 0, 0, 0)
	for _, col := range []string{"financials", "sentiment", "leadership", "type_sentiment"} {
		if !strings.Contains(got, col) {
			t.Errorf("BuildCompositeExpr missing column %q in %q", col, got)
		}
	}
}

func TestBuildCompositeExpr_FormatsWeights(t *testing.T) {
	got := BuildCompositeExpr(0.123456, 0.0, 0.0, 0.0)
	if !strings.HasPrefix(got, "(0.123456*financials") {
		t.Errorf("expected weight to be formatted as 0.123456, got %q", got)
	}
}

func TestScoreMetricToColumn_AllMetrics(t *testing.T) {
	tests := []struct {
		input string
		col   string
		ok    bool
	}{
		{"FINANCIALS", "financials", true},
		{"SENTIMENT", "sentiment", true},
		{"LEADERSHIP", "leadership", true},
		{"TYPE_SENTIMENT", "type_sentiment", true},
		{"SCORE_METRIC_FINANCIALS", "financials", true},
		{"SCORE_METRIC_SENTIMENT", "sentiment", true},
		{"SCORE_METRIC_LEADERSHIP", "leadership", true},
		{"SCORE_METRIC_TYPE_SENTIMENT", "type_sentiment", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			col, ok := ScoreMetricToColumn(tt.input)
			if ok != tt.ok {
				t.Errorf("ScoreMetricToColumn(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if col != tt.col {
				t.Errorf("ScoreMetricToColumn(%q) = %q, want %q", tt.input, col, tt.col)
			}
		})
	}
}

func TestScoreMetricToColumn_CaseInsensitive(t *testing.T) {
	col, ok := ScoreMetricToColumn("financials")
	if !ok || col != "financials" {
		t.Errorf("ScoreMetricToColumn(\"financials\") = (%q, %v), want (\"financials\", true)", col, ok)
	}
	col, ok = ScoreMetricToColumn("Sentiment")
	if !ok || col != "sentiment" {
		t.Errorf("ScoreMetricToColumn(\"Sentiment\") = (%q, %v), want (\"sentiment\", true)", col, ok)
	}
}

func TestScoreMetricToColumn_Unknown(t *testing.T) {
	_, ok := ScoreMetricToColumn("UNKNOWN")
	if ok {
		t.Error("expected ok=false for unknown metric")
	}
}

func TestScoreMetricToColumn_Empty(t *testing.T) {
	_, ok := ScoreMetricToColumn("")
	if ok {
		t.Error("expected ok=false for empty metric")
	}
}

func TestErrNotFound_IsSentinel(t *testing.T) {
	if ErrNotFound == nil {
		t.Fatal("ErrNotFound should not be nil")
	}
	if ErrNotFound.Error() != "evaluation not found" {
		t.Errorf("ErrNotFound.Error() = %q, want %q", ErrNotFound.Error(), "evaluation not found")
	}
}

func TestInitSchemaSQL_NonEmpty(t *testing.T) {
	sql := InitSchemaSQL()
	if sql == "" {
		t.Fatal("InitSchemaSQL() returned empty string")
	}
}

func TestInitSchemaSQL_ContainsTable(t *testing.T) {
	sql := InitSchemaSQL()
	if !strings.Contains(sql, "CREATE TABLE") {
		t.Error("InitSchemaSQL() should contain CREATE TABLE")
	}
	if !strings.Contains(sql, "evaluations") {
		t.Error("InitSchemaSQL() should reference evaluations table")
	}
}

func TestInitSchemaSQL_ContainsColumns(t *testing.T) {
	sql := InitSchemaSQL()
	for _, col := range []string{"symbol", "financials", "sentiment", "leadership", "type_sentiment", "evaluated_at"} {
		if !strings.Contains(sql, col) {
			t.Errorf("InitSchemaSQL() missing column %q", col)
		}
	}
}
