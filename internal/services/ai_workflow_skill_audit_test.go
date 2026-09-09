package services

import (
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
)

func TestParseWorkflowSkillRuntimeTrace(t *testing.T) {
	trace := parseWorkflowSkillRuntimeTrace(`{
  "agentNodes": {
    "reply_1": {
      "agentNodes": {
        "reply_1": {
          "skill": {
            "id": 3,
            "name": "操作与维修指导",
            "description": "依据知识给出操作步骤",
            "routeReason": "eino_skill_tool",
            "routeTrace": "{\"skillId\":\"3\"}",
            "filteredToolCodes": ["builtin/skill"],
            "middlewareEnabled": true,
            "visibleIds": [2, 3, 1]
          },
          "tools": {
            "items": [
              {"toolCode": "builtin/skill"},
              {"toolCode": "builtin/skill"}
            ]
          }
        }
      }
    }
  }
}`)

	if !trace.MiddlewareEnabled || trace.SelectedID != 3 || trace.SelectedName != "操作与维修指导" {
		t.Fatalf("unexpected selected skill trace: %#v", trace)
	}
	if len(trace.VisibleIDs) != 3 || trace.VisibleIDs[0] != 2 || trace.VisibleIDs[2] != 1 {
		t.Fatalf("unexpected visible skill ids: %#v", trace.VisibleIDs)
	}
	if len(trace.InvokedToolCodes) != 1 || trace.InvokedToolCodes[0] != "builtin/skill" {
		t.Fatalf("unexpected invoked tools: %#v", trace.InvokedToolCodes)
	}
}

func TestBuildWorkflowSkillAuditPrefersPersistedSelection(t *testing.T) {
	createdAt := time.Date(2026, 7, 26, 22, 51, 12, 0, time.UTC)
	audit := buildWorkflowSkillAudit(
		models.AIWorkflowRun{MessageID: 74},
		workflowSkillRuntimeTrace{
			MiddlewareEnabled: true,
			VisibleIDs:        []int64{2, 3, 1},
			SelectedID:        2,
			RouteReason:       "runtime_trace",
		},
		&models.SkillRunLog{
			SkillDefinitionID: 3,
			SourceMessageID:   74,
			MatchReason:       "eino_skill_tool",
			TraceData:         `{"skillId":"3"}`,
			CreatedAt:         createdAt,
		},
		map[int64]models.SkillDefinition{
			1: {ID: 1, Name: "设备故障诊断"},
			2: {ID: 2, Name: "安全停机与人工升级"},
			3: {ID: 3, Name: "操作与维修指导", Description: "依据知识给出操作步骤"},
		},
	)

	if audit.State != dto.AIWorkflowSkillStateSelected || audit.SelectedSkillID != 3 {
		t.Fatalf("unexpected selected audit: %#v", audit)
	}
	if audit.SelectedSkillName != "操作与维修指导" || audit.MatchReason != "eino_skill_tool" {
		t.Fatalf("unexpected selected skill metadata: %#v", audit)
	}
	if len(audit.CandidateSkills) != 3 || audit.CandidateSkills[0].ID != 2 {
		t.Fatalf("unexpected candidates: %#v", audit.CandidateSkills)
	}
	if !audit.CreatedAt.Equal(createdAt) {
		t.Fatalf("unexpected audit time: %v", audit.CreatedAt)
	}
}
