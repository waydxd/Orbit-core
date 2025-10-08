package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// Set environment variables for testing
	os.Setenv("SERVER_PORT", "9000")
	os.Setenv("DB_HOST", "testdb")
	os.Setenv("JWT_SECRET", "test-secret")

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

	if cfg.Auth.JWTSecret != "test-secret" {
		t.Errorf("Expected JWT secret 'test-secret', got %s", cfg.Auth.JWTSecret)
	}

	// Clean up
	os.Unsetenv("SERVER_PORT")
	os.Unsetenv("DB_HOST")
	os.Unsetenv("JWT_SECRET")
}

func TestGetEnv(t *testing.T) {
	os.Setenv("TEST_VAR", "test-value")

	value := getEnv("TEST_VAR", "default")
	if value != "test-value" {
		t.Errorf("Expected 'test-value', got %s", value)
	}

	value = getEnv("NON_EXISTENT_VAR", "default")
	if value != "default" {
		t.Errorf("Expected 'default', got %s", value)
	}

	os.Unsetenv("TEST_VAR")
}

func TestGetEnvAsInt(t *testing.T) {
	os.Setenv("TEST_INT", "42")

	value := getEnvAsInt("TEST_INT", 10)
	if value != 42 {
		t.Errorf("Expected 42, got %d", value)
	}

	value = getEnvAsInt("NON_EXISTENT_INT", 10)
	if value != 10 {
		t.Errorf("Expected 10, got %d", value)
	}

	os.Setenv("TEST_INT", "invalid")
	value = getEnvAsInt("TEST_INT", 10)
	if value != 10 {
		t.Errorf("Expected default value 10 for invalid int, got %d", value)
	}

	os.Unsetenv("TEST_INT")
}

func TestConnectionString(t *testing.T) {
	cfg := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "testuser",
		Password: "testpass",
		DBName:   "testdb",
		SSLMode:  "disable",
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
