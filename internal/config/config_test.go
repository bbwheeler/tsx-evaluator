package config

import (
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear any env vars that might interfere
	keys := []string{
		"GRPC_PORT", "DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD",
		"DB_NAME", "DB_SSLMODE", "TRACKER_ADDR", "EVAL_INTERVAL", "EVAL_BATCH_SIZE",
		"FMP_API_KEY", "FMP_BASE_URL",
	}
	for _, k := range keys {
		t.Setenv(k, "")
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GRPCPort != 50052 {
		t.Errorf("GRPCPort: got %d, want 50052", cfg.GRPCPort)
	}
	if cfg.DBHost != "localhost" {
		t.Errorf("DBHost: got %q, want %q", cfg.DBHost, "localhost")
	}
	if cfg.DBPort != 5432 {
		t.Errorf("DBPort: got %d, want 5432", cfg.DBPort)
	}
	if cfg.DBUser != "postgres" {
		t.Errorf("DBUser: got %q, want %q", cfg.DBUser, "postgres")
	}
	if cfg.DBPassword != "postgres" {
		t.Errorf("DBPassword: got %q, want %q", cfg.DBPassword, "postgres")
	}
	if cfg.DBName != "tsx_evaluator" {
		t.Errorf("DBName: got %q, want %q", cfg.DBName, "tsx_evaluator")
	}
	if cfg.DBSSLMode != "disable" {
		t.Errorf("DBSSLMode: got %q, want %q", cfg.DBSSLMode, "disable")
	}
	if cfg.TrackerAddr != "localhost:50051" {
		t.Errorf("TrackerAddr: got %q, want %q", cfg.TrackerAddr, "localhost:50051")
	}
	if cfg.EvalInterval != 5*time.Minute {
		t.Errorf("EvalInterval: got %v, want %v", cfg.EvalInterval, 5*time.Minute)
	}
	if cfg.EvalBatchSize != 1 {
		t.Errorf("EvalBatchSize: got %d, want 1", cfg.EvalBatchSize)
	}
	if cfg.FMPAPIKey != "" {
		t.Errorf("FMPAPIKey: got %q, want empty", cfg.FMPAPIKey)
	}
	if cfg.FMPBaseURL != "https://financialmodelingprep.com" {
		t.Errorf("FMPBaseURL: got %q, want %q", cfg.FMPBaseURL, "https://financialmodelingprep.com")
	}
}

func TestLoad_Overrides(t *testing.T) {
	t.Setenv("GRPC_PORT", "9999")
	t.Setenv("DB_HOST", "db.example.com")
	t.Setenv("DB_PORT", "5433")
	t.Setenv("DB_USER", "admin")
	t.Setenv("DB_PASSWORD", "secret")
	t.Setenv("DB_NAME", "mydb")
	t.Setenv("DB_SSLMODE", "require")
	t.Setenv("TRACKER_ADDR", "tracker:50051")
	t.Setenv("EVAL_INTERVAL", "10m")
	t.Setenv("EVAL_BATCH_SIZE", "10")
	t.Setenv("FMP_API_KEY", "abc123")
	t.Setenv("FMP_BASE_URL", "https://custom.fmp.api")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GRPCPort != 9999 {
		t.Errorf("GRPCPort: got %d, want 9999", cfg.GRPCPort)
	}
	if cfg.DBHost != "db.example.com" {
		t.Errorf("DBHost: got %q, want %q", cfg.DBHost, "db.example.com")
	}
	if cfg.DBPort != 5433 {
		t.Errorf("DBPort: got %d, want 5433", cfg.DBPort)
	}
	if cfg.DBUser != "admin" {
		t.Errorf("DBUser: got %q, want %q", cfg.DBUser, "admin")
	}
	if cfg.DBPassword != "secret" {
		t.Errorf("DBPassword: got %q, want %q", cfg.DBPassword, "secret")
	}
	if cfg.DBName != "mydb" {
		t.Errorf("DBName: got %q, want %q", cfg.DBName, "mydb")
	}
	if cfg.DBSSLMode != "require" {
		t.Errorf("DBSSLMode: got %q, want %q", cfg.DBSSLMode, "require")
	}
	if cfg.TrackerAddr != "tracker:50051" {
		t.Errorf("TrackerAddr: got %q, want %q", cfg.TrackerAddr, "tracker:50051")
	}
	if cfg.EvalInterval != 10*time.Minute {
		t.Errorf("EvalInterval: got %v, want %v", cfg.EvalInterval, 10*time.Minute)
	}
	if cfg.EvalBatchSize != 10 {
		t.Errorf("EvalBatchSize: got %d, want 10", cfg.EvalBatchSize)
	}
	if cfg.FMPAPIKey != "abc123" {
		t.Errorf("FMPAPIKey: got %q, want %q", cfg.FMPAPIKey, "abc123")
	}
	if cfg.FMPBaseURL != "https://custom.fmp.api" {
		t.Errorf("FMPBaseURL: got %q, want %q", cfg.FMPBaseURL, "https://custom.fmp.api")
	}
}

func TestLoad_InvalidEvalInterval(t *testing.T) {
	t.Setenv("EVAL_INTERVAL", "500ms")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for EVAL_INTERVAL < 1s")
	}
}

