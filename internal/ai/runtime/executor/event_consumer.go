package executor

import (
	"io"
	"strings"

	"remotehelpdesk/internal/ai/runtime/internal/impl/callbacks"
	"remotehelpdesk/internal/ai/runtime/internal/impl/factory"
	"remotehelpdesk/internal/ai/runtime/tooling"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/toolx"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

func consumeAgentEvents(events *adk.AsyncIterator[*adk.AgentEvent], summary *RunResult, collector *callbacks.RuntimeTraceCollector, toolDefsByModelName map[string]string, observers ...func(string)) {
	if summary == nil {
		return
	}
	if collector == nil {
		collector = callbacks.NewRuntimeTraceCollector()
	}
	suppressAssistantReply := false
	for {
		event, ok := events.Next()
		if !ok {
			break
		}
		if event == nil {
			continue
		}
		if event.Action != nil && event.Action.Interrupted != nil {
			summary.Status = "interrupted"
			summary.Interrupted = true
			summary.Interrupts = buildInterruptSummaries(event)
		}
		if event.Err != nil {
			errMsg := strings.TrimSpace(event.Err.Error())
			if errMsg != "" {
				summary.Status = "error"
				summary.ErrorMessage = errMsg
			}
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		messageOutput := event.Output.MessageOutput
		if messageOutput.IsStreaming && messageOutput.MessageStream != nil {
			var observer func(string)
			if messageOutput.Role == schema.Assistant && !suppressAssistantReply && len(observers) > 0 {
				observer = observers[0]
			}
			message, err := consumeMessageStream(messageOutput.MessageStream, observer)
			if err != nil {
				summary.Status = "error"
				summary.ErrorMessage = err.Error()
				continue
			}
			messageOutput.Message = message
		}
		if messageOutput.Message == nil {
			continue
		}
		if messageOutput.Message != nil {
			actualProvider, _ := messageOutput.Message.Extra[factory.ActualProviderExtraKey].(string)
			actualModel, _ := messageOutput.Message.Extra[factory.ActualModelExtraKey].(string)
			if strings.TrimSpace(actualProvider) != "" {
				summary.ModelProvider = strings.TrimSpace(actualProvider)
			}
			if strings.TrimSpace(actualModel) != "" {
				summary.ModelName = strings.TrimSpace(actualModel)
			}
			if strings.TrimSpace(actualProvider) != "" || strings.TrimSpace(actualModel) != "" {
				collector.SetModel(summary.ModelProvider, summary.ModelName)
			}
		}
		if messageOutput.Message != nil && messageOutput.Message.ResponseMeta != nil && messageOutput.Message.ResponseMeta.Usage != nil {
			summary.PromptTokens += messageOutput.Message.ResponseMeta.Usage.PromptTokens
			summary.CompletionTokens += messageOutput.Message.ResponseMeta.Usage.CompletionTokens
		}
		switch messageOutput.Role {
		case schema.Assistant:
			if suppressAssistantReply {
				continue
			}
			replyText := strings.TrimSpace(messageOutput.Message.Content)
			if replyText != "" {
				summary.ReplyText = replyText
			}
		case schema.Tool:
			toolName := strings.TrimSpace(messageOutput.ToolName)
			if toolName == "" {
				continue
			}
			toolCode := toolName
			if mappedCode, ok := toolDefsByModelName[toolName]; ok && strings.TrimSpace(mappedCode) != "" {
				toolCode = strings.TrimSpace(mappedCode)
			}
			summary.InvokedToolCodes = appendIfMissing(summary.InvokedToolCodes, toolCode)
			if strings.TrimSpace(summary.ReplyText) == "" && toolx.ResolveToolSourceType(toolCode) == enums.ToolSourceTypeGraph {
				toolReplyText := strings.TrimSpace(messageOutput.Message.Content)
				if result, ok := tooling.ParseToolResult(toolReplyText); ok {
					if result.ReplyText != "" && !result.ReplySent {
						summary.ReplyText = result.ReplyText
					}
					if result.Terminal && !result.ShouldRetry {
						suppressAssistantReply = true
					}
				}
			}
		}
	}
	if summary.Status == "started" {
		switch {
		case strings.TrimSpace(summary.ErrorMessage) != "":
			summary.Status = "error"
		case summary.Interrupted:
			summary.Status = "interrupted"
		case strings.TrimSpace(summary.ReplyText) != "":
			summary.Status = "completed"
		case hasInvokedGraphTool(summary.InvokedToolCodes):
			summary.Status = "completed"
		default:
			summary.Status = "fallback"
		}
	}
	summary.ToolCallCount = len(summary.InvokedToolCodes)
}

func consumeMessageStream(stream *schema.StreamReader[*schema.Message], observer func(string)) (*schema.Message, error) {
	defer stream.Close()
	var chunks []*schema.Message
	var content strings.Builder
	toolCall := false
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if chunk == nil {
			continue
		}
		chunks = append(chunks, chunk)
		if len(chunk.ToolCalls) > 0 && !toolCall {
			toolCall = true
			if observer != nil {
				observer("")
			}
		}
		if chunk.Content != "" {
			content.WriteString(chunk.Content)
			if observer != nil && !toolCall {
				observer(content.String())
			}
		}
	}
	if len(chunks) == 0 {
		return nil, nil
	}
	return schema.ConcatMessages(chunks)
}

func hasInvokedGraphTool(toolCodes []string) bool {
	for _, toolCode := range toolCodes {
		if toolx.ResolveToolSourceType(toolCode) == enums.ToolSourceTypeGraph {
			return true
		}
	}
	return false
}

func buildInterruptSummaries(event *adk.AgentEvent) []InterruptContextSummary {
	if event == nil || event.Action == nil || event.Action.Interrupted == nil {
		return nil
	}
	interrupts := event.Action.Interrupted.InterruptContexts
	result := make([]InterruptContextSummary, 0, len(interrupts))
	for _, item := range interrupts {
		if item == nil {
			continue
		}
		result = append(result, InterruptContextSummary{
			ID:          strings.TrimSpace(item.ID),
			InfoPreview: previewInterruptInfo(item.Info),
		})
	}
	return result
}
