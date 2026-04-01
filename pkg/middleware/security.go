package middleware

import (
	"net/http"
)

// ReferrerPolicyMiddleware returns a middleware that sets the Referrer-Policy header.
func ReferrerPolicyMiddleware(policy string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Referrer-Policy", policy)
			next.ServeHTTP(w, r)
		})
	}
}
