package config

import (
	"fmt"
	"os"
	"time"
)

// Config holds all application configuration.
type Config struct {
	AppEnv   string
	HTTPPort string
	DB       DBConfig
	Redis    RedisConfig
	JWT      JWTConfig
	Timezone string

	FeatureSMSEnabled    bool
	FeatureWechatEnabled bool
	FeatureOnlineBooking bool
}

type DBConfig struct {
	DSN string
}

type RedisConfig struct {
	Addr string
}

type JWTConfig struct {
	AccessSecret  string
	RefreshSecret string
	AccessTTL     time.Duration
	RefreshTTL    time.Duration
}

// Load reads configuration from environment variables.
func Load(configPath string) (*Config, error) {
	cfg := &Config{
		AppEnv:   getEnv("APP_ENV", "dev"),
		HTTPPort: getEnv("HTTP_PORT", "8080"),
		DB: DBConfig{
			DSN: os.Getenv("DB_DSN"),
		},
		Redis: RedisConfig{
			Addr: getEnv("REDIS_ADDR", "localhost:6379"),
		},
		JWT: JWTConfig{
			AccessSecret:  os.Getenv("JWT_ACCESS_SECRET"),
			RefreshSecret: os.Getenv("JWT_REFRESH_SECRET"),
		},
		Timezone: getEnv("DEFAULT_TIMEZONE", "Asia/Shanghai"),

		FeatureSMSEnabled:    getEnvBool("FEATURE_SMS_ENABLED", false),
		FeatureWechatEnabled: getEnvBool("FEATURE_WECHAT_ENABLED", false),
		FeatureOnlineBooking: getEnvBool("FEATURE_ONLINE_BOOKING_ENABLED", true),
	}

	// Parse TTLs
	if v := os.Getenv("JWT_ACCESS_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("invalid JWT_ACCESS_TTL: %w", err)
		}
		cfg.JWT.AccessTTL = d
	} else {
		cfg.JWT.AccessTTL = 2 * time.Hour
	}

	if v := os.Getenv("JWT_REFRESH_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("invalid JWT_REFRESH_TTL: %w", err)
		}
		cfg.JWT.RefreshTTL = d
	} else {
		cfg.JWT.RefreshTTL = 720 * time.Hour
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.DB.DSN == "" {
		return fmt.Errorf("DB_DSN is required")
	}
	if c.JWT.AccessSecret == "" {
		return fmt.Errorf("JWT_ACCESS_SECRET is required")
	}
	if c.JWT.RefreshSecret == "" {
		return fmt.Errorf("JWT_REFRESH_SECRET is required")
	}
	if c.Redis.Addr == "" {
		return fmt.Errorf("REDIS_ADDR is required")
	}
	return nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	return v == "true" || v == "1" || v == "yes"
}
