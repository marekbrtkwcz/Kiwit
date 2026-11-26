package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const DefaultInsecureJWTSecret = "dev-insecure-secret-change-me"

// Config holds application runtime configuration.
type Config struct {
	Addr              string
	DatabaseURL       string
	Env               string
	JWTSecret         string
	UploadDir         string
	AllowRegistration bool
	BaseURL           string
	StorageType       string // "local" (default) or "s3"
	S3Endpoint        string
	S3Region          string
	S3Bucket          string
	S3AccessKey       string
	S3SecretKey       string
	S3UsePathStyle    bool
	OIDCIssuer        string // empty = SSO disabled
	OIDCClientID      string
	OIDCClientSecret  string
	OIDCRedirectURL   string
	OIDCProviderName  string
	LDAPServer        string // empty = LDAP disabled
	LDAPBindDN        string
	LDAPBindPassword  string
	LDAPBaseDN        string
	LDAPUserFilter    string // e.g. (uid=%s)
	LDAPUsernameAttr  string
	LDAPEmailAttr     string
	LogLevel          string
	LogFormat         string // "json" (default) or "text"
	LogFile           string // empty = stdout only
	LogMaxSizeMB      int
	LogMaxFiles       int
}

// LDAPEnabled reports whether LDAP auth is configured.
func (c Config) LDAPEnabled() bool {
	return c.LDAPServer != "" && c.LDAPBaseDN != "" && c.LDAPUserFilter != ""
}

// OIDCEnabled reports whether SSO via OIDC is configured.
func (c Config) OIDCEnabled() bool {
	return c.OIDCIssuer != "" && c.OIDCClientID != "" && c.OIDCClientSecret != ""
}

// Load reads configuration from environment variables with sensible defaults.
func Load() Config {
	return Config{
		Addr:              getEnv("KIWIT_ADDR", ":8080"),
		DatabaseURL:       getEnv("KIWIT_DATABASE_URL", "sqlite://./kiwit.db"),
		Env:               getEnv("KIWIT_ENV", "development"),
		JWTSecret:         getEnv("KIWIT_JWT_SECRET", DefaultInsecureJWTSecret),
		UploadDir:         getEnv("KIWIT_UPLOAD_DIR", "./uploads"),
		AllowRegistration: getEnvBool("KIWIT_ALLOW_REGISTRATION", true),
		BaseURL:           getEnv("KIWIT_BASE_URL", "http://localhost:8080"),
		StorageType:       getEnv("KIWIT_STORAGE_TYPE", "local"),
		S3Endpoint:        getEnv("KIWIT_S3_ENDPOINT", ""),
		S3Region:          getEnv("KIWIT_S3_REGION", "us-east-1"),
		S3Bucket:          getEnv("KIWIT_S3_BUCKET", ""),
		S3AccessKey:       getEnv("KIWIT_S3_ACCESS_KEY", ""),
		S3SecretKey:       getEnv("KIWIT_S3_SECRET_KEY", ""),
		S3UsePathStyle:    getEnvBool("KIWIT_S3_USE_PATH_STYLE", true),
		OIDCIssuer:        getEnv("KIWIT_OIDC_ISSUER", ""),
		OIDCClientID:      getEnv("KIWIT_OIDC_CLIENT_ID", ""),
		OIDCClientSecret:  getEnv("KIWIT_OIDC_CLIENT_SECRET", ""),
		OIDCRedirectURL:   getEnv("KIWIT_OIDC_REDIRECT_URL", ""),
		OIDCProviderName:  getEnv("KIWIT_OIDC_PROVIDER_NAME", "SSO"),
		LDAPServer:        getEnv("KIWIT_LDAP_SERVER", ""),
		LDAPBindDN:        getEnv("KIWIT_LDAP_BIND_DN", ""),
		LDAPBindPassword:  getEnv("KIWIT_LDAP_BIND_PASSWORD", ""),
		LDAPBaseDN:        getEnv("KIWIT_LDAP_BASE_DN", ""),
		LDAPUserFilter:    getEnv("KIWIT_LDAP_USER_FILTER", "(uid=%s)"),
		LDAPUsernameAttr:  getEnv("KIWIT_LDAP_USERNAME_ATTR", "uid"),
		LDAPEmailAttr:     getEnv("KIWIT_LDAP_EMAIL_ATTR", "mail"),
		LogLevel:          strings.ToLower(getEnv("KIWIT_LOG_LEVEL", "info")),
		LogFormat:         strings.ToLower(getEnv("KIWIT_LOG_FORMAT", "json")),
		LogFile:           getEnv("KIWIT_LOG_FILE", ""),
		LogMaxSizeMB:      getEnvInt("KIWIT_LOG_MAX_SIZE", 10),
		LogMaxFiles:       getEnvInt("KIWIT_LOG_MAX_FILES", 3),
	}
}

func getEnvBool(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	return v == "1" || v == "true" || v == "yes"
}

func getEnvInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

// Validate checks that required security settings are configured safely for the active environment.
func (c Config) Validate() error {
	if c.Env == "production" {
		if c.JWTSecret == "" || c.JWTSecret == DefaultInsecureJWTSecret || len(c.JWTSecret) < 16 {
			return fmt.Errorf("KIWIT_JWT_SECRET must be set to a strong secret (>= 16 chars) in production")
		}
	}
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
