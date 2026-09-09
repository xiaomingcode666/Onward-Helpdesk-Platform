package capability

import (
	"strings"
	"testing"
)

func TestRuntimeInstructionReflectsUnavailableCapabilities(t *testing.T) {
	instruction := (Set{TicketCreation: true}).RuntimeInstruction()
	for _, expected := range []string{"转人工", "发起视频会议", "创建知识候选"} {
		if !strings.Contains(instruction, expected) {
			t.Fatalf("RuntimeInstruction() = %q, want %q", instruction, expected)
		}
	}
	if strings.Contains(instruction, "不提供创建工单") {
		t.Fatalf("RuntimeInstruction() incorrectly blocks an available capability: %q", instruction)
	}
	if got := (Set{HumanHandoff: true, TicketCreation: true, VideoMeeting: true, KnowledgeCandidate: true}).RuntimeInstruction(); got != "" {
		t.Fatalf("RuntimeInstruction() = %q, want empty for a fully capable workflow", got)
	}
}

func TestSanitizeFallbackMessageRemovesUnsupportedPromises(t *testing.T) {
	capabilities := Set{}
	if got := capabilities.SanitizeFallbackMessage("知识不足时将转接人工工程师"); got != SafeKnowledgeFallbackMessage {
		t.Fatalf("SanitizeFallbackMessage() = %q", got)
	}
	if got := capabilities.SanitizeFallbackMessage("请补充设备故障码"); got != "请补充设备故障码" {
		t.Fatalf("SanitizeFallbackMessage() changed a safe message: %q", got)
	}
}
