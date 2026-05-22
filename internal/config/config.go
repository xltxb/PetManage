package config

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Supabase SupabaseConfig `yaml:"supabase"`
	Redis    RedisConfig    `yaml:"redis"`
	JWT      JWTConfig      `yaml:"jwt"`
	Log      LogConfig      `yaml:"log"`
}

type ServerConfig struct {
	Port string `yaml:"port"`
	Mode string `yaml:"mode"`
}

type DatabaseConfig struct {
	Driver       string `yaml:"driver"`
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	User         string `yaml:"user"`
	Password     string `yaml:"password"`
	DBName       string `yaml:"dbname"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
}

type SupabaseConfig struct {
	URL            string `yaml:"url"`
	AnonKey        string `yaml:"anon_key"`
	ServiceRoleKey string `yaml:"service_role_key"`
}

type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type JWTConfig struct {
	Secret          string `yaml:"secret"`
	AccessTokenTTL  int    `yaml:"access_token_ttl"`
	RefreshTokenTTL int    `yaml:"refresh_token_ttl"`
}

type LogConfig struct {
	Level    string `yaml:"level"`
	Output   string `yaml:"output"`
	FilePath string `yaml:"file_path"`
}

var envVarPattern = regexp.MustCompile(`\$\{(\w+)(?::-([^}]*))?\}`)

// expandEnvVars replaces ${VAR} and ${VAR:-default} with environment variable values.
func expandEnvVars(s string) string {
	return envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		parts := envVarPattern.FindStringSubmatch(match)
		name := parts[1]
		defaultVal := parts[2]
		if val, ok := os.LookupEnv(name); ok && val != "" {
			return val
		}
		if defaultVal != "" {
			return defaultVal
		}
		return match
	})
}

// DSN builds a PostgreSQL connection string from database config.
func (c *Config) DSN() string {
	return "host=" + c.Database.Host +
		" port=" + fmt.Sprintf("%d", c.Database.Port) +
		" user=" + c.Database.User +
		" password=" + c.Database.Password +
		" dbname=" + c.Database.DBName +
		" sslmode=disable"
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Expand ${VAR:-default} and ${VAR} placeholders with env var values.
	expanded := expandEnvVars(string(data))

	cfg := &Config{}
	if err := yaml.Unmarshal([]byte(expanded), cfg); err != nil {
		return nil, err
	}

	if cfg.Server.Port == "" {
		cfg.Server.Port = "8080"
	}
	if cfg.Server.Mode == "" {
		cfg.Server.Mode = "debug"
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "debug"
	}

	return cfg, nil
}
