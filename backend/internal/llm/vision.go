package llm

import (
	"context"
	"fmt"
)

// VisionImage is one image passed to a vision-capable model, as a data URL
// ("data:image/png;base64,...").
type VisionImage struct {
	DataURL string
}

type visionTextPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type visionImageURL struct {
	URL string `json:"url"`
}

type visionImagePart struct {
	Type     string         `json:"type"`
	ImageURL visionImageURL `json:"image_url"`
}

// ChatVision sends a system+user prompt with attached images to a
// vision-capable OpenAI-compatible model.
func (c Config) ChatVision(ctx context.Context, system, user string, images []VisionImage) (string, error) {
	if len(images) == 0 {
		return "", fmt.Errorf("no images provided")
	}
	content := make([]any, 0, len(images)+1)
	content = append(content, visionTextPart{Type: "text", Text: user})
	for _, img := range images {
		if img.DataURL == "" {
			continue
		}
		content = append(content, visionImagePart{Type: "image_url", ImageURL: visionImageURL{URL: img.DataURL}})
	}
	return c.post(ctx, []chatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: content},
	})
}
