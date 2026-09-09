package services

import "testing"

func TestPartnerPortalMeetingStatusFilterTreatsEndedAsFinished(t *testing.T) {
	for _, meetingStatus := range []string{"ended", "finished"} {
		if !partnerPortalMeetingStatusMatchesFilter(meetingStatus, "ended") {
			t.Fatalf("meeting status %q should match ended filter", meetingStatus)
		}
		if !partnerPortalMeetingStatusMatchesFilter(meetingStatus, "finished") {
			t.Fatalf("meeting status %q should match legacy finished filter", meetingStatus)
		}
	}
	if partnerPortalMeetingStatusMatchesFilter("active", "ended") {
		t.Fatal("active meeting must not match ended filter")
	}
}
