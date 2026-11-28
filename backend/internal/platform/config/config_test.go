package config

import (
	"testing"
)

func TestGetEnv(t *testing.T) {
	t.Setenv("KIWIT_TEST_A", "")
	if got := getEnv("KIWIT_TEST_A", "fallback"); got != "fallback" {
		t.Errorf("empty env should fall back, got %q", got)
	}
	t.Setenv("KIWIT_TEST_A", "value")
	if got := getEnv("KIWIT_TEST_A", "fallback"); got != "value" {
		t.Errorf("expected value, got %q", got)
	}
}

func TestGetEnvBool(t *testing.T) {
	for _, v := range []string{"1", "true", "TRUE", "Yes"} {
		t.Setenv("KIWIT_TEST_B", v)
		if !getEnvBool("KIWIT_TEST_B", false) {
			t.Errorf("%q should be true", v)
		}
	}
	for _, v := range []string{"0", "false", "no", "whatever"} {
		t.Setenv("KIWIT_TEST_B", v)
		if getEnvBool("KIWIT_TEST_B", true) {
			t.Errorf("%q should be false", v)
		}
	}
	t.Setenv("KIWIT_TEST_B", "")
	if !getEnvBool("KIWIT_TEST_B", true) {
		t.Error("empty env should return fallback")
	}
}

func TestGetEnvInt(t *testing.T) {
	t.Setenv("KIWIT_TEST_N", "42")
	if got := getEnvInt("KIWIT_TEST_N", 10); got != 42 {
		t.Errorf("expected 42, got %d", got)
	}
	for _, v := range []string{"", "abc", "-5", "0"} {
		t.Setenv("KIWIT_TEST_N", v)
		if got := getEnvInt("KIWIT_TEST_N", 10); got != 10 {
			t.Errorf("%q should fall back to 10, got %d", v, got)
		}
	}
}

func TestLoadDefaultsAndOverrides(t *testing.T) {
	t.Setenv("KIWIT_ENV", "production")
	t.Setenv("KIWIT_JWT_SECRET", "a-strong-secret-1234567890")
	t.Setenv("KIWIT_LDAP_STARTTLS", "true")
	t.Setenv("KIWIT_TRUSTED_PROXIES", "10.0.0.0/8, 192.168.0.0/16")
	t.Setenv("KIWIT_LOG_LEVEL", "DEBUG")

	c := Load()
	if c.Env != "production" || c.JWTSecret != "a-strong-secret-1234567890" {
		t.Errorf("env override not applied: %+v", c)
	}
	if c.Addr != ":8080" || c.DatabaseURL != "sqlite://./kiwit.db" || c.StorageType != "local" {
		t.Errorf("unexpected defaults: addr=%q db=%q storage=%q", c.Addr, c.DatabaseURL, c.StorageType)
	}
	if !c.LDAPStartTLS {
		t.Error("KIWIT_LDAP_STARTTLS=true should be parsed")
	}
	if len(c.TrustedProxies) != 2 || c.TrustedProxies[0] != "10.0.0.0/8" {
		t.Errorf("trusted proxies not split: %v", c.TrustedProxies)
	}
	if c.LogLevel != "debug" {
		t.Errorf("log level should be lowercased, got %q", c.LogLevel)
	}
	if err := c.Validate(); err != nil {
		t.Errorf("valid production config should pass Validate: %v", err)
	}
}

func TestValidateProductionRequiresStrongJWT(t *testing.T) {
	cases := []struct {
		name    string
		secret  string
		wantErr bool
	}{
		{"default dev secret", DefaultInsecureJWTSecret, true},
		{"empty", "", true},
		{"too short", "short", true},
		{"ok 16 chars", "0123456789abcdef", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := Config{Env: "production", JWTSecret: tc.secret}
			err := c.Validate()
			if tc.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
	// development env does not require a strong secret
	dev := Config{Env: "development", JWTSecret: DefaultInsecureJWTSecret}
	if err := dev.Validate(); err != nil {
		t.Errorf("development should allow dev secret: %v", err)
	}
}

func TestLDAPAndOIDCEnabled(t *testing.T) {
	c := Config{}
	if c.LDAPEnabled() || c.OIDCEnabled() {
		t.Error("empty config should disable LDAP and OIDC")
	}
	c = Config{LDAPServer: "ldap:389", LDAPBaseDN: "dc=x", LDAPUserFilter: "(uid=%s)"}
	if !c.LDAPEnabled() {
		t.Error("complete LDAP config should enable LDAP")
	}
	c = Config{LDAPServer: "ldap:389"} // missing base DN / filter
	if c.LDAPEnabled() {
		t.Error("incomplete LDAP config must stay disabled")
	}
	c = Config{OIDCIssuer: "https://idp", OIDCClientID: "id", OIDCClientSecret: "sec"}
	if !c.OIDCEnabled() {
		t.Error("complete OIDC config should enable SSO")
	}
	c = Config{OIDCIssuer: "https://idp", OIDCClientID: "id"}
	if c.OIDCEnabled() {
		t.Error("OIDC without secret must stay disabled")
	}
}
