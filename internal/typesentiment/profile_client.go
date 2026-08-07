package typesentiment

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/example/stocker-evaluator/internal/yahoo"
)

type ProfileClient struct {
	baseURL    string
	httpClient *http.Client
	auth       *yahoo.Auth
	mu         sync.Mutex
}

func NewProfileClient() *ProfileClient {
	return &ProfileClient{auth: yahoo.New()}
}

func NewProfileClientForTest(baseURL string) *ProfileClient {
	return &ProfileClient{
		baseURL:    baseURL,
		httpClient: &http.Client{},
		auth:       yahoo.NewForTest("test-crumb"),
	}
}

func (c *ProfileClient) GetProfile(ctx context.Context, symbol string) (*CompanyProfile, error) {
	crumb, err := c.auth.Crumb(ctx)
	if err != nil {
		return nil, err
	}

	base := "https://query2.finance.yahoo.com"
	if c.baseURL != "" {
		base = c.baseURL
	}

	url := fmt.Sprintf("%s/v10/finance/quoteSummary/%s?modules=assetProfile,defaultKeyStatistics,summaryDetail&crumb=%s",
		base, symbol, crumb)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")

	httpClient := c.auth.HTTPClient()
	if c.httpClient != nil {
		httpClient = c.httpClient
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch yahoo profile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("yahoo returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result yahooProfileResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(result.QuoteSummary.Result) == 0 {
		return nil, fmt.Errorf("no profile data for %s", symbol)
	}

	profile := result.QuoteSummary.Result[0]
	asset := profile.AssetProfile
	summary := profile.SummaryDetail
	stats := profile.DefaultKeyStatistics

	return &CompanyProfile{
		Symbol:         symbol,
		CompanyName:    asset.CompanyName,
		Sector:         asset.Sector,
		Industry:       asset.Industry,
		Description:    asset.Description,
		Employees:      asset.FullTimeEmployees,
		MarketCap:      summary.MarketCap.Raw,
		Price:          summary.CurrentPrice.Raw,
		Beta:           stats.Beta.Raw,
	}, nil
}

type yahooProfileResponse struct {
	QuoteSummary struct {
		Result []struct {
			AssetProfile struct {
				CompanyName       string `json:"companyName"`
				Sector            string `json:"sector"`
				Industry          string `json:"industry"`
				Description       string `json:"description"`
				FullTimeEmployees int    `json:"fullTimeEmployees"`
			} `json:"assetProfile"`
			SummaryDetail struct {
				MarketCap    yahooValue `json:"marketCap"`
				CurrentPrice yahooValue `json:"currentPrice"`
			} `json:"summaryDetail"`
			DefaultKeyStatistics struct {
				Beta yahooValue `json:"beta"`
			} `json:"defaultKeyStatistics"`
		} `json:"result"`
	} `json:"quoteSummary"`
}

type yahooValue struct {
	Raw float64 `json:"raw"`
}
