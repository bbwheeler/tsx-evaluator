package leadership

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"
)

// YahooClient fetches executive data from Yahoo Finance's quoteSummary API.
type YahooClient struct {
	httpClient *http.Client
	crumb      string
}

// NewYahooClient creates a Yahoo Finance client with cookie/crumb support.
func NewYahooClient() *YahooClient {
	jar, _ := cookiejar.New(nil)
	return &YahooClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Jar:     jar,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return nil
			},
		},
	}
}

func (c *YahooClient) ensureCrumb(ctx context.Context) error {
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

// GetExecutives returns key executives from Yahoo Finance's assetProfile.
func (c *YahooClient) GetExecutives(ctx context.Context, symbol string) ([]Executive, error) {
	if err := c.ensureCrumb(ctx); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://query2.finance.yahoo.com/v10/finance/quoteSummary/%s?modules=assetProfile&crumb=%s",
		symbol, c.crumb)

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

	var result yahooQuoteSummary
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(result.QuoteSummary.Result) == 0 {
		return nil, nil
	}

	officers := result.QuoteSummary.Result[0].AssetProfile.CompanyOfficers
	var executives []Executive
	for _, o := range officers {
		executives = append(executives, Executive{
			Name:        o.Name,
			Title:       o.Title,
			Age:         o.Age,
			YearOfBirth: o.YearOfBirth,
		})
	}
	return executives, nil
}

type yahooQuoteSummary struct {
	QuoteSummary struct {
		Result []struct {
			AssetProfile struct {
				CompanyOfficers []yahooOfficer `json:"companyOfficers"`
			} `json:"assetProfile"`
		} `json:"result"`
	} `json:"quoteSummary"`
}

type yahooOfficer struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Age         int    `json:"age"`
	YearOfBirth int    `json:"yearOfBirth"`
}
