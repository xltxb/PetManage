package timeutil

import (
	"testing"
	"time"
)

func TestStartOfDayUTC(t *testing.T) {
	// 2026-06-02 15:30:00 UTC → Shanghai day starts at 2026-06-01 16:00 UTC
	tm := time.Date(2026, 6, 2, 15, 30, 0, 0, time.UTC)
	start := StartOfDay(tm, "Asia/Shanghai")

	expected := time.Date(2026, 6, 1, 16, 0, 0, 0, time.UTC)
	if !start.Equal(expected) {
		t.Errorf("StartOfDay = %v, want %v", start.Format(time.RFC3339), expected.Format(time.RFC3339))
	}
}

func TestEndOfDayUTC(t *testing.T) {
	tm := time.Date(2026, 6, 2, 15, 30, 0, 0, time.UTC)
	end := EndOfDay(tm, "Asia/Shanghai")

	expected := time.Date(2026, 6, 2, 16, 0, 0, 0, time.UTC)
	if !end.Equal(expected) {
		t.Errorf("EndOfDay = %v, want %v", end.Format(time.RFC3339), expected.Format(time.RFC3339))
	}
}

func TestFormatISO(t *testing.T) {
	tm := time.Date(2026, 6, 2, 15, 30, 0, 0, time.UTC)
	got := FormatISO(tm)
	if got != "2026-06-02T15:30:00Z" {
		t.Errorf("FormatISO = %q, want %q", got, "2026-06-02T15:30:00Z")
	}
}

func TestNowUTC(t *testing.T) {
	now := NowUTC()
	if now.Location() != time.UTC {
		t.Error("NowUTC should return UTC time")
	}
	if time.Since(now) > time.Second {
		t.Error("NowUTC should return recent time")
	}
}
