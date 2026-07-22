package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	GRPCPort   int
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	TrackerAddr    string
	EvalInterval   time.Duration
	EvalBatchSize  int

	FMPAPIKey  string
	FMPBaseURL string
}

func Load() (*Config, error) {
	cfg := &Config{
		GRPCPort:      getEnvInt("GRPC_PORT", 50052),
		DBHost:        getEnv("DB_HOST", "localhost"),
		DBPort:        getEnvInt("DB_PORT", 5432),
		DBUser:        getEnv("DB_USER", "postgres"),
		DBPassword:    getEnv("DB_PASSWORD", "postgres"),
		DBName:        getEnv("DB_NAME", "tsx_evaluator"),
		DBSSLMode:     getEnv("DB_SSLMODE", "disable"),
		TrackerAddr:   getEnv("TRACKER_ADDR", "localhost:50051"),
		EvalInterval:  getEnvDuration("EVAL_INTERVAL", 5*time.Minute),
		EvalBatchSize: getEnvInt("EVAL_BATCH_SIZE", 1),
		FMPAPIKey:     getEnv("FMP_API_KEY", ""),
		FMPBaseURL:    getEnv("FMP_BASE_URL", "https://financialmodelingprep.com"),
	}

	if cfg.EvalInterval < time.Second {
		return nil, fmt.Errorf("EVAL_INTERVAL must be >= 1s, got %s", cfg.EvalInterval)
	}
	return cfg, nil
}

func (c *Config) PostgresDSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName, c.DBSSLMode)
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
