package yahoo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"
	"time"
)

const consentURL = "https://guce.yahoo.com/consent"
const crumbURL = "https://query2.finance.yahoo.com/v1/test/getcrumb"

// Auth holds a Yahoo cookie jar and crumb for authenticated API requests.
type Auth struct {
	httpClient *http.Client
	mu         sync.Mutex
	crumb      string
}

// New creates an Auth that acquires cookies via the Yahoo consent page,
// then fetches a crumb. The crumb is cached for the process lifetime.
func New() *Auth {
	jar, _ := cookiejar.New(nil)
	return &Auth{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
			Jar:     jar,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return nil
			},
		},
	}
}

// NewForTest creates an Auth with a pre-set crumb, for use in tests.
func NewForTest(crumb string) *Auth {
	return &Auth{crumb: crumb}
}

// Crumb returns a valid Yahoo crumb, acquiring cookies and crumb on first call.
func (a *Auth) Crumb(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.crumb != "" {
		return a.crumb, nil
	}

	// Step 1: hit the consent page to acquire cookies (A1, A3, GUC, etc.)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, consentURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("yahoo consent: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("yahoo consent: status %d", resp.StatusCode)
	}

	// Step 2: get the crumb
	req, _ = http.NewRequestWithContext(ctx, http.MethodGet, crumbURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	resp, err = a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("yahoo crumb: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("yahoo crumb read: %w", err)
	}
	a.crumb = strings.TrimSpace(string(body))
	if a.crumb == "" {
		return "", fmt.Errorf("yahoo crumb: empty response")
	}
	return a.crumb, nil
}

// HTTPClient returns the underlying http.Client with cookies set.
func (a *Auth) HTTPClient() *http.Client {
	return a.httpClient
}
