package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateRandomUsername generates a random username in the format "user_<random_hex>"
func GenerateRandomUsername() string {
	// Generate 6 random bytes (12 hex characters)
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		// Fallback to a simple timestamp-based approach if crypto/rand fails
		return fmt.Sprintf("user_%d", randomInt63())
	}
	return fmt.Sprintf("user_%s", hex.EncodeToString(b))
}

// randomInt63 is a fallback random number generator
func randomInt63() int64 {
	b := make([]byte, 8)
	rand.Read(b)
	// Use only 63 bits to ensure positive number
	return int64(b[0]&0x7f)<<56 | int64(b[1])<<48 | int64(b[2])<<40 |
		int64(b[3])<<32 | int64(b[4])<<24 | int64(b[5])<<16 |
		int64(b[6])<<8 | int64(b[7])
}
