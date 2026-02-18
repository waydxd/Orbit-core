package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// Set environment variables for testing
	err := os.Setenv("SERVER_PORT", "9000")
	if err != nil {
		return
	}
	err = os.Setenv("DB_HOST", "testdb")
	if err != nil {
		return
	}
	err = os.Setenv("JWT_SECRET", "test-secret")
	if err != nil {
		return
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify configuration values
	if cfg.Server.Port != 9000 {
		t.Errorf("Expected server port 9000, got %d", cfg.Server.Port)
	}

	if cfg.Database.Host != "testdb" {
		t.Errorf("Expected DB host 'testdb', got %s", cfg.Database.Host)
	}

	if cfg.Auth.JWTKey != "test-secret" {
		t.Errorf("Expected JWT secret 'test-secret', got %s", cfg.Auth.JWTKey)
	}

	// Clean up
	err = os.Unsetenv("SERVER_PORT")
	if err != nil {
		return
	}
	err = os.Unsetenv("DB_HOST")
	if err != nil {
		return
	}
	err = os.Unsetenv("JWT_SECRET")
	if err != nil {
		return
	}
}

func TestGetEnv(t *testing.T) {
	err := os.Setenv("TEST_VAR", "test-value")
	if err != nil {
		return
	}

	value := getEnv("TEST_VAR", "default")
	if value != "test-value" {
		t.Errorf("Expected 'test-value', got %s", value)
	}

	value = getEnv("NON_EXISTENT_VAR", "default")
	if value != "default" {
		t.Errorf("Expected 'default', got %s", value)
	}

	err = os.Unsetenv("TEST_VAR")
	if err != nil {
		return
	}
}

func TestGetEnvAsInt(t *testing.T) {
	err := os.Setenv("TEST_INT", "42")
	if err != nil {
		return
	}

	value := getEnvAsInt("TEST_INT", 10)
	if value != 42 {
		t.Errorf("Expected 42, got %d", value)
	}

	value = getEnvAsInt("NON_EXISTENT_INT", 10)
	if value != 10 {
		t.Errorf("Expected 10, got %d", value)
	}

	err = os.Setenv("TEST_INT", "invalid")
	if err != nil {
		return
	}
	value = getEnvAsInt("TEST_INT", 10)
	if value != 10 {
		t.Errorf("Expected default value 10 for invalid int, got %d", value)
	}

	err = os.Unsetenv("TEST_INT")
	if err != nil {
		return
	}
}

func TestConnectionString(t *testing.T) {
	cfg := DatabaseConfig{
		Host:    "localhost",
		Port:    5432,
		User:    "testuser",
		Pass:    "testpass",
		DBName:  "testdb",
		SSLMode: "disable",
	}

	expected := "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=disable"
	result := cfg.ConnectionString()

	if result != expected {
		t.Errorf("Expected connection string '%s', got '%s'", expected, result)
	}
}

func TestRedisAddr(t *testing.T) {
	cfg := RedisConfig{
		Host: "localhost",
		Port: 6379,
	}

	expected := "localhost:6379"
	result := cfg.RedisAddr()

	if result != expected {
		t.Errorf("Expected Redis address '%s', got '%s'", expected, result)
	}
}
