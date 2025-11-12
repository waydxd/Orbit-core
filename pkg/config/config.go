package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	Redis      RedisConfig
	Auth       AuthConfig
	Orbi       OrbiConfig
	GRPCServer GRPCServerConfig
	Hashtag    HashtagConfig
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port int
	Host string
}

// DatabaseConfig holds PostgreSQL configuration
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// RedisConfig holds Redis configuration for rate limiting
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWTSecret     string
	JWTExpiration int // in hours
}

// OrbiConfig holds Orbi agent gRPC connection configuration
type OrbiConfig struct {
	Host string
	Port int
}

// GRPCServerConfig holds gRPC server configuration for Core
type GRPCServerConfig struct {
	Port int
}

// HashtagConfig holds hashtag service configuration
type HashtagConfig struct {
	Enabled bool
	GRPC    HashtagGRPCConfig
	Cache   HashtagCacheConfig
}

// HashtagGRPCConfig holds gRPC connection settings for hashtag service
type HashtagGRPCConfig struct {
	Host             string
	Port             int
	Timeout          int // in seconds
	MaxRetries       int
	KeepAlive        int // in seconds
	KeepAliveTimeout int // in seconds
}

// HashtagCacheConfig holds cache settings for hashtag predictions
type HashtagCacheConfig struct {
	Enabled bool
	TTL     int // in minutes
	MaxSize int
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// Detect and load a dotenv file if present. By default we look for `.env`.
	// The path can be overridden by setting the ENV_FILE environment variable.
	envFile := os.Getenv("ENV_FILE")
	if envFile == "" {
		envFile = ".env"
	}
	if _, err := os.Stat(envFile); err == nil {
		// Attempt to load the env file (ignore error — if keys conflict, os.Getenv still takes precedence)
		_ = godotenv.Load(envFile)
	}

	cfg := &Config{
		Server: ServerConfig{
			Port: getEnvAsInt("SERVER_PORT", 8080),
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvAsInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			DBName:   getEnv("DB_NAME", "orbit"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvAsInt("REDIS_PORT", 6379),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
		},
		Auth: AuthConfig{
			JWTSecret:     getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
			JWTExpiration: getEnvAsInt("JWT_EXPIRATION_HOURS", 24),
		},
		Orbi: OrbiConfig{
			Host: getEnv("ORBI_HOST", "localhost"),
			Port: getEnvAsInt("ORBI_PORT", 50051),
		},
		GRPCServer: GRPCServerConfig{
			Port: getEnvAsInt("GRPC_SERVER_PORT", 50052),
		},
		Hashtag: HashtagConfig{
			Enabled: getEnvAsBool("HASHTAG_ENABLED", true),
			GRPC: HashtagGRPCConfig{
				Host:             getEnv("HASHTAG_GRPC_HOST", "localhost"),
				Port:             getEnvAsInt("HASHTAG_GRPC_PORT", 50051),
				Timeout:          getEnvAsInt("HASHTAG_GRPC_TIMEOUT", 5),
				MaxRetries:       getEnvAsInt("HASHTAG_GRPC_MAX_RETRIES", 3),
				KeepAlive:        getEnvAsInt("HASHTAG_GRPC_KEEP_ALIVE", 30),
				KeepAliveTimeout: getEnvAsInt("HASHTAG_GRPC_KEEP_ALIVE_TIMEOUT", 10),
			},
			Cache: HashtagCacheConfig{
				Enabled: getEnvAsBool("HASHTAG_CACHE_ENABLED", true),
				TTL:     getEnvAsInt("HASHTAG_CACHE_TTL", 5),
				MaxSize: getEnvAsInt("HASHTAG_CACHE_MAX_SIZE", 1000),
			},
		},
	}

	return cfg, nil
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt gets an environment variable as int or returns a default value
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}

	return value
}

// getEnvAsBool gets an environment variable as bool or returns a default value
func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}

	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}

	return value
}

// ConnectionString returns PostgreSQL connection string
func (c *DatabaseConfig) ConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}

// RedisAddr returns Redis address
func (c *RedisConfig) RedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
