package runtime

import (
	"encoding/json"
	"strings"

	"remotehelpdesk/internal/ai/workflow/compiler"
	"remotehelpdesk/internal/ai/workflow/dsl"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
)

type resolvedWorkflow struct {
	Definition         dsl.Definition
	Compiled           compiler.Result
	WorkflowID         int64
	VersionID          int64
	DefinitionHash     string
	AgentReleaseID     int64
	AgentConfigHash    string
	KnowledgeScopeHash string
}

const workflowRuntimeActionGuardrails = `运行时动作约束：
- 只能建议客户转人工、建单、邀请供应商、发起视频或生成知识候选；除非当前工作流节点已经返回成功结果，否则不得声称动作已完成。
- “已转人工”“工单已创建”“供应商已邀请”“视频已发起”等完成态事实只能来自系统动作结果，不得根据对话内容自行推断。
- 对短接、旁路、带电操作等高风险请求必须明确拒绝，并要求保持停机和隔离；知识依据没有给出规格参数时，必须说明未知，不得猜测。`

const workflowRuntimeResponseStyle = `回复表达规范：
- 回复语言必须跟随客户原始最新消息的主要语言，而不是系统提示、历史消息、工作流提示或知识片段的语言。客户使用英文时，整条回复必须使用英文，不得因中文知识片段切回中文。
- 直接回答当前问题。除非是首次欢迎或用户主动问候，不要重复“您好”、自我介绍、设备型号、故障码或用户已经说过的现象。
- 不使用 Emoji、装饰符号、口号或模板化小标题。必要时使用普通小标题和短列表，小标题不超过三个。
- 先给结论或下一步动作，再补充原因；使用短句、明确的操作对象和可核对的结果。
- 安全提示只保留与当前风险直接相关的停机、隔离和禁止操作，不堆砌泛泛免责声明。
- 不编造扭矩、电压、时间、阈值等参数。引用检索结果时直接陈述事实，不要说“根据知识库”。`

func resolveAgentWorkflow(aiAgent models.AIAgent) (resolvedWorkflow, error) {
	if aiAgent.WorkflowVersionID <= 0 {
		return resolvedWorkflow{}, errorsx.InvalidParam("AI Agent workflow is not published; publish a workflow version before enabling automatic replies")
	}
	version := repositories.AIWorkflowVersionRepository.Get(sqls.DB(), aiAgent.WorkflowVersionID)
	if version == nil || version.Status != enums.StatusOk {
		return resolvedWorkflow{}, errorsx.InvalidParam("workflow version does not exist")
	}
	if version.ReleaseChannel == models.AIWorkflowReleaseChannelRevoked {
		return resolvedWorkflow{}, errorsx.InvalidParam("workflow version has been revoked")
	}
	if aiAgent.ID > 0 {
		workflow := repositories.AIWorkflowRepository.Get(sqls.DB(), version.WorkflowID)
		if workflow == nil || workflow.Status == enums.StatusDeleted {
			return resolvedWorkflow{}, errorsx.Forbidden("workflow version does not belong to the AI Agent")
		}
		if aiAgent.WorkflowID > 0 && aiAgent.WorkflowID != workflow.ID {
			return resolvedWorkflow{}, errorsx.Forbidden("workflow version does not match the AI Agent binding")
		}
		if workflow.Scope == models.AIWorkflowScopeTenant && workflow.TenantID != aiAgent.TenantID {
			return resolvedWorkflow{}, errorsx.Forbidden("workflow version belongs to another tenant")
		}
		if workflow.AgentID > 0 && workflow.AgentID != aiAgent.ID {
			return resolvedWorkflow{}, errorsx.Forbidden("legacy workflow version does not belong to the AI Agent")
		}
	}
	var def dsl.Definition
	if err := json.Unmarshal([]byte(version.Definition), &def); err != nil {
		return resolvedWorkflow{}, errorsx.InvalidParam("workflow definition is invalid")
	}
	return resolvedWorkflow{
		Definition:     def,
		Compiled:       compiler.Compile(def),
		WorkflowID:     version.WorkflowID,
		VersionID:      version.ID,
		DefinitionHash: strings.TrimSpace(version.DefinitionHash),
	}, nil
}

func prepareWorkflowAgent(aiAgent models.AIAgent) (models.AIAgent, resolvedWorkflow, error) {
	runtimeAgent, release, err := services.AIAgentReleaseService.MaterializeRuntimeAgent(sqls.DB(), aiAgent)
	if err != nil {
		return aiAgent, resolvedWorkflow{}, err
	}
	aiAgent = runtimeAgent
	workflow, err := resolveAgentWorkflow(aiAgent)
	if err != nil {
		return aiAgent, resolvedWorkflow{}, err
	}
	if release != nil {
		if workflow.WorkflowID != release.WorkflowID || workflow.VersionID != release.WorkflowVersionID || workflow.DefinitionHash != release.WorkflowDefinitionHash {
			return aiAgent, resolvedWorkflow{}, errorsx.InvalidParam("active release workflow snapshot does not match runtime workflow")
		}
		workflow.AgentReleaseID = release.ID
		workflow.AgentConfigHash = release.AgentConfigHash
		workflow.KnowledgeScopeHash = release.KnowledgeScopeHash
	}
	parts := []string{
		strings.TrimSpace(aiAgent.SystemPrompt),
		strings.TrimSpace(workflow.Compiled.Appendix),
		workflowRuntimeActionGuardrails,
		workflowRuntimeResponseStyle,
	}
	sections := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			sections = append(sections, part)
		}
	}
	aiAgent.SystemPrompt = strings.Join(sections, "\n\n")
	return aiAgent, workflow, nil
}
