package services

import (
	"testing"
	"time"
)

func TestConfigureEngineerScheduleTimezone(t *testing.T) {
	original := EngineerScheduleTimezone
	t.Cleanup(func() {
		if err := ConfigureEngineerScheduleTimezone(original); err != nil {
			t.Fatalf("restore timezone: %v", err)
		}
	})

	if err := ConfigureEngineerScheduleTimezone("America/New_York"); err != nil {
		t.Fatalf("configure timezone: %v", err)
	}
	if EngineerScheduleTimezone != "America/New_York" {
		t.Fatalf("timezone = %q, want America/New_York", EngineerScheduleTimezone)
	}
	location := engineerScheduleLocation()
	if location.String() != "America/New_York" {
		t.Fatalf("location = %q, want America/New_York", location.String())
	}
	// 排班锚点必须按配置时区解释墙上时间。
	anchor := weeklyScheduleAnchorInLocation(3, 9*60, location)
	if anchor.Location().String() != "America/New_York" || anchor.Hour() != 9 || weekdayForBatchRequest(anchor) != 3 {
		t.Fatalf("anchor = %s in %s", anchor.Format(time.RFC3339), anchor.Location())
	}

	if err := ConfigureEngineerScheduleTimezone("   "); err != nil {
		t.Fatalf("blank timezone should fall back: %v", err)
	}
	if EngineerScheduleTimezone != DefaultEngineerScheduleTimezone {
		t.Fatalf("blank timezone = %q, want %q", EngineerScheduleTimezone, DefaultEngineerScheduleTimezone)
	}

	if err := ConfigureEngineerScheduleTimezone("Not/AZone"); err == nil {
		t.Fatal("invalid timezone was accepted")
	}
	if EngineerScheduleTimezone != DefaultEngineerScheduleTimezone {
		t.Fatalf("invalid timezone changed value to %q", EngineerScheduleTimezone)
	}
}
