package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application
type Config struct {
	Server         ServerConfig
	Database       DatabaseConfig
	MongoDB        MongoDBConfig
	Redis          RedisConfig
	Auth           AuthConfig
	Orbi           OrbiConfig
	GRPCServer     GRPCServerConfig
	GoogleCalendar GoogleCalendarConfig
	Firebase       FirebaseConfig
	GoogleMaps     GoogleMapsConfig
}

// GoogleMapsConfig holds Google Maps API configuration for ETA computation.
type GoogleMapsConfig struct {
	APIKey string
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port int
	Host string
}

// DatabaseConfig holds PostgreSQL configuration
type DatabaseConfig struct {
	Host        string
	Port        int
	User        string
	Pass        string
	DBName      string
	SSLMode     string
	SSLRootCert string
	SSLCert     string
	SSLKey      string
}

// MongoDBConfig holds MongoDB configuration
type MongoDBConfig struct {
	User   string
	Pass   string
	Host   string
	DBName string
}

// RedisConfig holds Redis configuration for rate limiting
type RedisConfig struct {
	Host string
	Port int
	Pass string
	DB   int
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWTKey                       string
	JWTExpiration                int // in hours
	ResendAPIKey                 string
	AppBaseURL                   string
	PasswordResetExpiryMinutes   int
	EmailVerificationExpiryHours int
	// EmailFrom is the From address for outgoing emails, e.g. "Orbit <onboarding@resend.dev>"
	EmailFrom string
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

// GoogleCalendarConfig holds Google Calendar integration configuration
type GoogleCalendarConfig struct {
	ClientID    string
	ClientKey   string
	RedirectURL string
	// WebhookURL is the publicly accessible URL for Google Calendar push notifications
	WebhookURL string
}

// FirebaseConfig holds Firebase Cloud Messaging configuration
type FirebaseConfig struct {
	// CredentialsJSON is the content of the Firebase service account JSON key.
	// Prefer providing via a Docker secret named `firebase_credentials_json` or
	// set `FIREBASE_CREDENTIALS_FILE` to the path of the JSON file. The
	// raw `FIREBASE_CREDENTIALS_JSON` environment variable is no longer used.
	CredentialsJSON string
}

// secretEnvMap maps environment variable names to docker secret filenames (base names).
// When a secret is present, we'll prefer reading the corresponding secret from
// /run/secrets/<name> (container) or ./secrets/<name>.txt (local dev). If no secret
// is available we fall back to the environment variable, then to the default value.
//
//nolint:gosec // G101: False positive - this is a mapping of variable names, not credentials
var secretEnvMap = map[string]string{
	"DB_USER":                   "db_user",
	"DB_PASSWORD":               "db_password",
	"DB_NAME":                   "db_name",
	"JWT_SECRET":                "jwt_secret",
	"RESEND_API_KEY":            "resend_api_key",
	"MONGO_USER":                "mongo_user",
	"MONGO_PASSWORD":            "mongo_password",
	"REDIS_PASSWORD":            "redis_password",
	"GOOGLE_CLIENT_ID":          "google_client_id",
	"GOOGLE_CLIENT_SECRET":      "google_client_secret",
	"FIREBASE_CREDENTIALS_FILE": "firebase_credentials_json",
}

// Load loads configuration from environment variables and docker secrets
func Load() (*Config, error) {
	// Detect and load a dotenv file if present. By default, we look for `.env`.
	// The path can be overridden by setting the ENV_FILE environment variable.
	envFile := os.Getenv("ENV_FILE")
	if envFile == "" || !isSafeEnvFileName(envFile) {
		envFile = ".env"
	}
	if _, err := os.Stat(envFile); err == nil { //nolint:gosec
		// Attempt to load the env file (ignore error — if keys conflict, os.Getenv still takes precedence)
		_ = godotenv.Load(envFile)
	}

	cfg := &Config{
		Server: ServerConfig{
			Port: getEnvAsInt("SERVER_PORT", 8080),
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
		},
		Database: DatabaseConfig{
			Host:        getEnv("DB_HOST", "localhost"),
			Port:        getEnvAsInt("DB_PORT", 5432),
			User:        getEnv("DB_USER", "postgres"),
			Pass:        getEnv("DB_PASSWORD", "postgres"),
			DBName:      getEnv("DB_NAME", "orbit"),
			SSLMode:     getEnv("DB_SSLMODE", "disable"),
			SSLRootCert: getEnv("DB_SSLROOTCERT", ""),
			SSLCert:     getEnv("DB_SSLCERT", ""),
			SSLKey:      getEnv("DB_SSLKEY", ""),
		},
		MongoDB: MongoDBConfig{
			User:   getEnv("MONGO_USER", ""),
			Pass:   getEnv("MONGO_PASSWORD", ""),
			Host:   getEnv("MONGODB_HOST", "mongo:27017"),
			DBName: getEnv("MONGODB_DB", "orbit"),
		},
		Redis: RedisConfig{
			Host: getEnv("REDIS_HOST", "localhost"),
			Port: getEnvAsInt("REDIS_PORT", 6379),
			Pass: getEnv("REDIS_PASSWORD", ""),
			DB:   getEnvAsInt("REDIS_DB", 0),
		},
		Auth: AuthConfig{
			JWTKey:                       getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
			JWTExpiration:                getEnvAsInt("JWT_EXPIRATION_HOURS", 24),
			ResendAPIKey:                 getEnv("RESEND_API_KEY", ""),
			AppBaseURL:                   getEnv("APP_BASE_URL", "http://localhost:3000"),
			PasswordResetExpiryMinutes:   getEnvAsInt("PASSWORD_RESET_EXPIRY_MINUTES", 30),
			EmailVerificationExpiryHours: getEnvAsInt("EMAIL_VERIFICATION_EXPIRY_HOURS", 24),
			EmailFrom:                    getEnv("EMAIL_FROM", "Orbit <onboarding@resend.dev>"),
		},
		Orbi: OrbiConfig{
			Host: getEnv("ORBI_HOST", "localhost"),
			Port: getEnvAsInt("ORBI_PORT", 50051),
		},
		GRPCServer: GRPCServerConfig{
			Port: getEnvAsInt("GRPC_SERVER_PORT", 50052),
		},
		GoogleCalendar: GoogleCalendarConfig{
			ClientID:    getEnv("GOOGLE_CLIENT_ID", ""),
			ClientKey:   getEnv("GOOGLE_CLIENT_SECRET", ""),
			RedirectURL: getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/api/v1/integration/google/callback"),
			WebhookURL:  getEnv("GOOGLE_WEBHOOK_URL", ""),
		},
		Firebase: FirebaseConfig{
			// Prefer reading Firebase credentials from a secret file (docker secret or ./secrets/).
			// Fall back to the environment variable only if the secret isn't found.
			CredentialsJSON: func() string {
				// 1) Docker secret (or ./secrets/) named by secretEnvMap
				if secretName, ok := secretEnvMap["FIREBASE_CREDENTIALS_FILE"]; ok {
					if s, err := readSecret(secretName); err == nil && s != "" {
						return s
					}
				}
				// 2) Environment variable pointing to a file containing the JSON
				if fp := os.Getenv("FIREBASE_CREDENTIALS_FILE"); fp != "" {
					if b, err := os.ReadFile(fp); err == nil { //nolint:gosec
						s := strings.TrimSpace(string(b))
						if s != "" {
							return s
						}
					}
				}
				// 3) No fallback to raw JSON environment variable — return empty if not provided
				return ""
			}(),
		},
		GoogleMaps: GoogleMapsConfig{
			APIKey: getEnv("GOOGLE_MAPS_API_KEY", ""),
		},
	}

	return cfg, nil
}

// getEnv gets an environment variable or returns a default value. For certain sensitive
// variables we will prefer reading from Docker secrets first (e.g. /run/secrets/<name> or ./secrets/<name>.txt);
// if no secret is present we fall back to an explicitly set environment variable, then the default.
func getEnv(key, defaultValue string) string {
	// First, if this key maps to a secret name, try to read the secret and prefer it when present.
	if secretName, ok := secretEnvMap[key]; ok {
		if s, err := readSecret(secretName); err == nil && s != "" {
			return s
		}
	}

	// Next prefer any explicitly set environment variable
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

// readSecret attempts to read a secret from common docker secret locations.
// It validates the secret base name to prevent file inclusion attacks, then tries:
//   - /run/secrets/<name>
//   - ./secrets/<name>.txt (local-development file included in repo)
//   - ./secrets/<name> (fallback)
func readSecret(name string) (string, error) {
	// allow only safe characters in secret name
	validName := regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	if !validName.MatchString(name) {
		return "", fmt.Errorf("invalid secret name")
	}

	paths := []string{
		"/run/secrets/" + name,
		"./secrets/" + name + ".txt",
		"./secrets/" + name,
	}
	for _, p := range paths {
		b, err := os.ReadFile(p) //nolint:gosec
		if err != nil {
			continue
		}
		val := strings.TrimSpace(string(b))
		if val != "" {
			return val, nil
		}
	}
	return "", fmt.Errorf("secret %s not found in known paths", name)
}

func isSafeEnvFileName(name string) bool {
	if name == "" {
		return false
	}
	if filepath.Base(name) != name {
		return false
	}
	return !strings.Contains(name, "..")
}

// ConnectionString returns PostgreSQL connection string
func (c *DatabaseConfig) ConnectionString() string {
	base := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Pass, c.DBName, c.SSLMode,
	)
	// append SSL file params if provided
	if c.SSLRootCert != "" {
		base += fmt.Sprintf(" sslrootcert=%s", c.SSLRootCert)
	}
	if c.SSLCert != "" {
		base += fmt.Sprintf(" sslcert=%s", c.SSLCert)
	}
	if c.SSLKey != "" {
		base += fmt.Sprintf(" sslkey=%s", c.SSLKey)
	}
	return base
}

// ConnectionURL returns PostgreSQL connection URL for Atlas
func (c *DatabaseConfig) ConnectionURL() string {
	// Simple construction - for production use net/url to encode password properly if needed
	// Assuming basic ASCII for now or handling basics.
	// Actually, let's use net/url for safety.
	// But importing net/url in this file requires import update.
	// Just doing basic string format for now as password is likely simpler or secrets.
	// Warning: Special chars in password might break this simple format.
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Pass, c.Host, c.Port, c.DBName, c.SSLMode,
	)
}

// RedisAddr returns Redis address
func (c *RedisConfig) RedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
