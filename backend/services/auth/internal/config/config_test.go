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

// TestLoadSucceedsWithTwilioAndResendUnset guards the Level 2/3 addendum's
// fallback contract: TWILIO_*/RESEND_* must stay optional (the empty case
// is what triggers LoggingSmsSender/LoggingEmailSender in main.go), so
// Load() must not fail-fast on them the way it does for the required vars.
func TestLoadSucceedsWithTwilioAndResendUnset(t *testing.T) {
	setAllRequired(t)
	for _, v := range []string{
		"TWILIO_ACCOUNT_SID", "TWILIO_AUTH_TOKEN", "TWILIO_PHONE_NUMBER",
		"RESEND_API_KEY", "RESEND_FROM_EMAIL",
	} {
		t.Setenv(v, "")
	}

	if _, err := Load(); err != nil {
		t.Fatalf("Load() returned error with Twilio/Resend vars unset, want success: %v", err)
	}
}

func TestLoadReadsTwilioAndResendWhenSet(t *testing.T) {
	setAllRequired(t)
	t.Setenv("TWILIO_ACCOUNT_SID", "AC123")
	t.Setenv("TWILIO_AUTH_TOKEN", "token")
	t.Setenv("TWILIO_PHONE_NUMBER", "+14155551234")
	t.Setenv("RESEND_API_KEY", "re_123")
	t.Setenv("RESEND_FROM_EMAIL", "onboarding@resend.dev")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.TwilioAccountSID != "AC123" {
		t.Errorf("TwilioAccountSID = %q, want %q", cfg.TwilioAccountSID, "AC123")
	}
	if cfg.TwilioPhoneNumber != "+14155551234" {
		t.Errorf("TwilioPhoneNumber = %q, want %q", cfg.TwilioPhoneNumber, "+14155551234")
	}
	if cfg.ResendAPIKey != "re_123" {
		t.Errorf("ResendAPIKey = %q, want %q", cfg.ResendAPIKey, "re_123")
	}
	if cfg.ResendFromEmail != "onboarding@resend.dev" {
		t.Errorf("ResendFromEmail = %q, want %q", cfg.ResendFromEmail, "onboarding@resend.dev")
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
