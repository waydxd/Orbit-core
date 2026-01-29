package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/waydxd/Orbit-core/internal/shared/models"
	"github.com/waydxd/Orbit-core/pkg/middleware"
)

// UpdateProfileRequest represents profile update request payload
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
		s.logger.Error("failed to get user profile", "err", err, "user_id", userID)
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "user not found"})
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
		s.logger.Error("failed to get user", "err", err, "user_id", userID)
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "user not found"})
		return
	}

	// Validate and update fields
	if req.FirstName != nil {
		user.FirstName = *req.FirstName
	}

	if req.LastName != nil {
		user.LastName = *req.LastName
	}

	if req.Username != nil {
		if err := s.validateUsername(*req.Username); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		// Check if username is already taken by another user
		existingUser, _ := s.repo.GetUserByUsername(ctx, *req.Username)
		if existingUser != nil && existingUser.ID != userID {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "username already taken"})
			return
		}
		user.Username = *req.Username
	}

	if req.ProfilePicture != nil {
		if err := s.validateProfilePicture(*req.ProfilePicture); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		user.ProfilePicture = *req.ProfilePicture
	}

	if req.Region != nil {
		user.Region = *req.Region
	}

	if req.Timezone != nil {
		if err := s.validateTimezone(*req.Timezone); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		user.Timezone = *req.Timezone
	}

	if req.Gender != nil {
		if err := s.validateGender(*req.Gender); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		user.Gender = *req.Gender
	}

	if req.BirthDate != nil {
		birthDate, err := time.Parse("2006-01-02", *req.BirthDate)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid birth_date format, use YYYY-MM-DD"})
			return
		}
		if err := s.validateBirthDate(birthDate); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		user.BirthDate = birthDate
	}

	// Update user in database
	if err := s.repo.UpdateUser(ctx, user); err != nil {
		s.logger.Error("failed to update user profile", "err", err, "user_id", userID)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to update profile"})
		return
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

// validateUsername validates the username format and length
func (s *Service) validateUsername(username string) error {
	if len(username) < 3 || len(username) > 50 {
		return fmt.Errorf("username must be between 3 and 50 characters")
	}
	// Username can contain letters, numbers, underscores, and hyphens
	for _, c := range username {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
			return fmt.Errorf("username can only contain letters, numbers, underscores, and hyphens")
		}
	}
	return nil
}

// validateProfilePicture validates the profile picture URL and image properties
func (s *Service) validateProfilePicture(url string) error {
	// Basic URL validation
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "data:") {
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
	const maxSize = 5 * 1024 * 1024 // 5MB

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

	// Check size
	if len(dataURL) > maxSize {
		return fmt.Errorf("profile picture size exceeds 5MB limit")
	}

	// For a more thorough validation, we could decode the base64 and check image dimensions
	// But for now, basic validation is sufficient
	return nil
}

// validateImageDimensions validates image aspect ratio (unused for now, kept for future use)
func (s *Service) validateImageDimensions(img image.Image) error {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Check aspect ratio (should be roughly square, allow 0.8 to 1.2 ratio)
	aspectRatio := float64(width) / float64(height)
	if aspectRatio < 0.8 || aspectRatio > 1.2 {
		return fmt.Errorf("profile picture must have a square aspect ratio (0.8-1.2)")
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
		return fmt.Errorf("invalid birth date")
	}
	
	// Check if user is at least 13 years old (common age restriction)
	if age < 13 {
		return fmt.Errorf("you must be at least 13 years old to use this service")
	}
	
	return nil
}

// Helper function to check aspect ratio tolerance
func aspectRatioWithinTolerance(width, height int, targetRatio float64, tolerance float64) bool {
	actualRatio := float64(width) / float64(height)
	return math.Abs(actualRatio-targetRatio) <= tolerance
}

