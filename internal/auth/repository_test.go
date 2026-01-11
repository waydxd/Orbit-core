package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MockUser represents a user object for testing purposes.
// This should align with the actual User model used in the repository.
type MockUser struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"`
	Username      string             `bson:"username"`
	Email         string             `bson:"email"`
	PasswordHash  string             `bson:"passwordHash"`
	EmailVerified bool               `bson:"emailVerified"`
	CreatedAt     time.Time          `bson:"createdAt"`
	UpdatedAt     time.Time          `bson:"updatedAt"`
}

// MockAuthRepository is a mock implementation of the IAuthRepository interface.
// It uses an in-memory slice to simulate database operations.
type MockAuthRepository struct {
	users []MockUser
}

// NewMockAuthRepository creates a new mock repository.
func NewMockAuthRepository() *MockAuthRepository {
	return &MockAuthRepository{
		users: []MockUser{},
	}
}

// SaveUser simulates saving a user to the database.
func (r *MockAuthRepository) SaveUser(ctx context.Context, user *User) (*User, error) {
	// Convert user to MockUser for internal storage, assuming User struct is compatible or convertible
	mockUser := MockUser{
		ID:            primitive.NewObjectID(), // Generate a new ID for new users
		Username:      user.Username,
		Email:         user.Email,
		PasswordHash:  user.PasswordHash,
		EmailVerified: user.EmailVerified,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	r.users = append(r.users, mockUser)
	
	// Convert back to User struct to return
	createdUser := &User{
		ID:            mockUser.ID.Hex(), // Return as string ID
		Username:      mockUser.Username,
		Email:         user.Email,
		PasswordHash:  user.PasswordHash,
		EmailVerified: user.EmailVerified,
		CreatedAt:     mockUser.CreatedAt,
		UpdatedAt:     mockUser.UpdatedAt,
	}
	return createdUser, nil
}

// FindUserByEmail simulates finding a user by email.
func (r *MockAuthRepository) FindUserByEmail(ctx context.Context, email string) (*User, error) {
	for _, u := range r.users {
		if u.Email == email {
			// Convert MockUser to User struct to return
			return &User{
				ID:            u.ID.Hex(),
				Username:      u.Username,
				Email:         u.Email,
				PasswordHash:  u.PasswordHash,
				EmailVerified: u.EmailVerified,
				CreatedAt:     u.CreatedAt,
				UpdatedAt:     u.UpdatedAt,
			}, nil
		}
	}
	return nil, mongo.ErrNoDocuments // Simulate not found
}

// FindUserByID simulates finding a user by ID.
func (r *MockAuthRepository) FindUserByID(ctx context.Context, id string) (*User, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid user ID format")
	}
	for _, u := range r.users {
		if u.ID == objID {
			// Convert MockUser to User struct to return
			return &User{
				ID:            u.ID.Hex(),
				Username:      u.Username,
				Email:         u.Email,
				PasswordHash:  u.PasswordHash,
				EmailVerified: u.EmailVerified,
				CreatedAt:     u.CreatedAt,
				UpdatedAt:     u.UpdatedAt,
			}, nil
		}
	}
	return nil, mongo.ErrNoDocuments // Simulate not found
}

// UpdateUserEmailVerification simulates updating a user's email verification status.
func (r *MockAuthRepository) UpdateUserEmailVerification(ctx context.Context, userID string, verified bool) error {
	objID, err := primitive.ObjectIDFromHex(userID)
	if err != nil {
		return errors.New("invalid user ID format")
	}
	for i := range r.users {
		if r.users[i].ID == objID {
			r.users[i].EmailVerified = verified
			r.users[i].UpdatedAt = time.Now()
			return nil
		}
	}
	return mongo.ErrNoDocuments // Simulate not found
}

// --- Test Cases ---

func TestMockAuthRepository_SaveUser(t *testing.T) {
	repo := NewMockAuthRepository()
	ctx := context.Background()
	user := &User{
		Username:      "testuser",
		Email:         "test@example.com",
		PasswordHash:  "hashedpassword",
		EmailVerified: false,
	}

	createdUser, err := repo.SaveUser(ctx, user)
	if err != nil {
		t.Fatalf("SaveUser failed: %v", err)
	}
	if createdUser == nil {
		t.Fatal("SaveUser returned nil user")
	}
	if createdUser.ID == "" {
		t.Error("SaveUser did not assign an ID to the created user")
	}
	if createdUser.Username != user.Username {
		t.Errorf("SaveUser created user with wrong username: got %q, want %q", createdUser.Username, user.Username)
	}
	if len(repo.users) != 1 {
		t.Errorf("Expected 1 user in mock DB, got %d", len(repo.users))
	}
}

