package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

// RateLimiter provides rate limiting functionality using Redis
type RateLimiter struct {
	client *redis.Client
	limit  int
	window time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(redisAddr, password string, db, limit int, window time.Duration) *RateLimiter {
	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: password,
		DB:       db,
	})

	return &RateLimiter{
		client: client,
		limit:  limit,
		window: window,
	}
}

// Middleware returns a rate limiting middleware
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()

		// Use IP address as key for rate limiting
		key := fmt.Sprintf("rate_limit:%s", r.RemoteAddr)

		// Increment counter
		count, err := rl.client.Incr(ctx, key).Result()
		if err != nil {
			// If Redis fails, allow request (fail open)
			next.ServeHTTP(w, r)
			return
		}

		// Set expiration on first request
		if count == 1 {
			rl.client.Expire(ctx, key, rl.window)
		}

		// Check if rate limit exceeded
		if count > int64(rl.limit) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, err := w.Write([]byte(`{"error":"rate limit exceeded"}`))
			if err != nil {
				return
			}
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Close closes the Redis connection
func (rl *RateLimiter) Close() error {
	return rl.client.Close()
}
