package typesentiment

// CompanyProfile holds sector/industry data from FMP profile API.
type CompanyProfile struct {
	Symbol         string  `json:"symbol"`
	CompanyName    string  `json:"companyName"`
	Sector         string  `json:"sector"`
	Industry       string  `json:"industry"`
	Description    string  `json:"description"`
	MarketCap      float64 `json:"mktCap"`
	Price          float64 `json:"price"`
	Beta           float64 `json:"beta"`
	Exchange       string  `json:"exchange"`
	Currency       string  `json:"currency"`
	ISIN           string  `json:"isin"`
	CIK            string  `json:"cik"`
	Employees      int     `json:"fullTimeEmployees"`
	IPODate        string  `json:"ipoDate"`
	DCFDiff        float64 `json:"dcfDiff"`
	DCFFairValue   float64 `json:"dcf"`
	RevenueTTM     float64 `json:"revenue"`
	GrossProfitTTM float64 `json:"grossProfits"`
	EBITDA         float64 `json:"ebitda"`
	NetIncomeTTM   float64 `json:"netIncome"`
	FinalPrice     float64 `json:"final"`
}

// TypeSentimentResult holds the final type sentiment score.
type TypeSentimentResult struct {
	Score      float64
	Sector     string
	Industry   string
	Reasoning  string
	Confidence float64
}
