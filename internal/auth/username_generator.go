package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// GenerateRandomUsername generates a random username in the format "user_<random_hex>"
// Uses 8 random bytes (16 hex characters) to reduce collision probability
func GenerateRandomUsername() string {
	// Generate 8 random bytes (16 hex characters)
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based approach if crypto/rand fails
		return fmt.Sprintf("user_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("user_%s", hex.EncodeToString(b))
}
