package finance

// CalculatePiotroski computes the Piotroski F-Score (0-9) from two years
// of income statement and balance sheet data.
//
// Requires current period and one prior period. If prior data is missing,
// only absolute signals (ROA, CFO) are used and improvement signals are
// scored as false.
func CalculatePiotroski(
	currentIS, priorIS *IncomeStatement,
	currentBS, priorBS *BalanceSheet,
) (int, PiotroskiDetail) {
	var d PiotroskiDetail

	if currentIS == nil || currentBS == nil {
		return 0, d
	}

	// --- Profitability signals ---
	totalAssetsBegin := currentBS.TotalAssets
	if priorBS != nil && priorBS.TotalAssets > 0 {
		totalAssetsBegin = priorBS.TotalAssets
	}

	roa := safeDiv(currentIS.NetIncome, totalAssetsBegin)
	d.PositiveROA = roa > 0

	// CFO signal requires cash flow data; use net income as proxy if unavailable.
	// (Caller should provide CFO via the CashFlowStatement when possible.)
	d.PositiveCFO = currentIS.NetIncome > 0

	if priorIS != nil {
		priorROA := safeDiv(priorIS.NetIncome, totalAssetsBegin)
		d.IncreasingROA = roa > priorROA
	}

	if d.PositiveCFO {
		d.QualityEarnings = roa > 0 && d.PositiveCFO
	}

	// --- Leverage & liquidity signals ---
	if priorBS != nil && priorBS.LongTermDebt > 0 {
		currentLever := currentBS.LongTermDebt / currentBS.TotalAssets
		priorLever := priorBS.LongTermDebt / priorBS.TotalAssets
		d.DecreasingLever = currentLever < priorLever
	} else {
		d.DecreasingLever = currentBS.LongTermDebt <= 0
	}

	if priorBS != nil && priorBS.TotalCurrentLiabilities > 0 {
		currentRatio := safeDiv(currentBS.TotalCurrentAssets, currentBS.TotalCurrentLiabilities)
		priorRatio := safeDiv(priorBS.TotalCurrentAssets, priorBS.TotalCurrentLiabilities)
		d.IncreasingLiquids = currentRatio > priorRatio
	} else {
		d.IncreasingLiquids = currentBS.TotalCurrentAssets > currentBS.TotalCurrentLiabilities
	}

	if priorBS != nil {
		d.NoDilution = currentBS.CommonStock <= priorBS.CommonStock
	} else {
		d.NoDilution = true
	}

	// --- Efficiency signals ---
	if priorIS != nil && priorIS.GrossProfit != 0 && priorIS.Revenue != 0 {
		currentMargin := safeDiv(currentIS.GrossProfit, currentIS.Revenue)
		priorMargin := safeDiv(priorIS.GrossProfit, priorIS.Revenue)
		d.IncreasingMargin = currentMargin > priorMargin
	}

	if priorBS != nil && priorBS.TotalAssets > 0 && priorIS != nil && priorIS.Revenue > 0 {
		currentTurnover := safeDiv(currentIS.Revenue, currentBS.TotalAssets)
		priorTurnover := safeDiv(priorIS.Revenue, priorBS.TotalAssets)
		d.IncreasingTurn = currentTurnover > priorTurnover
	}

	score := 0
	if d.PositiveROA {
		score++
	}
	if d.PositiveCFO {
		score++
	}
	if d.IncreasingROA {
		score++
	}
	if d.QualityEarnings {
		score++
	}
	if d.DecreasingLever {
		score++
	}
	if d.IncreasingLiquids {
		score++
	}
	if d.NoDilution {
		score++
	}
	if d.IncreasingMargin {
		score++
	}
	if d.IncreasingTurn {
		score++
	}

	return score, d
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}
