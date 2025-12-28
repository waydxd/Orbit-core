package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/mail"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/golang-jwt/jwt/v5"
	"github.com/resend/resend-go/v3"
	"golang.org/x/crypto/argon2"
)

//go:embed templates/*.html
var emailTemplates embed.FS

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
	// sanitize and restrict to embedded templates/
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

// validateEmail verifies email format using net/mail
func (s *Service) validateEmail(email string) bool {
	if strings.TrimSpace(email) == "" {
		return false
	}
	_, err := mail.ParseAddress(email)
	return err == nil
}

// validatePassword enforces a minimal password policy: at least 8 chars, and must contain at least one letter, one number, and one special character (all three required)
func (s *Service) validatePassword(pw string) bool {
	// Disallow any whitespace characters
	for _, r := range pw {
		if unicode.IsSpace(r) {
			return false
		}
	}
	if len(pw) < 8 {
		return false
	}
	var hasLetter, hasNumber, hasSpecial bool
	for _, r := range pw {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasNumber = true
		default:
			// Any non-letter, non-digit counts as special
			hasSpecial = true
		}
		if hasLetter && hasNumber && hasSpecial {
			return true
		}
	}

	return hasLetter && hasNumber && hasSpecial
}

// equalizeResponseDelay performs a short, context-aware delay to reduce timing
// differences between fast-failing paths (e.g., missing Redis token) and
// slower paths (e.g., DB lookups). Duration is intentionally small to avoid
// creating a DoS amplification vector but large enough to make timing attacks harder.
func (s *Service) equalizeResponseDelay(ctx context.Context) {
	const delay = 300 * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}
