package logger

import (
	"log/slog"
	"testing"
)

func TestNew_ReturnsLogger(t *testing.T) {
	l := New()
	//nolint:staticcheck // New() never returns nil
	if l == nil {
		t.Fatal("expected non-nil logger from New()")
	}
	//nolint:staticcheck // l is guaranteed non-nil after t.Fatal
	if l.Logger == nil {
		t.Fatal("expected non-nil embedded slog.Logger")
	}
}

func TestNewWithLevel_ReturnsLogger(t *testing.T) {
	levels := []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError}
	for _, level := range levels {
		l := NewWithLevel(level)
		//nolint:staticcheck // NewWithLevel never returns nil
		if l == nil {
			t.Fatalf("expected non-nil logger for level %v", level)
		}
		//nolint:staticcheck // l is guaranteed non-nil after t.Fatalf
		if l.Logger == nil {
			t.Fatalf("expected non-nil embedded slog.Logger for level %v", level)
		}
	}
}

func TestLogger_CanLog(t *testing.T) {
	l := New()
	// Verify the logger can be used without panicking
	l.Info("test info message", "key", "value")
	l.Warn("test warn message")
	l.Error("test error message", "error", "something went wrong")
}
