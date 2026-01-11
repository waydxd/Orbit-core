package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Mock implementation of JWT secret for testing purposes
const testJWTSecret = "my-super-secret-testing-key-that-should-be-long-enough"

// TestGeneratePasswordHash tests the GeneratePasswordHash function.
func TestGeneratePasswordHash(t *testing.T) {
	password := "securePassword123"
	hash, err := GeneratePasswordHash(password)
	if err != nil {
		t.Fatalf("GeneratePasswordHash failed: %v", err)
	}
	if hash == "" {
		t.Error("GeneratePasswordHash returned an empty hash")
	}

	// Verify that the generated hash is different each time (due to salt)
	hash2, err := GeneratePasswordHash(password)
	if err != nil {
		t.Fatalf("GeneratePasswordHash failed on second call: %v", err)
	}
	if hash == hash2 {
		t.Error("GeneratePasswordHash returned the same hash for the same password, expected different hashes due to salt")
	}

	// Test with empty password
	_, err = GeneratePasswordHash("")
	if err == nil {
		t.Error("GeneratePasswordHash with empty password should return an error, but got nil")
	}
	// Check if the error is related to bcrypt's error for empty passwords.
	// bcrypt.GenerateFromPassword returns "crypto/bcrypt: Password and salt must be at least 16 bytes long" for empty password.
	if !errors.Is(err, bcrypt.ErrHashTooShort) && err.Error() != "Password and salt must be at least 16 bytes long" {
		t.Errorf("GeneratePasswordHash with empty password returned unexpected error: %v", err)
	}
}

// TestComparePasswordHash tests the ComparePasswordHash function.
func TestComparePasswordHash(t *testing.T) {
	password := "verifiablePassword456"
	hash, _ := GeneratePasswordHash(password) // Already tested GeneratePasswordHash

	// Test case 1: Correct password
	err := ComparePasswordHash(password, hash)
	if err != nil {
		t.Errorf("ComparePasswordHash with correct password failed: %v", err)
	}

	// Test case 2: Incorrect password
	wrongPassword := "wrongPassword789"
	err = ComparePasswordHash(wrongPassword, hash)
	if err == nil {
		t.Error("ComparePasswordHash with incorrect password should return an error, but got nil")
	}
	// bcrypt.CompareHashAndPassword returns "crypto/bcrypt: crypto/subtle: inputs not equal"
	if !errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) && err.Error() != "crypto/bcrypt: crypto/subtle: inputs not equal" {
		t.Errorf("ComparePasswordHash with incorrect password returned unexpected error: %v", err)
	}

	// Test case 3: Empty password
	err = ComparePasswordHash("", hash)
	if err == nil {
		t.Error("ComparePasswordHash with empty password should return an error, but got nil")
	}
	if !errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) && err.Error() != "crypto/bcrypt: crypto/subtle: inputs not equal" {
		t.Errorf("ComparePasswordHash with empty password returned unexpected error: %v", err)
	}

	// Test case 4: Empty hash
	err = ComparePasswordHash(password, "")
	if err == nil {
		t.Error("ComparePasswordHash with empty hash should return an error, but got nil")
	}
	// bcrypt.CompareHashAndPassword returns "crypto/bcrypt: invalid hash" for empty hash
	if !errors.Is(err, bcrypt.ErrHashTooShort) && err.Error() != "crypto/bcrypt: invalid hash" {
		t.Errorf("ComparePasswordHash with empty hash returned unexpected error: %v", err)
	}
}

// TestGenerateJWT tests the GenerateJWT function.
func TestGenerateJWT(t *testing.T) {
	userID := "test-user-id-123"
	expiryDuration := time.Hour * 24 // 24 hours

	tokenString, err := GenerateJWT(userID, expiryDuration)
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}
	if tokenString == "" {
		t.Error("GenerateJWT returned an empty token string")
	}

	// Basic check to see if it looks like a JWT (3 parts separated by dots)
	var parts int
	for _, r := range tokenString {
		if r == '.' {
			parts++
		}
	}
	if parts != 2 {
		t.Errorf("Generated token string %q does not look like a valid JWT (expected 2 dots)", tokenString)
	}

	// Test with zero expiry (should ideally error or have a default)
	_, err = GenerateJWT(userID, 0)
	// Depending on implementation, this might error or default. Assuming it should error if 0 is invalid.
	if err == nil {
		t.Error("GenerateJWT with zero expiry duration should return an error, but got nil")
	}
}

// TestValidateJWT tests the ValidateJWT function.
func TestValidateJWT(t *testing.T) {
	userID := "validate-user-id-456"
	expiryDuration := time.Minute * 5 // Short expiry for testing

	tokenString, _ := GenerateJWT(userID, expiryDuration)

	// Test case 1: Valid token
	claims, err := ValidateJWT(tokenString)
	if err != nil {
		t.Fatalf("ValidateJWT with valid token failed: %v", err)
	}
	if claims == nil {
		t.Fatal("ValidateJWT with valid token returned nil claims")
	}
	if claims.UserID != userID {
		t.Errorf("ValidateJWT returned incorrect UserID: got %q, want %q", claims.UserID, userID)
	}

	// Test case 2: Invalid token string (malformed)
	invalidToken := "this.is.not.a.valid.jwt"
	_, err = ValidateJWT(invalidToken)
	if err == nil {
		t.Error("ValidateJWT with malformed token string should return an error, but got nil")
	}
	// Check if the error is a jwt.ParseError or similar
	if !errors.Is(err, jwt.ErrTokenMalformed) && !errors.Is(err, jwt.ErrTokenUnverifiable) {
		t.Errorf("ValidateJWT with malformed token returned unexpected error type: %T", err)
	}

	// Test case 3: Expired token
	expiredTokenString, _ := GenerateJWT(userID, time.Second*-1) // Token that expired in the past
	time.Sleep(time.Second * 2) // Wait a bit to ensure expiry
	_, err = ValidateJWT(expiredTokenString)
	if err == nil {
		t.Error("ValidateJWT with expired token should return an error, but got nil")
	}
	// Check if the error indicates expiration
	if !errors.Is(err, jwt.ErrTokenExpired) {
		t.Errorf("ValidateJWT with expired token returned unexpected error type: %T", err)
	}

	// Test case 4: Token with different secret (simulated by generating with a different secret)
	otherSecret := "a-completely-different-secret-key"
	otherTokenString, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}).SignedString([]byte(otherSecret))

	_, err = ValidateJWT(otherTokenString)
	if err == nil {
		t.Error("ValidateJWT with token signed by different secret should return an error, but got nil")
	}
	if !errors.Is(err, jwt.ErrTokenSignatureInvalid) && !errors.Is(err, jwt.ErrTokenUnverifiable) {
		t.Errorf("ValidateJWT with token signed by different secret returned unexpected error type: %T", err)
	}
}

// Note: These tests assume the existence of GeneratePasswordHash, ComparePasswordHash,
// GenerateJWT, ValidateJWT, and the Claims struct within the auth package.
// They also assume the JWT secret is accessible or passed appropriately.
// In the actual implementation, Ensure the JWT secret is securely managed (e.g., via environment variables).
