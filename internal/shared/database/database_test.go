package database

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestStringToUUID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected pgtype.UUID
	}{
		{"valid uuid", "550e8400-e29b-41d4-a716-446655440000", pgtype.UUID{Bytes: [16]byte{0x55, 0x0e, 0x84, 0x00, 0xe2, 0x9b, 0x41, 0xd4, 0xa7, 0x16, 0x44, 0x66, 0x55, 0x44, 0x00, 0x00}, Valid: true}},
		{"empty string", "", pgtype.UUID{Valid: false}},
		{"invalid uuid", "invalid", pgtype.UUID{Valid: false}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StringToUUID(tt.input)
			if result.Valid != tt.expected.Valid {
				t.Errorf("StringToUUID(%q).Valid = %v, want %v", tt.input, result.Valid, tt.expected.Valid)
			}
		})
	}
}

func TestUUIDToString(t *testing.T) {
	validUUID := pgtype.UUID{
		Bytes: [16]byte{0x55, 0x0e, 0x84, 0x00, 0xe2, 0x9b, 0x41, 0xd4, 0xa7, 0x16, 0x44, 0x66, 0x55, 0x44, 0x00, 0x00},
		Valid: true,
	}

	tests := []struct {
		name     string
		input    pgtype.UUID
		expected string
	}{
		{"valid uuid", validUUID, "550e8400-e29b-41d4-a716-446655440000"},
		{"invalid uuid", pgtype.UUID{Valid: false}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := UUIDToString(tt.input)
			if result != tt.expected {
				t.Errorf("UUIDToString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestStringToText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected pgtype.Text
	}{
		{"non-empty string", "hello", pgtype.Text{String: "hello", Valid: true}},
		{"empty string", "", pgtype.Text{String: "", Valid: false}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StringToText(tt.input)
			if result.Valid != tt.expected.Valid || result.String != tt.expected.String {
				t.Errorf("StringToText(%q) = (%v, %q), want (%v, %q)", tt.input, result.Valid, result.String, tt.expected.Valid, tt.expected.String)
			}
		})
	}
}

func TestTextToString(t *testing.T) {
	tests := []struct {
		name     string
		input    pgtype.Text
		expected string
	}{
		{"valid text", pgtype.Text{String: "hello", Valid: true}, "hello"},
		{"invalid text", pgtype.Text{Valid: false}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TextToString(tt.input)
			if result != tt.expected {
				t.Errorf("TextToString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTimeToTimestamptz(t *testing.T) {
	now := time.Now()
	result := TimeToTimestamptz(now)
	if !result.Valid {
		t.Error("expected valid timestamptz for non-zero time")
	}

	zeroResult := TimeToTimestamptz(time.Time{})
	if zeroResult.Valid {
		t.Error("expected invalid timestamptz for zero time")
	}
}

func TestTimestamptzToTime(t *testing.T) {
	now := time.Now()
	input := pgtype.Timestamptz{Time: now, Valid: true}
	result := TimestamptzToTime(input)
	if result.IsZero() {
		t.Error("expected non-zero time from valid timestamptz")
	}

	invalidInput := pgtype.Timestamptz{Valid: false}
	result = TimestamptzToTime(invalidInput)
	if !result.IsZero() {
		t.Error("expected zero time from invalid timestamptz")
	}
}

func TestFloat64ToNumeric(t *testing.T) {
	result := Float64ToNumeric(123.45)
	if result.Valid {
		t.Error("expected invalid numeric without proper scan")
	}
}

func TestNumericToFloat64(t *testing.T) {
	tests := []struct {
		name     string
		input    pgtype.Numeric
		expected float64
	}{
		{"invalid numeric", pgtype.Numeric{Valid: false}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NumericToFloat64(tt.input)
			if result != tt.expected {
				t.Errorf("NumericToFloat64() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIntToInt32(t *testing.T) {
	result := IntToInt32(100)
	if result != 100 {
		t.Errorf("IntToInt32(100) = %d, want 100", result)
	}

	result = IntToInt32(-100)
	if result != -100 {
		t.Errorf("IntToInt32(-100) = %d, want -100", result)
	}
}

func TestTimeToDate(t *testing.T) {
	now := time.Now()
	result := TimeToDate(now)
	if !result.Valid {
		t.Error("expected valid date for non-zero time")
	}

	zeroResult := TimeToDate(time.Time{})
	if zeroResult.Valid {
		t.Error("expected invalid date for zero time")
	}
}

func TestDateToTime(t *testing.T) {
	now := time.Now()
	input := pgtype.Date{Time: now, Valid: true}
	result := DateToTime(input)
	if result.IsZero() {
		t.Error("expected non-zero time from valid date")
	}

	invalidInput := pgtype.Date{Valid: false}
	result = DateToTime(invalidInput)
	if !result.IsZero() {
		t.Error("expected zero time from invalid date")
	}
}

func TestBuildMongoURI_WithCredentials(t *testing.T) {
	t.Setenv("MONGODB_URI", "")
	t.Setenv("MONGO_USER", "")
	t.Setenv("MONGO_PASSWORD", "")

	uri := BuildMongoURI("user", "pass", "localhost", "testdb")
	if uri == "" {
		t.Error("expected non-empty URI with credentials")
	}
}

func TestBuildMongoURI_EnvVar(t *testing.T) {
	t.Setenv("MONGODB_URI", "mongodb://envhost:27017/envdb")
	t.Setenv("MONGO_USER", "")
	t.Setenv("MONGO_PASSWORD", "")

	uri := BuildMongoURI("", "", "", "")
	if uri != "mongodb://envhost:27017/envdb" {
		t.Errorf("BuildMongoURI() = %q, want mongodb://envhost:27017/envdb", uri)
	}
}

func TestBuildMongoURI_Default(t *testing.T) {
	t.Setenv("MONGODB_URI", "")
	t.Setenv("MONGO_USER", "")
	t.Setenv("MONGO_PASSWORD", "")

	uri := BuildMongoURI("", "", "", "")
	if uri != "mongodb://localhost:27017/orbit" {
		t.Errorf("BuildMongoURI() = %q, want mongodb://localhost:27017/orbit", uri)
	}
}

func TestGetHostAndDB(t *testing.T) {
	t.Setenv("MONGODB_HOST", "")
	t.Setenv("MONGODB_DB", "")

	host, db := getHostAndDB("customhost", "customdb")
	if host != "customhost" {
		t.Errorf("getHostAndDB host = %q, want customhost", host)
	}
	if db != "customdb" {
		t.Errorf("getHostAndDB db = %q, want customdb", db)
	}
}

func TestGetHostAndDB_EnvFallback(t *testing.T) {
	t.Setenv("MONGODB_HOST", "envhost")
	t.Setenv("MONGODB_DB", "envdb")

	host, db := getHostAndDB("", "")
	if host != "envhost" {
		t.Errorf("getHostAndDB host = %q, want envhost", host)
	}
	if db != "envdb" {
		t.Errorf("getHostAndDB db = %q, want envdb", db)
	}
}

func TestGetCredentials_Explicit(t *testing.T) {
	user, pass, ok := getCredentials("user", "pass")
	if !ok || user != "user" || pass != "pass" {
		t.Errorf("getCredentials() = (%q, %q, %v), want (user, pass, true)", user, pass, ok)
	}
}

func TestGetCredentials_EnvFallback(t *testing.T) {
	t.Setenv("MONGO_USER", "envuser")
	t.Setenv("MONGO_PASSWORD", "envpass")

	user, pass, ok := getCredentials("", "")
	if !ok || user != "envuser" || pass != "envpass" {
		t.Errorf("getCredentials() = (%q, %q, %v), want (envuser, envpass, true)", user, pass, ok)
	}
}

func TestGetCredentials_None(t *testing.T) {
	t.Setenv("MONGO_USER", "")
	t.Setenv("MONGO_PASSWORD", "")

	_, _, ok := getCredentials("", "")
	if ok {
		t.Error("expected getCredentials to return false with no credentials")
	}
}

func TestBuildParams(t *testing.T) {
	t.Setenv("MONGODB_AUTH_SOURCE", "")
	t.Setenv("MONGODB_PARAMS", "")

	params := buildParams()
	if params != "authSource=admin" {
		t.Errorf("buildParams() = %q, want authSource=admin", params)
	}
}

func TestBuildBaseURI(t *testing.T) {
	uri := buildBaseURI("user", "pass", "localhost:27017", "testdb")
	if uri != "mongodb://user:pass@localhost:27017/testdb" {
		t.Errorf("buildBaseURI() = %q, want mongodb://user:pass@localhost:27017/testdb", uri)
	}
}

func TestBuildBaseURI_NoCredentials(t *testing.T) {
	uri := buildBaseURI("", "", "localhost:27017", "testdb")
	if uri != "mongodb://localhost:27017/testdb" {
		t.Errorf("buildBaseURI() = %q, want mongodb://localhost:27017/testdb", uri)
	}
}
