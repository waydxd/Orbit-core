package auth

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/waydxd/Orbit-core/internal/shared/database"
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
	db *database.DB
}

// NewSQLRepository creates a new SQL repository
func NewSQLRepository(db *database.DB) Repository {
	return &SQLRepository{db: db}
}

// CreateUser inserts a new user into the database
func (r *SQLRepository) CreateUser(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (id, email, password_hash, first_name, last_name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.ExecContext(ctx, query,
		user.ID,
		user.Email,
		user.PasswordHash,
		user.FirstName,
		user.LastName,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

// GetUserByEmail retrieves a user by email
func (r *SQLRepository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, first_name, last_name, created_at, updated_at
		FROM users WHERE email = $1
	`
	user := &models.User{}
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.FirstName,
		&user.LastName,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return user, nil
}

// GetUserByID retrieves a user by ID
func (r *SQLRepository) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, first_name, last_name, created_at, updated_at
		FROM users WHERE id = $1
	`
	user := &models.User{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.FirstName,
		&user.LastName,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}
	return user, nil
}

// UpdateUser updates an existing user
func (r *SQLRepository) UpdateUser(ctx context.Context, user *models.User) error {
	query := `
		UPDATE users
		SET email = $1, password_hash = $2, first_name = $3, last_name = $4, updated_at = $5
		WHERE id = $6
	`
	_, err := r.db.ExecContext(ctx, query,
		user.Email,
		user.PasswordHash,
		user.FirstName,
		user.LastName,
		time.Now(),
		user.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	return nil
}

// DeleteUser deletes a user
func (r *SQLRepository) DeleteUser(ctx context.Context, id string) error {
	query := "DELETE FROM users WHERE id = $1"
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	return nil
}

// SaveSession saves a session to the database
func (r *SQLRepository) SaveSession(ctx context.Context, userID, tokenHash string, expiresAt time.Time) (string, error) {
	query := `
		INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at)
		VALUES (gen_random_uuid(), $1, $2, $3, CURRENT_TIMESTAMP)
		RETURNING id
	`
	var sessionID string
	err := r.db.QueryRowContext(ctx, query, userID, tokenHash, expiresAt).Scan(&sessionID)
	if err != nil {
		return "", fmt.Errorf("failed to save session: %w", err)
	}
	return sessionID, nil
}

// GetSessionByToken retrieves a session by token hash
func (r *SQLRepository) GetSessionByToken(ctx context.Context, tokenHash string) (*models.Session, error) {
	query := `
		SELECT id, user_id, token_hash, expires_at, created_at
		FROM sessions WHERE token_hash = $1 AND expires_at > CURRENT_TIMESTAMP
	`
	session := &models.Session{}
	err := r.db.QueryRowContext(ctx, query, tokenHash).Scan(
		&session.ID,
		&session.UserID,
		&session.TokenHash,
		&session.ExpiresAt,
		&session.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found or expired")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	return session, nil
}

// DeleteSession deletes a session
func (r *SQLRepository) DeleteSession(ctx context.Context, sessionID string) error {
	query := "DELETE FROM sessions WHERE id = $1"
	_, err := r.db.ExecContext(ctx, query, sessionID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}
