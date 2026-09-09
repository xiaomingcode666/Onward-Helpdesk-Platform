package routes

import "testing"

func TestNormalizeEnterpriseActionURL(t *testing.T) {
	tests := []struct {
		name      string
		actionURL string
		bizType   string
		bizID     int64
		want      string
	}{
		{
			name:      "legacy numeric ticket detail",
			actionURL: "/enterprise/tickets/345",
			want:      "/enterprise/ticket-workbench?ticket_id=345",
		},
		{
			name:      "legacy non numeric ticket detail",
			actionURL: "https://remotehelpdesk.digintelspace.com:8443/enterprise/tickets/enterprise",
			want:      "/enterprise/ticket-workbench",
		},
		{
			name:      "legacy non numeric ticket detail falls back to biz id",
			actionURL: "/enterprise/tickets/enterprise",
			bizType:   "ticket",
			bizID:     345,
			want:      "/enterprise/ticket-workbench?ticket_id=345",
		},
		{
			name:    "empty ticket action",
			bizType: "ticket",
			bizID:   345,
			want:    "/enterprise/ticket-workbench?ticket_id=345",
		},
		{
			name:    "empty conversation action",
			bizType: "conversation",
			bizID:   266,
			want:    "/enterprise/ticket-workbench?conversationId=266",
		},
		{
			name:      "ticket list filter is preserved",
			actionURL: "/enterprise/tickets?status=processing",
			want:      "/enterprise/tickets?status=processing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeEnterpriseActionURL(tt.actionURL, tt.bizType, tt.bizID); got != tt.want {
				t.Fatalf("NormalizeEnterpriseActionURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
