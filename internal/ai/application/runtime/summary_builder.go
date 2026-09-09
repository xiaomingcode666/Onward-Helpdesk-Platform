package runtime

import (
	"remotehelpdesk/internal/ai/runtime/executor"
	"strings"
)

func toSummary(summary *executor.RunResult) *Summary {
	if summary == nil {
		return nil
	}
	ret := &Summary{
		RunID:                 summary.RunID,
		Status:                summary.Status,
		ReplyText:             summary.ReplyText,
		PlannedSkillID:        summary.SelectedSkillID,
		PlannedSkillName:      strings.TrimSpace(summary.SelectedSkillName),
		PlanReason:            strings.TrimSpace(summary.SkillRouteReason),
		SkillRouteTrace:       strings.TrimSpace(summary.SkillRouteTrace),
		SkillAllowedToolCodes: append([]string(nil), summary.SkillAllowedToolCodes...),
		ModelProvider:         summary.ModelProvider,
		ModelName:             summary.ModelName,
		PromptTokens:          summary.PromptTokens,
		CompletionTokens:      summary.CompletionTokens,
		HistoryMessageCount:   summary.HistoryMessageCount,
		RetrieverCount:        summary.RetrieverCount,
		ToolCallCount:         summary.ToolCallCount,
		ToolCodes:             append([]string(nil), summary.ToolCodes...),
		InvokedToolCodes:      append([]string(nil), summary.InvokedToolCodes...),
		CheckPointID:          summary.CheckPointID,
		Interrupted:           summary.Interrupted,
		TraceData:             summary.TraceData,
		ErrorMessage:          summary.ErrorMessage,
	}
	if len(summary.Interrupts) > 0 {
		ret.Interrupts = make([]InterruptContextSummary, 0, len(summary.Interrupts))
		for _, item := range summary.Interrupts {
			ret.Interrupts = append(ret.Interrupts, InterruptContextSummary{
				Type:        item.Type,
				ID:          item.ID,
				InfoPreview: item.InfoPreview,
			})
		}
	}
	return ret
}
