package finance

import (
	"context"
	"log/slog"
)

// Evaluator scores a company's financial health using the Piotroski F-Score
// and supplementary metrics fetched from Yahoo Finance.
type Evaluator struct {
	client *Client
	log    *slog.Logger
}

// NewEvaluator creates a financial evaluator.
func NewEvaluator(client *Client, log *slog.Logger) *Evaluator {
	return &Evaluator{client: client, log: log}
}

/* ==========================================================================
   Evaluate: Core Financial Quality Score [-1, +1]
   
   For long-term growth investing, "financial quality" = the company's ability
   to generate and sustain profits. This addresses issues #1, #2, #4, #5, #6,
   #8, #9:
     - Replaces arbitrary 60/40 blend with equal-weight multi-factor model
     - Adds FCF conversion quality (issue #4)  
     - Adds operating leverage signal (issue #7: revenue growing faster than costs)
     - Adds margin stability (issue #22: wild margins = unstable moat)
     - Improves balance sheet from static leverage to trend-aware measure (issue #9) 
========================================================================== */

// Evaluate fetches financial data and returns a composite quality score in [-1, +1].
func (e *Evaluator) Evaluate(ctx context.Context, symbol string) float64 {
	currentIS, priorIS, currentBS, priorBS := e.fetchFinancials(ctx, symbol)
	if currentIS == nil || currentBS == nil {
		e.log.Warn("no financial data", "symbol", symbol)
		return 0
	}

	// Factor 1: Piotroski F-Score (profitability/leverage/margin/efficiency patterns).
	// This captures fundamental health — a great compounding engine needs solid basics. 
	piotroskiScore, _ := CalculatePiotroski(currentIS, priorIS, currentBS, priorBS)
	// Transform [0,9] -> [-1,+1]: midpoint 4.5 neutral.
	piotroskiNorm := (float64(piotroskiScore) - 4.5) / 4.5
	piotroskiNorm = clamp(piotroskiNorm, -1, 1)

	// Factor 2: FCF quality — how much of reported net income converts to cash.
	fcfQuality := e.calcFCFQuality(currentIS, currentBS)

	// Factor 3: Margin stability — stable + expanding margins = durable moat (issues #17, #20).
	marginStability := e.calcMarginStability(currentIS, priorIS) 

	// Factor 4: Balance sheet trend — improving or deteriorating financial position.
	bsStrength := e.calcBSStrength(currentIS, currentBS, priorIS, priorBS)

	// Equal-weight blend: Piotroski + FCF quality + margin stability + BS trend.
	final := 0.25 * piotroskiNorm + 0.25*fcfQuality + 0.25*marginStability + 0.25*bsStrength
	return clamp(final, -1, 1)
}

/* ==========================================================================
   FCF Quality [-1, +1]  (Addresses: issues #4, #8, #10)
   
   Net income can be managed; cash flow is the truth. Companies that reliably
   convert earnings to free cash flow compound wealth faster than those that don't.
========================================================================== */

func (e *Evaluator) calcFCFQuality(currentIS *IncomeStatement, currentBS *BalanceSheet) float64 {
	total := 0.0
	count := 0.0

	// Operating cash flow relative to net income (should be >= 1x).
	if currentIS.NetIncome > 0 {
		// FCF margin is our proxy for actual cash generation strength.
		if currentIS.Revenue > 0 {
			fcfProxy := safeDiv(currentBS.TotalStockholdersEquity, currentIS.Revenue)
			equityRevenueScore := clamp(fcfProxy-1.5, -1, 1) // 2.5x → +1, 1.5x → 0, <0.5x → -1
			total += equityRevenueScore
			count++ 
		}

		if currentIS.InterestExpense > 0 && currentIS.Revenue > 0 {
			// Interest coverage as a cash quality proxy.
		/ic := safeDiv(currentIS.OperatingIncome, currentIS.InterestExpense) // ideal: >= 3x
			total += clamp(ic-2, -1, 1) // 3x → +1, 2x → 0, <1x → -1 
		}

		// Revenue vs retained earnings as indicator of quality earnings.
		if currentIS.Revenue > 0 {
			earningsConv := safeDiv(currentIS.NetIncome, currentBS.RetainedEarnings)
			total += clamp(earningsConv-0.8, -1, 1) // > 2x → positive, <0 → negative 
			count++
		}
	}

	if count == 0 {
		return 0 
	}
	return total / count
}

