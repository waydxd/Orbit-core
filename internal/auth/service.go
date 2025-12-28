package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"github.com/resend/resend-go/v3"
	"golang.org/x/crypto/argon2"

	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/logger"
)

//go:embed templates/*.html
var emailTemplates embed.FS

// Service represents the Authentication Service
type Service struct {
	config       *config.Config
	logger       *logger.Logger
	repo         Repository
	redisClient  *redis.Client
	resendClient *resend.Client
}

// NewService creates a new Authentication Service
func NewService(cfg *config.Config, log *logger.Logger, repo Repository) *Service {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.RedisAddr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	var resendClient *resend.Client
	if cfg.Auth.ResendAPIKey != "" {
		resendClient = resend.NewClient(cfg.Auth.ResendAPIKey)
	}

	return &Service{
		config:       cfg,
		logger:       log,
		repo:         repo,
		redisClient:  redisClient,
		resendClient: resendClient,
	}
}

// RegisterRoutes registers authentication routes
func (s *Service) RegisterRoutes(router *mux.Router) {
	authRouter := router.PathPrefix("/auth").Subrouter()
	authRouter.HandleFunc("/register", s.register).Methods("POST")
	authRouter.HandleFunc("/login", s.login).Methods("POST")
	authRouter.HandleFunc("/verify", s.verify).Methods("POST")
	authRouter.HandleFunc("/logout", s.logout).Methods("POST")
	authRouter.HandleFunc("/password-reset-request", s.passwordResetRequest).Methods("POST")
	authRouter.HandleFunc("/password-reset-confirm", s.passwordResetConfirm).Methods("POST")
	authRouter.HandleFunc("/verify-email", s.verifyEmail).Methods("GET")
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

	// Generate email verification token
	verifyToken, err := s.generateSecureToken()
	if err == nil {
		tokenHash := s.hashToken(verifyToken)
		redisKey := fmt.Sprintf("auth:token:email_verification:%s", tokenHash)
		ttl := time.Duration(s.config.Auth.EmailVerificationExpiryHours) * time.Hour

		if err := s.redisClient.Set(ctx, redisKey, user.ID, ttl).Err(); err != nil {
			s.logger.Error("failed to store verification token in redis", "err", err)
		} else {
			verifyLink := fmt.Sprintf("%s/auth/verify-email?token=%s", s.config.Auth.AppBaseURL, verifyToken)
			if err := s.sendEmail(user.Email, "Verify your email", "email-verification", map[string]interface{}{
				"verify_link": verifyLink,
				"first_name":  user.FirstName,
			}); err != nil {
				s.logger.Error("failed to send verification email", "err", err)
			}
		}
	} else {
		s.logger.Error("failed to generate verification token", "err", err)
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

// passwordResetRequest handles password reset requests
func (s *Service) passwordResetRequest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		s.logger.Error("invalid password reset request", "err", err)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Always return success to prevent email enumeration
	defer func() {
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "if the email exists, a reset link has been sent"})
	}()

	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		// User not found, do nothing (or log debug)
		return
	}

	// Rate limit check (optional, skipping for brevity as per plan instructions to focus on core logic, but good to have)

	// Generate secure random token
	token, err := s.generateSecureToken()
	if err != nil {
		s.logger.Error("failed to generate reset token", "err", err)
		return
	}

	tokenHash := s.hashToken(token)
	redisKey := fmt.Sprintf("auth:token:password_reset:%s", tokenHash)
	ttl := time.Duration(s.config.Auth.PasswordResetExpiryMinutes) * time.Minute

	if err := s.redisClient.Set(ctx, redisKey, user.ID, ttl).Err(); err != nil {
		s.logger.Error("failed to store reset token in redis", "err", err)
		return
	}

	// Send reset email
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", s.config.Auth.AppBaseURL, token)
	if err := s.sendEmail(user.Email, "Password Reset Request", "password-reset", map[string]interface{}{
		"reset_link": resetLink,
		"first_name": user.FirstName,
	}); err != nil {
		s.logger.Error("failed to send password reset email", "err", err)
	}
}

