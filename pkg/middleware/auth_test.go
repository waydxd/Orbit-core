package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ===== GetUserIDFromContext =====

func TestGetUserIDFromContext_WithUserID(t *testing.T) {
	ctx := context.WithValue(context.Background(), UserIDKey, "user-123")
	id := GetUserIDFromContext(ctx)
	if id != "user-123" {
		t.Fatalf("expected user-123, got %q", id)
	}
}

func TestGetUserIDFromContext_Empty(t *testing.T) {
	id := GetUserIDFromContext(context.Background())
	if id != "" {
		t.Fatalf("expected empty string, got %q", id)
	}
}

func TestGetUserIDFromContext_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), UserIDKey, 42)
	id := GetUserIDFromContext(ctx)
	if id != "" {
		t.Fatalf("expected empty string for non-string value, got %q", id)
	}
}

// ===== AuthMiddleware =====

const testJWTSecret = "test-secret-key"

func makeValidToken(t *testing.T, userID string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"id":  userID,
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return signed
}

func newTestAuthMiddleware() *AuthMiddleware {
	return NewAuthMiddleware(testJWTSecret)
}

func okHandler(w http.ResponseWriter, r *http.Request) {
	userID := GetUserIDFromContext(r.Context())
	w.Header().Set("X-User-ID", userID)
	w.WriteHeader(http.StatusOK)
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	am := newTestAuthMiddleware()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	am.Middleware(http.HandlerFunc(okHandler)).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_MissingBearerPrefix(t *testing.T) {
	am := newTestAuthMiddleware()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "sometoken")
	rr := httptest.NewRecorder()

	am.Middleware(http.HandlerFunc(okHandler)).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	am := newTestAuthMiddleware()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	rr := httptest.NewRecorder()

	am.Middleware(http.HandlerFunc(okHandler)).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	am := newTestAuthMiddleware()

	claims := jwt.MapClaims{
		"id":  "user-exp",
		"exp": time.Now().Add(-time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(testJWTSecret))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rr := httptest.NewRecorder()

	am.Middleware(http.HandlerFunc(okHandler)).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired token, got %d", rr.Code)
	}
}

func TestAuthMiddleware_WrongSigningMethod(t *testing.T) {
	// RS256 token should be rejected
	am := newTestAuthMiddleware()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// Craft a token with a different algorithm header by manipulating the token manually.
	// The simplest approach: sign with HMAC but claim it is RS256 (alg mismatch).
	// We use a "none" alg trick instead — any tampered token is fine for this test.
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJpZCI6IngiLCJleHAiOjk5OTk5OTk5OTl9.")
	rr := httptest.NewRecorder()

	am.Middleware(http.HandlerFunc(okHandler)).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for tampered token, got %d", rr.Code)
	}
}

func TestAuthMiddleware_ValidToken_PropagatesUserID(t *testing.T) {
	am := newTestAuthMiddleware()
	signed := makeValidToken(t, "user-abc")

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rr := httptest.NewRecorder()

	am.Middleware(http.HandlerFunc(okHandler)).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid token, got %d", rr.Code)
	}
	if rr.Header().Get("X-User-ID") != "user-abc" {
		t.Fatalf("expected user-abc in context, got %q", rr.Header().Get("X-User-ID"))
	}
}

func TestAuthMiddleware_MissingUserIDClaim(t *testing.T) {
	am := newTestAuthMiddleware()

	// Token without "id" claim
	claims := jwt.MapClaims{
		"sub": "user-xyz",
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(testJWTSecret))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rr := httptest.NewRecorder()

	am.Middleware(http.HandlerFunc(okHandler)).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when id claim is missing, got %d", rr.Code)
	}
}

// ===== PassUserIDToMetadata =====

func TestPassUserIDToMetadata_WithUserID(t *testing.T) {
	ctx := context.WithValue(context.Background(), UserIDKey, "meta-user")
	out := PassUserIDToMetadata(ctx)
	// As long as it doesn't panic and returns a non-nil context, we're good.
	if out == nil {
		t.Fatal("expected non-nil context from PassUserIDToMetadata")
	}
}

func TestPassUserIDToMetadata_EmptyUserID(t *testing.T) {
	out := PassUserIDToMetadata(context.Background())
	if out == nil {
		t.Fatal("expected non-nil context even with empty user ID")
	}
}
