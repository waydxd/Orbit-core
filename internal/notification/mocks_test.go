package notification

import (
	"context"
	"sync"

	"github.com/hibiken/asynq"
	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/fcm"
)

// errInvalidToken is the sentinel error used in tests to simulate an invalid FCM token.
var errInvalidToken = fcm.ErrInvalidToken

// mockRepo is a test-only implementation of Repository.
type mockRepo struct {
	mu sync.Mutex

	// Controlled responses
	upsertErr          error
	deleteTokenErr     error
	tokensResp         []*models.DeviceToken
	tokensErr          error
	createSubErr       error
	createSubID        string
	deleteSubErr       error
	existsResp         bool
	existsErr          error
	getSubByIDResp     *models.EventSubscription
	getSubByIDErr      error
	getSubResp         *models.EventSubscription
	getSubErr          error
	getSubsByEventResp []*models.EventSubscription
	getSubsByEventErr  error
	markStatusErr      error
	updateJobIDErr     error

	// Call records
	UpsertCalled      bool
	DeleteTokenCalled bool
	CreateSubCalled   bool
	DeleteSubCalled   bool
	GetSubByIDCalled  bool
	GetSubCalled      bool
	MarkStatusCalled  bool
	MarkStatusValue   string
	UpdateJobIDValue  string
	UpdateJobIDCalled bool
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

func (m *mockRepo) DeleteDeviceTokenByUser(_ context.Context, _, _ string) error {
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

func (m *mockRepo) CreateSubscription(_ context.Context, sub *models.EventSubscription) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CreateSubCalled = true
	if m.createSubID == "" {
		m.createSubID = "sub-created"
	}
	if sub != nil {
		sub.ID = m.createSubID
	}
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

func (m *mockRepo) GetSubscriptionByID(_ context.Context, _ string) (*models.EventSubscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetSubByIDCalled = true
	return m.getSubByIDResp, m.getSubByIDErr
}

func (m *mockRepo) GetSubscriptionByUserAndEvent(_ context.Context, _, _ string) (*models.EventSubscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.GetSubCalled = true
	return m.getSubResp, m.getSubErr
}

func (m *mockRepo) GetSubscriptionsByEventID(_ context.Context, _ string) ([]*models.EventSubscription, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getSubsByEventResp, m.getSubsByEventErr
}

func (m *mockRepo) MarkSubscriptionStatus(_ context.Context, _, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.MarkStatusCalled = true
	m.MarkStatusValue = status
	return m.markStatusErr
}

func (m *mockRepo) UpdateSubscriptionJobID(_ context.Context, _, jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UpdateJobIDCalled = true
	m.UpdateJobIDValue = jobID
	return m.updateJobIDErr
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

// mockEnqueuer records Asynq enqueue calls.
type mockEnqueuer struct {
	mu            sync.Mutex
	returnErr     error
	returnInfo    *asynq.TaskInfo
	EnqueueCalled bool
}

func (m *mockEnqueuer) EnqueueContext(_ context.Context, _ *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.EnqueueCalled = true
	if m.returnInfo == nil && m.returnErr == nil {
		return &asynq.TaskInfo{ID: "mock-task-id-123"}, nil
	}
	return m.returnInfo, m.returnErr
}

// mockCanceller records Asynq task cancellation calls.
type mockCanceller struct {
	mu            sync.Mutex
	returnErr     error
	DeleteCalled  bool
	DeletedTaskID string
}

func (m *mockCanceller) DeleteTask(_, taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.DeleteCalled = true
	m.DeletedTaskID = taskID
	return m.returnErr
}
