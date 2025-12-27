package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type GeminiClient struct {
	apiKey     string
	httpClient *http.Client
	model      string
	apiURL     string
}

type geminiRequest struct {
	Model       string          `json:"model"`
	Messages    []geminiMessage `json:"messages"`
	Temperature float64         `json:"temperature,omitempty"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
}

type geminiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type geminiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func NewGeminiClient(apiKey string) *GeminiClient {
	return &GeminiClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		model:  "anthropic/claude-3.5-sonnet", // OpenRouter model name
		apiURL: "https://openrouter.ai/api/v1/chat/completions",
	}
}

func (c *GeminiClient) Generate(prompt string) (string, error) {
	reqBody := geminiRequest{
		Model: c.model,
		Messages: []geminiMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 0.7,
		MaxTokens:   2048,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", c.apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("HTTP-Referer", "https://browser-agent.com")
	req.Header.Set("X-Title", "Browser Agent")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	var geminiResp geminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	// Check for API errors
	if geminiResp.Error != nil {
		return "", fmt.Errorf("API returned error: %s", geminiResp.Error.Message)
	}

	if len(geminiResp.Choices) == 0 || geminiResp.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("no response generated")
	}

	return geminiResp.Choices[0].Message.Content, nil
}