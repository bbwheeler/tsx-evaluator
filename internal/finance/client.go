package finance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client fetches financial data from the Financial Modeling Prep API.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new FMP API client.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) fetchJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil // symbol not found, empty result
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("fmp api returned status %d: %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// GetIncomeStatement returns income statements for the given symbol.
// limit=2 gives current + prior year (needed for Piotroski).
func (c *Client) GetIncomeStatement(ctx context.Context, symbol string, limit int) ([]IncomeStatement, error) {
	url := fmt.Sprintf("%s/v3/income-statement/%s?limit=%d&apikey=%s",
		c.baseURL, symbol, limit, c.apiKey)

	var result []IncomeStatement
	if err := c.fetchJSON(ctx, url, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetBalanceSheet returns balance sheet statements for the given symbol.
func (c *Client) GetBalanceSheet(ctx context.Context, symbol string, limit int) ([]BalanceSheet, error) {
	url := fmt.Sprintf("%s/v3/balance-sheet-statement/%s?limit=%d&apikey=%s",
		c.baseURL, symbol, limit, c.apiKey)

	var result []BalanceSheet
	if err := c.fetchJSON(ctx, url, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetCashFlowStatement returns cash flow statements for the given symbol.
func (c *Client) GetCashFlowStatement(ctx context.Context, symbol string, limit int) ([]CashFlowStatement, error) {
	url := fmt.Sprintf("%s/v3/cash-flow-statement/%s?limit=%d&apikey=%s",
		c.baseURL, symbol, limit, c.apiKey)

	var result []CashFlowStatement
	if err := c.fetchJSON(ctx, url, &result); err != nil {
		return nil, err
	}
	return result, nil
}
