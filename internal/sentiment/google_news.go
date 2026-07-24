package sentiment

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GoogleNewsClient fetches news from Google News RSS feed.
type GoogleNewsClient struct {
	httpClient *http.Client
	locale     string
}

// NewGoogleNewsClient creates a new Google News RSS client.
// locale defaults to "en-US" if empty.
func NewGoogleNewsClient(locale string) *GoogleNewsClient {
	if locale == "" {
		locale = "en-US"
	}
	return &GoogleNewsClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		locale:     locale,
	}
}

// googleRSSFeed represents the Google News RSS XML structure.
type googleRSSFeed struct {
	Channel googleRSSChannel `xml:"channel"`
}

type googleRSSChannel struct {
	Items []googleRSSItem `xml:"item"`
}

type googleRSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Source      string `xml:"source"`
}

// FetchArticles retrieves news articles for a symbol from Google News RSS.
func (c *GoogleNewsClient) FetchArticles(ctx context.Context, symbol string, limit int) ([]Article, error) {
	query := fmt.Sprintf("%s stock when:7d", symbol)
	encodedQuery := url.QueryEscape(query)

	// Parse locale to extract gl and ceid
	parts := strings.Split(c.locale, "-")
	gl := "US"
	ceid := "US:en"
	if len(parts) == 2 {
		gl = strings.ToUpper(parts[1])
		ceid = gl + ":" + parts[0]
	}

	feedURL := fmt.Sprintf(
		"https://news.google.com/rss/search?q=%s&hl=%s&gl=%s&ceid=%s",
		encodedQuery, c.locale, gl, ceid,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch google news rss: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("google news rss returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var feed googleRSSFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("parse rss: %w", err)
	}

	var articles []Article
	for i, item := range feed.Channel.Items {
		if i >= limit {
			break
		}

		pubTime, _ := time.Parse(time.RFC1123Z, item.PubDate)
		source := item.Source
		if source == "" {
			source = "Google News"
		}

		articles = append(articles, Article{
			Title:     item.Title,
			Snippet:   cleanHTML(item.Description),
			Source:    source,
			URL:       item.Link,
			Published: pubTime,
		})
	}

	return articles, nil
}
