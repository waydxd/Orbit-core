package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"github.com/resend/resend-go/v3"

	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/logger"
)

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
		Password: cfg.Redis.Pass,
		DB:       cfg.Redis.DB,
	})

	// Validate Redis connectivity at startup to avoid runtime failures in
	// email verification or password reset flows.
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		panic(fmt.Sprintf("auth service: failed to connect to Redis at %s: %v", cfg.Redis.RedisAddr(), err))
	}
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

// RegisterRoutes registers authentication routes (public routes)
func (s *Service) RegisterRoutes(router *mux.Router) {
	authRouter := router.PathPrefix("/auth").Subrouter()
	authRouter.HandleFunc("/register", s.register).Methods("POST")
	authRouter.HandleFunc("/login", s.login).Methods("POST")
	authRouter.HandleFunc("/verify", s.verify).Methods("POST")
	authRouter.HandleFunc("/logout", s.logout).Methods("POST")
	authRouter.HandleFunc("/password-reset-request", s.passwordResetRequest).Methods("POST")
	authRouter.HandleFunc("/password-reset-confirm", s.passwordResetConfirm).Methods("POST")
	authRouter.HandleFunc("/verify-email", s.verifyEmail).Methods("GET")
	authRouter.HandleFunc("/reset-password", s.resetPasswordPage).Methods("GET")
}

// RegisterProtectedRoutes registers protected authentication routes (requires authentication)
func (s *Service) RegisterProtectedRoutes(router *mux.Router) {
	// Profile routes (protected)
	router.HandleFunc("/profile", s.getProfile).Methods("GET")
	router.HandleFunc("/profile", s.updateProfile).Methods("PUT")
}

// LoginRequest represents login/register request payload
type LoginRequest struct {
	email    string
	password string
}

func (r *LoginRequest) UnmarshalJSON(data []byte) error {
	var aux map[string]string
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	r.email = aux["email"]
	r.password = aux["password"]
	return nil
}

func (r LoginRequest) Email() string {
	return r.email
}

func (r LoginRequest) Password() string {
	return r.password
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

	// Input validation
	if req.Email() == "" || !s.validateEmail(req.Email()) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid email"})
		return
	}
	if req.Password() == "" || !s.validatePassword(req.Password()) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "password does not meet requirements (min 8 chars, must include letters, numbers, and special characters; no spaces)"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Check if user already exists
	existingUser, _ := s.repo.GetUserByEmail(ctx, req.Email())
	if existingUser != nil {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "user already exists"})
		return
	}

	passwordHash := s.hashPassword(req.Password())
	user := &models.User{
		ID:           uuid.New().String(),
		Email:        req.Email(),
		PasswordHash: passwordHash,
		Username:     GenerateRandomUsername(),
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
			verifyLink := fmt.Sprintf("%s/api/v1/auth/verify-email?token=%s", s.config.Auth.AppBaseURL, verifyToken)
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

	// Input validation
	if req.Email() == "" || !s.validateEmail(req.Email()) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid email"})
		return
	}
	if req.Password() == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "password required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	user, err := s.repo.GetUserByEmail(ctx, req.Email())
	if err != nil {
		s.logger.Error("user not found", "email", req.Email(), "err", err)
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials"})
		return
	}

	if !s.verifyPassword(req.Password(), user.PasswordHash) {
		s.logger.Error("invalid password", "email", req.Email())
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
		return []byte(s.config.Auth.JWTKey), nil
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

	// Create a request-scoped context early so we can equalize response timing
	// for invalid inputs to mitigate timing-based enumeration attacks.
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Ensure Redis is available before proceeding. If Redis is nil or unavailable
	// we should surface service-unavailable so the client knows the reset cannot
	// be performed (avoids silently claiming success while no token/email will be sent).
	if s.redisClient == nil {
		s.logger.Error("redis client is not initialized")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "service unavailable"})
		return
	}
	if err := s.redisClient.Ping(ctx).Err(); err != nil {
		s.logger.Error("redis unavailable", "err", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "service unavailable"})
		return
	}

	// Validate email; if invalid, perform a short, context-aware delay so the
	// response timing is closer to the path that does a DB lookup. Then return
	// the same generic success message to avoid email enumeration.
	if req.Email == "" || !s.validateEmail(req.Email) {
		s.equalizeResponseDelay(ctx)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "if the email exists, a reset link has been sent"})
		return
	}

	// Always return success to prevent email enumeration
	defer func() {
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "if the email exists, a reset link has been sent"})
	}()

	// Basic per-email rate limiting for password reset requests — do this before
	// the DB lookup so attackers can't distinguish existing vs non-existing
	// emails by timing whether the rate limiter was applied.
	rateLimitKey := fmt.Sprintf("auth:rl:password_reset:%s", strings.ToLower(req.Email))
	const maxPasswordResetRequests = int64(5)
	const passwordResetRateWindow = time.Hour

	reqCount, rlErr := s.redisClient.Incr(ctx, rateLimitKey).Result()
	if rlErr != nil {
		// If we cannot reliably track rate limits, avoid sending emails to prevent abuse
		s.logger.Error("failed to apply password reset rate limit", "err", rlErr)
		return
	}

	if reqCount == 1 {
		// Set the window only on first increment
		if err := s.redisClient.Expire(ctx, rateLimitKey, passwordResetRateWindow).Err(); err != nil {
			s.logger.Error("failed to set password reset rate limit expiry", "err", err)
			return
		}
	}

	if reqCount > maxPasswordResetRequests {
		// Rate limit exceeded; do not generate a token or send another email
		s.logger.Warn("password reset rate limit exceeded", "email", req.Email, "count", reqCount)
		return
	}

	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		// User not found, do nothing (or log debug)
		return
	}

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

	// Build the reset link. Use the configured page URL when provided; otherwise
	// fall back to the built-in HTML form served at /api/v1/auth/reset-password.
	resetPageURL := strings.TrimSpace(s.config.Auth.PasswordResetPageURL)
	if resetPageURL == "" {
		resetPageURL = strings.TrimRight(s.config.Auth.AppBaseURL, "/") + "/api/v1/auth/reset-password"
	}

	parsedResetURL, err := url.Parse(resetPageURL)
	if err != nil {
		s.logger.Error("failed to parse password reset page url", "url", resetPageURL, "err", err)
		return
	}

	query := parsedResetURL.Query()
	query.Set("token", token)
	parsedResetURL.RawQuery = query.Encode()
	resetLink := parsedResetURL.String()
	if err := s.sendEmail(user.Email, "Password Reset Request", "password-reset", map[string]interface{}{
		"reset_link":         resetLink,
		"first_name":         user.FirstName,
		"support_center_url": s.config.Auth.AppBaseURL,
	}); err != nil {
		s.logger.Error("failed to send password reset email", "err", err)
	}
}