// passwordResetConfirm handles password reset confirmations
func (s *Service) passwordResetConfirm(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		s.logger.Error("invalid password reset confirmation", "err", err)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	tokenHash := s.hashToken(req.Token)
	redisKey := fmt.Sprintf("auth:token:password_reset:%s", tokenHash)

	userID, err := s.redisClient.Get(ctx, redisKey).Result()
	if errors.Is(err, redis.Nil) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid or expired token"})
		return
	} else if err != nil {
		s.logger.Error("redis error", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return
	}

	// Delete token (single-use)
	s.redisClient.Del(ctx, redisKey)

	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		s.logger.Error("user not found", "id", userID, "err", err)
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "user not found"})
		return
	}

	// Update password
	passwordHash := s.hashPassword(req.Password)
	user.PasswordHash = passwordHash
	if err := s.repo.UpdateUser(ctx, user); err != nil {
		s.logger.Error("failed to update user", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to update password"})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]string{"message": "password reset successfully"})
}

// verifyEmail handles email verification
func (s *Service) verifyEmail(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	token := r.URL.Query().Get("token")
	if token == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "missing token"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	tokenHash := s.hashToken(token)
	redisKey := fmt.Sprintf("auth:token:email_verification:%s", tokenHash)

	userID, err := s.redisClient.Get(ctx, redisKey).Result()
	if errors.Is(err, redis.Nil) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid or expired token"})
		return
	} else if err != nil {
		s.logger.Error("redis error", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return
	}

	// Delete token (single-use)
	s.redisClient.Del(ctx, redisKey)

	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		s.logger.Error("user not found", "id", userID, "err", err)
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "user not found"})
		return
	}

	// Update email verified status
	user.EmailVerified = true
	if err := s.repo.UpdateUser(ctx, user); err != nil {
		s.logger.Error("failed to update user", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to verify email"})
		return
	}

	// Redirect to frontend success page or return JSON
	// For now, return JSON
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "email verified successfully"})
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

// generateSecureToken generates a cryptographically secure random token
func (s *Service) generateSecureToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// sendEmail sends an email using Resend or local HTML templates
func (s *Service) sendEmail(to, subject, templateID string, data map[string]interface{}) error {
	if s.resendClient == nil {
		s.logger.Warn("resend client not initialized, skipping email", "to", to, "subject", subject)
		return nil
	}

	// Try to render a local HTML template if available
	var htmlBody string
	var templatePath string
	switch templateID {
	case "password-reset":
		templatePath = "templates/passwordReset.html"
	case "email-verification":
		templatePath = "templates/emailVerification.html"
	}

	if templatePath != "" {
		rendered, err := s.renderHTMLTemplate(templatePath, data)
		if err != nil {
			s.logger.Error("failed to render email template", "err", err, "template", templatePath)
		} else {
			htmlBody = rendered
		}
	}

	// Fallback simple HTML if no template rendered
	if htmlBody == "" {
		switch templateID {
		case "password-reset":
			if link, ok := data["reset_link"].(string); ok {
				htmlBody = fmt.Sprintf(`<p>Click <a href="%s">here</a> to reset your password.</p>`, link)
			} else {
				htmlBody = "<p>Notification from Orbit</p>"
			}
		case "email-verification":
			if link, ok := data["verify_link"].(string); ok {
				htmlBody = fmt.Sprintf(`<p>Click <a href="%s">here</a> to verify your email.</p>`, link)
			} else {
				htmlBody = "<p>Notification from Orbit</p>"
			}
		default:
			htmlBody = "<p>Notification from Orbit</p>"
		}
	}

	params := &resend.SendEmailRequest{
		From:    s.config.Auth.EmailFrom,
		To:      []string{to},
		Subject: subject,
	}

	params.Html = htmlBody

	_, err := s.resendClient.Emails.Send(params)
	return err
}

// renderHTMLTemplate loads an HTML file and executes it as a Go template with provided data
func (s *Service) renderHTMLTemplate(path string, data map[string]interface{}) (string, error) {
	// sanitize and restrict to embedded templates/ templates
	clean := filepath.Clean(path)
	if strings.Contains(clean, "..") || !strings.HasPrefix(clean, "templates/") {
		return "", fmt.Errorf("invalid template path")
	}

	b, err := emailTemplates.ReadFile(clean)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New(filepath.Base(clean)).Parse(string(b))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
