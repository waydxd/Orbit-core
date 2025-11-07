package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/argon2"

	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/logger"
)

// Service represents the Authentication Service
type Service struct {
	config *config.Config
	logger *logger.Logger
	repo   Repository
}

// NewService creates a new Authentication Service
func NewService(cfg *config.Config, log *logger.Logger, repo Repository) *Service {
	return &Service{
		config: cfg,
		logger: log,
		repo:   repo,
	}
}

// RegisterRoutes registers authentication routes
func (s *Service) RegisterRoutes(router *mux.Router) {
	authRouter := router.PathPrefix("/auth").Subrouter()
	authRouter.HandleFunc("/register", s.register).Methods("POST")
	authRouter.HandleFunc("/login", s.login).Methods("POST")
	authRouter.HandleFunc("/verify", s.verify).Methods("POST")
	authRouter.HandleFunc("/logout", s.logout).Methods("POST")
}

// LoginRequest represents login/register request payload
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

func (s *Service) register(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		s.logger.Error("invalid register request", "err", err)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Check if user already exists
	existingUser, _ := s.repo.GetUserByEmail(ctx, req.Email)
	if existingUser != nil {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "user already exists"})
		return
	}

	passwordHash := s.hashPassword(req.Password)
	user := &models.User{
		ID:           uuid.New().String(),
		Email:        req.Email,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		s.logger.Error("failed to create user", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to register user"})
		return
	}

	// generate token and save session
	token, err := s.generateJWT(user.Email, user.ID)
	if err != nil {
		s.logger.Error("failed to generate token", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to generate token"})
		return
	}

	tokenHash := s.hashToken(token)
	expiresAt := time.Now().Add(time.Duration(s.config.Auth.JWTExpiration) * time.Hour)
	if _, err := s.repo.SaveSession(ctx, user.ID, tokenHash, expiresAt); err != nil {
		s.logger.Error("failed to save session", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to create session"})
		return
	}

	resp := LoginResponse{Token: token}
	resp.User.ID = user.ID
	resp.User.Email = user.Email

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// login handles user login
func (s *Service) login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		s.logger.Error("invalid login request", "err", err)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		s.logger.Error("user not found", "email", req.Email, "err", err)
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials"})
		return
	}

	if !s.verifyPassword(req.Password, user.PasswordHash) {
		s.logger.Error("invalid password", "email", req.Email)
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials"})
		return
	}

	// generate token and save session
	token, err := s.generateJWT(user.Email, user.ID)
	if err != nil {
		s.logger.Error("failed to generate token", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to generate token"})
		return
	}

	tokenHash := s.hashToken(token)
	expiresAt := time.Now().Add(time.Duration(s.config.Auth.JWTExpiration) * time.Hour)
	if _, err := s.repo.SaveSession(ctx, user.ID, tokenHash, expiresAt); err != nil {
		s.logger.Error("failed to save session", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to create session"})
		return
	}

	resp := LoginResponse{Token: token}
	resp.User.ID = user.ID
	resp.User.Email = user.Email
	_ = json.NewEncoder(w).Encode(resp)
}

// logout handles user logout
func (s *Service) logout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	tokenString := extractBearerToken(r.Header.Get("Authorization"))
	if tokenString == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "no token provided"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	tokenHash := s.hashToken(tokenString)
	session, err := s.repo.GetSessionByToken(ctx, tokenHash)
	if err != nil {
		s.logger.Error("session not found", "err", err)
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid session"})
		return
	}

	if err := s.repo.DeleteSession(ctx, session.ID); err != nil {
		s.logger.Error("failed to delete session", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to logout"})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]string{"message": "logged out successfully"})
}

// verify handles JWT token verification
func (s *Service) verify(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	tokenString := extractBearerToken(r.Header.Get("Authorization"))
	if tokenString == "" {
		w.WriteHeader(http.StatusUnauthorized)
		s.logger.Error("no token provided")
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "no token provided"})
		return
	}

	// Parse and validate JWT
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.config.Auth.JWTSecret), nil
	})

	if err != nil || !token.Valid {
		w.WriteHeader(http.StatusUnauthorized)
		s.logger.Error("invalid token", "err", err)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid token"})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]bool{"valid": true})
}

// generateJWT generates a JWT token for a user
func (s *Service) generateJWT(email, userID string) (string, error) {
	claims := jwt.MapClaims{
		"email": email,
		"id":    userID,
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
	_, err := rand.Read(salt)
	if err != nil {
		s.logger.Error("failed to read random salt", "err", err)
		return ""
	}

	// Hash password with Argon2id
	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)

	// Combine salt and hash for storage
	salt = append(salt, hash...)
	return base64.StdEncoding.EncodeToString(salt)
}

// verifyPassword verifies a password against a stored Argon2id hash
func (s *Service) verifyPassword(password, hashedPassword string) bool {
	decoded, err := base64.StdEncoding.DecodeString(hashedPassword)
	if err != nil {
		s.logger.Error("failed to decode stored password", "err", err)
		return false
	}

	if len(decoded) < 16 {
		s.logger.Error("invalid hashed password length", "len", len(decoded))
		return false
	}

	salt := decoded[:16]
	storedHash := decoded[16:]

	computed := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	if len(computed) != len(storedHash) {
		s.logger.Error("hashed lengths differ during password verification", "expected", len(storedHash), "got", len(computed))
		return false
	}

	// constant time compare
	if subtle.ConstantTimeCompare(computed, storedHash) != 1 {
		s.logger.Info("password verification failed")
		return false
	}
	return true
}

// hashToken creates a SHA-256 hash of the token for storage
func (s *Service) hashToken(token string) string {
	sum := sha256.Sum256([]byte(token + s.config.Auth.JWTSecret))
	return fmt.Sprintf("%x", sum)
}

// extractBearerToken strips optional "Bearer " prefix from Authorization header
func extractBearerToken(header string) string {
	header = strings.TrimSpace(header)
	if header == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return header
}
