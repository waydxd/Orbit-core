package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/middleware"
)

// UpdateProfileRequest represents profile update request payload.
// All fields are optional - only provided fields will be updated.
type UpdateProfileRequest struct {
	FirstName      *string `json:"first_name,omitempty"`
	LastName       *string `json:"last_name,omitempty"`
	Username       *string `json:"username,omitempty"`
	ProfilePicture *string `json:"profile_picture,omitempty"`
	Region         *string `json:"region,omitempty"`
	Timezone       *string `json:"timezone,omitempty"`
	Gender         *string `json:"gender,omitempty"`
	BirthDate      *string `json:"birth_date,omitempty"` // ISO 8601 date format (YYYY-MM-DD)
}

// ProfileResponse represents the profile response
type ProfileResponse struct {
	ID             string `json:"id"`
	Email          string `json:"email"`
	FirstName      string `json:"first_name,omitempty"`
	LastName       string `json:"last_name,omitempty"`
	Username       string `json:"username"`
	ProfilePicture string `json:"profile_picture,omitempty"`
	Region         string `json:"region,omitempty"`
	Timezone       string `json:"timezone,omitempty"`
	Gender         string `json:"gender,omitempty"`
	BirthDate      string `json:"birth_date,omitempty"`
	EmailVerified  bool   `json:"email_verified"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

// getProfile handles GET /profile
func (s *Service) getProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract user ID from context (set by auth middleware)
	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "user not found"})
			return
		}
		s.logger.Error("failed to get user profile", "err", err, "user_id", userID)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return
	}

	response := s.userToProfileResponse(user)
	_ = json.NewEncoder(w).Encode(response)
}

// updateProfile handles PUT /profile
func (s *Service) updateProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract user ID from context (set by auth middleware)
	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	// Limit request body size to prevent large allocations (e.g., from large data URLs)
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB limit

	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		s.logger.Error("invalid update profile request", "err", err)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Get existing user
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "user not found"})
			return
		}
		s.logger.Error("failed to get user", "err", err, "user_id", userID)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return
	}

	// Apply updates to user
	if err := s.applyProfileUpdates(ctx, user, &req); err != nil {
		s.encodeProfileError(w, err)
		return
	}

	// Update user in database
	if err := s.repo.UpdateUser(ctx, user); err != nil {
		s.handleProfileUpdateError(w, err)
		return
	}

	// Re-fetch user to ensure we have the latest data (including updated_at)
	if updatedUser, err := s.repo.GetUserByID(ctx, userID); err == nil {
		user = updatedUser
	} else {
		s.logger.Error("failed to reload user after update", "err", err, "user_id", userID)
	}

	response := s.userToProfileResponse(user)
	_ = json.NewEncoder(w).Encode(response)
}

// userToProfileResponse converts a User model to ProfileResponse
func (s *Service) userToProfileResponse(user *models.User) ProfileResponse {
	response := ProfileResponse{
		ID:            user.ID,
		Email:         user.Email,
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		Username:      user.Username,
		Region:        user.Region,
		Timezone:      user.Timezone,
		Gender:        user.Gender,
		EmailVerified: user.EmailVerified,
		CreatedAt:     user.CreatedAt.Format(time.RFC3339),
		UpdatedAt:     user.UpdatedAt.Format(time.RFC3339),
	}

	if user.ProfilePicture != "" {
		response.ProfilePicture = user.ProfilePicture
	}

	if !user.BirthDate.IsZero() {
		response.BirthDate = user.BirthDate.Format("2006-01-02")
	}

	return response
}

// applyProfileUpdates applies the profile update request fields to the user model
func (s *Service) applyProfileUpdates(ctx context.Context, user *models.User, req *UpdateProfileRequest) error {
	if req.FirstName != nil {
		user.FirstName = *req.FirstName
	}

	if req.LastName != nil {
		user.LastName = *req.LastName
	}

	if req.Username != nil {
		if err := s.applyUsernameUpdate(ctx, user, *req.Username); err != nil {
			return err
		}
	}

	if req.ProfilePicture != nil {
		if err := s.applyProfilePictureUpdate(user, *req.ProfilePicture); err != nil {
			return err
		}
	}

	if req.Region != nil {
		user.Region = *req.Region
	}

	if req.Timezone != nil {
		if err := s.applyTimezoneUpdate(user, *req.Timezone); err != nil {
			return err
		}
	}

	if req.Gender != nil {
		if err := s.applyGenderUpdate(user, *req.Gender); err != nil {
			return err
		}
	}

	if req.BirthDate != nil {
		if err := s.applyBirthDateUpdate(user, *req.BirthDate); err != nil {
			return err
		}
	}

	return nil
}

// applyUsernameUpdate validates and applies username changes
func (s *Service) applyUsernameUpdate(ctx context.Context, user *models.User, username string) error {
	if err := s.validateUsername(username); err != nil {
		return err
	}
	// Check if username is already taken by another user
	existingUser, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil && !errors.Is(err, ErrUserNotFound) {
		return fmt.Errorf("failed to check username availability: %w", err)
	}
	if existingUser != nil && existingUser.ID != user.ID {
		return fmt.Errorf("username already taken")
	}
	user.Username = username
	return nil
}

// applyProfilePictureUpdate validates and applies profile picture changes
func (s *Service) applyProfilePictureUpdate(user *models.User, url string) error {
	if err := s.validateProfilePicture(url); err != nil {
		return err
	}
	user.ProfilePicture = url
	return nil
}

// applyTimezoneUpdate validates and applies timezone changes
func (s *Service) applyTimezoneUpdate(user *models.User, tz string) error {
	if err := s.validateTimezone(tz); err != nil {
		return err
	}
	user.Timezone = tz
	return nil
}

// applyGenderUpdate validates and applies gender changes
func (s *Service) applyGenderUpdate(user *models.User, gender string) error {
	if err := s.validateGender(gender); err != nil {
		return err
	}
	user.Gender = strings.ToLower(gender)
	return nil
}

// applyBirthDateUpdate validates and applies birth date changes
func (s *Service) applyBirthDateUpdate(user *models.User, birthDateStr string) error {
	if birthDateStr == "" {
		user.BirthDate = time.Time{}
		return nil
	}
	birthDate, err := time.Parse("2006-01-02", birthDateStr)
	if err != nil {
		return fmt.Errorf("invalid birth_date format, use YYYY-MM-DD")
	}
	if err := s.validateBirthDate(birthDate); err != nil {
		return err
	}
	user.BirthDate = birthDate
	return nil
}

// encodeProfileError encodes a profile error response
func (s *Service) encodeProfileError(w http.ResponseWriter, err error) {
	errMsg := err.Error()

	switch {
	case strings.Contains(errMsg, "already taken"):
		w.WriteHeader(http.StatusConflict)
	case strings.Contains(errMsg, "failed to check username availability"):
		w.WriteHeader(http.StatusInternalServerError)
	default:
		w.WriteHeader(http.StatusBadRequest)
	}

	_ = json.NewEncoder(w).Encode(map[string]string{"error": errMsg})
}

// handleProfileUpdateError handles database update errors
func (s *Service) handleProfileUpdateError(w http.ResponseWriter, err error) {
	errStr := err.Error()
	if strings.Contains(errStr, "unique_username") || strings.Contains(errStr, "duplicate key") {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "username already taken"})
		return
	}
	s.logger.Error("failed to update user profile", "err", err)
	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to update profile"})
}

// validateUsername validates the username format and length
func (s *Service) validateUsername(username string) error {
	matched, err := regexp.MatchString(`^[a-zA-Z0-9_-]{3,50}$`, username)
	if err != nil {
		return fmt.Errorf("invalid username")
	}
	if !matched {
		return fmt.Errorf("username must be between 3 and 50 characters and contain only ASCII letters, numbers, underscores, and hyphens")
	}
	// Check for consecutive special characters
	re := regexp.MustCompile(`[_-]{2,}`)
	if re.MatchString(username) {
		return fmt.Errorf("username cannot contain consecutive underscores or hyphens")
	}
	return nil
}

// validateProfilePicture validates the profile picture URL and image properties
func (s *Service) validateProfilePicture(url string) error {
	if url == "" {
		return nil
	}
	// Basic URL validation
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "data:image/") {
		return fmt.Errorf("invalid profile picture URL")
	}

	// If it's a data URL, validate the image
	if strings.HasPrefix(url, "data:image/") {
		return s.validateDataURLImage(url)
	}

	// For regular URLs, we could fetch and validate, but for now just check length
	if len(url) > 2048 {
		return fmt.Errorf("profile picture URL too long")
	}

	return nil
}

// validateDataURLImage validates a data URL image
func (s *Service) validateDataURLImage(dataURL string) error {
	const maxBinarySize = 5 * 1024 * 1024 // 5MB for the actual image
	// Base64 encoding increases size by ~33%, so limit is ~6.67MB for encoded data
	const maxEncodedSize = maxBinarySize * 4 / 3

	// Extract the image data
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid data URL format")
	}

	// Check image format from MIME type
	mimeType := strings.TrimPrefix(parts[0], "data:")
	mimeType = strings.TrimSuffix(mimeType, ";base64")

	allowedFormats := []string{"image/jpeg", "image/png", "image/gif"}
	valid := false
	for _, format := range allowedFormats {
		if mimeType == format {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid image format, allowed formats: JPEG, PNG, GIF")
	}

	// Check size - comparing against encoded size including base64 overhead
	if len(dataURL) > maxEncodedSize {
		return fmt.Errorf("profile picture size exceeds 5MB limit")
	}

	// Decode base64 to check actual binary size
	decoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(parts[1]))
	actualSize, err := io.Copy(io.Discard, decoder)
	if err != nil {
		return fmt.Errorf("invalid base64 data")
	}

	if actualSize > maxBinarySize {
		return fmt.Errorf("profile picture size exceeds 5MB limit")
	}

	return nil
}

// validateTimezone validates the timezone string
func (s *Service) validateTimezone(tz string) error {
	if tz == "" {
		return nil
	}
	// Try to load the timezone
	_, err := time.LoadLocation(tz)
	if err != nil {
		return fmt.Errorf("invalid timezone: %s", tz)
	}
	return nil
}

// validateGender validates the gender value
func (s *Service) validateGender(gender string) error {
	if gender == "" {
		return nil
	}
	// Inclusive gender options
	validGenders := []string{
		"male",
		"female",
		"non-binary",
		"prefer_not_to_say",
		"other",
	}

	genderLower := strings.ToLower(gender)
	for _, valid := range validGenders {
		if genderLower == valid {
			return nil
		}
	}

	return fmt.Errorf("invalid gender value, allowed values: male, female, non-binary, prefer_not_to_say, other")
}

// validateBirthDate validates the birth date
func (s *Service) validateBirthDate(birthDate time.Time) error {
	now := time.Now()

	// Check if birth date is in the future
	if birthDate.After(now) {
		return fmt.Errorf("birth date cannot be in the future")
	}

	// Check if age is reasonable (e.g., not more than 150 years old)
	age := now.Year() - birthDate.Year()
	if birthDate.After(now.AddDate(-age, 0, 0)) {
		age--
	}

	if age > 150 {
		return fmt.Errorf("birth date must be within the last 150 years")
	}

	// Check if user is at least 13 years old (common age restriction)
	if age < 13 {
		return fmt.Errorf("you must be at least 13 years old to use this service")
	}

	return nil
}
