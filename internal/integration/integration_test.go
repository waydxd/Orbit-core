package integration

import (
	"context"
	"errors"
	"testing"
	"time"
)

// MockExternalAPI represents a mock for an external API client.
type MockExternalAPI struct {
	// Simulate responses or errors for different API calls
	GetDataFunc func(ctx context.Context, id string) (string, error)
	PostDataFunc func(ctx context.Context, data map[string]interface{}) (string, error)
}

func (m *MockExternalAPI) GetData(ctx context.Context, id string) (string, error) {
	if m.GetDataFunc != nil {
		return m.GetDataFunc(ctx, id)
	}
	// Default mock behavior
	if id == "exists-id" {
		return "data-for-exists-id", nil
	}
	return "", errors.New("external API: data not found")
}

func (m *MockExternalAPI) PostData(ctx context.Context, data map[string]interface{}) (string, error) {
	if m.PostDataFunc != nil {
		return m.PostDataFunc(ctx, data)
	}
	// Default mock behavior
	if data != nil {
		return "success_response_id", nil
	}
	return "", errors.New("external API: invalid data")
}

// MockIntegrationRepository is a mock for the IIntegrationRepository.
type MockIntegrationRepository struct {
	// Simulate data storage
	StoredData map[string]interface{}
}

// NewMockIntegrationRepository creates a new mock repository.
func NewMockIntegrationRepository() *MockIntegrationRepository {
	return &MockIntegrationRepository{
		StoredData: make(map[string]interface{}),
	}
}

// SaveIntegrationData simulates saving data.
func (r *MockIntegrationRepository) SaveIntegrationData(ctx context.Context, key string, data map[string]interface{}) error {
	r.StoredData[key] = data
	return nil
}

// GetIntegrationData simulates retrieving data.
func (r *MockIntegrationRepository) GetIntegrationData(ctx context.Context, key string) (map[string]interface{}, error) {
	val, ok := r.StoredData[key]
	if !ok {
		return nil, errors.New("data not found")
	}
	// Type assertion is important here if the stored value is more specific
	data, ok := val.(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid data format")
	}
	return data, nil
}

// --- Integration Service Tests ---

// IntegrationService handles interactions with external APIs and repositories.
type IntegrationService struct {
	externalAPI IExternalAPI // Interface for external API client
	repo        IIntegrationRepository // Interface for repository
}

// NewIntegrationService creates a new IntegrationService.
func NewIntegrationService(api IExternalAPI, repo IIntegrationRepository) *IntegrationService {
	return &IntegrationService{
		externalAPI: api,
		repo:        repo,
	}
}

// FetchAndStoreData retrieves data from an external API and stores it.
func (s *IntegrationService) FetchAndStoreData(ctx context.Context, apiID string, repoKey string) error {
	// 1. Fetch data from external API
	apiData, err := s.externalAPI.GetData(ctx, apiID)
	if err != nil {
		return err // Propagate error from API call
	}

	// 2. Simulate processing or transforming apiData into a map[string]interface{}
	// For this mock, we'll just create a simple map.
	processedData := map[string]interface{}{
		"source_id": apiID,
		"value":     apiData,
		"processed_at": time.Now(),
	}

	// 3. Save processed data to repository
	err = s.repo.SaveIntegrationData(ctx, repoKey, processedData)
	if err != nil {
		return err // Propagate error from repository save
	}

	return nil // Success
}

// ProcessIntegrationData fetches data from the repository and posts it to an external API.
func (s *IntegrationService) ProcessIntegrationData(ctx context.Context, repoKey string, postData map[string]interface{}) (string, error) {
	// 1. Retrieve data from repository
	storedData, err := s.repo.GetIntegrationData(ctx, repoKey)
	if err != nil {
		return "", err // Propagate error from repository get
	}

	// 2. Merge or augment storedData with postData if necessary
	// For simplicity, we'll just use postData directly here, assuming it contains what's needed.
	// A real implementation might merge storedData into postData.

	// 3. Post data to external API
	responseID, err := s.externalAPI.PostData(ctx, postData)
	if err != nil {
		return "", err // Propagate error from API call
	}

	return responseID, nil // Return response ID from API
}

// --- Test Cases for Integration Service ---

