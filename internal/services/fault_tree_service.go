package services

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"
	"unicode"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"

	"github.com/google/uuid"
	"github.com/mlogclub/simple/sqls"
)

var FaultTreeService = newFaultTreeService()

func newFaultTreeService() *faultTreeService {
	return &faultTreeService{}
}

type faultTreeService struct{}

var faultTreeNodeTypes = map[string]bool{
	"symptom":   true,
	"check":     true,
	"action":    true,
	"diagnosis": true,
}

var faultTreeRiskLevels = map[string]bool{
	"low":      true,
	"medium":   true,
	"high":     true,
	"critical": true,
}

var faultTreePatterns = map[string]bool{
	"persistent":   true,
	"intermittent": true,
	"conditional":  true,
}

var faultTreeStatuses = map[string]bool{
	"draft":     true,
	"published": true,
	"archived":  true,
}

func (s *faultTreeService) ListManagedNodes(tenantID, productID int64) ([]models.FaultTreeNode, error) {
	if err := s.requireTenantProduct(tenantID, productID); err != nil {
		return nil, err
	}
	return repositories.FaultTreeRepository.ListByTenantProduct(sqls.DB(), tenantID, strconv.FormatInt(productID, 10))
}

func (s *faultTreeService) CreateManagedNode(tenantID int64, req dto.EnterpriseFaultTreeNodeCreateRequest) (*models.FaultTreeNode, error) {
	if err := s.requireTenantProduct(tenantID, req.ProductID); err != nil {
		return nil, err
	}

	title := strings.TrimSpace(req.Title)
	nodeType := strings.TrimSpace(req.NodeType)
	pattern := strings.TrimSpace(req.FaultPattern)
	riskLevel := strings.TrimSpace(req.RiskLevel)
	parentID := strings.TrimSpace(req.ParentID)
	conditions := strings.TrimSpace(req.TriggerConditions)
	if title == "" {
		return nil, errorsx.InvalidParam("title is required")
	}
	if !faultTreeNodeTypes[nodeType] {
		return nil, errorsx.InvalidParam("invalid node type")
	}
	if pattern == "" {
		pattern = "persistent"
	}
	if !faultTreePatterns[pattern] {
		return nil, errorsx.InvalidParam("invalid fault pattern")
	}
	if riskLevel == "" {
		riskLevel = "low"
	}
	if !faultTreeRiskLevels[riskLevel] {
		return nil, errorsx.InvalidParam("invalid risk level")
	}
	if conditions == "" {
		conditions = "[]"
	}
	if !json.Valid([]byte(conditions)) {
		return nil, errorsx.InvalidParam("trigger conditions must be valid JSON")
	}
	productID := strconv.FormatInt(req.ProductID, 10)
	if err := s.validateManagedParent(tenantID, productID, parentID, ""); err != nil {
		return nil, err
	}

	now := time.Now()
	row := &models.FaultTreeNode{
		ID:                uuid.NewString(),
		TenantID:          tenantID,
		ProductID:         productID,
		ParentID:          parentID,
		Title:             title,
		Description:       strings.TrimSpace(req.Description),
		NodeType:          nodeType,
		FaultPattern:      pattern,
		TriggerConditions: conditions,
		IsComposite:       req.IsComposite,
		RiskLevel:         riskLevel,
		OrderIndex:        req.OrderIndex,
		Status:            "draft",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := repositories.FaultTreeRepository.Create(sqls.DB(), row); err != nil {
		return nil, err
	}
	return row, nil
}

func (s *faultTreeService) UpdateManagedNode(tenantID int64, nodeID string, req dto.EnterpriseFaultTreeNodeUpdateRequest) (*models.FaultTreeNode, error) {
	row := repositories.FaultTreeRepository.GetByTenant(sqls.DB(), tenantID, strings.TrimSpace(nodeID))
	if row == nil {
		return nil, errorsx.InvalidParam("fault tree node not found")
	}
	productID, err := strconv.ParseInt(row.ProductID, 10, 64)
	if err != nil || productID <= 0 {
		return nil, errorsx.InvalidParam("fault tree node has invalid product")
	}
	if err := s.requireTenantProduct(tenantID, productID); err != nil {
		return nil, err
	}

	updates := map[string]any{"updated_at": time.Now()}
	if req.ParentID != nil {
		parentID := strings.TrimSpace(*req.ParentID)
		if err := s.validateManagedParent(tenantID, row.ProductID, parentID, row.ID); err != nil {
			return nil, err
		}
		updates["parent_id"] = parentID
	}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, errorsx.InvalidParam("title is required")
		}
		updates["title"] = title
	}
	if req.Description != nil {
		updates["description"] = strings.TrimSpace(*req.Description)
	}
	if req.NodeType != nil {
		nodeType := strings.TrimSpace(*req.NodeType)
		if !faultTreeNodeTypes[nodeType] {
			return nil, errorsx.InvalidParam("invalid node type")
		}
		updates["node_type"] = nodeType
	}
	if req.FaultPattern != nil {
		pattern := strings.TrimSpace(*req.FaultPattern)
		if !faultTreePatterns[pattern] {
			return nil, errorsx.InvalidParam("invalid fault pattern")
		}
		updates["fault_pattern"] = pattern
	}
	if req.TriggerConditions != nil {
		conditions := strings.TrimSpace(*req.TriggerConditions)
		if conditions == "" {
			conditions = "[]"
		}
		if !json.Valid([]byte(conditions)) {
			return nil, errorsx.InvalidParam("trigger conditions must be valid JSON")
		}
		updates["trigger_conditions"] = conditions
	}
	if req.IsComposite != nil {
		updates["is_composite"] = *req.IsComposite
	}
	if req.RiskLevel != nil {
		riskLevel := strings.TrimSpace(*req.RiskLevel)
		if !faultTreeRiskLevels[riskLevel] {
			return nil, errorsx.InvalidParam("invalid risk level")
		}
		updates["risk_level"] = riskLevel
	}
	if req.OrderIndex != nil {
		updates["order_index"] = *req.OrderIndex
	}
	if req.Status != nil {
		status := strings.TrimSpace(*req.Status)
		if !faultTreeStatuses[status] {
			return nil, errorsx.InvalidParam("invalid fault tree status")
		}
		updates["status"] = status
	}

	if err := repositories.FaultTreeRepository.UpdatesByTenant(sqls.DB(), tenantID, row.ID, updates); err != nil {
		return nil, err
	}
	updated := repositories.FaultTreeRepository.GetByTenant(sqls.DB(), tenantID, row.ID)
	if updated == nil {
		return nil, errorsx.InvalidParam("fault tree node not found")
	}
	return updated, nil
}

