package graphs

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"remotehelpdesk/internal/ai/runtime/tooling"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/i18nx"
	"remotehelpdesk/internal/pkg/tracex"
	"remotehelpdesk/internal/services"

	componenttool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type HandoffGraphState struct {
	Reason string `json:"reason"`
}

type HandoffGraphInterruptInfo struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type handoffGraphArgs struct {
	Reason string `json:"reason"`
}

func init() {
	schema.RegisterName[HandoffGraphState]("cs_ai_agent_handoff_graph_state")
	schema.RegisterName[HandoffGraphInterruptInfo]("cs_ai_agent_handoff_graph_interrupt_info")
}

type HandoffGraph struct {
	conversation models.Conversation
	aiAgent      models.AIAgent
}

func NewHandoffGraph(conversation models.Conversation, aiAgent models.AIAgent) *HandoffGraph {
	return &HandoffGraph{
		conversation: conversation,
		aiAgent:      aiAgent,
	}
}

func (g *HandoffGraph) Run(ctx context.Context, argumentsInJSON string) (string, error) {
	wasInterrupted, hasState, state := componenttool.GetInterruptState[HandoffGraphState](ctx)
	if !wasInterrupted {
		reason, err := g.buildReason(argumentsInJSON)
		if err != nil {
			return "", err
		}
		info := HandoffGraphInterruptInfo{
			Type:    InterruptTypeHandoffConfirmation,
			Message: g.buildConfirmationPrompt(reason),
		}
		return "", componenttool.StatefulInterrupt(ctx, info, HandoffGraphState{Reason: reason})
	}
	if !hasState {
		return "", fmt.Errorf("handoff graph state missing")
	}
	isResumeTarget, hasData, resumeText := componenttool.GetResumeContext[string](ctx)
	if !isResumeTarget {
		info := HandoffGraphInterruptInfo{
			Type:    InterruptTypeHandoffConfirmation,
			Message: g.buildConfirmationPrompt(state.Reason),
		}
		return "", componenttool.StatefulInterrupt(ctx, info, state)
	}
	if !hasData {
		info := HandoffGraphInterruptInfo{
			Type:    InterruptTypeHandoffConfirmation,
			Message: i18nx.Getf(handoffLocaleForMessage(state.Reason), "graph.confirmOrCancel"),
		}
		return "", componenttool.StatefulInterrupt(ctx, info, state)
	}
	switch parseHandoffDecision(resumeText) {
	case ConfirmationDecisionConfirm:
		locale := handoffLocaleForMessage(state.Reason)
		if _, err := services.ConversationHumanDispatchService.HandoffByAIWithRequestIDForLocale(g.conversation.ID, g.aiAgent, state.Reason, tracex.RequestIDFromContext(ctx), locale); err != nil {
			return "", err
		}
		// The dispatch service sends the customer-visible handoff notice in the request language.
		return tooling.MarshalToolResult(tooling.ToolResult{
			Handled:     true,
			Terminal:    true,
			Action:      "handoff_confirmed",
			ReplySent:   true,
			ShouldRetry: false,
		}), nil
	case ConfirmationDecisionCancel:
		return tooling.MarshalToolResult(tooling.ToolResult{
			Handled:     true,
			Terminal:    true,
			Action:      "handoff_cancelled",
			ReplyText:   i18nx.Getf(handoffLocaleForMessage(state.Reason), "graph.cancelHandoff"),
			ShouldRetry: false,
		}), nil
	default:
		info := HandoffGraphInterruptInfo{
			Type:    InterruptTypeHandoffConfirmation,
			Message: i18nx.Getf(handoffLocaleForMessage(state.Reason), "graph.needExplicitConfirmation"),
		}
		return "", componenttool.StatefulInterrupt(ctx, info, state)
	}
}

func (g *HandoffGraph) buildReason(argumentsInJSON string) (string, error) {
	reason := i18nx.Get("graph.defaultHandoffReason")
	var args handoffGraphArgs
	if strings.TrimSpace(argumentsInJSON) != "" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
			return "", fmt.Errorf("invalid handoff arguments: %w", err)
		}
	}
	if parsed := strings.TrimSpace(args.Reason); parsed != "" {
		reason = parsed
	}
	return reason, nil
}

func (g *HandoffGraph) buildConfirmationPrompt(reason string) string {
	return i18nx.Getf(handoffLocaleForMessage(reason), "graph.handoffConfirmPrompt", strings.TrimSpace(reason))
}

func handoffLocaleForMessage(value string) string {
	hanCount := 0
	latinCount := 0
	for _, r := range strings.TrimSpace(value) {
		switch {
		case unicode.Is(unicode.Han, r):
			hanCount++
		case unicode.IsLetter(r) && unicode.Is(unicode.Latin, r):
			latinCount++
		}
	}
	if hanCount == 0 && latinCount >= 2 {
		return i18nx.LocaleEnUS
	}
	return i18nx.LocaleZhCN
}

func parseHandoffDecision(value string) ConfirmationDecision {
	return ParseConfirmationDecision(value)
}
