package sentiment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// LLMClient communicates with an Ollama-compatible LLM API.
type LLMClient struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewLLMClient creates a new LLM client for Ollama.
func NewLLMClient(baseURL, model string, timeout time.Duration) *LLMClient {
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	return &LLMClient{
		baseURL:    baseURL,
		model:      model,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// chatRequest represents the request to the LLM API.
type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Format   string        `json:"format,omitempty"`
	Stream   bool          `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatResponse represents the response from the LLM API.
type chatResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

// AnalyzeSentiment sends articles to the LLM and returns a sentiment score.
func (c *LLMClient) AnalyzeSentiment(ctx context.Context, symbol string, articlesText string) (*SentimentResult, error) {
	prompt := fmt.Sprintf(`You are a financial sentiment analyst. Analyze the sentiment of these news articles and social media posts about the stock symbol %s.

Based on the headlines and content provided, determine the overall sentiment.

Articles and posts:
%s

Respond ONLY with a JSON object in this exact format:
{"score": <number>, "reasoning": "<brief explanation>", "confidence": <number>}

Score must be between -1.0 and 1.0:
- -1.0 = extremely negative sentiment (lawsuits, bankruptcy, major losses, fraud)
- -0.5 = negative sentiment (missed earnings, declining sales, analyst downgrades)
- 0.0 = neutral/mixed sentiment
- 0.5 = positive sentiment (good earnings, growth, upgrades)
- 1.0 = extremely positive sentiment (record profits, major deals, breakthroughs)

Confidence must be between 0.0 and 1.0 based on how much data you had to work with.

Do not include any text before or after the JSON.`, symbol, articlesText)

	return c.callLLM(ctx, prompt)
}

// AnalyzeSectorSentiment sends a custom sector analysis prompt to the LLM.
func (c *LLMClient) AnalyzeSectorSentiment(ctx context.Context, prompt string) (*SentimentResult, error) {
	return c.callLLM(ctx, prompt)
}

// callLLM sends a prompt to the LLM and parses the sentiment response.
func (c *LLMClient) callLLM(ctx context.Context, prompt string) (*SentimentResult, error) {
	req := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "user", Content: prompt},
		},
		Stream: false,
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := c.baseURL + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call llm: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("llm returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var chatResp chatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return parseSentimentResponse(chatResp.Message.Content)
}

// parseSentimentResponse extracts the sentiment score from LLM output.
func parseSentimentResponse(content string) (*SentimentResult, error) {
	// Find JSON in the response (LLM might add extra text)
	start := -1
	end := -1
	for i, ch := range content {
		if ch == '{' && start == -1 {
			start = i
		}
		if ch == '}' {
			end = i + 1
			break
		}
	}

	if start == -1 || end == -1 {
		return nil, fmt.Errorf("no JSON found in response: %s", content)
	}

	jsonStr := content[start:end]
	var result SentimentResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, fmt.Errorf("parse sentiment json: %w", err)
	}

	result.Score = clamp(result.Score, -1.0, 1.0)
	result.Confidence = clamp(result.Confidence, 0.0, 1.0)

	return &result, nil
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
