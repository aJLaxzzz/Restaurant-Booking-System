package handlers

import "time"

const bookingPastGrace = 90 * time.Second

func bookingLocationMoscow() *time.Location {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		return time.UTC
	}
	return loc
}

// bookingStartNotAllowedAt — то же, что bookingStartNotAllowed, но с фиксированным «сейчас» (для тестов).
func bookingStartNotAllowedAt(start, now time.Time) string {
	if start.Before(now.Add(-bookingPastGrace)) {
		return "нельзя бронировать на прошедшую дату или время"
	}
	loc := bookingLocationMoscow()
	today := now.In(loc)
	dayStart := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc)
	startInLoc := start.In(loc)
	if startInLoc.Before(dayStart) {
		return "нельзя бронировать на прошедшую дату или время"
	}
	return ""
}

// bookingStartNotAllowed returns a Russian error message if the slot must be rejected, else "".
// Rejects: (1) start before now minus grace; (2) calendar date in Europe/Moscow before today there.
func bookingStartNotAllowed(start time.Time) string {
	return bookingStartNotAllowedAt(start, time.Now())
}
