package grpc

import (
	"context"
	"testing"

	"github.com/waydxd/Orbit-core/pkg/logger"
	pb "github.com/waydxd/Orbit-core/proto/calendar"
	"google.golang.org/grpc/metadata"
)

func TestShouldIntercept(t *testing.T) {
	tests := []struct {
		method   string
		expected bool
	}{
		{"/calendar.CalendarService/CreateEvent", true},
		{"/calendar.CalendarService/UpdateEvent", true},
		{"/calendar.CalendarService/DeleteEvent", true},
		{"/calendar.CalendarService/GetEvent", false},
		{"/calendar.CalendarService/ListEvents", false},
		{"/other.Service/Method", false},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			result := shouldIntercept(tt.method)
			if result != tt.expected {
				t.Errorf("shouldIntercept(%q) = %v, want %v", tt.method, result, tt.expected)
			}
		})
	}
}

func TestConvertCreateEventRequest(t *testing.T) {
	req := &pb.CreateEventRequest{
		UserId:      "user123",
		Title:       "Test Event",
		Description: "Test Description",
		StartTime:   1704103200,
		EndTime:     1704106800,
		Location:    "Test Location",
		Attendees:   []string{"attendee1@example.com"},
	}

	actionType, actionData, summary, err := convertCreateEventRequest(req)
	if err != nil {
		t.Fatalf("convertCreateEventRequest failed: %v", err)
	}

	if actionType != "create_event" {
		t.Errorf("actionType = %q, want create_event", actionType)
	}

	if actionData["user_id"] != "user123" {
		t.Errorf("user_id = %v, want user123", actionData["user_id"])
	}

	if actionData["title"] != "Test Event" {
		t.Errorf("title = %v, want Test Event", actionData["title"])
	}

	if summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestConvertUpdateEventRequest(t *testing.T) {
	req := &pb.UpdateEventRequest{
		Id:          "event123",
		UserId:      "user123",
		Title:       "Updated Event",
		Description: "Updated Description",
	}

	actionType, actionData, summary, err := convertUpdateEventRequest(req)
	if err != nil {
		t.Fatalf("convertUpdateEventRequest failed: %v", err)
	}

	if actionType != "update_event" {
		t.Errorf("actionType = %q, want update_event", actionType)
	}

	if actionData["id"] != "event123" {
		t.Errorf("id = %v, want event123", actionData["id"])
	}

	if summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestConvertDeleteEventRequest(t *testing.T) {
	req := &pb.DeleteEventRequest{
		Id:     "event123",
		UserId: "user123",
	}

	actionType, actionData, summary, err := convertDeleteEventRequest(req)
	if err != nil {
		t.Fatalf("convertDeleteEventRequest failed: %v", err)
	}

	if actionType != "delete_event" {
		t.Errorf("actionType = %q, want delete_event", actionType)
	}

	if actionData["id"] != "event123" {
		t.Errorf("id = %v, want event123", actionData["id"])
	}

	if summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestConvertRequestToAction_Invalid(t *testing.T) {
	_, _, _, err := convertRequestToAction("/unknown/Method", nil)
	if err == nil {
		t.Error("expected error for unknown method")
	}
}

func TestCreatePendingResponse_CreateEvent(t *testing.T) {
	resp, err := createPendingResponse("/calendar.CalendarService/CreateEvent", "action123", "Create event: Test")
	if err != nil {
		t.Fatalf("createPendingResponse failed: %v", err)
	}

	createResp, ok := resp.(*pb.CreateEventResponse)
	if !ok {
		t.Fatalf("expected CreateEventResponse, got %T", resp)
	}

	if createResp.Success {
		t.Error("expected Success to be false")
	}

	if createResp.Message == "" {
		t.Error("expected non-empty message")
	}
}

func TestCreatePendingResponse_UpdateEvent(t *testing.T) {
	resp, err := createPendingResponse("/calendar.CalendarService/UpdateEvent", "action123", "Update event: Test")
	if err != nil {
		t.Fatalf("createPendingResponse failed: %v", err)
	}

	updateResp, ok := resp.(*pb.UpdateEventResponse)
	if !ok {
		t.Fatalf("expected UpdateEventResponse, got %T", resp)
	}

	if updateResp.Success {
		t.Error("expected Success to be false")
	}
}

func TestCreatePendingResponse_DeleteEvent(t *testing.T) {
	resp, err := createPendingResponse("/calendar.CalendarService/DeleteEvent", "action123", "Delete event")
	if err != nil {
		t.Fatalf("createPendingResponse failed: %v", err)
	}

	deleteResp, ok := resp.(*pb.DeleteEventResponse)
	if !ok {
		t.Fatalf("expected DeleteEventResponse, got %T", resp)
	}

	if deleteResp.Success {
		t.Error("expected Success to be false")
	}
}

func TestCreatePendingResponse_Invalid(t *testing.T) {
	_, err := createPendingResponse("/unknown/Method", "action123", "test")
	if err == nil {
		t.Error("expected error for unknown method")
	}
}

func TestExtractMetadata(t *testing.T) {
	md := metadata.Pairs("user_id", "user123", "conversation_id", "session456")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	userID, sessionID := extractMetadata(ctx)
	if userID != "user123" {
		t.Errorf("userID = %q, want user123", userID)
	}
	if sessionID != "session456" {
		t.Errorf("sessionID = %q, want session456", sessionID)
	}
}

func TestExtractMetadata_Empty(t *testing.T) {
	ctx := context.Background()
	userID, sessionID := extractMetadata(ctx)
	if userID != "" {
		t.Errorf("userID = %q, want empty", userID)
	}
	if sessionID != "" {
		t.Errorf("sessionID = %q, want empty", sessionID)
	}
}

func TestExtractCorrelationID(t *testing.T) {
	md := metadata.Pairs("correlation_id", "corr123")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	correlationID := extractCorrelationID(ctx)
	if correlationID != "corr123" {
		t.Errorf("correlationID = %q, want corr123", correlationID)
	}
}

func TestExtractCorrelationID_Empty(t *testing.T) {
	ctx := context.Background()
	correlationID := extractCorrelationID(ctx)
	if correlationID == "" {
		t.Error("expected non-empty correlation ID when not in metadata")
	}
}

func TestGenerateActionID(t *testing.T) {
	actionID := generateActionID()
	if actionID == "" {
		t.Error("expected non-empty action ID")
	}
}

func TestGenerateIdempotencyKey(t *testing.T) {
	key := generateIdempotencyKey("user123", "session456", "create_event")
	if key == "" {
		t.Error("expected non-empty idempotency key")
	}
	if len(key) < 20 {
		t.Errorf("key too short: %s", key)
	}
}

func TestNewServer(t *testing.T) {
	log := logger.New()
	cfg := ServerConfig{
		Port:         50052,
		Interceptors: nil,
	}

	server, err := NewServer(cfg, log)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}

	if server == nil {
		t.Fatal("expected non-nil server")
	}

	if server.port != 50052 {
		t.Errorf("port = %d, want 50052", server.port)
	}

	server.Stop()
}

func TestServer_GetPort(t *testing.T) {
	log := logger.New()
	cfg := ServerConfig{Port: 50053}
	server, _ := NewServer(cfg, log)

	if server.GetPort() != 50053 {
		t.Errorf("GetPort() = %d, want 50053", server.GetPort())
	}

	server.Stop()
}
