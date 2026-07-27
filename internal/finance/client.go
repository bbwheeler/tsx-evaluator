package finance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/cookiejar"
	"sort"
	"strings"
	"sync"
	"time"
)

const yahooTimeseriesURL = "https://query2.finance.yahoo.com/ws/fundamentals-timeseries/v1/finance/timeseries/%s"

const timeseriesTypeKeys = "annualTotalRevenue,annualGrossProfit,annualOperatingIncome,annualNetIncome," +
	"annualCostOfRevenue,annualInterestExpense,annualTaxProvision,annualEBITDA," +
	"annualTotalAssets,annualCurrentAssets,annualCurrentLiabilities," +
	"annualStockholdersEquity,annualCommonStock,annualLongTermDebt," +
	"annualRetainedEarnings,annualTotalDebt,annualTotalLiabilitiesNetMinorityInterest," +
	"annualCashAndCashEquivalents,annualAccountsReceivable,annualInventory," +
	"annualOperatingCashFlow,annualFreeCashFlow,annualCashDividendsPaid"

type Client struct {
	baseURL    string
	httpClient *http.Client
	mu         sync.Mutex
	cache      map[string]cachedFinancials
}

type cachedFinancials struct {
	income    []IncomeStatement
	balance   []BalanceSheet
	cashflow  []CashFlowStatement
	fetchedAt time.Time
}

func NewClient() *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return nil
			},
		},
		cache: make(map[string]cachedFinancials),
	}
}

func NewClientWithBaseURL(baseURL string) *Client {
	jar, _ := cookiejar.New(nil)
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return nil
			},
		},
		cache: make(map[string]cachedFinancials),
	}
}

func (c *Client) fetchFinancials(ctx context.Context, symbol string) (
	[]IncomeStatement, []BalanceSheet, []CashFlowStatement, error,
) {
	c.mu.Lock()
	if cached, ok := c.cache[symbol]; ok && time.Since(cached.fetchedAt) < 10*time.Minute {
		c.mu.Unlock()
		return cached.income, cached.balance, cached.cashflow, nil
	}
	c.mu.Unlock()

	now := time.Now()
	period1 := now.AddDate(-4, 0, 0).Unix()
	period2 := now.Unix()

	url := fmt.Sprintf(yahooTimeseriesURL, symbol)
	if c.baseURL != "" {
		url = fmt.Sprintf("%s/ws/fundamentals-timeseries/v1/finance/timeseries/%s", c.baseURL, symbol)
	}
	url += fmt.Sprintf("?type=%s&period1=%d&period2=%d&merge=false&padTimeSeries=true&lang=en-US&region=US",
		timeseriesTypeKeys, period1, period2)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; tsx-evaluator/1.0)")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("fetch timeseries: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, nil, nil, fmt.Errorf("yahoo returned status %d: %s", resp.StatusCode, string(body))
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read response: %w", err)
	}

	income, balance, cashflow, err := parseTimeseries(raw, symbol)
	if err != nil {
		return nil, nil, nil, err
	}

	c.mu.Lock()
	c.cache[symbol] = cachedFinancials{income: income, balance: balance, cashflow: cashflow, fetchedAt: now}
	c.mu.Unlock()

	return income, balance, cashflow, nil
}

func (c *Client) GetIncomeStatement(ctx context.Context, symbol string, limit int) ([]IncomeStatement, error) {
	income, _, _, err := c.fetchFinancials(ctx, symbol)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(income) > limit {
		income = income[:limit]
	}
	return income, nil
}

func (c *Client) GetBalanceSheet(ctx context.Context, symbol string, limit int) ([]BalanceSheet, error) {
	_, balance, _, err := c.fetchFinancials(ctx, symbol)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(balance) > limit {
		balance = balance[:limit]
	}
	return balance, nil
}

func (c *Client) GetCashFlowStatement(ctx context.Context, symbol string, limit int) ([]CashFlowStatement, error) {
	_, _, cashflow, err := c.fetchFinancials(ctx, symbol)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(cashflow) > limit {
		cashflow = cashflow[:limit]
	}
	return cashflow, nil
}

type yahooTimeseriesResponse struct {
	Timeseries struct {
		Result []map[string]json.RawMessage `json:"result"`
	} `json:"timeseries"`
}

type tsEntry struct {
	AsOfDate      string `json:"asOfDate"`
	PeriodType    string `json:"periodType"`
	ReportedValue struct {
		Raw float64 `json:"raw"`
	} `json:"reportedValue"`
}

