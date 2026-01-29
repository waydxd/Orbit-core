package database

import (
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// StringToUUID converts a string to pgtype.UUID
func StringToUUID(s string) pgtype.UUID {
	var uuid pgtype.UUID
	err := uuid.Scan(s)
	if err != nil {
		return pgtype.UUID{Valid: false}
	}
	return uuid
}

// UUIDToString converts pgtype.UUID to string
func UUIDToString(uuid pgtype.UUID) string {
	if !uuid.Valid {
		return ""
	}
	// UUID byte layout: 4-2-2-2-6
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		uuid.Bytes[0:4],
		uuid.Bytes[4:6],
		uuid.Bytes[6:8],
		uuid.Bytes[8:10],
		uuid.Bytes[10:16])
}

// StringToText converts string to pgtype.Text
func StringToText(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}

// TextToString converts pgtype.Text to string
func TextToString(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

// TimeToTimestamptz converts time.Time to pgtype.Timestamptz
func TimeToTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()}
}

// TimestamptzToTime converts pgtype.Timestamptz to time.Time
func TimestamptzToTime(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

// Float64ToNumeric converts float64 to pgtype.Numeric
func Float64ToNumeric(f float64) pgtype.Numeric {
	var num pgtype.Numeric
	// Scan handles float64 conversion
	_ = num.Scan(f)
	return num
}

// NumericToFloat64 converts pgtype.Numeric to float64
func NumericToFloat64(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, _ := n.Float64Value()
	return f.Float64
}

// IntToInt32 attempts to convert an int to int32 safely
func IntToInt32(num int) int32 {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Failed to convert Int32 to Int: %v\n", r)
		}
	}()

	if num > 1<<31-1 || num < -1<<31 {
		panic(fmt.Sprintf("%d int32 out of range [-2147483648, 2147483647]", num))
	}

	return int32(num)
}

// TimeToDate converts time.Time to pgtype.Date
func TimeToDate(t time.Time) pgtype.Date {
	if t.IsZero() {
		return pgtype.Date{Valid: false}
	}
	var date pgtype.Date
	_ = date.Scan(t)
	return date
}

// DateToTime converts pgtype.Date to time.Time
func DateToTime(d pgtype.Date) time.Time {
	if !d.Valid {
		return time.Time{}
	}
	return d.Time
}