func TestMockAuthRepository_FindUserByEmail(t *testing.T) {
	repo := NewMockAuthRepository()
	ctx := context.Background()
	userEmail := "find@example.com"
	user := &User{Username: "finder", Email: userEmail, PasswordHash: "hash"}
	repo.SaveUser(ctx, user)

	// Test case 1: User exists
	foundUser, err := repo.FindUserByEmail(ctx, userEmail)
	if err != nil {
		t.Fatalf("FindUserByEmail(%q) failed: %v", userEmail, err)
	}
	if foundUser == nil {
		t.Fatalf("FindUserByEmail(%q) returned nil user", userEmail)
	}
	if foundUser.Email != userEmail {
		t.Errorf("FindUserByEmail(%q) found wrong user: got %q, want %q", userEmail, foundUser.Email, userEmail)
	}

	// Test case 2: User does not exist
	nonExistentEmail := "notfound@example.com"
	_, err = repo.FindUserByEmail(ctx, nonExistentEmail)
	if err == nil {
		t.Errorf("FindUserByEmail(%q) should return an error (ErrNoDocuments), but got nil", nonExistentEmail)
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		t.Errorf("FindUserByEmail(%q) returned wrong error type: got %T, want mongo.ErrNoDocuments", nonExistentEmail, err)
	}
}

func TestMockAuthRepository_FindUserByID(t *testing.T) {
	repo := NewMockAuthRepository()
	ctx := context.Background()
	user := &User{Username: "findbyid", Email: "findbyid@example.com", PasswordHash: "hash"}
	createdUser, _ := repo.SaveUser(ctx, user)
	userID := createdUser.ID

	// Test case 1: User exists
	foundUser, err := repo.FindUserByID(ctx, userID)
	if err != nil {
		t.Fatalf("FindUserByID(%q) failed: %v", userID, err)
	}
	if foundUser == nil {
		t.Fatalf("FindUserByID(%q) returned nil user", userID)
	}
	if foundUser.ID != userID {
		t.Errorf("FindUserByID(%q) found wrong user ID: got %q, want %q", userID, foundUser.ID, userID)
	}

	// Test case 2: User does not exist (valid ID format, but not in repo)
	nonExistentID := primitive.NewObjectID().Hex()
	_, err = repo.FindUserByID(ctx, nonExistentID)
	if err == nil {
		t.Errorf("FindUserByID(%q) should return an error (ErrNoDocuments), but got nil", nonExistentID)
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		t.Errorf("FindUserByID(%q) returned wrong error type: got %T, want mongo.ErrNoDocuments", nonExistentID, err)
	}

	// Test case 3: Invalid ID format
	invalidID := "not-a-valid-objectid"
	_, err = repo.FindUserByID(ctx, invalidID)
	if err == nil {
		t.Errorf("FindUserByID(%q) should return an error for invalid ID format, but got nil", invalidID)
	}
	if err.Error() != "invalid user ID format" {
		t.Errorf("FindUserByID(%q) returned wrong error message for invalid ID: got %q, want %q", invalidID, err.Error(), "invalid user ID format")
	}
}

func TestMockAuthRepository_UpdateUserEmailVerification(t *testing.T) {
	repo := NewMockAuthRepository()
	ctx := context.Background()
	user := &User{Username: "updateverify", Email: "updateverify@example.com", PasswordHash: "hash", EmailVerified: false}
	createdUser, _ := repo.SaveUser(ctx, user)
	userID := createdUser.ID

	// Test case 1: Update to verified
	err := repo.UpdateUserEmailVerification(ctx, userID, true)
	if err != nil {
		t.Fatalf("UpdateUserEmailVerification(%q, true) failed: %v", userID, err)
	}
	updatedUser, _ := repo.FindUserByID(ctx, userID)
	if !updatedUser.EmailVerified {
		t.Errorf("UpdateUserEmailVerification(%q, true) failed to set EmailVerified to true", userID)
	}

	// Test case 2: Update to unverified
	err = repo.UpdateUserEmailVerification(ctx, userID, false)
	if err != nil {
		t.Fatalf("UpdateUserEmailVerification(%q, false) failed: %v", userID, err)
	}
	updatedUser, _ = repo.FindUserByID(ctx, userID)
	if updatedUser.EmailVerified {
		t.Errorf("UpdateUserEmailVerification(%q, false) failed to set EmailVerified to false", userID)
	}

	// Test case 3: User not found
	nonExistentID := primitive.NewObjectID().Hex()
	err = repo.UpdateUserEmailVerification(ctx, nonExistentID, true)
	if err == nil {
		t.Errorf("UpdateUserEmailVerification(%q, true) should return an error (ErrNoDocuments), but got nil", nonExistentID)
	}
	if !errors.Is(err, mongo.ErrNoDocuments) {
		t.Errorf("UpdateUserEmailVerification(%q, true) returned wrong error type: got %T, want mongo.ErrNoDocuments", nonExistentID, err)
	}

	// Test case 4: Invalid ID format
	invalidID := "not-a-valid-objectid"
	err = repo.UpdateUserEmailVerification(ctx, invalidID, true)
	if err == nil {
		t.Errorf("UpdateUserEmailVerification(%q, true) should return an error for invalid ID format, but got nil", invalidID)
	}
	if err.Error() != "invalid user ID format" {
		t.Errorf("UpdateUserEmailVerification(%q, true) returned wrong error message for invalid ID: got %q, want %q", invalidID, err.Error(), "invalid user ID format")
	}
}

// Note: In a real scenario, the User struct in auth/service.go would be imported and used directly.
// For this mock, we've used a MockUser struct for simulation. Ensure to adjust if the actual User struct differs.
// Also, the real repository would interact with a MongoDB client, which would need proper mocking or a test database setup.
// The above tests use mongo.ErrNoDocuments to simulate DB behavior.
