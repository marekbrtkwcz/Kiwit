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

	"log/slog"

	"github.com/kiwit/kiwit/internal/platform/secrets"
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
	Content any    `json:"content"` // string, or a vision content array
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
	return c.post(ctx, []chatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	})
}

// post sends the messages to the chat/completions endpoint with retry.
// Every call is logged (model, duration, outcome) without prompt or response
// content; the API key is registered in the secret vault for log redaction.
func (c Config) post(ctx context.Context, messages []chatMessage) (string, error) {
	start := time.Now()
	var callErr error
	defer func() {
		if callErr != nil {
			slog.Warn("llm call", "model", c.Model, "duration_ms", time.Since(start).Milliseconds(), "ok", false, "err", callErr.Error())
		} else {
			slog.Info("llm call", "model", c.Model, "duration_ms", time.Since(start).Milliseconds(), "ok", true)
		}
	}()
	if !c.Enabled() {
		callErr = fmt.Errorf("LLM not configured (set KIWIT_LLM_API_KEY)")
		return "", callErr
	}
	secrets.DefaultVault().Register(c.APIKey)
	body, err := json.Marshal(chatRequest{
		Model:       c.Model,
		Messages:    messages,
		Temperature: c.Temperature,
		MaxTokens:   c.MaxTokens,
	})
	if err != nil {
		callErr = err
		return "", callErr
	}
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
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
		if err != nil {
			callErr = err
			return "", callErr
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
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
			callErr = fmt.Errorf("invalid LLM response: %w", err)
			return "", callErr
		}
		if cr.Error != nil {
			callErr = fmt.Errorf("LLM error: %s", cr.Error.Message)
			return "", callErr
		}
		if len(cr.Choices) == 0 {
			callErr = fmt.Errorf("LLM returned no choices")
			return "", callErr
		}
		return cr.Choices[0].Message.Content, nil
	}
	callErr = lastErr
	return "", callErr
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
