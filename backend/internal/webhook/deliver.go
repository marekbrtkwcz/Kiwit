package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/kiwit/kiwit/internal/platform/secrets"
)

// Payload is the JSON body delivered to every webhook target.
type Payload struct {
	Event      string         `json:"event"`
	EntityType string         `json:"entity_type,omitempty"`
	EntityID   int64          `json:"entity_id,omitempty"`
	Title      string         `json:"title"`
	Body       string         `json:"body,omitempty"`
	Data       map[string]any `json:"data,omitempty"` // event-specific extra fields (e.g. bug.created: tracker, url, reporter)
	Timestamp  string         `json:"timestamp"`
}

// Sign returns the X-Kiwit-Signature header value: "t=<unix>,v1=<hex>" where
// hex is the HMAC-SHA256 of "<unix>.<body>" using the webhook's secret
// (GitHub-style timestamped signature, which enables replay protection). An
// empty secret disables signing (the header is not sent).
func Sign(secret string, t int64, body []byte) string {
	if secret == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = fmt.Fprintf(mac, "%d.%s", t, body)
	return fmt.Sprintf("t=%d,v1=%s", t, hex.EncodeToString(mac.Sum(nil)))
}

// AllowPrivateTargets toggles the SSRF dial guard off. It is false by
// default (loopback/private/link-local webhook targets are refused) and is
// enabled explicitly for local development and tests via the
// KIWIT_WEBHOOK_ALLOW_PRIVATE environment variable.
var AllowPrivateTargets bool

func isBlockedTargetIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified()
}

// ssrfGuardDialContext resolves the target host and refuses the connection
// when any resolved IP is loopback, private, or link-local and private targets
// are not allowed. Only the first resolved (and already vetted) IP is dialed;
// because every connection (including redirects) goes through this dialer,
// the guard also covers redirect targets.
func ssrfGuardDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("webhook dial %s: %w", addr, err)
	}
	var d net.Dialer
	if ip := net.ParseIP(host); ip != nil {
		if !AllowPrivateTargets && isBlockedTargetIP(ip) {
			return nil, fmt.Errorf("webhook target %s is blocked (loopback/private/link-local address)", host)
		}
		return d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve webhook target %s: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("webhook target %s did not resolve", host)
	}
	for _, ia := range ips {
		if !AllowPrivateTargets && isBlockedTargetIP(ia.IP) {
			return nil, fmt.Errorf("webhook target %s is blocked (resolves to loopback/private/link-local address %s)", host, ia.IP)
		}
	}
	return d.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
}

var deliverClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		DialContext: ssrfGuardDialContext,
	},
}

// Deliver posts the payload to the webhook target and reports the HTTP status.
func Deliver(ctx context.Context, w Webhook, p Payload) (int, error) {
	body, err := json.Marshal(p)
	if err != nil {
		return 0, fmt.Errorf("marshal webhook payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Kiwit-Webhook/1.0")
	req.Header.Set("X-Kiwit-Event", p.Event)
	secrets.DefaultVault().Register(w.Secret)
	if sig := Sign(w.Secret, time.Now().Unix(), body); sig != "" {
		req.Header.Set("X-Kiwit-Signature", sig)
	}

	res, err := deliverClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 400 {
		return res.StatusCode, fmt.Errorf("webhook returned status %d", res.StatusCode)
	}
	return res.StatusCode, nil
}

// EmitEvent fans an event out to all active webhooks subscribed to it. It is
// fire-and-forget: delivery happens in background goroutines with their own
// timeout and never blocks the calling request. Delivery failures are logged
// and panics in the goroutines are recovered so a bad target can never take
// the process down.
func (s *Store) EmitEvent(eventType, entityType string, entityID int64, title, body string) {
	if s == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	targets, err := s.listActive(ctx)
	cancel()
	if err != nil {
		return
	}

	p := Payload{
		Event:      eventType,
		EntityType: entityType,
		EntityID:   entityID,
		Title:      title,
		Body:       body,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}
	for _, w := range targets {
		if !containsEvent(w.Events, eventType) {
			continue
		}
		go func(target Webhook) {
			defer func() {
				if r := recover(); r != nil {
					slog.Default().Error("webhook emit panic", "event", eventType, "err", r)
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if _, err := Deliver(ctx, target, p); err != nil {
				slog.Default().Warn("webhook delivery failed", "event", eventType, "err", err)
			}
		}(w)
	}
}

// EmitEventWithData is like EmitEvent but includes extra event-specific data
// in the payload's "data" field (e.g. for bug.created: tracker name, external
// issue URL, reporter username — the equivalent of Kiwi TCMS's
// EXTERNAL_ISSUE_POST_PROCESSORS receiving the new issue object).
func (s *Store) EmitEventWithData(eventType, entityType string, entityID int64, title, body string, data map[string]any) {
	if s == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	targets, err := s.listActive(ctx)
	cancel()
	if err != nil {
		return
	}

	p := Payload{
		Event:      eventType,
		EntityType: entityType,
		EntityID:   entityID,
		Title:      title,
		Body:       body,
		Data:       data,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
	}
	for _, w := range targets {
		if !containsEvent(w.Events, eventType) {
			continue
		}
		go func(target Webhook) {
			defer func() {
				if r := recover(); r != nil {
					slog.Default().Error("webhook emit panic", "event", eventType, "err", r)
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if _, err := Deliver(ctx, target, p); err != nil {
				slog.Default().Warn("webhook delivery failed", "event", eventType, "err", err)
			}
		}(w)
	}
}

func containsEvent(events []string, eventType string) bool {
	for _, e := range events {
		if e == eventType {
			return true
		}
	}
	return false
}
