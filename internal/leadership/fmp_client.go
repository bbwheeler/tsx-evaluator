package leadership

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// FMPClient fetches executive data from the Financial Modeling Prep API.
type FMPClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewFMPClient creates a new FMP client for leadership data.
func NewFMPClient(baseURL, apiKey string) *FMPClient {
	return &FMPClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *FMPClient) fetchJSON(ctx context.Context, url string, out any) error {
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
		return nil
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

// GetExecutives returns the list of key executives for a given symbol.
func (c *FMPClient) GetExecutives(ctx context.Context, symbol string) ([]Executive, error) {
	url := fmt.Sprintf("%s/stable/key-executives?symbol=%s&apikey=%s",
		c.baseURL, symbol, c.apiKey)

	var result []Executive
	if err := c.fetchJSON(ctx, url, &result); err != nil {
		return nil, err
	}
	return result, nil
}