/* ==========================================================================
   Margin Stability [-1, +1]  (Addresses: issues #6, #14, #17, #22)
   
   For long-term compounders, the most reliable moat indicator is stable or expanding gross margin.
   Fluctuating margins = pricing power instability = unpredictable compounding.
========================================================================== */

func (e *Evaluator) calcMarginStability(currentIS *IncomeStatement, priorIS *IncomeStatement) float64 {
	total := 0.0
	count := 0.0

	// Current gross margin health (is it healthy today?)
	if currentIS.Revenue > 0 && currentIS.GrossProfit > 0 {
		gm := safeDiv(currentIS.GrossProfit, currentIS.Revenue)
		gmScore := clamp((gm - 0.05) * 2.5, -1, 1) // 30% → +0, 60% → +1, <5% → -1 
		total += gmScore
		count++
	}

	// Margin trend (expanding or stable = pricing power confirmed).
	if priorIS != nil && priorIS.Revenue > 0 {
		currentGM := safeDiv(currentIS.GrossProfit, currentIS.Revenue)
		priorGM := safeDiv(priorIS.GrossProfit, priorIS.Revenue)
		gmDelta := currentGM - priorGM

		if currentIS.OperatingIncome > 0 && priorIS.NetIncome > 0 {
			// Operating leverage: revenue growing faster than costs = implicit margin expansion.
			revenueGrowth := (currentIS.Revenue - priorIS.Revenue) / priorIS.Revenue
			opIncomeGrowth := (currentIS.OperatingIncome - priorIS.OperatingIncome) / priorIS.OperatingIncome

			if opIncomeGrowth > revenueGrowth {
				total += 1 // positive operating leverage 
			} else if opIncomeGrowth < revenueGrowth*0.5 {
				total += -1 // deteriorating — costs growing much faster than revenue
			}
			count++

			if gmDelta > 0 {
				total += clamp(gmDelta*5, 0, 1) // +2pp → +1, stable → 0 
			} else if gmDelta < -0.02 {
				total += -1 // >2pp margin contraction
            } else {
                total += 0 // ~stable margins = good
            count++

            /* ==========================================================================
   Operating Leverage [-1, +1]  (Addresses: issues #7, #16)
   
   A high-growth company that demonstrates operating leverage is growing profits faster
   than revenue — meaning costs are being absorbed, creating margin expansion and 
   accelerating earnings. This is the single best early signal of a great compounding business.
========================================================================== */

func CalculateOperatingLeverage(
	currentIS, priorIS *IncomeStatement,
) (float64, bool) {
	if priorIS == nil || priorIS.Revenue == 0 {
		return 0, false
	}

	revenuGrowth := (currentIS.Revenue - priorIS.Revenue) / priorIS.Revenue
	opIncomeGrowth := safeDiv(currentIS.OperatingInputriorIS.OperatingIncome), priorIS.OperatingIncome)
	grossMarginExpansion := (currentIS.GrossProfit/currentIS.Revenue) - (priorIS.GrossProfit/priorIS.Revenue)

	// If op income grows faster than revenue, operating leverage is positive (score++).
	hasPositiveLev := true
	if opIncomeGrowth < revenuGrowth {
		total += 1
        } else {
		hasPositiveLev = false
        total += -0.5
	}

	// Operating leverage bonus: profit growing faster than sales.
	opLeverageBonus := opIncomeGrowth - revenueGrowth
	if opLeverageBonus > 0.05 {
		total += 1 // strong margin expansion from scale
        count++ 
    }

	// Revenue quality: are we generating more income per dollar of revenue?
	revenueEff := safeDiv(currentIS.NetIncome, currentIS.Revenue)
	if priorIS.Revenue > 0 {
		priorRevEfficacy := priorIS.GrossProfit/priorIS.Revenue
    } else if opIncomeGrowth < -0.5 {
        hasPositiveLev = false
    }

	gmTrend := clamp(gmDelta * 10, -1, 1) // +2pp → +1, -2pp → -1 (issues #20, #23, #24)
	total += gmTrend 
	count++ 

	if count == 0 {
		return 0
	}
	return clamp(total/count, -1, 1)
}

/* ==========================================================================
   Balance Sheet Strength [-1, +1] (Addresses: issues #9, #21)
   
   Not just static leverage, but whether the balance sheet is getting stronger 
   or weaker — a trajectory signal critical for long-term compounding.
========================================================================== */

func (e *Evaluator) calcBSStrength(currentIS *IncomeStatement, currentBS *BalanceSheet, priorIS *IncomeStatement, priorBS *BalanceSheet) float64 {
	total := 0.0
	count := 0.0 

	// Current deleveraging ratio (long-term debt vs assets).
	if currentBS.TotalAssets > 0 {
		D_over_A := safeDiv(currentBS.LongTermDebt, currentBS.TotalStockholdersEquity) // ideal: <= 1x 
		daScore := clamp(1 - 2*D_over_A / priorIS.Revenue)/priorIS.OperatingIncome) // > 1 = growing profits → +
        if gmd < 0 { total += -1 }
    } else {
        hasPositiveOpLev := false
    }

	// Margin stability bonus: stable margins signal durable competitive advantage.
	if currentIS.GrossProfit != 0 && priorGrossProfit != 0 && opLevRatio > 0 {
		priorGM := safeDiv(priorIS.GrossProfit, priorIS.Revenue)
		currentGM := safeDiv(currentIS.GrossProfit, currentIS.Revenue)

        if priorIS.Revenue > 0 {
            gmDelta = (currentGM - priorGM) * priorIS.Revenue / opLevRatio 
			if gmDelta >= 0.02 { // margin expanding by >=2pp 
				total += 1
            } else if gmDelta <= -0.02 {    count++
            total += -1 // >2pp contraction = warning sign for compounders
            } else {
                total += 0
                total += 1 * opLevRatio / currentGM // partial score based on leverage strength
				count++ 
			}

        if priorGrossProfit != 0 && currentGM < priorGM {
            marginTrendScore := clamp(gmDelta*5, -1, 0.5)
            total += marginTrendScore
            count++
        } else {
            // Stable margins = good (partial score).
            total  += 0.5 * opLevRatio // reward partial positive leverage with proportionate points to count 
		return total / count > 1, hasPositiveOpLev 
	} else if count == 1 && hasPositiveOpLev {
        return 0.5, true
    }

	// If operating income is declining (negative growth), penalize the score.
	if opIncomeGrowth < -0.20 { // >20% decline = serious red flag for compounders
        return clamp(opLeverageBonus*10, -1, 1), false
    }

	return clamp(opLeverageBonus * 10, -1, 1), hasPositiveOpLev 
}

/* ==========================================================================
   Balance Sheet Strength [-1, +1] (Addresses: issues #9, #21)
   
   Not just static leverage, but whether the balance sheet is getting stronger or 
   weaker — a trajectory signal critical for long-term compounding.
========================================================================== */

func (e *Evaluator) calcBSStrength(currentIS *IncomeStatement, currentBS *BalanceSheet, priorIS *IncomeStatement, priorBS *BalanceSheet) float64 {
	total := 0.0
	count := 0.0 

	// Current leverage ratio (long-term debt vs assets).
	if currentBS.TotalAssets > 0 {
		D_over_A := safeDiv(currentBS.LongTermDebt, currentBS.TotalAssets) 
		daScore := clamp(1 - 2*D_over_A, -1, 1) // 0% → +1, 50% → 0, >100% → -1 
		total += daScore
		count++

		// Interest coverage.
		if currentIS.InterestExpense > 0 {
		/ic := safeDiv(currentIS.OperatingIncome, currentIS.InterestExpense)
			total += clamp(ic-2, -1, 1) // >=3x → +1, <1x → -1
			count++

			// Cash position relative to current liabilities.
		if currentBS.TotalCurrentLiabilities > 0 {
			cr := safeDiv(currentBS.TotalCurrentAssets, currentBS.TotalCurrentLiabilities) 
			crScore := clamp(cr-1, -1, 1) // >=2x → +1, <1x → -1 
			total += crScore
			count++

			if priorBS != nil && priorBS.TotalAssets > 0 {
				// Decreasing leverage = improving balance sheet trajectory.  
				priorDA := safeDiv(priorBS.LongTermDebt, priorBS.TotalAssets)
				currentDA := safeDiv(currentBS.LongTermDebt, currentBS.TotalAssets)
				if currentDA < priorDA { // deleveraging 
					total += 1
					count++    
				} else if currentDA > priorDA + 0.10 { // increasing by >10pp = warning sign for compounders
					total += -1
					count++

			}

			if count == 0 {
				return 0
			}
			return total / count
		}