func (s *faultTreeService) requireTenantProduct(tenantID, productID int64) error {
	if tenantID <= 0 || productID <= 0 {
		return errorsx.InvalidParam("tenant and product are required")
	}
	if repositories.ProductRepository.GetByTenant(sqls.DB(), productID, tenantID) == nil {
		return errorsx.InvalidParam("product not found")
	}
	return nil
}

func (s *faultTreeService) validateManagedParent(tenantID int64, productID, parentID, nodeID string) error {
	visited := map[string]bool{}
	for parentID != "" {
		if parentID == nodeID || visited[parentID] {
			return errorsx.InvalidParam("fault tree parent would create a cycle")
		}
		visited[parentID] = true
		parent := repositories.FaultTreeRepository.GetByTenant(sqls.DB(), tenantID, parentID)
		if parent == nil || parent.ProductID != productID {
			return errorsx.InvalidParam("parent fault tree node not found")
		}
		parentID = parent.ParentID
	}
	return nil
}

// MatchFaultTree 根据症状匹配故障树根节点
func (s *faultTreeService) MatchFaultTree(ctx context.Context, productID string, symptoms []string) ([]*models.FaultTreeNode, error) {
	return s.matchFaultTree(ctx, 0, productID, symptoms)
}

func (s *faultTreeService) MatchFaultTreeForTenant(ctx context.Context, tenantID int64, productID string, symptoms []string) ([]*models.FaultTreeNode, error) {
	return s.matchFaultTree(ctx, tenantID, productID, symptoms)
}

