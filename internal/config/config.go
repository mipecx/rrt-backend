// Package config package contains environment variables and other config
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config - main app config
type Config struct {
	Env      string
	HTTPPort string
	DB       DBConfig
	Redis    RedisConfig
	JWT      JWTConfig
	SMS      SMSConfig
	CORS     CORSConfig
}

// DBConfig - PostgreSQL config
type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

// RedisConfig - Redis config
type RedisConfig struct {
	Host     string
	Port     string
	Password string
}

type JWTConfig struct {
	Secret          string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

type SMSConfig struct {
    AccountSID string
    AuthToken  string
    FromPhone  string 
	APIURL     string
}

type CORSConfig struct {
	AllowedOrigins []string
}

// Load gets env variables and combines them into Config struct
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Env:      getEnv("APP_ENV", "local"),
		HTTPPort: getEnv("HTTP_PORT", ":8080"),
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "postgres"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", ""),
			Password: getEnv("DB_PASSWORD", ""),
			Name:     getEnv("DB_NAME", ""),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "redis"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
		},
		JWT: JWTConfig{
			Secret:          getEnv("JWT_SECRET", ""),
			AccessTokenTTL:  getEnvDuration("JWT_ACCESS_TTL", 15*time.Minute),
			RefreshTokenTTL: getEnvDuration("JWT_REFRESH_TTL", 240*time.Hour),
		},
		SMS: SMSConfig{
			AccountSID: getEnv("SMS_ACCOUNT_SID", ""),
			AuthToken:  getEnv("SMS_AUTH_TOKEN", ""),
			FromPhone:  getEnv("SMS_FROM_PHONE", ""),
			APIURL: 	getEnv("SMS_API_URL", ""),
		},
		CORS: CORSConfig{
			AllowedOrigins: getEnvSlice("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000", "http://localhost:5173", "http://127.0.0.1:3000", "http://127.0.0.1:5173"}),
		},
	}

	if cfg.DB.User == "" || cfg.DB.Password == "" || cfg.DB.Name == "" {
		return nil, fmt.Errorf("критические переменные PostgreSQL (USER, PASSWORD, NAME) не заполнены")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvSlice(key string, defaultValue []string) []string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		parts := strings.Split(value, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}

func (c *Config) IsDev() bool {
    return c.Env == "local" || c.Env == "dev"
}