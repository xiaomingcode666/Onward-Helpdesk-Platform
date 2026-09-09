package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"remotehelpdesk/internal/ai/workflow/dsl"
	workflowregistry "remotehelpdesk/internal/ai/workflow/registry"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/dto/response"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
)

type AIWorkflowTestRunInput struct {
	Workflow models.AIWorkflow
	Agent    models.AIAgent
	Request  request.TestAIWorkflowRequest
}

var AIWorkflowTestRunHook func(context.Context, AIWorkflowTestRunInput) (*response.AIWorkflowTestRunResponse, error)

func (s *aiWorkflowService) TestRun(
	ctx context.Context,
	workflowID int64,
	req request.TestAIWorkflowRequest,
	operator *dto.AuthPrincipal,
) (*response.AIWorkflowTestRunResponse, error) {
	workflow := s.GetForOperator(workflowID, operator)
	if !s.IsReusableTemplateForOperator(workflow, operator) {
		return nil, errorsx.InvalidParam("工作流不存在或无权测试")
	}
	if strings.TrimSpace(req.UserMessage) == "" {
		return nil, errorsx.InvalidParam("请输入测试问题")
	}
	if validation := s.ValidateDefinition(req.Definition); !validation.Valid {
		message := "工作流定义校验失败"
		if len(validation.Errors) > 0 && strings.TrimSpace(validation.Errors[0].Message) != "" {
			message = validation.Errors[0].Message
		}
		return nil, errorsx.InvalidParam(message)
	}
	if err := validateAIWorkflowTestBranchOverrides(req.Definition, req.BranchOverrides); err != nil {
		return nil, err
	}
	agent := AIAgentService.Get(req.AIAgentID)
	if agent == nil || agent.Status != enums.StatusOk {
		return nil, errorsx.InvalidParam("测试产品机器人不存在或未启用")
	}
	if operator == nil || operator.TenantID <= 0 || agent.TenantID != operator.TenantID {
		return nil, errorsx.Forbidden("测试产品机器人不在当前企业范围内")
	}
	if AIWorkflowTestRunHook == nil {
		return nil, fmt.Errorf("workflow test runner is not initialized")
	}
	return AIWorkflowTestRunHook(ctx, AIWorkflowTestRunInput{
		Workflow: *workflow,
		Agent:    *agent,
		Request:  req,
	})
}

func validateAIWorkflowTestBranchOverrides(definition dsl.Definition, overrides map[string]string) error {
	if len(overrides) == 0 {
		return nil
	}
	nodeByID := make(map[string]dsl.Node, len(definition.Nodes))
	for _, node := range definition.Nodes {
		nodeByID[strings.TrimSpace(node.ID)] = node
	}
	for rawNodeID, rawBranchID := range overrides {
		nodeID := strings.TrimSpace(rawNodeID)
		branchID := strings.TrimSpace(rawBranchID)
		node, exists := nodeByID[nodeID]
		if !exists || node.Type != workflowregistry.NodeTypeCondition {
			return errorsx.InvalidParam(fmt.Sprintf("试运行分支覆盖节点不存在或不是条件节点：%s", nodeID))
		}
		config := dsl.ConditionConfig{}
		if err := json.Unmarshal(node.Config, &config); err != nil {
			return errorsx.InvalidParam(fmt.Sprintf("条件节点配置无效：%s", nodeID))
		}
		matched := false
		for _, branch := range config.Branches {
			if strings.TrimSpace(branch.ID) == branchID {
				matched = true
				break
			}
		}
		if !matched {
			return errorsx.InvalidParam(fmt.Sprintf("条件节点 %s 不存在分支 %s", nodeID, branchID))
		}
	}
	return nil
}
