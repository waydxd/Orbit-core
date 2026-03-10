package auth

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/waydxd/Orbit-core/internal/shared/models"
)

func TestValidateUsername(t *testing.T) {
	s := &Service{}

	valid := "john_doe"
	if err := s.validateUsername(valid); err != nil {
		t.Fatalf("expected valid username, got error: %v", err)
	}

	cases := []string{"ab", "john$", "john__doe"}
	for _, c := range cases {
		if err := s.validateUsername(c); err == nil {
			t.Fatalf("expected username %q to be invalid", c)
		}
	}
}

func TestValidateDataURLImage(t *testing.T) {
	s := &Service{}

	// small valid PNG-ish payload (not a real PNG, but sufficient for base64 decode)
	payload := []byte{0x89, 0x50, 0x4E, 0x47}
	enc := base64.StdEncoding.EncodeToString(payload)
	dataURL := "data:image/png;base64," + enc
	if err := s.validateDataURLImage(dataURL); err != nil {
		t.Fatalf("expected valid data URL image, got: %v", err)
	}

	// invalid format
	badFormat := "data:image/svg+xml;base64," + enc
	if err := s.validateDataURLImage(badFormat); err == nil {
		t.Fatalf("expected invalid image format to error")
	}

	// invalid base64
	badBase64 := "data:image/png;base64,@@@"
	if err := s.validateDataURLImage(badBase64); err == nil {
		t.Fatalf("expected invalid base64 to error")
	}
}

func TestValidateTimezone(t *testing.T) {
	s := &Service{}

	if err := s.validateTimezone("UTC"); err != nil {
		t.Fatalf("expected UTC to be valid timezone: %v", err)
	}
	if err := s.validateTimezone("Mars/Phobos"); err == nil {
		t.Fatalf("expected invalid timezone to error")
	}
	if err := s.validateTimezone("America/New_York"); err != nil {
		t.Fatalf("expected America/New_York to be valid timezone: %v", err)
	}
}

func TestValidateGender(t *testing.T) {
	s := &Service{}

	if err := s.validateGender("Non-Binary"); err != nil {
		t.Fatalf("expected 'Non-Binary' to be valid: %v", err)
	}
	if err := s.validateGender("robot"); err == nil {
		t.Fatalf("expected invalid gender to error")
	}
}

func TestValidateBirthDate(t *testing.T) {
	s := &Service{}

	// too young (10 years)
	tooYoung := time.Now().AddDate(-10, 0, 0)
	if err := s.validateBirthDate(tooYoung); err == nil {
		t.Fatalf("expected too young birth date to error")
	}

	// too old (151 years)
	tooOld := time.Now().AddDate(-151, 0, 0)
	if err := s.validateBirthDate(tooOld); err == nil {
		t.Fatalf("expected too old birth date to error")
	}

	// valid (30 years)
	valid := time.Now().AddDate(-30, 0, 0)
	if err := s.validateBirthDate(valid); err != nil {
		t.Fatalf("expected valid birth date, got: %v", err)
	}
}

func TestApplyBirthDateUpdate(t *testing.T) {
	s := &Service{}
	user := &models.User{}

	// empty string should clear birthdate
	if err := s.applyBirthDateUpdate(user, ""); err != nil {
		t.Fatalf("unexpected error clearing birth date: %v", err)
	}
	if !user.BirthDate.IsZero() {
		t.Fatalf("expected birth date to be zero after clearing")
	}

	// valid date string
	if err := s.applyBirthDateUpdate(user, "2000-01-02"); err != nil {
		t.Fatalf("unexpected error applying birth date: %v", err)
	}
	if user.BirthDate.Year() != 2000 {
		t.Fatalf("expected birth year 2000, got %d", user.BirthDate.Year())
	}
}

func TestUserToProfileResponseDates(t *testing.T) {
	s := &Service{}
	now := time.Now().UTC().Truncate(time.Second)
	user := &models.User{
		ID:        "u1",
		Email:     "a@b.com",
		Username:  "u1",
		CreatedAt: now,
		UpdatedAt: now,
	}

	resp := s.userToProfileResponse(user)
	if resp.CreatedAt == "" || resp.UpdatedAt == "" {
		t.Fatalf("expected created/updated to be formatted")
	}
}

func TestValidateUsernameConcurrentSpecials(t *testing.T) {
	s := &Service{}
	if err := s.validateUsername("john--doe"); err == nil {
		t.Fatalf("expected consecutive hyphens to be invalid")
	}
}

// small helper to satisfy linter for unused import in some environments
var _ = context.Background
