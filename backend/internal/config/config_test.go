package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("APP_ENV", "test")
	os.Setenv("HTTP_PORT", "9090")
	os.Setenv("DB_DSN", "postgres://test:test@localhost:5432/testdb?sslmode=disable")
	os.Setenv("REDIS_ADDR", "localhost:6380")
	os.Setenv("JWT_ACCESS_SECRET", "test-access")
	os.Setenv("JWT_REFRESH_SECRET", "test-refresh")
	os.Setenv("JWT_ACCESS_TTL", "15m")
	os.Setenv("JWT_REFRESH_TTL", "168h")
	os.Setenv("DEFAULT_TIMEZONE", "Asia/Shanghai")
	defer func() {
		for _, k := range []string{
			"APP_ENV", "HTTP_PORT", "DB_DSN", "REDIS_ADDR",
			"JWT_ACCESS_SECRET", "JWT_REFRESH_SECRET",
			"JWT_ACCESS_TTL", "JWT_REFRESH_TTL", "DEFAULT_TIMEZONE",
		} {
			os.Unsetenv(k)
		}
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.HTTPPort != "9090" {
		t.Errorf("HTTPPort = %q, want 9090", cfg.HTTPPort)
	}
	if cfg.DB.DSN != "postgres://test:test@localhost:5432/testdb?sslmode=disable" {
		t.Errorf("DB DSN mismatch: %q", cfg.DB.DSN)
	}
	if cfg.JWT.AccessTTL != 15*time.Minute {
		t.Errorf("AccessTTL = %v, want 15m", cfg.JWT.AccessTTL)
	}
	if cfg.JWT.RefreshTTL != 168*time.Hour {
		t.Errorf("RefreshTTL = %v, want 168h", cfg.JWT.RefreshTTL)
	}
}

func TestLoadMissingRequired(t *testing.T) {
	os.Clearenv()
	_, err := Load("")
	if err == nil {
		t.Error("expected error for missing DB_DSN, got nil")
	}
}

func TestDefaultValues(t *testing.T) {
	os.Setenv("DB_DSN", "postgres://localhost/test")
	os.Setenv("JWT_ACCESS_SECRET", "secret")
	os.Setenv("JWT_REFRESH_SECRET", "refresh")
	os.Setenv("REDIS_ADDR", "localhost:6379")
	defer os.Clearenv()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.HTTPPort != "8080" {
		t.Errorf("default HTTPPort = %q, want 8080", cfg.HTTPPort)
	}
	if cfg.AppEnv != "dev" {
		t.Errorf("default AppEnv = %q, want dev", cfg.AppEnv)
	}
}
