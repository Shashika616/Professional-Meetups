// Package config loads services/meetup's environment configuration,
// matching the variable names docker-compose.yml's `meetup` service sets
// locally (Cloud Run sets the same names in production).
package config

import (
	"fmt"
	"os"
)

// Config is loaded once at startup. Every field here except
// FirebaseServiceAccountJSON is required — a missing value fails Load()
// outright rather than leaving a zero-value default that surfaces as a
// confusing failure on the first request that needs it.
type Config struct {
	GRPCPort     string
	DatabaseURL  string
	GCPProjectID string

	// FirebaseServiceAccountJSON is deliberately NOT in the required list
	// below — backend/meetup-scheduling-PLAN.md's prerequisite falls back
	// to LoggingPushSender when this is empty, so local dev/tests keep
	// working before Shashika provides real Firebase credentials.
	// main.go decides which sender to construct based on whether it's set.
	FirebaseServiceAccountJSON string
}

// Load reads Config from the environment, failing fast if any required
// variable is missing or empty.
func Load() (Config, error) {
	cfg := Config{
		GRPCPort:                   os.Getenv("GRPC_PORT"),
		DatabaseURL:                os.Getenv("DATABASE_URL"),
		GCPProjectID:               os.Getenv("GCP_PROJECT_ID"),
		FirebaseServiceAccountJSON: os.Getenv("FIREBASE_SERVICE_ACCOUNT_JSON"),
	}

	required := []struct {
		name  string
		value string
	}{
		{"GRPC_PORT", cfg.GRPCPort},
		{"DATABASE_URL", cfg.DatabaseURL},
		{"GCP_PROJECT_ID", cfg.GCPProjectID},
	}
	for _, req := range required {
		if req.value == "" {
			return Config{}, fmt.Errorf("config: required environment variable %s is not set", req.name)
		}
	}

	return cfg, nil
}
