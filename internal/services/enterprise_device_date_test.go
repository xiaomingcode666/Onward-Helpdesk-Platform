package services

import (
	"testing"
	"time"
)

func TestEnterpriseDeviceDatesKeepSubmittedCalendarDay(t *testing.T) {
	for _, input := range []string{
		"2031-04-05",
		"2031/04/05",
		"2031-04-05T00:00:00+08:00",
		"2031-04-05T00:00:00+14:00",
		"2031-04-05T23:59:59-12:00",
		"2031-04-05T23:59:59Z",
	} {
		t.Run(input, func(t *testing.T) {
			date, err := parseOptionalEnterpriseDeviceDate(input, "warranty_end")
			if err != nil || date == nil {
				t.Fatalf("parse date: %v, %v", date, err)
			}
			if got := date.Format(time.DateOnly); got != "2031-04-05" {
				t.Fatalf("server timezone shifted submitted date: %s -> %s", input, got)
			}
			if !enterpriseDeviceDateOnly(*date).Equal(*date) {
				t.Fatal("normalizing a stored date changed it")
			}
		})
	}
}
