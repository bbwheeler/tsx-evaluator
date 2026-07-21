package analyzer

import (
	"math"
	"testing"
)

func TestAnalyze_Deterministic(t *testing.T) {
	s1 := Analyze("SHOP.TO")
	s2 := Analyze("SHOP.TO")

	if s1.Symbol != s2.Symbol {
		t.Fatalf("symbols differ: %q vs %q", s1.Symbol, s2.Symbol)
	}
	if s1.Financials != s2.Financials {
		t.Fatalf("financials differ: %v vs %v", s1.Financials, s2.Financials)
	}
	if s1.Sentiment != s2.Sentiment {
		t.Fatalf("sentiment differs: %v vs %v", s1.Sentiment, s2.Sentiment)
	}
	if s1.Leadership != s2.Leadership {
		t.Fatalf("leadership differs: %v vs %v", s1.Leadership, s2.Leadership)
	}
	if s1.TypeSentiment != s2.TypeSentiment {
		t.Fatalf("type_sentiment differs: %v vs %v", s1.TypeSentiment, s2.TypeSentiment)
	}
}

func TestAnalyze_ScoresInRange(t *testing.T) {
	symbols := []string{"SHOP.TO", "RY.TO", "TD.TO", "BNS.TO", "ENB.TO", "CNQ.TO", "SU.TO", "BAM.TO"}
	for _, sym := range symbols {
		s := Analyze(sym)
		for _, score := range []float64{s.Financials, s.Sentiment, s.Leadership, s.TypeSentiment} {
			if score < -1 || score > 1 {
				t.Errorf("Analyze(%q) score %v out of range [-1, 1]", sym, score)
			}
		}
	}
}

func TestAnalyze_SymbolPreserved(t *testing.T) {
	s := Analyze("BAM.TO")
	if s.Symbol != "BAM.TO" {
		t.Fatalf("expected symbol %q, got %q", "BAM.TO", s.Symbol)
	}
}

func TestAnalyze_DifferentSymbolsDifferentScores(t *testing.T) {
	s1 := Analyze("SHOP.TO")
	s2 := Analyze("RY.TO")

	same := s1.Financials == s2.Financials &&
		s1.Sentiment == s2.Sentiment &&
		s1.Leadership == s2.Leadership &&
		s1.TypeSentiment == s2.TypeSentiment

	if same {
		t.Fatal("expected different scores for different symbols")
	}
}

func TestAnalyze_ScoresAreRoundedToThreeDecimals(t *testing.T) {
	symbols := []string{"A", "BB", "CCC", "DDDD", "EEEEE"}
	for _, sym := range symbols {
		s := Analyze(sym)
		for _, score := range []float64{s.Financials, s.Sentiment, s.Leadership, s.TypeSentiment} {
			rounded := math.Round(score*1000) / 1000
			if score != rounded {
				t.Errorf("Analyze(%q) score %v is not rounded to 3 decimals", sym, score)
			}
		}
	}
}

func TestScoreFromSeed_KnownValues(t *testing.T) {
	// seed=0x01020304, offset=0 -> byte 0x04 -> 4/255 = 0.01568... -> 2*0.01568-1 = -0.96862... -> round to 3 decimals
	seed := uint32(0x01020304)
	v := scoreFromSeed(seed, 0)
	if v < -1 || v > 1 {
		t.Fatalf("scoreFromSeed out of range: %v", v)
	}

	// offset=1 -> byte 0x03
	v1 := scoreFromSeed(seed, 1)
	if v1 < -1 || v1 > 1 {
		t.Fatalf("scoreFromSeed out of range: %v", v1)
	}

	// Different offsets should give different values for this seed
	if v == v1 {
		t.Fatal("expected different values for different offsets")
	}
}

func TestScoreFromSeed_BoundaryZero(t *testing.T) {
	// All bytes are 0 -> each metric gets 0/255=0 -> 2*0-1 = -1
	v := scoreFromSeed(0x00000000, 0)
	if v != -1 {
		t.Fatalf("expected -1 for zero seed byte, got %v", v)
	}
}

func TestScoreFromSeed_BoundaryMax(t *testing.T) {
	// All bytes are 0xFF -> 255/255=1 -> 2*1-1 = 1
	v := scoreFromSeed(0xFFFFFFFF, 0)
	if v != 1 {
		t.Fatalf("expected 1 for max seed byte, got %v", v)
	}
}

func TestAnalyze_EmptyString(t *testing.T) {
	s := Analyze("")
	if s.Symbol != "" {
		t.Fatalf("expected empty symbol, got %q", s.Symbol)
	}
	for _, score := range []float64{s.Financials, s.Sentiment, s.Leadership, s.TypeSentiment} {
		if score < -1 || score > 1 {
			t.Errorf("empty symbol score out of range: %v", score)
		}
	}
}

func BenchmarkAnalyze(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Analyze("SHOP.TO")
	}
}
