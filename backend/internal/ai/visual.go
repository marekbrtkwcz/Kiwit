package ai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kiwit/kiwit/internal/attachment"
	"github.com/kiwit/kiwit/internal/auth"
	"github.com/kiwit/kiwit/internal/llm"
	"github.com/kiwit/kiwit/internal/storage"
	"github.com/kiwit/kiwit/internal/visual"
)

// maxVisionImageBytes caps each image sent to the model.
const maxVisionImageBytes = 8 << 20 // 8 MB

type visualVerdictPayload struct {
	ComparisonID int64 `json:"comparison_id"`
	ServerID     int64 `json:"server_id,omitempty"`
}

// RegisterVisualRoutes mounts POST /api/v1/ai/visual-verdict — an LLM with
// vision judges a visual-regression diff as a real defect or an acceptable
// change, cutting false positives of pixel comparison.
func RegisterVisualRoutes(api huma.API, servers *llm.ServerStore, vstore *visual.Store, attRepo attachment.Repository, st storage.Storage, guard *auth.AccessGuard) {
	huma.Register(api, huma.Operation{
		OperationID: "ai-visual-verdict",
		Method:      http.MethodPost,
		Path:        "/api/v1/ai/visual-verdict",
		Summary:     "LLM judges a visual comparison diff: real defect or acceptable change",
		Tags:        []string{"AI"},
	}, func(ctx context.Context, input *struct {
		Body visualVerdictPayload
	}) (*struct {
		Body struct {
			Verdict      string  `json:"verdict" enum:"real-defect,acceptable-change,unclear"`
			Confidence   int     `json:"confidence"`
			Explanation  string  `json:"explanation"`
			DiffPercent  float64 `json:"diff_percent"`
			PixelVerdict string  `json:"pixel_verdict"`
			Model        string  `json:"model"`
		}
	}, error) {
		if err := auth.CheckWriteRole(ctx, "admin", "tester"); err != nil {
			return nil, err
		}
		cfg, _, err := resolve(ctx, servers, input.Body.ServerID)
		if err != nil {
			return nil, err
		}
		comp, err := vstore.GetComparison(ctx, input.Body.ComparisonID)
		if err != nil {
			return nil, huma.Error404NotFound("comparison not found")
		}
		if err := guard.EnforceExecution(ctx, comp.ExecutionID, false); err != nil {
			return nil, err
		}
		if comp.BaselineID == 0 {
			return nil, huma.Error400BadRequest("comparison has no baseline to judge")
		}
		base, err := vstore.GetBaseline(ctx, comp.BaselineID)
		if err != nil {
			return nil, huma.Error404NotFound("baseline not found")
		}
		readAtt := func(attID int64) (attachment.Attachment, []byte, error) {
			a, err := attRepo.GetByID(ctx, attID)
			if err != nil {
				return attachment.Attachment{}, nil, err
			}
			rc, err := st.Open(ctx, a.StoragePath)
			if err != nil {
				return a, nil, err
			}
			defer func() { _ = rc.Close() }()
			data, err := visual.ReadAll(rc)
			if err != nil {
				return a, nil, err
			}
			return a, data, nil
		}
		baseAtt, baseBytes, err := readAtt(base.AttachmentID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to read baseline image", err)
		}
		actAtt, actBytes, err := readAtt(comp.AttachmentID)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to read actual image", err)
		}
		if len(baseBytes) > maxVisionImageBytes || len(actBytes) > maxVisionImageBytes {
			return nil, huma.Error400BadRequest("image too large for AI verdict (max 8 MB each)")
		}
		toDataURL := func(a attachment.Attachment, data []byte) llm.VisionImage {
			ct := a.ContentType
			if ct == "" || !strings.HasPrefix(ct, "image/") {
				ct = "image/png"
			}
			return llm.VisionImage{DataURL: "data:" + ct + ";base64," + base64.StdEncoding.EncodeToString(data)}
		}
		sys := `You are a visual regression testing judge. You receive two screenshots of the same UI: the baseline (expected) and the actual (current). The pixel-diff reported a mismatch. Decide whether the difference is a REAL DEFECT (broken layout, missing element, overlapping text, wrong content, corrupted rendering) or an ACCEPTABLE CHANGE (dynamic content such as timestamps, counters, avatars, ads; font rendering shifts; minor anti-aliasing; intentional data variation). Respond ONLY with JSON: {"verdict": "real-defect" | "acceptable-change" | "unclear", "confidence": 0-100, "explanation": "one or two sentences"}.`
		user := fmt.Sprintf("Baseline image is the first image. Actual (current) image is the second. Reported pixel difference: %.2f%%. Baseline label: %s.", comp.DiffPercent, base.Label)
		raw, err := cfg.ChatVision(ctx, sys, user, []llm.VisionImage{toDataURL(baseAtt, baseBytes), toDataURL(actAtt, actBytes)})
		if err != nil {
			return nil, huma.Error502BadGateway(err.Error())
		}
		var parsed struct {
			Verdict     string `json:"verdict"`
			Confidence  int    `json:"confidence"`
			Explanation string `json:"explanation"`
		}
		if err := json.Unmarshal([]byte(llm.ExtractJSON(raw)), &parsed); err != nil {
			return nil, huma.Error502BadGateway("LLM output not JSON: " + err.Error())
		}
		parsed.Verdict = strings.ToLower(strings.TrimSpace(parsed.Verdict))
		switch parsed.Verdict {
		case "real-defect", "acceptable-change", "unclear":
		default:
			parsed.Verdict = "unclear"
		}
		verdictJSON, _ := json.Marshal(parsed)
		if err := vstore.UpdateAIVerdict(ctx, comp.ID, string(verdictJSON)); err != nil {
			return nil, huma.Error500InternalServerError("failed to persist AI verdict", err)
		}
		return &struct {
			Body struct {
				Verdict      string  `json:"verdict" enum:"real-defect,acceptable-change,unclear"`
				Confidence   int     `json:"confidence"`
				Explanation  string  `json:"explanation"`
				DiffPercent  float64 `json:"diff_percent"`
				PixelVerdict string  `json:"pixel_verdict"`
				Model        string  `json:"model"`
			}
		}{Body: struct {
			Verdict      string  `json:"verdict" enum:"real-defect,acceptable-change,unclear"`
			Confidence   int     `json:"confidence"`
			Explanation  string  `json:"explanation"`
			DiffPercent  float64 `json:"diff_percent"`
			PixelVerdict string  `json:"pixel_verdict"`
			Model        string  `json:"model"`
		}{Verdict: parsed.Verdict, Confidence: parsed.Confidence, Explanation: parsed.Explanation, DiffPercent: comp.DiffPercent, PixelVerdict: comp.Verdict, Model: cfg.Model}}, nil
	})
}
