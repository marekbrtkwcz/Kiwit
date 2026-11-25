package config

import (
	"fmt"
	"os"
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
	}
}

func getEnvBool(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if v == "" {
		return fallback
	}
	return v == "1" || v == "true" || v == "yes"
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
