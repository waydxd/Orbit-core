package fcm

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestInitEmptyCredentials(t *testing.T) {
	instance = nil
	once = sync.Once{}
	initErr = nil

	ctx := context.Background()
	client, err := Init(ctx, "")

	if err == nil {
		t.Error("expected error for empty credentials")
	}
	if client != nil {
		t.Error("expected nil client for empty credentials")
	}
}

func TestIsInvalidToken(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"ErrInvalidToken", ErrInvalidToken, true},
		{"wrapped ErrInvalidToken", fmt.Errorf("wrapped: %w", ErrInvalidToken), true},
		{"other error", errors.New("other error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsInvalidToken(tt.err)
			if result != tt.expected {
				t.Errorf("IsInvalidToken() = %v, want %v", result, tt.expected)
			}
		})
	}
}

type messagingError struct {
	code string
}

func (e *messagingError) Error() string {
	return e.code
}

func TestIsInvalidToken_OtherCode(t *testing.T) {
	err := &messagingError{code: "INTERNAL"}
	if IsInvalidToken(err) {
		t.Error("expected other error codes not to be invalid token")
	}
}
