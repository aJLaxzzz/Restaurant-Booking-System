package handlers

import "net/http"

// handleBookingDefaultsGet — публичные числа для UI брони (таблица settings).
func (a *Handlers) handleBookingDefaultsGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	out := map[string]int{
		"default_slot_duration_hours": a.getSettingInt(ctx, "default_slot_duration_hours", 2),
		"booking_open_hour":           a.getSettingInt(ctx, "booking_open_hour", 10),
		"booking_close_hour":          a.getSettingInt(ctx, "booking_close_hour", 23),
		"slot_minutes":                a.getSettingInt(ctx, "slot_minutes", 30),
		"deposit_percent":             a.getSettingInt(ctx, "deposit_percent", 30),
		"avg_check_kopecks":           a.getSettingInt(ctx, "avg_check_kopecks", 150000),
	}
	a.json(w, http.StatusOK, out)
}
