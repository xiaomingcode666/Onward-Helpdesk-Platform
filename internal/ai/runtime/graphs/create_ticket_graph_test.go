package graphs

import (
	"strings"
	"testing"
	"unicode/utf8"

	"remotehelpdesk/internal/models"
)

func TestCreateTicketGraphBuildRequestNormalizesExplicitTitle(t *testing.T) {
	graph := NewCreateTicketGraph(models.Conversation{ID: 17}, models.AIAgent{})

	req, err := graph.buildCreateRequest(`{"title":"## **液压泵压力异常**\n请尽快处理","description":"压力持续波动"}`)
	if err != nil {
		t.Fatalf("build create request: %v", err)
	}
	if req.Title != "液压泵压力异常 请尽快处理" {
		t.Fatalf("title = %q, want normalized plain text", req.Title)
	}
	if req.Description != "压力持续波动" {
		t.Fatalf("description = %q", req.Description)
	}
}

func TestCreateTicketGraphBuildRequestLimitsFallbackTitle(t *testing.T) {
	summary := "根据知识库，故障码 **RHD-FLOW-ALPHA-7742** 复位前需要先断开主电源三十秒，然后长按蓝色复位键八秒；如果状态灯仍然红色，请停止重复上电并联系技术支持。"
	graph := NewCreateTicketGraph(models.Conversation{ID: 18, LastMessageSummary: summary}, models.AIAgent{})

	req, err := graph.buildCreateRequest("")
	if err != nil {
		t.Fatalf("build create request: %v", err)
	}
	if utf8.RuneCountInString(req.Title) > 80 {
		t.Fatalf("title length = %d, want at most 80: %q", utf8.RuneCountInString(req.Title), req.Title)
	}
	if strings.ContainsAny(req.Title, "*\n\r`") {
		t.Fatalf("title still contains markdown or line breaks: %q", req.Title)
	}
	if req.Description != summary {
		t.Fatalf("description should preserve the full fallback summary")
	}
}
