// Package captcha provides a self-hosted, no-external-dependency captcha
// mechanism for the registration endpoint. It combines three lightweight
// techniques:
//
//  1. Math challenge — GET /api/v1/auth/captcha returns a captcha_id and a
//     human-readable question ("Ile to 7 + 3?"). The answer hash is kept
//     in memory with a TTL. The client submits captcha_id + captcha_answer
//     with the registration.
//  2. Honeypot — the registration form has a hidden "website" field; if it
//     is non-empty the request is silently rejected as a bot.
//  3. Time-trap — the captcha carries an issued-at timestamp; registration
//     must arrive at least minHumanSeconds later (bots submit instantly).
package captcha

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// Store keeps issued challenges in memory. Safe for concurrent use.
type Store struct {
	mu         sync.Mutex
	challenges map[string]challenge
	ttl        time.Duration
	minHuman   time.Duration
}

type challenge struct {
	answerHash string
	issuedAt   time.Time
	expiresAt  time.Time
}

// NewStore creates a captcha store with the given challenge TTL and minimum
// human response time. A minHuman <= 0 means "no time-trap" (useful for tests).
func NewStore(ttl, minHuman time.Duration) *Store {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &Store{
		challenges: map[string]challenge{},
		ttl:        ttl,
		minHuman:   minHuman,
	}
}

// Challenge is the public representation returned by GET /api/v1/auth/captcha.
type Challenge struct {
	CaptchaID string `json:"captcha_id"`
	Question  string `json:"question"`
	ExpiresIn int    `json:"expires_in"` // seconds
}

// Issue creates a new math challenge and returns its public form.
func (s *Store) Issue() Challenge {
	a, _ := rand.Int(rand.Reader, big.NewInt(9))
	b, _ := rand.Int(rand.Reader, big.NewInt(9))
	answer := a.Int64() + b.Int64()

	raw := make([]byte, 16)
	_, _ = rand.Read(raw)
	id := hex.EncodeToString(raw)

	h := sha256.Sum256([]byte(fmt.Sprintf("%d", answer)))
	now := time.Now()

	s.mu.Lock()
	s.challenges[id] = challenge{
		answerHash: hex.EncodeToString(h[:]),
		issuedAt:   now,
		expiresAt:  now.Add(s.ttl),
	}
	s.mu.Unlock()

	return Challenge{
		CaptchaID: id,
		Question:  fmt.Sprintf("Ile to %d + %d?", a.Int64(), b.Int64()),
		ExpiresIn: int(s.ttl.Seconds()),
	}
}

// Verify checks a captcha answer. Returns nil on success, an error otherwise.
// The challenge is single-use: it is deleted on any verification attempt.
func (s *Store) Verify(captchaID, answer string) error {
	if captchaID == "" {
		return fmt.Errorf("captcha_id required")
	}
	s.mu.Lock()
	ch, ok := s.challenges[captchaID]
	if ok {
		delete(s.challenges, captchaID)
	}
	s.mu.Unlock()

	if !ok {
		return fmt.Errorf("unknown or expired captcha")
	}
	if time.Now().After(ch.expiresAt) {
		return fmt.Errorf("captcha expired")
	}
	h := sha256.Sum256([]byte(answer))
	if hex.EncodeToString(h[:]) != ch.answerHash {
		return fmt.Errorf("wrong captcha answer")
	}
	if time.Since(ch.issuedAt) < s.minHuman {
		return fmt.Errorf("submitted too fast — are you a bot?")
	}
	return nil
}

// IsHoneypotTripped returns true if the hidden honeypot field is non-empty.
func IsHoneypotTripped(website string) bool {
	return website != ""
}

// RegisterRoutes mounts GET /api/v1/auth/captcha on the huma API.
func RegisterRoutes(api huma.API, store *Store) {
	huma.Register(api, huma.Operation{
		OperationID: "auth-captcha",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/captcha",
		Summary:     "Get a math captcha challenge for registration",
		Tags:        []string{"Auth"},
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body Challenge
	}, error) {
		return &struct{ Body Challenge }{Body: store.Issue()}, nil
	})
}
