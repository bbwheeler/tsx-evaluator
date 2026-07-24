package typesentiment

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ProfileClient fetches company profile data from the FMP API.
type ProfileClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewProfileClient creates a new FMP profile client.
func NewProfileClient(baseURL, apiKey string) *ProfileClient {
	return &ProfileClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetProfile returns the company profile for a symbol, including sector and industry.
func (c *ProfileClient) GetProfile(ctx context.Context, symbol string) (*CompanyProfile, error) {
	url := fmt.Sprintf("%s/stable/profile?symbol=%s&apikey=%s",
		c.baseURL, symbol, c.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("profile not found for %s", symbol)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fmp api returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	// FMP returns an array for profile endpoints
	var results []CompanyProfile
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no profile data for %s", symbol)
	}

	return &results[0], nil
}
