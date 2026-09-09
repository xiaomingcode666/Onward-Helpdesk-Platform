package dbtime

import (
	"testing"
	"time"
)

func TestWallClockUnixMilliInLocation(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	stored := time.Date(2026, time.August, 2, 18, 2, 52, 0, time.UTC)
	want := time.Date(2026, time.August, 2, 18, 2, 52, 0, location).UnixMilli()
	if got := WallClockUnixMilliInLocation(stored, location); got != want {
		t.Fatalf("wall clock epoch = %d, want %d", got, want)
	}
}
