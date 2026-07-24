package sentiment

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// RedditClient fetches posts from Reddit's public JSON API.
type RedditClient struct {
	httpClient *http.Client
	userAgent  string
	subreddits []string
}

// NewRedditClient creates a new Reddit JSON client.
// subreddits defaults to wallstreetbets, stocks, investing if empty.
func NewRedditClient(subreddits []string) *RedditClient {
	if len(subreddits) == 0 {
		subreddits = []string{"wallstreetbets", "stocks", "investing"}
	}
	return &RedditClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		userAgent:  "tsx-evaluator/1.0",
		subreddits: subreddits,
	}
}

// redditResponse represents the Reddit JSON API response structure.
type redditResponse struct {
	Data redditData `json:"data"`
}

type redditData struct {
	Children []redditChild `json:"children"`
}

type redditChild struct {
	Data redditPost `json:"data"`
}

type redditPost struct {
	Title        string  `json:"title"`
	SelfText     string  `json:"selftext"`
	Author       string  `json:"author"`
	Score        int     `json:"score"`
	NumComments  int     `json:"num_comments"`
	CreatedUTC   float64 `json:"created_utc"`
	URL          string  `json:"url"`
	Permalink    string  `json:"permalink"`
	Subreddit    string  `json:"subreddit"`
	UpvoteRatio  float64 `json:"upvote_ratio"`
}

// FetchArticles retrieves posts for a symbol from Reddit subreddits.
func (c *RedditClient) FetchArticles(ctx context.Context, symbol string, limit int) ([]Article, error) {
	var articles []Article
	limitPerSub := limit / len(c.subreddits)
	if limitPerSub < 1 {
		limitPerSub = 1
	}

	for _, sub := range c.subreddits {
		subArticles, err := c.fetchSubreddit(ctx, sub, symbol, limitPerSub)
		if err != nil {
			// Log error but continue with other subreddits
			continue
		}
		articles = append(articles, subArticles...)
	}

	return articles, nil
}

func (c *RedditClient) fetchSubreddit(ctx context.Context, subreddit, symbol string, limit int) ([]Article, error) {
	searchURL := fmt.Sprintf(
		"https://www.reddit.com/r/%s/search.json?q=%s&sort=new&limit=%d&restrict_sr=1",
		subreddit, symbol, limit,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch reddit: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("reddit rate limited")
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("reddit returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var response redditResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}

	var articles []Article
	for _, child := range response.Data.Children {
		post := child.Data
		pubTime := time.Unix(int64(post.CreatedUTC), 0)

		// Combine title and selftext for better sentiment analysis
		content := post.Title
		if post.SelfText != "" {
			// Truncate long selftext to save LLM tokens
			selfText := post.SelfText
			if len(selfText) > 500 {
				selfText = selfText[:500] + "..."
			}
			content = post.Title + "\n\n" + selfText
		}

		articles = append(articles, Article{
			Title:     post.Title,
			Snippet:   content,
			Source:    fmt.Sprintf("r/%s", subreddit),
			URL:       "https://www.reddit.com" + post.Permalink,
			Published: pubTime,
			Score:     post.Score,
		})
	}

	return articles, nil
}

// FormatArticlesForLLM formats articles into a string for the LLM prompt.
func FormatArticlesForLLM(articles []Article) string {
	var sb strings.Builder
	for i, a := range articles {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, a.Source, a.Title))
		if a.Snippet != "" && a.Snippet != a.Title {
			// Truncate snippet to save tokens
			snippet := a.Snippet
			if len(snippet) > 200 {
				snippet = snippet[:200] + "..."
			}
			sb.WriteString(fmt.Sprintf("   %s\n", snippet))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}
