package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReferrerPolicyMiddleware_SetsHeader(t *testing.T) {
	policy := "strict-origin-when-cross-origin"
	mw := ReferrerPolicyMiddleware(policy)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	got := rr.Header().Get("Referrer-Policy")
	if got != policy {
		t.Fatalf("expected Referrer-Policy=%q, got %q", policy, got)
	}
}

func TestReferrerPolicyMiddleware_NoReferrer(t *testing.T) {
	mw := ReferrerPolicyMiddleware("no-referrer")

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodPost, "/submit", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("expected Referrer-Policy=no-referrer, got %q", rr.Header().Get("Referrer-Policy"))
	}
}

func TestReferrerPolicyMiddleware_CallsNextHandler(t *testing.T) {
	called := false
	mw := ReferrerPolicyMiddleware("no-referrer")

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("expected next handler to be called")
	}
}
