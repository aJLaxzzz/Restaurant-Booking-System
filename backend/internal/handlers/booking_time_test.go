package handlers

import (
	"testing"
	"time"
)

func TestBookingStartNotAllowedAt_futureOK(t *testing.T) {
	loc := bookingLocationMoscow()
	now := time.Date(2026, 4, 11, 12, 0, 0, 0, loc)
	start := time.Date(2026, 4, 15, 19, 0, 0, 0, loc)
	if msg := bookingStartNotAllowedAt(start, now); msg != "" {
		t.Fatalf("expected ok, got %q", msg)
	}
}

func TestBookingStartNotAllowedAt_pastInstant(t *testing.T) {
	loc := bookingLocationMoscow()
	now := time.Date(2026, 4, 11, 12, 0, 0, 0, loc)
	start := now.Add(-2 * time.Hour)
	if msg := bookingStartNotAllowedAt(start, now); msg == "" {
		t.Fatal("expected error for past instant")
	}
}

func TestBookingStartNotAllowedAt_yesterdayMoscow(t *testing.T) {
	loc := bookingLocationMoscow()
	now := time.Date(2026, 4, 11, 8, 0, 0, 0, loc)
	start := time.Date(2026, 4, 10, 23, 0, 0, 0, loc)
	if msg := bookingStartNotAllowedAt(start, now); msg == "" {
		t.Fatal("expected error for previous calendar day in Moscow")
	}
}

func TestBookingStartNotAllowedAt_todayLaterOK(t *testing.T) {
	loc := bookingLocationMoscow()
	now := time.Date(2026, 4, 11, 10, 0, 0, 0, loc)
	start := time.Date(2026, 4, 11, 20, 0, 0, 0, loc)
	if msg := bookingStartNotAllowedAt(start, now); msg != "" {
		t.Fatalf("expected ok for later today, got %q", msg)
	}
}
