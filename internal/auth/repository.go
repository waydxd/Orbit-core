package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/waydxd/Orbit-core/internal/shared/database"
	"github.com/waydxd/Orbit-core/internal/shared/database/db"
	"github.com/waydxd/Orbit-core/internal/shared/models"
)

// Repository defines database operations for auth
type Repository interface {
	CreateUser(ctx context.Context, user *models.User) error
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByID(ctx context.Context, id string) (*models.User, error)
	UpdateUser(ctx context.Context, user *models.User) error
	DeleteUser(ctx context.Context, id string) error
	SaveSession(ctx context.Context, userID, tokenHash string, expiresAt time.Time) (string, error)
	GetSessionByToken(ctx context.Context, tokenHash string) (*models.Session, error)
	DeleteSession(ctx context.Context, sessionID string) error
}

// SQLRepository implements Repository interface using PostgreSQL
type SQLRepository struct {
	queries *db.Queries
	pool    *database.DB
}

// NewSQLRepository creates a new SQL repository
func NewSQLRepository(pool *database.DB) Repository {
	return &SQLRepository{
		queries: db.New(pool.Pool),
		pool:    pool,
	}
}

// CreateUser inserts a new user into the database
func (r *SQLRepository) CreateUser(ctx context.Context, user *models.User) error {
	params := db.CreateUserParams{
		ID:            database.StringToUUID(user.ID),
		Email:         user.Email,
		PasswordHash:  user.PasswordHash,
		FirstName:     database.StringToText(user.FirstName),
		LastName:      database.StringToText(user.LastName),
		EmailVerified: user.EmailVerified,
		CreatedAt:     database.TimeToTimestamptz(user.CreatedAt),
		UpdatedAt:     database.TimeToTimestamptz(user.UpdatedAt),
	}

	err := r.queries.CreateUser(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

// GetUserByEmail retrieves a user by email
func (r *SQLRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	row, err := r.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	return &models.User{
		ID:            database.UUIDToString(row.ID),
		Email:         row.Email,
		PasswordHash:  row.PasswordHash,
		FirstName:     database.TextToString(row.FirstName),
		LastName:      database.TextToString(row.LastName),
		EmailVerified: row.EmailVerified,
		CreatedAt:     database.TimestamptzToTime(row.CreatedAt),
		UpdatedAt:     database.TimestamptzToTime(row.UpdatedAt),
	}, nil
}

// GetUserByID retrieves a user by ID
func (r *SQLRepository) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	row, err := r.queries.GetUserByID(ctx, database.StringToUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}

	return &models.User{
		ID:            database.UUIDToString(row.ID),
		Email:         row.Email,
		PasswordHash:  row.PasswordHash,
		FirstName:     database.TextToString(row.FirstName),
		LastName:      database.TextToString(row.LastName),
		EmailVerified: row.EmailVerified,
		CreatedAt:     database.TimestamptzToTime(row.CreatedAt),
		UpdatedAt:     database.TimestamptzToTime(row.UpdatedAt),
	}, nil
}

// UpdateUser updates an existing user
func (r *SQLRepository) UpdateUser(ctx context.Context, user *models.User) error {
	params := db.UpdateUserParams{
		Email:         user.Email,
		PasswordHash:  user.PasswordHash,
		FirstName:     database.StringToText(user.FirstName),
		LastName:      database.StringToText(user.LastName),
		EmailVerified: user.EmailVerified,
		UpdatedAt:     database.TimeToTimestamptz(time.Now()),
		ID:            database.StringToUUID(user.ID),
	}

	err := r.queries.UpdateUser(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	return nil
}

// DeleteUser deletes a user
func (r *SQLRepository) DeleteUser(ctx context.Context, id string) error {
	err := r.queries.DeleteUser(ctx, database.StringToUUID(id))
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	return nil
}

// SaveSession saves a session to the database
func (r *SQLRepository) SaveSession(ctx context.Context, userID, tokenHash string, expiresAt time.Time) (string, error) {
	params := db.CreateSessionParams{
		UserID:    database.StringToUUID(userID),
		TokenHash: tokenHash,
		ExpiresAt: database.TimeToTimestamptz(expiresAt),
	}

	id, err := r.queries.CreateSession(ctx, params)
	if err != nil {
		return "", fmt.Errorf("failed to save session: %w", err)
	}
	return database.UUIDToString(id), nil
}

// GetSessionByToken retrieves a session by token hash
func (r *SQLRepository) GetSessionByToken(ctx context.Context, tokenHash string) (*models.Session, error) {
	row, err := r.queries.GetSessionByToken(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("session not found or expired")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return &models.Session{
		ID:        database.UUIDToString(row.ID),
		UserID:    database.UUIDToString(row.UserID),
		TokenHash: row.TokenHash,
		ExpiresAt: database.TimestamptzToTime(row.ExpiresAt),
		CreatedAt: database.TimestamptzToTime(row.CreatedAt),
	}, nil
}

// DeleteSession deletes a session
func (r *SQLRepository) DeleteSession(ctx context.Context, sessionID string) error {
	err := r.queries.DeleteSession(ctx, database.StringToUUID(sessionID))
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}
