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
// JWT_PUBLIC_KEY_PATH was already set in docker-compose.yml before this was
// ever read anywhere — forward-provisioned for this exact moment (per
// ADR-009, the gateway is what will own verification). This is genuinely
// new wiring, confirmed by reading the actual pre-addendum gateway code:
// nothing constructed a jwt.Verifier before backend/PLAN.md's Level 2/3
// addendum, Step F.
type Config struct {
	Port             string
	AuthServiceAddr  string
	RedisAddr        string
	JWTPublicKeyPath string
}

// Load reads Config from the environment, failing fast if any required
// variable is missing or empty.
func Load() (Config, error) {
	cfg := Config{
		Port:             os.Getenv("PORT"),
		AuthServiceAddr:  os.Getenv("AUTH_SERVICE_ADDR"),
		RedisAddr:        os.Getenv("REDIS_ADDR"),
		JWTPublicKeyPath: os.Getenv("JWT_PUBLIC_KEY_PATH"),
	}

	required := []struct {
		name  string
		value string
	}{
		{"PORT", cfg.Port},
		{"AUTH_SERVICE_ADDR", cfg.AuthServiceAddr},
		{"REDIS_ADDR", cfg.RedisAddr},
		{"JWT_PUBLIC_KEY_PATH", cfg.JWTPublicKeyPath},
	}
	for _, req := range required {
		if req.value == "" {
			return Config{}, fmt.Errorf("config: required environment variable %s is not set", req.name)
		}
	}

	return cfg, nil
}
