package notify

import (
	"fmt"
	"net/smtp"
	"os"
	"strings"
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
	cfg SMTPConfig
}

func NewEmailClient(cfg SMTPConfig) *EmailClient {
	return &EmailClient{cfg: cfg}
}

func (c *EmailClient) Enabled() bool {
	return c != nil && c.cfg.Host != ""
}

func (c *EmailClient) Send(to []string, subject, body string) error {
	if !c.Enabled() || len(to) == 0 {
		return nil
	}
	addr := c.cfg.Host + ":" + c.cfg.Port
	var auth smtp.Auth
	if c.cfg.Username != "" {
		auth = smtp.PlainAuth("", c.cfg.Username, c.cfg.Password, c.cfg.Host)
	}
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n",
		c.cfg.From, strings.Join(to, ", "), subject, body)
	return smtp.SendMail(addr, auth, c.cfg.From, to, []byte(msg))
}
