package config

import "testing"

func setAllRequired(t *testing.T) {
	t.Helper()
	t.Setenv("GRPC_PORT", "9091")
	t.Setenv("DATABASE_URL", "postgres://app:app@localhost:5432/professional_connections?sslmode=disable")
	t.Setenv("GCP_PROJECT_ID", "local-dev")
}

func TestLoadSucceedsWhenAllRequiredVarsSet(t *testing.T) {
	setAllRequired(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.GRPCPort != "9091" {
		t.Errorf("GRPCPort = %q, want %q", cfg.GRPCPort, "9091")
	}
	if cfg.GCPProjectID != "local-dev" {
		t.Errorf("GCPProjectID = %q, want %q", cfg.GCPProjectID, "local-dev")
	}
}

// TestLoadSucceedsWithFirebaseUnset guards the fallback contract:
// FIREBASE_SERVICE_ACCOUNT_JSON must stay optional (the empty case is what
// triggers LoggingPushSender in main.go), so Load() must not fail-fast on
// it the way it does for the required vars.
func TestLoadSucceedsWithFirebaseUnset(t *testing.T) {
	setAllRequired(t)
	t.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", "")

	if _, err := Load(); err != nil {
		t.Fatalf("Load() returned error with FIREBASE_SERVICE_ACCOUNT_JSON unset, want success: %v", err)
	}
}

func TestLoadReadsFirebaseWhenSet(t *testing.T) {
	setAllRequired(t)
	t.Setenv("FIREBASE_SERVICE_ACCOUNT_JSON", `{"type":"service_account","project_id":"test"}`)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.FirebaseServiceAccountJSON != `{"type":"service_account","project_id":"test"}` {
		t.Errorf("FirebaseServiceAccountJSON = %q, want the value set", cfg.FirebaseServiceAccountJSON)
	}
}

func TestLoadFailsFastOnMissingVar(t *testing.T) {
	requiredVars := []string{
		"GRPC_PORT",
		"DATABASE_URL",
		"GCP_PROJECT_ID",
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
