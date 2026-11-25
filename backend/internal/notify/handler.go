package notify

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
)

func RegisterRoutes(api huma.API, client *WebhookClient) {
	huma.Register(api, huma.Operation{
		OperationID: "send-test-notification",
		Method:      http.MethodPost,
		Path:        "/api/v1/notifications/test",
		Summary:     "Send a test webhook notification (Slack / Teams)",
		Tags:        []string{"Notifications"},
	}, func(ctx context.Context, input *struct {
		Body struct {
			Message string `json:"message" minLength:"1"`
		}
	}) (*struct {
		Body struct {
			Sent bool `json:"sent"`
		}
	}, error) {
		if !client.Enabled() {
			return nil, huma.Error400BadRequest("KIWIT_WEBHOOK_URL is not configured")
		}
		if err := client.Send(ctx, input.Body.Message); err != nil {
			return nil, huma.Error502BadGateway("failed to deliver webhook", err)
		}
		out := &struct {
			Body struct {
				Sent bool `json:"sent"`
			}
		}{}
		out.Body.Sent = true
		return out, nil
	})
}