func parseTimeseries(raw []byte, symbol string) ([]IncomeStatement, []BalanceSheet, []CashFlowStatement, error) {
	var resp yahooTimeseriesResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, nil, nil, fmt.Errorf("decode timeseries: %w", err)
	}

	if len(resp.Timeseries.Result) == 0 {
		return nil, nil, nil, nil
	}

	entries := make(map[string][]tsEntry)
	for _, result := range resp.Timeseries.Result {
		for key, val := range result {
			if key == "meta" || key == "timestamp" {
				continue
			}
			if !strings.HasPrefix(key, "annual") {
				continue
			}
			var arr []tsEntry
			if err := json.Unmarshal(val, &arr); err != nil {
				continue
			}
			baseKey := strings.TrimPrefix(key, "annual")
			entries[baseKey] = arr
		}
	}

	dates := collectDates(entries)
	sort.Slice(dates, func(i, j int) bool {
		return dates[i] > dates[j]
	})

	incomeMap := make(map[string]*IncomeStatement)
	balanceMap := make(map[string]*BalanceSheet)
	cashflowMap := make(map[string]*CashFlowStatement)

	for _, date := range dates {
		incomeMap[date] = &IncomeStatement{Symbol: symbol, Date: date}
		balanceMap[date] = &BalanceSheet{Symbol: symbol, Date: date}
		cashflowMap[date] = &CashFlowStatement{Symbol: symbol, Date: date}
	}

	assignIS := func(field string, setFn func(*IncomeStatement, float64)) {
		for _, e := range entries[field] {
			if is, ok := incomeMap[e.AsOfDate]; ok {
				setFn(is, e.ReportedValue.Raw)
			}
		}
	}
	assignBS := func(field string, setFn func(*BalanceSheet, float64)) {
		for _, e := range entries[field] {
			if bs, ok := balanceMap[e.AsOfDate]; ok {
				setFn(bs, e.ReportedValue.Raw)
			}
		}
	}
	assignCF := func(field string, setFn func(*CashFlowStatement, float64)) {
		for _, e := range entries[field] {
			if cf, ok := cashflowMap[e.AsOfDate]; ok {
				setFn(cf, e.ReportedValue.Raw)
			}
		}
	}

	assignIS("TotalRevenue", func(is *IncomeStatement, v float64) { is.Revenue = v })
	assignIS("CostOfRevenue", func(is *IncomeStatement, v float64) { is.CostOfRevenue = v })
	assignIS("GrossProfit", func(is *IncomeStatement, v float64) { is.GrossProfit = v })
	assignIS("OperatingIncome", func(is *IncomeStatement, v float64) { is.OperatingIncome = v })
	assignIS("NetIncome", func(is *IncomeStatement, v float64) { is.NetIncome = v })
	assignIS("EBITDA", func(is *IncomeStatement, v float64) { is.Ebitda = v })
	assignIS("InterestExpense", func(is *IncomeStatement, v float64) { is.InterestExpense = v })
	assignIS("TaxProvision", func(is *IncomeStatement, v float64) { is.IncomeTaxExpense = v })

	assignBS("TotalAssets", func(bs *BalanceSheet, v float64) { bs.TotalAssets = v })
	assignBS("CurrentAssets", func(bs *BalanceSheet, v float64) { bs.TotalCurrentAssets = v })
	assignBS("TotalLiabilitiesNetMinorityInterest", func(bs *BalanceSheet, v float64) { bs.TotalLiabilities = v })
	assignBS("CurrentLiabilities", func(bs *BalanceSheet, v float64) { bs.TotalCurrentLiabilities = v })
	assignBS("TotalDebt", func(bs *BalanceSheet, v float64) { bs.TotalDebt = v })
	assignBS("LongTermDebt", func(bs *BalanceSheet, v float64) { bs.LongTermDebt = v })
	assignBS("StockholdersEquity", func(bs *BalanceSheet, v float64) { bs.TotalStockholdersEquity = v })
	assignBS("CommonStock", func(bs *BalanceSheet, v float64) { bs.CommonStock = v })
	assignBS("RetainedEarnings", func(bs *BalanceSheet, v float64) { bs.RetainedEarnings = v })
	assignBS("CashAndCashEquivalents", func(bs *BalanceSheet, v float64) { bs.CashAndCashEquivalents = v })
	assignBS("AccountsReceivable", func(bs *BalanceSheet, v float64) { bs.NetReceivables = v })
	assignBS("Inventory", func(bs *BalanceSheet, v float64) { bs.Inventory = v })

	assignCF("OperatingCashFlow", func(cf *CashFlowStatement, v float64) { cf.OperatingCashFlow = v })
	assignCF("FreeCashFlow", func(cf *CashFlowStatement, v float64) { cf.FreeCashFlow = v })
	assignCF("CashDividendsPaid", func(cf *CashFlowStatement, v float64) { cf.DividendsPaid = math.Abs(v) })

	var income []IncomeStatement
	for _, date := range dates {
		is := incomeMap[date]
		if is.Revenue == 0 && is.NetIncome == 0 {
			continue
		}
		income = append(income, *is)
	}

	var balance []BalanceSheet
	for _, date := range dates {
		bs := balanceMap[date]
		if bs.TotalAssets == 0 {
			continue
		}
		balance = append(balance, *bs)
	}

	var cashflow []CashFlowStatement
	for _, date := range dates {
		cf := cashflowMap[date]
		if cf.OperatingCashFlow == 0 && cf.FreeCashFlow == 0 {
			continue
		}
		cashflow = append(cashflow, *cf)
	}

	return income, balance, cashflow, nil
}

func collectDates(entries map[string][]tsEntry) []string {
	seen := make(map[string]struct{})
	for _, arr := range entries {
		for _, e := range arr {
			seen[e.AsOfDate] = struct{}{}
		}
	}
	dates := make([]string, 0, len(seen))
	for d := range seen {
		dates = append(dates, d)
	}
	return dates
}
