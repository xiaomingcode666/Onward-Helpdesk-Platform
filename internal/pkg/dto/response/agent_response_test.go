package response

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAgentProfileResponseIncludesDisabledTeamDispatch(t *testing.T) {
	raw, err := json.Marshal(AgentProfileResponse{
		ID:                  1,
		AutoAssignEnabled:   true,
		TeamDispatchEnabled: false,
	})
	if err != nil {
		t.Fatalf("marshal agent profile response: %v", err)
	}
	if !strings.Contains(string(raw), `"teamDispatchEnabled":false`) {
		t.Fatalf("team dispatch disabled flag was omitted: %s", raw)
	}
}

func TestAgentTeamScheduleResponseOmitsSourceType(t *testing.T) {
	payload, err := json.Marshal(AgentTeamScheduleResponse{
		ID:      1,
		TeamID:  2,
		StartAt: "2026-04-29 09:00:00",
		EndAt:   "2026-04-29 18:00:00",
		Remark:  "test",
	})
	if err != nil {
		t.Fatalf("marshal response error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal response error = %v", err)
	}
	if _, ok := decoded["sourceType"]; ok {
		t.Fatalf("sourceType should not be exposed: %s", payload)
	}
}
