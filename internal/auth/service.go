package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/logger"
	"golang.org/x/crypto/argon2"
)

// Service represents the Authentication Service
type Service struct {
	config *config.Config
	logger *logger.Logger
}

// NewService creates a new Authentication Service
func NewService(cfg *config.Config, log *logger.Logger) *Service {
	return &Service{
		config: cfg,
		logger: log,
	}
}

// RegisterRoutes registers authentication routes
func (s *Service) RegisterRoutes(router *mux.Router) {
	authRouter := router.PathPrefix("/auth").Subrouter()
	authRouter.HandleFunc("/register", s.register).Methods("POST")
	authRouter.HandleFunc("/login", s.login).Methods("POST")
	authRouter.HandleFunc("/verify", s.verify).Methods("POST")
}

// LoginRequest represents login request payload
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse represents login response
type LoginResponse struct {
	Token string `json:"token"`
	User  struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
}

// register handles user registration
func (s *Service) register(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	// Hash password using Argon2id
	_ = s.hashPassword(req.Password) // TODO: Store in database

	// TODO: Store user in PostgreSQL database
	s.logger.Info("User registered", "email", req.Email)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "user registered successfully"})
}

// login handles user login
func (s *Service) login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	// TODO: Verify credentials against PostgreSQL database

	// Generate JWT token
	token, err := s.generateJWT(req.Email)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to generate token"})
		return
	}

	response := LoginResponse{
		Token: token,
	}
	response.User.Email = req.Email
	response.User.ID = "user-123" // TODO: Get from database

	json.NewEncoder(w).Encode(response)
}

// verify handles JWT token verification
func (s *Service) verify(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	tokenString := r.Header.Get("Authorization")
	if tokenString == "" {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "no token provided"})
		return
	}

	// Parse and validate JWT
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.config.Auth.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid token"})
		return
	}

	json.NewEncoder(w).Encode(map[string]bool{"valid": true})
}

// generateJWT generates a JWT token for a user
func (s *Service) generateJWT(email string) (string, error) {
	claims := jwt.MapClaims{
		"email": email,
		"exp":   time.Now().Add(time.Hour * time.Duration(s.config.Auth.JWTExpiration)).Unix(),
		"iat":   time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.Auth.JWTSecret))
}

// hashPassword hashes a password using Argon2id
func (s *Service) hashPassword(password string) string {
	// Generate random salt
	salt := make([]byte, 16)
	rand.Read(salt)

	// Hash password with Argon2id
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)

	// Combine salt and hash for storage
	combined := append(salt, hash...)
	return base64.StdEncoding.EncodeToString(combined)
}

// verifyPassword verifies a password against a hash
func (s *Service) verifyPassword(password, hashedPassword string) bool {
	// Decode the stored hash
	decoded, err := base64.StdEncoding.DecodeString(hashedPassword)
	if err != nil {
		return false
	}

	// Extract salt and hash
	salt := decoded[:16]
	storedHash := decoded[16:]

	// Hash the provided password with the same salt
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)

	// Compare hashes
	if len(hash) != len(storedHash) {
		return false
	}

	for i := range hash {
		if hash[i] != storedHash[i] {
			return false
		}
	}

	return true
}
