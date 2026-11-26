package web

import (
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5/middleware"
)

// reqIDError extends huma.ErrorModel with the request ID so API clients can
// correlate an error response with the server log entry.
type reqIDError struct {
	huma.ErrorModel
	RequestID string `json:"request_id,omitempty"`
}

func (e *reqIDError) Error() string  { return e.ErrorModel.Error() }
func (e *reqIDError) GetStatus() int { return e.ErrorModel.GetStatus() }

// RequestIDMiddleware echoes the chi request ID back to the client in the
// X-Request-Id response header (or the client-supplied value, if given).
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := middleware.GetReqID(r.Context()); id != "" {
			w.Header().Set(middleware.RequestIDHeader, id)
		}
		next.ServeHTTP(w, r)
	})
}

// ErrorRequestIDTransformer is a huma response transformer that injects the
// request ID into every API error body (validation errors, handler errors
// and unexpected 5xx alike). Attach it via huma.Config.Transformers.
func ErrorRequestIDTransformer(ctx huma.Context, _ string, v any) (any, error) {
	em, ok := v.(*huma.ErrorModel)
	if !ok {
		return v, nil
	}
	return &reqIDError{
		ErrorModel: *em,
		RequestID:  middleware.GetReqID(ctx.Context()),
	}, nil
}
