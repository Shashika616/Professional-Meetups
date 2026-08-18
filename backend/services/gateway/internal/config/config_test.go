package config

import "testing"

func setAllRequired(t *testing.T) {
	t.Helper()
	t.Setenv("PORT", "8080")
	t.Setenv("AUTH_SERVICE_ADDR", "auth:9090")
	t.Setenv("REDIS_ADDR", "redis:6379")
	t.Setenv("JWT_PUBLIC_KEY_PATH", "/secrets/jwt_public.pem")
}

func TestLoadSucceedsWhenAllRequiredVarsSet(t *testing.T) {
	setAllRequired(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want %q", cfg.Port, "8080")
	}
	if cfg.AuthServiceAddr != "auth:9090" {
		t.Errorf("AuthServiceAddr = %q, want %q", cfg.AuthServiceAddr, "auth:9090")
	}
	if cfg.RedisAddr != "redis:6379" {
		t.Errorf("RedisAddr = %q, want %q", cfg.RedisAddr, "redis:6379")
	}
	if cfg.JWTPublicKeyPath != "/secrets/jwt_public.pem" {
		t.Errorf("JWTPublicKeyPath = %q, want %q", cfg.JWTPublicKeyPath, "/secrets/jwt_public.pem")
	}
}

func TestLoadFailsFastOnMissingVar(t *testing.T) {
	for _, missing := range []string{"PORT", "AUTH_SERVICE_ADDR", "REDIS_ADDR", "JWT_PUBLIC_KEY_PATH"} {
		t.Run(missing, func(t *testing.T) {
			setAllRequired(t)
			t.Setenv(missing, "")

			if _, err := Load(); err == nil {
				t.Fatalf("Load() succeeded with %s unset, want error", missing)
			}
		})
	}
}
