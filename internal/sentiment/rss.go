package sentiment

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// YahooRSSClient fetches news from Yahoo Finance RSS feed.
type YahooRSSClient struct {
	httpClient *http.Client
}

// NewYahooRSSClient creates a new Yahoo Finance RSS client.
func NewYahooRSSClient() *YahooRSSClient {
	return &YahooRSSClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// rssFeed represents the RSS XML structure.
type rssFeed struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

// FetchArticles retrieves news articles for a symbol from Yahoo Finance RSS.
func (c *YahooRSSClient) FetchArticles(ctx context.Context, symbol string, limit int) ([]Article, error) {
	url := fmt.Sprintf("https://feeds.finance.yahoo.com/rss/2.0/headline?s=%s&region=US&lang=en-US", symbol)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch yahoo rss: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("yahoo rss returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var feed rssFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("parse rss: %w", err)
	}

	var articles []Article
	for i, item := range feed.Channel.Items {
		if i >= limit {
			break
		}

		pubTime, _ := time.Parse(time.RFC1123Z, item.PubDate)
		articles = append(articles, Article{
			Title:     item.Title,
			Snippet:   cleanHTML(item.Description),
			Source:    "Yahoo Finance",
			URL:       item.Link,
			Published: pubTime,
		})
	}

	return articles, nil
}

// cleanHTML removes HTML tags from a string.
func cleanHTML(s string) string {
	s = strings.ReplaceAll(s, "<p>", "")
	s = strings.ReplaceAll(s, "</p>", "")
	s = strings.ReplaceAll(s, "<br>", "")
	s = strings.ReplaceAll(s, "<br/>", "")
	s = strings.ReplaceAll(s, "<br />", "")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	return strings.TrimSpace(s)
}
