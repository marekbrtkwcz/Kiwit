package config

import "os"

// Config holds application runtime configuration.
type Config struct {
	Addr        string
	DatabaseURL string
	Env         string
	JWTSecret   string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() Config {
	return Config{
		Addr:        getEnv("KIWIT_ADDR", ":8080"),
		DatabaseURL: getEnv("KIWIT_DATABASE_URL", "sqlite://./kiwit.db"),
		Env:         getEnv("KIWIT_ENV", "development"),
		JWTSecret:   getEnv("KIWIT_JWT_SECRET", "dev-insecure-secret-change-me"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
