// Package config loads services/auth's environment configuration, matching
// the variable names docker-compose.yml's `auth` service sets locally
// (Cloud Run sets the same names in production).
package config

import (
	"fmt"
	"os"
)

// Config is loaded once at startup. Every field here is required — a
// missing value fails Load() outright rather than leaving a zero-value
// default that surfaces as a confusing failure on the first request that
// needs it.
type Config struct {
	GRPCPort             string
	DatabaseURL          string
	GCPProjectID         string
	JWTPrivateKeyPath    string
	LinkedInClientID     string
	LinkedInClientSecret string
	LinkedInRedirectURI  string
}

// Load reads Config from the environment, failing fast if any required
// variable is missing or empty.
func Load() (Config, error) {
	cfg := Config{
		GRPCPort:             os.Getenv("GRPC_PORT"),
		DatabaseURL:          os.Getenv("DATABASE_URL"),
		GCPProjectID:         os.Getenv("GCP_PROJECT_ID"),
		JWTPrivateKeyPath:    os.Getenv("JWT_PRIVATE_KEY_PATH"),
		LinkedInClientID:     os.Getenv("LINKEDIN_CLIENT_ID"),
		LinkedInClientSecret: os.Getenv("LINKEDIN_CLIENT_SECRET"),
		LinkedInRedirectURI:  os.Getenv("LINKEDIN_REDIRECT_URI"),
	}

	required := []struct {
		name  string
		value string
	}{
		{"GRPC_PORT", cfg.GRPCPort},
		{"DATABASE_URL", cfg.DatabaseURL},
		{"GCP_PROJECT_ID", cfg.GCPProjectID},
		{"JWT_PRIVATE_KEY_PATH", cfg.JWTPrivateKeyPath},
		{"LINKEDIN_CLIENT_ID", cfg.LinkedInClientID},
		{"LINKEDIN_CLIENT_SECRET", cfg.LinkedInClientSecret},
		{"LINKEDIN_REDIRECT_URI", cfg.LinkedInRedirectURI},
	}
	for _, req := range required {
		if req.value == "" {
			return Config{}, fmt.Errorf("config: required environment variable %s is not set", req.name)
		}
	}

	return cfg, nil
}
