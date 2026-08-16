// Package config loads services/gateway's environment configuration,
// matching the variable names docker-compose.yml's `gateway` service sets
// locally (Cloud Run sets the same names in production).
package config

import (
	"fmt"
	"os"
)

// Config is loaded once at startup. Every field here is required and
// actually used by this slice's code — a missing value fails Load()
// outright.
//
// docker-compose.yml also sets JWT_PUBLIC_KEY_PATH for the gateway
// container, forward-provisioning it for a future slice that adds a
// JWT-protected route (per ADR-009, the gateway is what will own
// verification). Nothing in this Level-1a-only onboarding slice has a
// protected route yet, so it's deliberately not loaded here — see
// backend/PLAN.md's scope boundary. Add it here when that slice needs it,
// not before.
type Config struct {
	Port            string
	AuthServiceAddr string
	RedisAddr       string
}

// Load reads Config from the environment, failing fast if any required
// variable is missing or empty.
func Load() (Config, error) {
	cfg := Config{
		Port:            os.Getenv("PORT"),
		AuthServiceAddr: os.Getenv("AUTH_SERVICE_ADDR"),
		RedisAddr:       os.Getenv("REDIS_ADDR"),
	}

	required := []struct {
		name  string
		value string
	}{
		{"PORT", cfg.Port},
		{"AUTH_SERVICE_ADDR", cfg.AuthServiceAddr},
		{"REDIS_ADDR", cfg.RedisAddr},
	}
	for _, req := range required {
		if req.value == "" {
			return Config{}, fmt.Errorf("config: required environment variable %s is not set", req.name)
		}
	}

	return cfg, nil
}
