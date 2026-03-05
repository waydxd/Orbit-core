package chat

import (
	"testing"

	"github.com/google/uuid"
)

func TestGenerateActionID_IsUUID(t *testing.T) {
	actionID := GenerateActionID()
	if _, err := uuid.Parse(actionID); err != nil {
		t.Fatalf("expected action id to be UUID, got %q: %v", actionID, err)
	}
}
