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
	APIKey  string
	BaseURL string
	Model   string
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
		APIKey:  os.Getenv("KIWIT_LLM_API_KEY"),
		BaseURL: strings.TrimRight(base, "/"),
		Model:   model,
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
		Temperature: 0.3,
		MaxTokens:   2000,
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
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("LLM request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM returned HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 300))
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

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
