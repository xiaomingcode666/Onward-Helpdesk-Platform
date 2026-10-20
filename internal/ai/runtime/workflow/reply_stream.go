package workflow

import (
	"context"

	workflowcapability "remotehelpdesk/internal/ai/workflow/capability"
	workflowregistry "remotehelpdesk/internal/ai/workflow/registry"
	"remotehelpdesk/internal/pkg/aistream"
)

// Only an LLM node wired directly into a public reply can emit provisional
// text. Intermediate analysis and control-flow node output stays private.
func workflowReplyStreamContext(ctx context.Context, state *runState, nodeID string) context.Context {
	if !aistream.Enabled(ctx) || state.input.DryRun {
		return aistream.WithHandler(ctx, nil)
	}
	for _, edge := range state.outgoing[nodeID] {
		target := state.nodesByID[edge.Target]
		binding := target.Inputs["replyText"]
		if target.Type == workflowregistry.NodeTypeSendReply && binding.NodeID == nodeID && binding.Field == "replyText" {
			capabilities := workflowcapability.FromDefinition(state.input.Definition)
			return aistream.WithHandler(ctx, func(text string) {
				aistream.Emit(ctx, sanitizeUnverifiedActionClaims(text, capabilities))
			})
		}
	}
	return aistream.WithHandler(ctx, nil)
}
