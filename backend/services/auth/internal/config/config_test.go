package config

import "testing"

func setAllRequired(t *testing.T) {
	t.Helper()
	t.Setenv("GRPC_PORT", "9090")
	t.Setenv("DATABASE_URL", "postgres://app:app@localhost:5432/professional_connections?sslmode=disable")
	t.Setenv("GCP_PROJECT_ID", "local-dev")
	t.Setenv("JWT_PRIVATE_KEY_PATH", "/secrets/jwt_private.pem")
	t.Setenv("LINKEDIN_CLIENT_ID", "client-id")
	t.Setenv("LINKEDIN_CLIENT_SECRET", "client-secret")
	t.Setenv("LINKEDIN_REDIRECT_URI", "professionalconnections://auth/linkedin/callback")
}

func TestLoadSucceedsWhenAllRequiredVarsSet(t *testing.T) {
	setAllRequired(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.GRPCPort != "9090" {
		t.Errorf("GRPCPort = %q, want %q", cfg.GRPCPort, "9090")
	}
	if cfg.LinkedInClientID != "client-id" {
		t.Errorf("LinkedInClientID = %q, want %q", cfg.LinkedInClientID, "client-id")
	}
}

func TestLoadFailsFastOnMissingVar(t *testing.T) {
	requiredVars := []string{
		"GRPC_PORT",
		"DATABASE_URL",
		"GCP_PROJECT_ID",
		"JWT_PRIVATE_KEY_PATH",
		"LINKEDIN_CLIENT_ID",
		"LINKEDIN_CLIENT_SECRET",
		"LINKEDIN_REDIRECT_URI",
	}

	for _, missing := range requiredVars {
		t.Run(missing, func(t *testing.T) {
			setAllRequired(t)
			t.Setenv(missing, "")

			if _, err := Load(); err == nil {
				t.Fatalf("Load() succeeded with %s unset, want error", missing)
			}
		})
	}
}