func TestIntegrationService_FetchAndStoreData(t *testing.T) {
	mockAPI := &MockExternalAPI{}
	mockRepo := NewMockIntegrationRepository()
	svc := NewIntegrationService(mockAPI, mockRepo)
	ctx := context.Background()

	apiID := "some-api-id"
	repoKey := "data-key-1"

	// Test case 1: Successful fetch and store
	mockAPI.GetDataFunc = func(ctx context.Context, id string) (string, error) {
		if id == apiID {
			return "api-response-data", nil
		}
		return "", errors.New("mock API error: id not found")
	}
	
	err := svc.FetchAndStoreData(ctx, apiID, repoKey)
	if err != nil {
		t.Fatalf("FetchAndStoreData failed: %v", err)
	}

	// Verify data was stored
	stored, err := mockRepo.GetIntegrationData(ctx, repoKey)
	if err != nil {
		t.Fatalf("Failed to get stored data: %v", err)
	}
	if stored == nil {
		t.Fatal("Stored data is nil")
	}
	if val, ok := stored["value"].(string); !ok || val != "api-response-data" {
		t.Errorf("Stored data 'value' incorrect: got %v, want 'api-response-data'", stored["value"])
	}
	if _, ok := stored["processed_at"]; !ok {
		t.Error("Stored data missing 'processed_at' timestamp")
	}

	// Test case 2: API returns an error
	mockAPI.GetDataFunc = func(ctx context.Context, id string) (string, error) {
		return "", errors.New("external API error")
	}
	err = svc.FetchAndStoreData(ctx, apiID, repoKey)
	if err == nil {
		t.Error("FetchAndStoreData should return error when API fails, but got nil")
	}
	if err.Error() != "external API error" {
		t.Errorf("FetchAndStoreData returned wrong error: got %q, want 'external API error'", err.Error())
	}

	// Test case 3: Repository save fails
	mockAPI.GetDataFunc = func(ctx context.Context, id string) (string, error) { return "some data", nil }
	mockRepo.SaveIntegrationData = func(ctx context.Context, key string, data map[string]interface{}) error {
		return errors.New("repository save error")
	}
	err = svc.FetchAndStoreData(ctx, apiID, repoKey)
	if err == nil {
		t.Error("FetchAndStoreData should return error when repository fails, but got nil")
	}
	if err.Error() != "repository save error" {
		t.Errorf("FetchAndStoreData returned wrong error: got %q, want 'repository save error'", err.Error())
	}
}

func TestIntegrationService_ProcessIntegrationData(t *testing.T) {
	mockAPI := &MockExternalAPI{}
	mockRepo := NewMockIntegrationRepository()
	svc := NewIntegrationService(mockAPI, mockRepo)
	ctx := context.Background()

	repoKey := "data-to-post-key"
	postPayload := map[string]interface{}{"field1": "value1", "field2": 123}

	// Populate repository
	mockRepo.StoredData[repoKey] = map[string]interface{}{"stored_field": "stored_value"}

	// Test case 1: Successful processing and post
	mockAPI.PostDataFunc = func(ctx context.Context, data map[string]interface{}) (string, error) {
		// Verify data received by API mock (if logic was to merge)
		// For now, just check if data is not nil and return a success ID
		if data == nil {
			return "", errors.New("API received nil data")
		}
		return "post-success-id", nil
	}
	responseID, err := svc.ProcessIntegrationData(ctx, repoKey, postPayload)
	if err != nil {
		t.Fatalf("ProcessIntegrationData failed: %v", err)
	}
	if responseID != "post-success-id" {
		t.Errorf("ProcessIntegrationData returned wrong response ID: got %q, want 'post-success-id'", responseID)
	}

	// Test case 2: Repository returns an error
	mockRepo.GetIntegrationData = func(ctx context.Context, key string) (map[string]interface{}, error) {
		return nil, errors.New("repository get error")
	}
	_, err = svc.ProcessIntegrationData(ctx, repoKey, postPayload)
	if err == nil {
		t.Error("ProcessIntegrationData should return error when repository fails, but got nil")
	}
	if err.Error() != "repository get error" {
		t.Errorf("ProcessIntegrationData returned wrong error: got %q, want 'repository get error'", err.Error())
	}

	// Test case 3: API returns an error
	mockRepo.GetIntegrationData = func(ctx context.Context, key string) (map[string]interface{}, error) {
		return map[string]interface{}{"dummy": "data"}, nil
	}
	mockAPI.PostDataFunc = func(ctx context.Context, data map[string]interface{}) (string, error) {
		return "", errors.New("external API post error")
	}
	_, err = svc.ProcessIntegrationData(ctx, repoKey, postPayload)
	if err == nil {
		t.Error("ProcessIntegrationData should return error when API fails, but got nil")
	}
	if err.Error() != "external API post error" {
		t.Errorf("ProcessIntegrationData returned wrong error: got %q, want 'external API post error'", err.Error())
	}
}

// NOTE: The 'IExternalAPI' and 'IIntegrationRepository' interfaces, along with
// potentially other structs like 'IntegrationService' fields, need to be defined
// and imported from the 'integration' package.
// These mocks and tests are illustrative and assume basic method signatures and return types.
