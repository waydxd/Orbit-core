package chat

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/config"
	"github.com/waydxd/Orbit-core/pkg/logger"
	"github.com/waydxd/Orbit-core/pkg/middleware"
	pb "github.com/waydxd/Orbit-core/proto/calendar"
	"google.golang.org/grpc"
)

// ===== Mock GRPCClient =====

type mockGRPCClient struct {
	healthErr      error
	processResp    *pb.ProcessMessageResponse
	processErr     error
	calendarClient pb.CalendarServiceClient
}

func (m *mockGRPCClient) HealthCheck(_ context.Context) error { return m.healthErr }

func (m *mockGRPCClient) ProcessMessage(_ context.Context, _ *pb.ProcessMessageRequest) (*pb.ProcessMessageResponse, error) {
	return m.processResp, m.processErr
}

func (m *mockGRPCClient) GetCalendarServiceClient() pb.CalendarServiceClient {
	return m.calendarClient
}

// ===== Mock CalendarServiceClient =====

type mockCalendarServiceClient struct {
	createResp *pb.CreateEventResponse
	createErr  error
	updateResp *pb.UpdateEventResponse
	updateErr  error
	deleteResp *pb.DeleteEventResponse
	deleteErr  error
	slotsResp  *pb.GetAvailableSlotsResponse
	slotsErr   error
}

func (m *mockCalendarServiceClient) CreateEvent(_ context.Context, _ *pb.CreateEventRequest, _ ...grpc.CallOption) (*pb.CreateEventResponse, error) {
	return m.createResp, m.createErr
}
func (m *mockCalendarServiceClient) GetEvents(_ context.Context, _ *pb.GetEventsRequest, _ ...grpc.CallOption) (*pb.GetEventsResponse, error) {
	return nil, nil
}
func (m *mockCalendarServiceClient) UpdateEvent(_ context.Context, _ *pb.UpdateEventRequest, _ ...grpc.CallOption) (*pb.UpdateEventResponse, error) {
	return m.updateResp, m.updateErr
}
func (m *mockCalendarServiceClient) DeleteEvent(_ context.Context, _ *pb.DeleteEventRequest, _ ...grpc.CallOption) (*pb.DeleteEventResponse, error) {
	return m.deleteResp, m.deleteErr
}
func (m *mockCalendarServiceClient) GetAvailableSlots(_ context.Context, _ *pb.GetAvailableSlotsRequest, _ ...grpc.CallOption) (*pb.GetAvailableSlotsResponse, error) {
	return m.slotsResp, m.slotsErr
}

// ===== Helper =====

func newServiceWithGRPC(grpcClient GRPCClient, repo Repository) *Service {
	return NewService(&config.Config{}, logger.New(), repo, grpcClient)
}

func withUserIDGRPC(r *http.Request, userID string) *http.Request {
	ctx := context.WithValue(r.Context(), middleware.UserIDKey, userID)
	return r.WithContext(ctx)
}

// ===== Tests for forwardToAgent =====

func TestForwardToAgent_Success(t *testing.T) {
	grpcMock := &mockGRPCClient{
		processResp: &pb.ProcessMessageResponse{
			Success:  true,
			Response: "Hello from agent",
		},
	}
	repo := &mockRepo{
		conversation: &models.Conversation{
			ID:     "conv-1",
			UserID: "user-1",
			Status: "active",
		},
	}
	svc := newServiceWithGRPC(grpcMock, repo)

	reply, proposed, err := svc.forwardToAgent(context.Background(), "user-1", "hello", "corr-1", "conv-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "Hello from agent" {
		t.Errorf("expected agent reply 'Hello from agent', got %q", reply)
	}
	if proposed != nil {
		t.Errorf("expected no proposed action, got %+v", proposed)
	}
}

func TestForwardToAgent_TransportError(t *testing.T) {
	grpcMock := &mockGRPCClient{
		processErr: errors.New("connection refused"),
	}
	svc := newServiceWithGRPC(grpcMock, &mockRepo{})

	_, _, err := svc.forwardToAgent(context.Background(), "user-1", "hello", "corr-1", "conv-1")
	if err == nil {
		t.Fatal("expected error from transport failure")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("error should mention connection refused, got: %v", err)
	}
}

