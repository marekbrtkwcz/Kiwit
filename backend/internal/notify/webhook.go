package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WebhookClient sends JSON notifications (compatible with Slack, Teams, and generic webhooks).
type WebhookClient struct {
	url        string
	httpClient *http.Client
}

func NewWebhookClient(webhookURL string) *WebhookClient {
	return &WebhookClient{
		url:        webhookURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *WebhookClient) Enabled() bool {
	return c != nil && c.url != ""
}

// Send posts a message payload to the configured webhook URL.
func (c *WebhookClient) Send(ctx context.Context, text string) error {
	if !c.Enabled() {
		return nil
	}
	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode >= 400 {
		return fmt.Errorf("webhook returned status %d", res.StatusCode)
	}
	return nil
}
