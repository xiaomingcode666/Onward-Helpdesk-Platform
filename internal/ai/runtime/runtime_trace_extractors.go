package runtime

import (
	"encoding/json"
	"strings"

	applicationruntime "remotehelpdesk/internal/ai/application/runtime"
)

func extractToolSearchTrace(summary *applicationruntime.Summary) string {
	if summary == nil {
		return ""
	}
	trace := parseRuntimeTraceData(summary.TraceData)
	if len(trace.ToolSearch.Items) == 0 {
		return ""
	}
	buf, err := json.Marshal(trace.ToolSearch)
	if err != nil {
		return ""
	}
	return string(buf)
}

func extractGraphToolTrace(summary *applicationruntime.Summary) string {
	if summary == nil {
		return ""
	}
	trace := parseRuntimeTraceData(summary.TraceData)
	if len(trace.GraphTools.Items) == 0 {
		return ""
	}
	buf, err := json.Marshal(trace.GraphTools)
	if err != nil {
		return ""
	}
	return string(buf)
}

func firstGraphToolCode(summary *applicationruntime.Summary) string {
	if summary == nil {
		return ""
	}
	trace := parseRuntimeTraceData(summary.TraceData)
	for _, item := range trace.GraphTools.Items {
		toolCode := strings.TrimSpace(item.ToolCode)
		if toolCode != "" {
			return toolCode
		}
	}
	return ""
}

type runtimeTraceProjection struct {
	ToolSearch struct {
		Items []struct {
			TargetToolCode     string   `json:"targetToolCode"`
			CandidateToolCodes []string `json:"candidateToolCodes"`
		} `json:"items"`
	} `json:"toolSearch"`
	GraphTools struct {
		Items []struct {
			ToolCode          string          `json:"toolCode"`
			Arguments         json.RawMessage `json:"arguments"`
			RecommendedAction string          `json:"recommendedAction"`
			RiskLevel         string          `json:"riskLevel"`
			TicketDraftReady  bool            `json:"ticketDraftReady"`
		} `json:"items"`
	} `json:"graphTools"`
}

func parseRuntimeTraceData(raw string) runtimeTraceProjection {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return runtimeTraceProjection{}
	}
	var trace runtimeTraceProjection
	if err := json.Unmarshal([]byte(raw), &trace); err != nil {
		return runtimeTraceProjection{}
	}
	return trace
}