func TestForwardToAgent_AgentReturnsFailure(t *testing.T) {
	grpcMock := &mockGRPCClient{
		processResp: &pb.ProcessMessageResponse{
			Success: false,
			Error:   "agent internal error",
		},
	}
	svc := newServiceWithGRPC(grpcMock, &mockRepo{})

	_, _, err := svc.forwardToAgent(context.Background(), "user-1", "hello", "corr-1", "conv-1")
	if err == nil {
		t.Fatal("expected error when agent returns failure")
	}
	if !strings.Contains(err.Error(), "agent internal error") {
		t.Errorf("error should mention agent internal error, got: %v", err)
	}
}

// ===== Tests for executeCreateEvent =====

func TestExecuteCreateEvent_Success(t *testing.T) {
	calClient := &mockCalendarServiceClient{
		createResp: &pb.CreateEventResponse{
			Success: true,
			Message: "created",
			Event:   &pb.Event{Id: "evt-1"},
		},
	}
	grpcMock := &mockGRPCClient{calendarClient: calClient}
	svc := newServiceWithGRPC(grpcMock, &mockRepo{})

	actionData := map[string]interface{}{
		"title":       "Team meeting",
		"description": "Q1 planning",
		"location":    "Room A",
		"start_time":  float64(time.Date(2025, 1, 10, 9, 0, 0, 0, time.UTC).Unix()),
		"end_time":    float64(time.Date(2025, 1, 10, 10, 0, 0, 0, time.UTC).Unix()),
	}

	result, opID, err := svc.executeCreateEvent(context.Background(), actionData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opID != "evt-1" {
		t.Errorf("expected operation ID evt-1, got %s", opID)
	}
	if result["event_id"] != "evt-1" {
		t.Errorf("expected event_id evt-1 in result, got %v", result["event_id"])
	}
}

func TestExecuteCreateEvent_RPCError(t *testing.T) {
	calClient := &mockCalendarServiceClient{
		createErr: errors.New("rpc error: deadline exceeded"),
	}
	grpcMock := &mockGRPCClient{calendarClient: calClient}
	svc := newServiceWithGRPC(grpcMock, &mockRepo{})

	_, _, err := svc.executeCreateEvent(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error from RPC failure")
	}
	if !strings.Contains(err.Error(), "gRPC CreateEvent failed") {
		t.Errorf("error should mention gRPC CreateEvent failed, got: %v", err)
	}
}

// ===== Tests for executeUpdateEvent =====

func TestExecuteUpdateEvent_Success(t *testing.T) {
	calClient := &mockCalendarServiceClient{
		updateResp: &pb.UpdateEventResponse{
			Success: true,
			Message: "updated",
			Event:   &pb.Event{Id: "evt-2"},
		},
	}
	grpcMock := &mockGRPCClient{calendarClient: calClient}
	svc := newServiceWithGRPC(grpcMock, &mockRepo{})

	actionData := map[string]interface{}{
		"id":          "evt-2",
		"title":       "Updated meeting",
		"description": "Updated description",
		"start_time":  float64(time.Date(2025, 2, 20, 14, 0, 0, 0, time.UTC).Unix()),
		"end_time":    float64(time.Date(2025, 2, 20, 15, 0, 0, 0, time.UTC).Unix()),
	}

	result, opID, err := svc.executeUpdateEvent(context.Background(), actionData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opID != "evt-2" {
		t.Errorf("expected operation ID evt-2, got %s", opID)
	}
	if result["event_id"] != "evt-2" {
		t.Errorf("expected event_id evt-2 in result, got %v", result["event_id"])
	}
}

func TestExecuteUpdateEvent_NonSuccessResponse(t *testing.T) {
	calClient := &mockCalendarServiceClient{
		updateResp: &pb.UpdateEventResponse{
			Success: false,
			Message: "validation failed",
		},
	}
	grpcMock := &mockGRPCClient{calendarClient: calClient}
	svc := newServiceWithGRPC(grpcMock, &mockRepo{})

	_, _, err := svc.executeUpdateEvent(context.Background(), map[string]interface{}{"id": "evt-x"})
	if err == nil {
		t.Fatal("expected error when update response indicates failure")
	}
	if !strings.Contains(err.Error(), "update event failed") {
		t.Errorf("error should mention update event failed, got: %v", err)
	}
}

// ===== Tests for executeDeleteEvent =====

func TestExecuteDeleteEvent_Success(t *testing.T) {
	calClient := &mockCalendarServiceClient{
		deleteResp: &pb.DeleteEventResponse{
			Success: true,
			Message: "deleted",
		},
	}
	grpcMock := &mockGRPCClient{calendarClient: calClient}
	svc := newServiceWithGRPC(grpcMock, &mockRepo{})

	result, opID, err := svc.executeDeleteEvent(context.Background(), map[string]interface{}{"id": "evt-42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opID != "evt-42" {
		t.Errorf("expected op ID evt-42, got %s", opID)
	}
	if result["message"] != "deleted" {
		t.Errorf("unexpected result message: %v", result["message"])
	}
}

// ===== Tests for executeAction dispatch =====

func TestExecuteAction_UnsupportedType(t *testing.T) {
	svc := newServiceWithGRPC(&mockGRPCClient{}, &mockRepo{})

	raw, _ := json.Marshal(map[string]interface{}{"foo": "bar"})
	action := &models.PendingAction{
		ActionType:     "unsupported_action",
		ProposedAction: raw,
	}

	_, _, err := svc.executeAction(context.Background(), action)
	if err == nil {
		t.Fatal("expected error for unsupported action type")
	}
	if !strings.Contains(err.Error(), "unsupported action type") {
		t.Errorf("error should mention unsupported action type, got: %v", err)
	}
}

// ===== Tests for health endpoint with mock gRPC =====

func TestHandleHealth_Healthy(t *testing.T) {
	grpcMock := &mockGRPCClient{healthErr: nil}
	svc := newServiceWithGRPC(grpcMock, &mockRepo{})

	router := mux.NewRouter()
	svc.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/chat/health", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if body["status"] != "healthy" {
		t.Errorf("expected status=healthy, got %v", body["status"])
	}
}

func TestHandleHealth_Unhealthy(t *testing.T) {
	grpcMock := &mockGRPCClient{healthErr: errors.New("agent down")}
	svc := newServiceWithGRPC(grpcMock, &mockRepo{})

	router := mux.NewRouter()
	svc.RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/chat/health", nil)
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", res.Code)
	}
}

// ===== Test handlePostMessage with mock agent (gRPC path) =====

func TestHandlePostMessage_AgentCommunicationFailure(t *testing.T) {
	grpcMock := &mockGRPCClient{
		processErr: errors.New("agent unavailable"),
	}
	repo := &mockRepo{
		conversation: &models.Conversation{
			ID:     "11111111-1111-1111-1111-111111111111",
			UserID: "user-1",
			Status: "active",
		},
	}
	svc := newServiceWithGRPC(grpcMock, repo)
	router := mux.NewRouter()
	svc.RegisterRoutes(router)

	body := `{"message":"hello","conversation_id":"11111111-1111-1111-1111-111111111111"}`
	req := httptest.NewRequest(http.MethodPost, "/chat/messages", strings.NewReader(body))
	req = withUserIDGRPC(req, "user-1")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", res.Code)
	}
}

func TestHandlePostMessage_AgentSuccessReturnsReply(t *testing.T) {
	grpcMock := &mockGRPCClient{
		processResp: &pb.ProcessMessageResponse{
			Success:  true,
			Response: "sure, I can help!",
		},
	}
	repo := &mockRepo{
		conversation: &models.Conversation{
			ID:     "11111111-1111-1111-1111-111111111111",
			UserID: "user-1",
			Status: "active",
		},
	}
	svc := newServiceWithGRPC(grpcMock, repo)
	router := mux.NewRouter()
	svc.RegisterRoutes(router)

	body := `{"message":"hello","conversation_id":"11111111-1111-1111-1111-111111111111"}`
	req := httptest.NewRequest(http.MethodPost, "/chat/messages", strings.NewReader(body))
	req = withUserIDGRPC(req, "user-1")
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}

	var payload PostMessageResponse
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if payload.Reply != "sure, I can help!" {
		t.Errorf("expected reply 'sure, I can help!', got %q", payload.Reply)
	}
}