// passwordResetConfirm handles password reset confirmations
func (s *Service) passwordResetConfirm(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var payload map[string]string
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		s.logger.Error("invalid password reset confirmation", "err", err)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	token := payload["token"]
	password := payload["password"]

	// Validate password
	if password == "" || !s.validatePassword(password) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "password does not meet requirements (min 8 chars, must include letters, numbers, and special characters; no spaces)"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	tokenHash := s.hashToken(token)
	redisKey := fmt.Sprintf("auth:token:password_reset:%s", tokenHash)

	userID, err := s.redisClient.Get(ctx, redisKey).Result()
	if errors.Is(err, redis.Nil) {
		// Mitigate timing attacks by performing a short, consistent delay so an
		// attacker can't distinguish between an invalid token and other failures
		// based on response time.
		s.equalizeResponseDelay(ctx)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid or expired token"})
		return
	} else if err != nil {
		s.logger.Error("redis error", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return
	}

	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		s.logger.Error("user not found", "id", userID, "err", err)
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "user not found"})
		return
	}

	// Security: prevent reusing the same password
	if s.verifyPassword(password, user.PasswordHash) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "new password must be different from the old password"})
		return
	}

	// Update password
	user.PasswordHash = s.hashPassword(password)
	if err := s.repo.UpdateUser(ctx, user); err != nil {
		s.logger.Error("failed to update user", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to update password"})
		return
	}

	// Delete token (single-use) only after successful DB update. Log but do not fail
	if err := s.redisClient.Del(ctx, redisKey).Err(); err != nil {
		s.logger.Error("failed to delete password reset token from redis", "err", err, "key", redisKey)
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
		// Mitigate timing attacks similarly for email verification
		s.equalizeResponseDelay(ctx)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid or expired token"})
		return
	} else if err != nil {
		s.logger.Error("redis error", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return
	}

	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		s.logger.Error("user not found", "id", userID, "err", err)
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "user not found"})
		return
	}

	// Idempotency: if email already verified, avoid unnecessary DB update.
	if user.EmailVerified {
		// Token is single-use; delete it now to avoid reuse. Log any deletion error.
		if err := s.redisClient.Del(ctx, redisKey).Err(); err != nil {
			s.logger.Error("failed to delete email verification token from redis (idempotent path)", "err", err, "key", redisKey)
		}
		// Redirect to frontend success page (no DB update required). Use APP_BASE_URL for consistency with verification link.
		redirectURL := strings.TrimRight(s.config.Auth.AppBaseURL, "/") + "/email-verified"
		http.Redirect(w, r, redirectURL, http.StatusSeeOther)
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

	// Delete token (single-use) only after successful DB update. Log but do not fail
	if err := s.redisClient.Del(ctx, redisKey).Err(); err != nil {
		s.logger.Error("failed to delete email verification token from redis", "err", err, "key", redisKey)
	}

	// Redirect to frontend success page after successful verification
	redirectURL := strings.TrimRight(s.config.Auth.AppBaseURL, "/") + "/email-verified"
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// resetPasswordPage serves the HTML form that lets a user choose a new password.
// It is linked from the password-reset email (GET /auth/reset-password?token=…).
func (s *Service) resetPasswordPage(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("<html><body><p>Missing or invalid reset token.</p></body></html>"))
		return
	}

	rendered, err := s.renderHTMLTemplate("templates/passwordResetForm.html", map[string]interface{}{
		"token": token,
	})
	if err != nil {
		s.logger.Error("failed to render password reset form", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(rendered))
}

// Helper functions (password hashing, token helpers, email rendering/sending,
// validation, delay etc.) have been moved to internal/auth/helpers.go to
// keep this file focused on the Service handlers and wiring.
