package finance

import (
	"testing"
)

func TestCalculatePiotroski_StrongCompany(t *testing.T) {
	currentIS := &IncomeStatement{
		Revenue:         100e6,
		NetIncome:       15e6,
		GrossProfit:     40e6,
		GrossProfitRatio: 0.4,
	}
	priorIS := &IncomeStatement{
		Revenue:         80e6,
		NetIncome:       10e6,
		GrossProfit:     30e6,
		GrossProfitRatio: 0.375,
	}
	currentBS := &BalanceSheet{
		TotalAssets:              200e6,
		TotalCurrentAssets:       100e6,
		TotalLiabilities:         80e6,
		TotalCurrentLiabilities:  50e6,
		TotalDebt:                30e6,
		LongTermDebt:             20e6,
		TotalStockholdersEquity:  120e6,
		CommonStock:              10e6,
	}
	priorBS := &BalanceSheet{
		TotalAssets:              180e6,
		TotalCurrentAssets:       90e6,
		TotalLiabilities:         85e6,
		TotalCurrentLiabilities:  55e6,
		TotalDebt:                40e6,
		LongTermDebt:             30e6,
		TotalStockholdersEquity:  95e6,
		CommonStock:              10e6,
	}

	score, detail := CalculatePiotroski(currentIS, priorIS, currentBS, priorBS)

	if score < 7 {
		t.Errorf("expected high Piotroski score for a strong company, got %d", score)
	}
	t.Logf("Piotroski score: %d/9", score)
	t.Logf("Detail: %+v", detail)
}

func TestCalculatePiotroski_WeakCompany(t *testing.T) {
	currentIS := &IncomeStatement{
		Revenue:         50e6,
		NetIncome:       -10e6,
		GrossProfit:     10e6,
		GrossProfitRatio: 0.2,
	}
	priorIS := &IncomeStatement{
		Revenue:         70e6,
		NetIncome:       5e6,
		GrossProfit:     25e6,
		GrossProfitRatio: 0.357,
	}
	currentBS := &BalanceSheet{
		TotalAssets:              300e6,
		TotalCurrentAssets:       60e6,
		TotalLiabilities:         250e6,
		TotalCurrentLiabilities:  150e6,
		TotalDebt:                200e6,
		LongTermDebt:             180e6,
		TotalStockholdersEquity:  50e6,
		CommonStock:              15e6,
	}
	priorBS := &BalanceSheet{
		TotalAssets:              250e6,
		TotalCurrentAssets:       70e6,
		TotalLiabilities:         180e6,
		TotalCurrentLiabilities:  100e6,
		TotalDebt:                120e6,
		LongTermDebt:             100e6,
		TotalStockholdersEquity:  70e6,
		CommonStock:              12e6,
	}

	score, _ := CalculatePiotroski(currentIS, priorIS, currentBS, priorBS)

	if score > 3 {
		t.Errorf("expected low Piotroski score for a weak company, got %d", score)
	}
	t.Logf("Piotroski score: %d/9", score)
}

func TestCalculatePiotroski_NilCurrent(t *testing.T) {
	score, _ := CalculatePiotroski(nil, nil, nil, nil)
	if score != 0 {
		t.Errorf("expected 0 for nil inputs, got %d", score)
	}
}

func TestCalculatePiotroski_OnlyCurrentData(t *testing.T) {
	currentIS := &IncomeStatement{
		Revenue:         100e6,
		NetIncome:       20e6,
		GrossProfit:     40e6,
		GrossProfitRatio: 0.4,
	}
	currentBS := &BalanceSheet{
		TotalAssets:              200e6,
		TotalCurrentAssets:       120e6,
		TotalCurrentLiabilities:  40e6,
		TotalDebt:                0,
		LongTermDebt:             0,
		TotalStockholdersEquity:  200e6,
		CommonStock:              10e6,
	}

	score, detail := CalculatePiotroski(currentIS, nil, currentBS, nil)
	t.Logf("score=%d detail=%+v", score, detail)

	if !detail.PositiveROA {
		t.Error("expected PositiveROA=true")
	}
	if !detail.NoDilution {
		t.Error("expected NoDilution=true with no prior data")
	}
}

func TestCalculatePiotroski_NoPriorAssets(t *testing.T) {
	currentIS := &IncomeStatement{
		Revenue:         100e6,
		NetIncome:       10e6,
		GrossProfit:     30e6,
		GrossProfitRatio: 0.3,
	}
	currentBS := &BalanceSheet{
		TotalAssets:              150e6,
		TotalCurrentAssets:       80e6,
		TotalCurrentLiabilities:  40e6,
		TotalDebt:                20e6,
		LongTermDebt:             10e6,
		TotalStockholdersEquity:  130e6,
		CommonStock:              5e6,
	}

	score, _ := CalculatePiotroski(currentIS, nil, currentBS, nil)
	t.Logf("score=%d", score)
	if score < 1 {
		t.Error("expected at least 1 point for a profitable company")
	}
}

func TestSafeDiv(t *testing.T) {
	if v := safeDiv(10, 2); v != 5 {
		t.Errorf("safeDiv(10,2) = %v, want 5", v)
	}
	if v := safeDiv(10, 0); v != 0 {
		t.Errorf("safeDiv(10,0) = %v, want 0", v)
	}
}

func TestPiotroskiScoreRange(t *testing.T) {
	is := &IncomeStatement{Revenue: 100, NetIncome: 10, GrossProfit: 30}
	bs := &BalanceSheet{
		TotalAssets: 200, TotalCurrentAssets: 100,
		TotalCurrentLiabilities: 50, TotalStockholdersEquity: 150,
	}
	score, _ := CalculatePiotroski(is, is, bs, bs)
	if score < 0 || score > 9 {
		t.Errorf("Piotroski score out of range: %d", score)
	}
}