func (s *faultTreeService) matchFaultTree(ctx context.Context, tenantID int64, productID string, symptoms []string) ([]*models.FaultTreeNode, error) {
	if productID == "" || len(symptoms) == 0 {
		return nil, nil
	}

	// 查询该产品下已发布的故障树节点（根节点: parent_id为空）
	allNodes, err := repositories.FaultTreeRepository.ListPublishedByProductType(sqls.DB(), tenantID, productID, "symptom")
	if err != nil {
		return nil, err
	}

	if len(allNodes) == 0 {
		return nil, nil
	}

	// 按症状关键词匹配
	matched := make([]*models.FaultTreeNode, 0)
	for i := range allNodes {
		node := allNodes[i]
		if s.nodeMatchesSymptoms(&node, symptoms) {
			matched = append(matched, &node)
		}
	}

	return matched, nil
}

// GetNextNode 获取故障树下一步节点
func (s *faultTreeService) GetNextNode(ctx context.Context, currentNodeID string) (*models.FaultTreeNode, error) {
	if currentNodeID == "" {
		return nil, nil
	}

	node := repositories.FaultTreeRepository.GetPublished(sqls.DB(), currentNodeID)
	if node == nil {
		return nil, nil
	}

	next := repositories.FaultTreeRepository.FindNextPublished(sqls.DB(), node)
	if next == nil {
		return node, nil
	}
	return next, nil
}

// GetDiagnosisPath 获取会话的诊断路径（已匹配的节点序列）
func (s *faultTreeService) GetDiagnosisPath(ctx context.Context, sessionID string) ([]*models.FaultTreeNode, error) {
	if sessionID == "" {
		return nil, nil
	}

	// 从诊断步骤中提取匹配的故障树节点ID
	steps, err := repositories.DiagnosisStepRepository.ListBySession(sqls.DB(), sessionID)
	if err != nil {
		return nil, err
	}

	nodeIDSet := make(map[string]bool)
	var nodeIDs []string
	for _, step := range steps {
		if step.FaultNodeIDs == "" {
			continue
		}
		var ids []string
		if err := json.Unmarshal([]byte(step.FaultNodeIDs), &ids); err != nil {
			continue
		}
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id != "" && !nodeIDSet[id] {
				nodeIDSet[id] = true
				nodeIDs = append(nodeIDs, id)
			}
		}
	}

	if len(nodeIDs) == 0 {
		return nil, nil
	}

	// 批量查询节点
	nodes, err := repositories.FaultTreeRepository.ListPublishedByIDs(sqls.DB(), nodeIDs)
	if err != nil {
		return nil, err
	}

	// 按nodeIDs顺序排序
	nodeMap := make(map[string]*models.FaultTreeNode, len(nodes))
	for i := range nodes {
		nodeMap[nodes[i].ID] = &nodes[i]
	}

	result := make([]*models.FaultTreeNode, 0, len(nodeIDs))
	for _, id := range nodeIDs {
		if node, ok := nodeMap[id]; ok {
			result = append(result, node)
		}
	}

	return result, nil
}

// nodeMatchesSymptoms 判断节点标题/描述是否匹配症状
func (s *faultTreeService) nodeMatchesSymptoms(node *models.FaultTreeNode, symptoms []string) bool {
	if node == nil || len(symptoms) == 0 {
		return false
	}

	searchText := strings.ToLower(node.Title + " " + node.Description)
	for _, symptom := range symptoms {
		symptom = strings.TrimSpace(strings.ToLower(symptom))
		if symptom == "" {
			continue
		}
		if strings.Contains(searchText, symptom) || hasFaultTreeBigramMatch(searchText, symptom) {
			return true
		}
	}
	return false
}

func hasFaultTreeBigramMatch(searchText, symptom string) bool {
	compact := func(value string) []rune {
		result := make([]rune, 0, len(value))
		for _, char := range []rune(strings.ToLower(value)) {
			if unicode.IsLetter(char) || unicode.IsNumber(char) {
				result = append(result, char)
			}
		}
		return result
	}
	query := compact(symptom)
	text := compact(searchText)
	if len(query) < 2 || len(text) < 2 {
		return false
	}
	queryPairs := map[string]bool{}
	for i := 0; i < len(query)-1; i++ {
		queryPairs[string(query[i:i+2])] = true
	}
	textPairs := map[string]bool{}
	for i := 0; i < len(text)-1; i++ {
		textPairs[string(text[i:i+2])] = true
	}
	matched := 0
	for pair := range queryPairs {
		if textPairs[pair] {
			matched++
		}
	}
	minimum := 1
	if len(queryPairs) >= 3 {
		minimum = 2
	}
	return matched >= minimum && matched*5 >= len(queryPairs)*3
}
