package handlers

import (
	"net/http"

	"github.com/google/uuid"
)

// handleBookingDefaultsGet — публичные числа для UI брони (пер-ресторан + fallback на settings).
func (a *Handlers) handleBookingDefaultsGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var rid uuid.UUID
	if s := r.URL.Query().Get("restaurant_id"); s != "" {
		if parsed, err := uuid.Parse(s); err == nil {
			rid = parsed
		}
	}
	out := map[string]int{
		"default_slot_duration_hours": a.getSettingIntForRestaurant(ctx, rid, "default_slot_duration_hours", defaultIntForOwnerSettingKey("default_slot_duration_hours")),
		"booking_open_hour":           a.getSettingIntForRestaurant(ctx, rid, "booking_open_hour", defaultIntForOwnerSettingKey("booking_open_hour")),
		"booking_close_hour":          a.getSettingIntForRestaurant(ctx, rid, "booking_close_hour", defaultIntForOwnerSettingKey("booking_close_hour")),
		"slot_minutes":                a.getSettingIntForRestaurant(ctx, rid, "slot_minutes", defaultIntForOwnerSettingKey("slot_minutes")),
		"deposit_percent":             a.getSettingIntForRestaurant(ctx, rid, "deposit_percent", defaultIntForOwnerSettingKey("deposit_percent")),
		"avg_check_kopecks":           a.getSettingIntForRestaurant(ctx, rid, "avg_check_kopecks", defaultIntForOwnerSettingKey("avg_check_kopecks")),
	}
	a.json(w, http.StatusOK, out)
}
