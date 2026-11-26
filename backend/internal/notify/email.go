package notify

import (
	"fmt"
	"net/smtp"
	"os"
	"strings"
	"sync"
)

type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

func LoadSMTPConfigFromEnv() SMTPConfig {
	port := os.Getenv("KIWIT_SMTP_PORT")
	if port == "" {
		port = "587"
	}
	from := os.Getenv("KIWIT_SMTP_FROM")
	if from == "" {
		from = "kiwit@localhost"
	}
	return SMTPConfig{
		Host:     os.Getenv("KIWIT_SMTP_HOST"),
		Port:     port,
		Username: os.Getenv("KIWIT_SMTP_USER"),
		Password: os.Getenv("KIWIT_SMTP_PASS"),
		From:     from,
	}
}

type EmailClient struct {
	mu  sync.RWMutex
	cfg SMTPConfig
}

func NewEmailClient(cfg SMTPConfig) *EmailClient {
	return &EmailClient{cfg: cfg}
}

// SetConfig atomically replaces the SMTP configuration (runtime update via admin API).
func (c *EmailClient) SetConfig(cfg SMTPConfig) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.cfg = cfg
	c.mu.Unlock()
}

// Config returns a copy of the current SMTP configuration.
func (c *EmailClient) Config() SMTPConfig {
	if c == nil {
		return SMTPConfig{}
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cfg
}

func (c *EmailClient) Enabled() bool {
	if c == nil {
		return false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cfg.Host != ""
}

func sanitizeHeader(s string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(s)
}

func (c *EmailClient) Send(to []string, subject, body string) error {
	if !c.Enabled() || len(to) == 0 {
		return nil
	}
	cleanTo := make([]string, 0, len(to))
	for _, addr := range to {
		if a := strings.TrimSpace(sanitizeHeader(addr)); a != "" {
			cleanTo = append(cleanTo, a)
		}
	}
	if len(cleanTo) == 0 {
		return nil
	}
	c.mu.RLock()
	cfg := c.cfg
	c.mu.RUnlock()
	cleanFrom := sanitizeHeader(cfg.From)
	cleanSubject := sanitizeHeader(subject)
	addr := cfg.Host + ":" + cfg.Port
	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n",
		cleanFrom, strings.Join(cleanTo, ", "), cleanSubject, body)
	return smtp.SendMail(addr, auth, cleanFrom, cleanTo, []byte(msg))
}
