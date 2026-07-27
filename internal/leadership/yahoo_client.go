package leadership

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/example/tsx-evaluator/internal/yahoo"
)

// YahooClient fetches executive data from Yahoo Finance's quoteSummary API.
type YahooClient struct {
	auth *yahoo.Auth
}

// NewYahooClient creates a Yahoo Finance client with cookie/crumb support.
func NewYahooClient() *YahooClient {
	return &YahooClient{auth: yahoo.New()}
}

// GetExecutives returns key executives from Yahoo Finance's assetProfile.
func (c *YahooClient) GetExecutives(ctx context.Context, symbol string) ([]Executive, error) {
	crumb, err := c.auth.Crumb(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://query2.finance.yahoo.com/v10/finance/quoteSummary/%s?modules=assetProfile&crumb=%s",
		symbol, crumb)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")

	resp, err := c.auth.HTTPClient().Do(req)
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
