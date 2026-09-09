package services

import (
	"testing"
	"time"

	"remotehelpdesk/internal/pkg/dto"
)

func TestResolveSupplierAuthorizationEndUsesDatabaseSessionLocation(t *testing.T) {
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	now := time.Date(2026, time.August, 10, 19, 30, 0, 0, location)
	want := now.Add(75 * time.Second)

	got, explicit, err := resolveSupplierAuthorizationEnd(dto.TicketSupplierInviteRequest{
		AuthorizationEndsAt: want.UTC().Format(time.RFC3339Nano),
	}, now)
	if err != nil {
		t.Fatalf("resolveSupplierAuthorizationEnd() error = %v", err)
	}
	if !explicit {
		t.Fatal("resolveSupplierAuthorizationEnd() did not mark the explicit deadline")
	}
	if !got.Equal(want) {
		t.Fatalf("resolved instant = %v, want %v", got, want)
	}
	if got.Location() != location {
		t.Fatalf("resolved location = %v, want %v", got.Location(), location)
	}
	if got.Hour() != want.Hour() || got.Minute() != want.Minute() || got.Second() != want.Second() {
		t.Fatalf("resolved wall clock = %v, want %v", got, want)
	}
}