func TestLoad_InvalidPortIgnored(t *testing.T) {
	t.Setenv("GRPC_PORT", "notanumber")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GRPCPort != 50052 {
		t.Errorf("expected default GRPCPort, got %d", cfg.GRPCPort)
	}
}

func TestPostgresDSN(t *testing.T) {
	cfg := &Config{
		DBUser:     "myuser",
		DBPassword: "mypass",
		DBHost:     "dbhost",
		DBPort:     5432,
		DBName:     "testdb",
		DBSSLMode:  "require",
	}
	got := cfg.PostgresDSN()
	want := "postgres://myuser:mypass@dbhost:5432/testdb?sslmode=require"
	if got != want {
		t.Errorf("PostgresDSN():\n  got  %q\n  want %q", got, want)
	}
}

func TestPostgresDSN_DefaultConfig(t *testing.T) {
	t.Setenv("GRPC_PORT", "")
	t.Setenv("DB_HOST", "")
	t.Setenv("DB_PORT", "")
	t.Setenv("DB_USER", "")
	t.Setenv("DB_PASSWORD", "")
	t.Setenv("DB_NAME", "")
	t.Setenv("DB_SSLMODE", "")
	t.Setenv("TRACKER_ADDR", "")
	t.Setenv("EVAL_INTERVAL", "")
	t.Setenv("EVAL_BATCH_SIZE", "")
	t.Setenv("FMP_API_KEY", "")
	t.Setenv("FMP_BASE_URL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := cfg.PostgresDSN()
	want := "postgres://postgres:postgres@localhost:5432/tsx_evaluator?sslmode=disable"
	if got != want {
		t.Errorf("PostgresDSN() with defaults:\n  got  %q\n  want %q", got, want)
	}
}

func TestGetEnv(t *testing.T) {
	t.Setenv("TEST_GETENV_PRESENT", "hello")
	if got := getEnv("TEST_GETENV_PRESENT", "fallback"); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
	if got := getEnv("TEST_GETENV_MISSING", "fallback"); got != "fallback" {
		t.Errorf("got %q, want %q", got, "fallback")
	}
}

func TestGetEnv_EmptyStringReturnsFallback(t *testing.T) {
	t.Setenv("TEST_GETENV_EMPTY", "")
	if got := getEnv("TEST_GETENV_EMPTY", "fallback"); got != "fallback" {
		t.Errorf("got %q, want %q", got, "fallback")
	}
}

func TestGetEnvInt(t *testing.T) {
	t.Setenv("TEST_GETENVINT_PRESENT", "42")
	if got := getEnvInt("TEST_GETENVINT_PRESENT", 0); got != 42 {
		t.Errorf("got %d, want 42", got)
	}
	if got := getEnvInt("TEST_GETENVINT_MISSING", 7); got != 7 {
		t.Errorf("got %d, want 7", got)
	}
}

func TestGetEnvInt_InvalidReturnsFallback(t *testing.T) {
	t.Setenv("TEST_GETENVINT_BAD", "abc")
	if got := getEnvInt("TEST_GETENVINT_BAD", 99); got != 99 {
		t.Errorf("got %d, want 99", got)
	}
}

func TestGetEnvDuration(t *testing.T) {
	t.Setenv("TEST_GETENV_DUR", "30s")
	if got := getEnvDuration("TEST_GETENV_DUR", 0); got != 30*time.Second {
		t.Errorf("got %v, want %v", got, 30*time.Second)
	}
	if got := getEnvDuration("TEST_GETENV_DUR_MISSING", time.Minute); got != time.Minute {
		t.Errorf("got %v, want %v", got, time.Minute)
	}
}

func TestGetEnvDuration_InvalidReturnsFallback(t *testing.T) {
	t.Setenv("TEST_GETENV_DUR_BAD", "notaduration")
	if got := getEnvDuration("TEST_GETENV_DUR_BAD", 5*time.Second); got != 5*time.Second {
		t.Errorf("got %v, want %v", got, 5*time.Second)
	}
}

func TestLoad_EvalIntervalOneSecond(t *testing.T) {
	t.Setenv("EVAL_INTERVAL", "1s")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.EvalInterval != time.Second {
		t.Errorf("EvalInterval: got %v, want %v", cfg.EvalInterval, time.Second)
	}
}
