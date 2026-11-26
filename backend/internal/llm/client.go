package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Config is read from environment:
//   - KIWIT_LLM_API_KEY   — provider API key (empty = disabled)
//   - KIWIT_LLM_BASE_URL  — OpenAI-compatible base, default https://api.openai.com/v1 (Ollama: http://localhost:11434/v1)
//   - KIWIT_LLM_MODEL     — model name, default gpt-4o-mini
type Config struct {
	APIKey      string
	BaseURL     string
	Model       string
	Temperature float64
	MaxTokens   int
	TimeoutSec  int
	Retries     int
}

func LoadConfig() Config {
	base := os.Getenv("KIWIT_LLM_BASE_URL")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	model := os.Getenv("KIWIT_LLM_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}
	return Config{
		APIKey:      os.Getenv("KIWIT_LLM_API_KEY"),
		BaseURL:     strings.TrimRight(base, "/"),
		Model:       model,
		Temperature: 0.3,
		MaxTokens:   2000,
		TimeoutSec:  60,
		Retries:     1,
	}
}

func (c Config) Enabled() bool { return c.APIKey != "" }

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Chat sends a system+user prompt to the configured LLM and returns the text.
func (c Config) Chat(ctx context.Context, system, user string) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("LLM not configured (set KIWIT_LLM_API_KEY)")
	}
	body, err := json.Marshal(chatRequest{
		Model: c.Model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature: c.Temperature,
		MaxTokens:   c.MaxTokens,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	timeout := time.Duration(c.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	retries := c.Retries
	if retries < 1 {
		retries = 1
	}
	client := &http.Client{Timeout: timeout}
	var lastErr error
	for attempt := 0; attempt < retries; attempt++ {
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("LLM request failed: %w", err)
			continue
		}
		respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		_ = resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("LLM returned HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 300))
			continue
		}
		var cr chatResponse
		if err := json.Unmarshal(respBody, &cr); err != nil {
			return "", fmt.Errorf("invalid LLM response: %w", err)
		}
		if cr.Error != nil {
			return "", fmt.Errorf("LLM error: %s", cr.Error.Message)
		}
		if len(cr.Choices) == 0 {
			return "", fmt.Errorf("LLM returned no choices")
		}
		return cr.Choices[0].Message.Content, nil
	}
	return "", lastErr
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
