package handlers

import "testing"

func TestDefaultIntForBookingKey(t *testing.T) {
	if defaultIntForBookingKey("avg_check_kopecks") != 150000 {
		t.Fatal("avg_check_kopecks")
	}
	if defaultIntForBookingKey("unknown") != 0 {
		t.Fatal("unknown")
	}
	if len(bookingSettingKeysList()) < 6 {
		t.Fatal("keys list")
	}
}
