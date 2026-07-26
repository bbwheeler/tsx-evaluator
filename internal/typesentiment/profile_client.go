package typesentiment

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"
	"time"
)

type ProfileClient struct {
	baseURL    string
	httpClient *http.Client
	mu         sync.Mutex
	crumb      string
}

func NewProfileClient() *ProfileClient {
	jar, _ := cookiejar.New(nil)
	return &ProfileClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Jar:     jar,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return nil
			},
		},
	}
}

func NewProfileClientForTest(baseURL string) *ProfileClient {
	return &ProfileClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		crumb: "test-crumb",
	}
}

func (c *ProfileClient) baseURLForRequest() string {
	if c.baseURL != "" {
		return c.baseURL
	}
	return "https://query2.finance.yahoo.com"
}

func (c *ProfileClient) ensureCrumb(ctx context.Context) error {
	if c.crumb != "" {
		return nil
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://finance.yahoo.com/", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; tsx-evaluator/1.0)")
	if _, err := c.httpClient.Do(req); err != nil {
		return fmt.Errorf("get yahoo cookies: %w", err)
	}

	req, _ = http.NewRequestWithContext(ctx, http.MethodGet, "https://query2.finance.yahoo.com/v1/test/getcrumb", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; tsx-evaluator/1.0)")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("get yahoo crumb: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read crumb: %w", err)
	}
	c.crumb = strings.TrimSpace(string(body))
	if c.crumb == "" {
		return fmt.Errorf("empty crumb from yahoo")
	}
	return nil
}

func (c *ProfileClient) GetProfile(ctx context.Context, symbol string) (*CompanyProfile, error) {
	c.mu.Lock()
	err := c.ensureCrumb(ctx)
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/v10/finance/quoteSummary/%s?modules=assetProfile,defaultKeyStatistics,summaryDetail&crumb=%s",
		c.baseURLForRequest(), symbol, c.crumb)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; tsx-evaluator/1.0)")

	resp, err := c.httpClient.Do(req)
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
				CompanyName      string `json:"companyName"`
				Sector           string `json:"sector"`
				Industry         string `json:"industry"`
				Description      string `json:"description"`
				FullTimeEmployees int   `json:"fullTimeEmployees"`
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
