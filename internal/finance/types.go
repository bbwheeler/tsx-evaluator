package finance

type IncomeStatement struct {
	Symbol             string  `json:"symbol"`
	Date               string  `json:"date"`
	Period             string  `json:"period"`
	Revenue            float64 `json:"revenue"`
	CostOfRevenue      float64 `json:"costOfRevenue"`
	GrossProfit        float64 `json:"grossProfit"`
	GrossProfitRatio   float64 `json:"grossProfitRatio"`
	OperatingIncome    float64 `json:"operatingIncome"`
	NetIncome          float64 `json:"netIncome"`
	NetIncomeRatio     float64 `json:"netIncomeRatio"`
	Ebitda             float64 `json:"ebitda"`
	InterestExpense    float64 `json:"interestExpense"`
	IncomeTaxExpense   float64 `json:"incomeTaxExpense"`
}

type BalanceSheet struct {
	Symbol                          string  `json:"symbol"`
	Date                            string  `json:"date"`
	Period                          string  `json:"period"`
	TotalAssets                     float64 `json:"totalAssets"`
	TotalCurrentAssets              float64 `json:"totalCurrentAssets"`
	TotalLiabilities                float64 `json:"totalLiabilities"`
	TotalCurrentLiabilities         float64 `json:"totalCurrentLiabilities"`
	TotalDebt                       float64 `json:"totalDebt"`
	LongTermDebt                    float64 `json:"longTermDebt"`
	ShortTermDebt                   float64 `json:"shortTermDebt"`
	TotalStockholdersEquity         float64 `json:"totalStockholdersEquity"`
	CommonStock                     float64 `json:"commonStock"`
	RetainedEarnings                float64 `json:"retainedEarnings"`
	CashAndCashEquivalents          float64 `json:"cashAndCashEquivalents"`
	NetReceivables                  float64 `json:"netReceivables"`
	Inventory                       float64 `json:"inventory"`
}

type CashFlowStatement struct {
	Symbol                          string  `json:"symbol"`
	Date                            string  `json:"date"`
	Period                          string  `json:"period"`
	OperatingCashFlow               float64 `json:"operatingCashFlow"`
	FreeCashFlow                    float64 `json:"freeCashFlow"`
	NetIncome                       float64 `json:"netIncome"`
	DividendsPaid                   float64 `json:"dividendsPaid"`
}

type PiotroskiDetail struct {
	PositiveROA       bool
	PositiveCFO       bool
	IncreasingROA     bool
	QualityEarnings   bool
	DecreasingLever   bool
	IncreasingLiquids bool
	NoDilution        bool
	IncreasingMargin  bool
	IncreasingTurn    bool
}
