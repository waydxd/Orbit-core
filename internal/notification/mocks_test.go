package notification

import (
	"context"
	"sync"
	"time"

	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/fcm"
)

// errInvalidToken is the sentinel error used in tests to simulate an invalid FCM token.
var errInvalidToken = fcm.ErrInvalidToken

// mockRepo is a test-only implementation of Repository.
type mockRepo struct {
	mu sync.Mutex

	// Controlled responses
	upsertErr       error
	deleteTokenErr  error
	tokensResp      []*models.DeviceToken
	tokensErr       error
	createSubErr    error
	deleteSubErr    error
	existsResp      bool
	existsErr       error
	pendingSubsResp []*models.EventSubscription
	pendingSubsErr  error
	markSentErr     error

	// Call records
	UpsertCalled      bool
	DeleteTokenCalled bool
	CreateSubCalled   bool
	DeleteSubCalled   bool
	MarkSentCalled    bool
}

func (m *mockRepo) UpsertDeviceToken(_ context.Context, _ *models.DeviceToken) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UpsertCalled = true
	return m.upsertErr
}

func (m *mockRepo) DeleteDeviceToken(_ context.Context, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeleteTokenCalled = true
	return m.deleteTokenErr
}

func (m *mockRepo) GetDeviceTokensByUserID(_ context.Context, _ string) ([]*models.DeviceToken, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.tokensResp, m.tokensErr
}

func (m *mockRepo) CreateSubscription(_ context.Context, _ *models.EventSubscription) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CreateSubCalled = true
	return m.createSubErr
}

func (m *mockRepo) DeleteSubscription(_ context.Context, _, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeleteSubCalled = true
	return m.deleteSubErr
}

func (m *mockRepo) SubscriptionExists(_ context.Context, _, _ string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.existsResp, m.existsErr
}

func (m *mockRepo) GetPendingSubscriptions(_ context.Context, _ time.Time) ([]*models.EventSubscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pendingSubsResp, m.pendingSubsErr
}

func (m *mockRepo) MarkSubscriptionSent(_ context.Context, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MarkSentCalled = true
	return m.markSentErr
}

// mockFCMClient records FCM send calls.
type mockFCMClient struct {
	mu        sync.Mutex
	returnErr error
	callCount int
}

func (m *mockFCMClient) send(_ context.Context, _, _, _ string, _ map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return m.returnErr
}
