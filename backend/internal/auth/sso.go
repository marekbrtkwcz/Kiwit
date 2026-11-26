package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-chi/chi/v5"
	"golang.org/x/oauth2"
)

// SSOConfig holds OIDC provider settings.
type SSOConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	ProviderName string
	FrontendURL  string // where to redirect after successful login (e.g. http://localhost:8080)
}

type ssoState struct {
	state string
	nonce string
	exp   time.Time
}

// SSOHandler serves OIDC login/callback endpoints on a chi router.
type SSOHandler struct {
	cfg      SSOConfig
	provider *oidc.Provider
	oauth    oauth2.Config
	verifier *oidc.IDTokenVerifier
	svc      *Service
	mu       sync.Mutex
	states   map[string]ssoState
}

// NewSSOHandler initializes the OIDC provider and OAuth2 config. Returns nil if issuer is unreachable.
func NewSSOHandler(ctx context.Context, cfg SSOConfig, svc *Service) (*SSOHandler, error) {
	provider, err := oidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc provider: %w", err)
	}
	oauthCfg := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  cfg.RedirectURL,
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}
	return &SSOHandler{
		cfg:      cfg,
		provider: provider,
		oauth:    oauthCfg,
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		svc:      svc,
		states:   make(map[string]ssoState),
	}, nil
}

// Register mounts /api/v1/auth/sso/login and /api/v1/auth/sso/callback on the chi router.
func (h *SSOHandler) Register(r chi.Router) {
	r.Get("/api/v1/auth/sso/login", h.handleLogin)
	r.Get("/api/v1/auth/sso/callback", h.handleCallback)
	r.Get("/api/v1/auth/sso/config", h.handleConfig)
}

func (h *SSOHandler) handleConfig(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(fmt.Sprintf(`{"enabled":true,"provider":"%s","login_url":"/api/v1/auth/sso/login"}`, h.cfg.ProviderName)))
}

func (h *SSOHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	state, err := randomToken(16)
	if err != nil {
		http.Error(w, "failed to generate state", http.StatusInternalServerError)
		return
	}
	nonce, err := randomToken(16)
	if err != nil {
		http.Error(w, "failed to generate nonce", http.StatusInternalServerError)
		return
	}
	h.mu.Lock()
	h.states[state] = ssoState{state: state, nonce: nonce, exp: time.Now().Add(5 * time.Minute)}
	// Cleanup expired states opportunistically.
	for k, v := range h.states {
		if time.Now().After(v.exp) {
			delete(h.states, k)
		}
	}
	h.mu.Unlock()

	http.Redirect(w, r, h.oauth.AuthCodeURL(state, oidc.Nonce(nonce)), http.StatusFound)
}

func (h *SSOHandler) handleCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if e := q.Get("error"); e != "" {
		http.Error(w, "SSO error: "+e, http.StatusBadRequest)
		return
	}
	state := q.Get("state")
	code := q.Get("code")
	if state == "" || code == "" {
		http.Error(w, "missing state or code", http.StatusBadRequest)
		return
	}

	h.mu.Lock()
	st, ok := h.states[state]
	if ok {
		delete(h.states, state)
	}
	h.mu.Unlock()
	if !ok || time.Now().After(st.exp) {
		http.Error(w, "invalid or expired state", http.StatusBadRequest)
		return
	}

	token, err := h.oauth.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, "failed to exchange code", http.StatusBadGateway)
		return
	}
	rawID, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "no id_token in response", http.StatusBadGateway)
		return
	}
	idToken, err := h.verifier.Verify(r.Context(), rawID)
	if err != nil {
		http.Error(w, "failed to verify id_token", http.StatusUnauthorized)
		return
	}
	if idToken.Nonce != st.nonce {
		http.Error(w, "nonce mismatch", http.StatusUnauthorized)
		return
	}

	var claims struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Username string `json:"preferred_username"`
	}
	if err := idToken.Claims(&claims); err != nil {
		http.Error(w, "failed to parse claims", http.StatusInternalServerError)
		return
	}
	if claims.Email == "" {
		http.Error(w, "no email claim from provider", http.StatusBadRequest)
		return
	}

	user, err := h.svc.FindOrCreateSSOUser(r.Context(), claims.Email, claims.Username)
	if err != nil {
		http.Error(w, "failed to provision user", http.StatusInternalServerError)
		return
	}
	jwtToken, err := h.svc.IssueJWT(user)
	if err != nil {
		http.Error(w, "failed to issue session", http.StatusInternalServerError)
		return
	}

	// Redirect to frontend with token in fragment (not sent to server in logs).
	redirect := fmt.Sprintf("%s/#/auth/callback?token=%s", h.cfg.FrontendURL, url.QueryEscape(jwtToken))
	http.Redirect(w, r, redirect, http.StatusFound)
}

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
