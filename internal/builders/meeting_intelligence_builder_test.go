package builders

import (
	"testing"

	"remotehelpdesk/internal/models"
)

func TestBuildMeetingARAnnotationIncludesAuthorizedFrameURL(t *testing.T) {
	item := &models.MeetingARAnnotation{
		ID: "annotation-1", MeetingID: "meeting-1", TicketID: "42", FrameAssetID: 7,
		Label: "控制面板", X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4,
	}
	result := BuildMeetingARAnnotation(item, "/storage/signed-frame.png")
	if result == nil || result.FrameURL != "/storage/signed-frame.png" {
		t.Fatalf("frame URL missing from response: %+v", result)
	}
	list := BuildMeetingARAnnotationList([]models.MeetingARAnnotation{*item}, map[int64]string{
		7: "/storage/signed-frame.png",
	})
	if len(list) != 1 || list[0].FrameURL != "/storage/signed-frame.png" {
		t.Fatalf("frame URL missing from list response: %+v", list)
	}
}
