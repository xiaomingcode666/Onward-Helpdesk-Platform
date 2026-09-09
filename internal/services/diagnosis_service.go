package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"remotehelpdesk/internal/ai"
	"remotehelpdesk/internal/ai/rag"
	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/pkg/utils"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var DiagnosisService = newDiagnosisService()

func newDiagnosisService() *diagnosisService {
	return &diagnosisService{}
}

type diagnosisService struct{}

type DiagnosisSessionCreateRequest struct {
	TenantID       int64
	CustomerID     int64
	DeviceID       int64
	ProductID      int64
	ServiceCodeID  int64
	ConversationID int64
	Language       string
	RegionCode     string
	Audience       string
	Symptoms       string
	MaxRounds      int
}

// CreateDiagnosisSession 为客户创建诊断会话（绑定设备、产品、服务码上下文）
func (s *diagnosisService) CreateDiagnosisSession(ctx context.Context, tenantID int64, customerID, deviceID, productID, serviceCodeID, conversationID, language, symptoms string, maxRounds int) (*models.DiagnosisSession, error) {
	return s.CreateDiagnosisSessionWithContext(ctx, DiagnosisSessionCreateRequest{
		TenantID:       tenantID,
		CustomerID:     parseDiagnosisInt64(customerID),
		DeviceID:       parseDiagnosisInt64(deviceID),
		ProductID:      parseDiagnosisInt64(productID),
		ServiceCodeID:  parseDiagnosisInt64(serviceCodeID),
		ConversationID: parseDiagnosisInt64(conversationID),
		Language:       language,
		Audience:       "agent",
		Symptoms:       symptoms,
		MaxRounds:      maxRounds,
	})
}

func (s *diagnosisService) CreateDiagnosisSessionWithContext(ctx context.Context, req DiagnosisSessionCreateRequest) (*models.DiagnosisSession, error) {
	if req.MaxRounds <= 0 {
		req.MaxRounds = 10
	}

	// 解析症状为JSON数组
	var symptomsJSON string
	if req.Symptoms != "" {
		symptomArr := []string{req.Symptoms}
		b, _ := json.Marshal(symptomArr)
		symptomsJSON = string(b)
	} else {
		symptomsJSON = "[]"
	}

	scopeContext := dto.KnowledgeScopeContext{
		TenantID:      req.TenantID,
		ProductID:     req.ProductID,
		DeviceID:      req.DeviceID,
		ServiceCodeID: req.ServiceCodeID,
		CustomerID:    req.CustomerID,
		Locale:        req.Language,
		RegionCode:    req.RegionCode,
		Audience:      req.Audience,
	}
	resolvedScope, err := ProductKnowledgeResolver.ResolveScope(scopeContext)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve diagnosis scope: %w", err)
	}
	scopeSnapshot := marshalDiagnosisScopeSnapshot(resolvedScope)

	now := time.Now()
	session := &models.DiagnosisSession{
		ID:                     utils.UUID(),
		TenantID:               req.TenantID,
		CustomerID:             formatDiagnosisLegacyID(req.CustomerID),
		DeviceID:               formatDiagnosisLegacyID(resolvedScope.Context.DeviceID),
		ProductID:              formatDiagnosisLegacyID(resolvedScope.Context.ProductID),
		ProductModelID:         resolvedScope.Context.ProductModelID,
		ServiceCodeID:          formatDiagnosisLegacyID(resolvedScope.Context.ServiceCodeID),
		ConversationID:         formatDiagnosisLegacyID(req.ConversationID),
		DeviceRefID:            resolvedScope.Context.DeviceID,
		ProductRefID:           resolvedScope.Context.ProductID,
		ServiceCodeRefID:       resolvedScope.Context.ServiceCodeID,
		ConversationRefID:      req.ConversationID,
		Status:                 "active",
		Language:               resolvedScope.Context.Locale,
		RegionCode:             resolvedScope.Context.RegionCode,
		Audience:               resolvedScope.Context.Audience,
		KnowledgeScopeSnapshot: scopeSnapshot,
		Symptoms:               symptomsJSON,
		FaultCodes:             "[]",
		MaxRounds:              req.MaxRounds,
		RetrieveCount:          0,
		RetrieveTopScore:       0,
		RetrieveHitCount:       0,
		ConfidenceScore:        0,
		CreatedAt:              now,
		BaseModel: models.BaseModel{
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	if err := sqls.DB().Create(session).Error; err != nil {
		slog.Error("create diagnosis session failed", "error", err)
		return nil, fmt.Errorf("failed to create diagnosis session: %w", err)
	}

	slog.Info("diagnosis session created", "sessionID", session.ID, "tenantID", req.TenantID)
	return session, nil
}

// Diagnose 执行一轮诊断
//  1. RAG 检索相关知识（调用 knowledge_retrieve_log_service）
//  2. 匹配故障树节点
//  3. 生成诊断建议（调用 ai.LLM.Chat）
//  4. 记录诊断步骤到 diagnosis_steps
func (s *diagnosisService) Diagnose(ctx context.Context, sessionID string, userInput string) (*models.DiagnosisStep, error) {
	session := s.getSession(sessionID)
	if session == nil {
		return nil, fmt.Errorf("diagnosis session not found: %s", sessionID)
	}
	return s.DiagnoseForTenant(ctx, session.TenantID, sessionID, userInput)
}

func (s *diagnosisService) DiagnoseForTenant(ctx context.Context, tenantID int64, sessionID string, userInput string) (*models.DiagnosisStep, error) {
	session := s.getSessionForTenant(tenantID, sessionID)
	if session == nil {
		return nil, fmt.Errorf("diagnosis session not found: %s", sessionID)
	}
	if session.Status != "active" {
		return nil, fmt.Errorf("diagnosis session is not active, status: %s", session.Status)
	}

	// 更新会话轮次
	session.TotalRounds++
	sqls.DB().Model(&models.DiagnosisSession{}).Where("tenant_id = ? AND id = ?", session.TenantID, sessionID).
		UpdateColumn("total_rounds", session.TotalRounds)

	// 解析现有症状
	var symptoms []string
	if session.Symptoms != "" && session.Symptoms != "[]" {
		json.Unmarshal([]byte(session.Symptoms), &symptoms)
	}
	if len(symptoms) == 0 && userInput != "" {
		symptoms = []string{userInput}
	}

	// 1. RAG 检索
	retrieveResult, retrieveErr := s.retrieveKnowledge(ctx, session, userInput)
	if retrieveErr != nil {
		slog.Warn("diagnose retrieve failed", "sessionID", sessionID, "error", retrieveErr)
	}

	// 2. 匹配故障树节点
	matchedNodes, _ := FaultTreeService.MatchFaultTreeForTenant(ctx, session.TenantID, session.ProductID, symptoms)

	// 3. 构建上下文并调用 LLM 生成诊断建议
	ragContext := ""
	if retrieveResult != nil && strings.TrimSpace(retrieveResult.ContextText) != "" {
		ragContext = retrieveResult.ContextText
	}
	faultTreeText := s.buildFaultTreeContext(matchedNodes)
	llmCtx := ai.WithCapabilityScope(ctx, ai.CapabilityScope{
		TenantID:  session.TenantID,
		ProductID: session.ProductRefID,
	})
	llmResponse, llmErr := s.callDiagnosisLLM(llmCtx, session.Language, userInput, ragContext, faultTreeText)
	if llmErr != nil {
		slog.Error("diagnose llm call failed", "sessionID", sessionID, "error", llmErr)
		llmResponse = "Unable to generate diagnosis suggestion at this time."
	}

	// 计算置信度
	confidence := s.calculateConfidence(retrievalHitsForDiagnosis(retrieveResult), matchedNodes, retrieveErr, llmErr)

	// 4. 记录诊断步骤
	step := s.createDiagnosisStep(session, userInput, llmResponse, matchedNodes, retrieveResult, confidence)

	// 更新会话检索统计
	s.updateSessionStats(session, retrieveResult, confidence)

	return step, nil
}

// ShouldEscalateToHuman 判断是否需要转人工
func (s *diagnosisService) ShouldEscalateToHuman(sessionID string) (bool, string) {
	session := s.getSession(sessionID)
	if session == nil {
		return false, ""
	}
	return s.ShouldEscalateToHumanForTenant(session.TenantID, sessionID)
}

func (s *diagnosisService) ShouldEscalateToHumanForTenant(tenantID int64, sessionID string) (bool, string) {
	session := s.getSessionForTenant(tenantID, sessionID)
	if session == nil {
		return false, ""
	}

	var reasons []string

	// 低置信度（无命中、TopScore低于阈值）
	if session.ConfidenceScore > 0 && session.ConfidenceScore < 0.4 {
		reasons = append(reasons, "low_confidence")
	}

	// 多轮无效（3轮仍未解决或用户连续否定）
	if session.TotalRounds >= 3 && session.ConfidenceScore < 0.5 {
		reasons = append(reasons, "multiple_rounds_no_resolution")
	}

	// 超过最大轮次
	if session.TotalRounds >= session.MaxRounds {
		reasons = append(reasons, "max_rounds_reached")
	}

	// 检查最近步骤的否定反馈
	if s.hasNegativeUserFeedbackForTenant(tenantID, sessionID) {
		reasons = append(reasons, "user_rejected_suggestion")
	}

	if len(reasons) > 0 {
		return true, strings.Join(reasons, ",")
	}

	return false, ""
}

// GetDiagnosisSummary 获取诊断摘要（用于转人工时传递上下文）
func (s *diagnosisService) GetDiagnosisSummary(sessionID string) (map[string]any, error) {
	session := s.getSession(sessionID)
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	return s.GetDiagnosisSummaryForTenant(session.TenantID, sessionID)
}

func (s *diagnosisService) GetDiagnosisSummaryForTenant(tenantID int64, sessionID string) (map[string]any, error) {
	session := s.getSessionForTenant(tenantID, sessionID)
	if session == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	var steps []models.DiagnosisStep
	sqls.DB().Where("tenant_id = ? AND session_id = ?", session.TenantID, sessionID).
		Order("sequence_no asc, id asc").
		Find(&steps)

	nodes, _ := FaultTreeService.GetDiagnosisPath(context.Background(), sessionID)

	summary := map[string]any{
		"session_id":         session.ID,
		"tenant_id":          session.TenantID,
		"customer_id":        session.CustomerID,
		"device_id":          session.DeviceID,
		"product_id":         session.ProductID,
		"product_model_id":   session.ProductModelID,
		"service_code_id":    session.ServiceCodeID,
		"conversation_id":    session.ConversationID,
		"region_code":        session.RegionCode,
		"audience":           session.Audience,
		"knowledge_scope":    session.KnowledgeScopeSnapshot,
		"status":             session.Status,
		"symptoms":           session.Symptoms,
		"fault_codes":        session.FaultCodes,
		"total_rounds":       session.TotalRounds,
		"confidence_score":   session.ConfidenceScore,
		"retrieve_count":     session.RetrieveCount,
		"retrieve_hit_count": session.RetrieveHitCount,
		"retrieve_top_score": session.RetrieveTopScore,
		"created_at":         session.CreatedAt,
		"steps":              steps,
		"fault_tree_path":    nodes,
	}

	return summary, nil
}

// EndDiagnosisSession 结束诊断（转人工、问题解决或放弃）
func (s *diagnosisService) EndDiagnosisSession(sessionID string, status string, resolution string) error {
	session := s.getSession(sessionID)
	if session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	return s.EndDiagnosisSessionForTenant(session.TenantID, sessionID, status, resolution)
}

func (s *diagnosisService) EndDiagnosisSessionForTenant(tenantID int64, sessionID string, status string, resolution string) error {
	if status != "escalated" && status != "resolved" && status != "abandoned" {
		return fmt.Errorf("invalid end status: %s, must be escalated/resolved/abandoned", status)
	}

	changed := false
	err := sqls.DB().Transaction(func(tx *gorm.DB) error {
		var current models.DiagnosisSession
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, sessionID).First(&current).Error; err != nil {
			return fmt.Errorf("diagnosis session not found: %w", err)
		}
		if current.EndedAt != nil && (current.Status == "escalated" || current.Status == "resolved" || current.Status == "abandoned") {
			if current.Status == status && strings.TrimSpace(current.Resolution) == strings.TrimSpace(resolution) {
				return nil
			}
			return fmt.Errorf("diagnosis session already ended with status %s", current.Status)
		}
		now := time.Now()
		if err := tx.Model(&models.DiagnosisSession{}).Where("tenant_id = ? AND id = ?", tenantID, sessionID).Updates(map[string]any{
			"status":     status,
			"resolution": resolution,
			"ended_at":   &now,
		}).Error; err != nil {
			return fmt.Errorf("failed to end diagnosis session: %w", err)
		}
		eventID := "tenant:" + strconv.FormatInt(tenantID, 10) + ":diagnosis.completed:" + sessionID
		if _, err := eventbus.EnqueueTx(tx, eventbus.DurableEvent{
			TenantID:       tenantID,
			IdempotencyKey: eventID,
			EventType:      events.EventDiagnosisCompleted,
			Payload: events.DiagnosisCompletedEvent{
				EventID:         eventID,
				SessionID:       sessionID,
				TenantID:        strconv.FormatInt(tenantID, 10),
				CustomerID:      current.CustomerID,
				Status:          status,
				ConfidenceScore: current.ConfidenceScore,
				HandoffReason:   resolution,
			},
			Source:      "diagnosis_service",
			AggregateID: sessionID,
			CreatedAt:   now,
		}); err != nil {
			return err
		}
		changed = true
		return nil
	})
	if err != nil {
		return err
	}

	slog.Info("diagnosis session ended", "sessionID", sessionID, "status", status)
	if changed {
		eventbus.WakeDefaultOutboxPublisher()
	}

	return nil
}

// ---------- 私有方法 ----------

func (s *diagnosisService) getSession(sessionID string) *models.DiagnosisSession {
	var session models.DiagnosisSession
	if err := sqls.DB().Where("id = ?", sessionID).First(&session).Error; err != nil {
		return nil
	}
	return &session
}

func (s *diagnosisService) getSessionForTenant(tenantID int64, sessionID string) *models.DiagnosisSession {
	if tenantID <= 0 {
		return nil
	}
	var session models.DiagnosisSession
	if err := sqls.DB().Where("tenant_id = ? AND id = ?", tenantID, sessionID).First(&session).Error; err != nil {
		return nil
	}
	return &session
}

func (s *diagnosisService) getSessionConfidence(sessionID string) float64 {
	var session models.DiagnosisSession
	if err := sqls.DB().Select("confidence_score").Where("id = ?", sessionID).First(&session).Error; err != nil {
		return 0
	}
	return session.ConfidenceScore
}

// retrieveKnowledge 通过产品知识绑定获取知识库ID并进行RAG检索
func (s *diagnosisService) retrieveKnowledge(ctx context.Context, session *models.DiagnosisSession, query string) (*KnowledgeRetrievalResult, error) {
	if query == "" {
		return nil, nil
	}

	req := KnowledgeRetrievalRequest{
		Scope: dto.KnowledgeScopeContext{
			TenantID:       session.TenantID,
			ProductID:      session.ProductRefID,
			ProductModelID: session.ProductModelID,
			DeviceID:       session.DeviceRefID,
			ServiceCodeID:  session.ServiceCodeRefID,
			CustomerID:     parseDiagnosisInt64(session.CustomerID),
			Locale:         session.Language,
			RegionCode:     session.RegionCode,
			Audience:       session.Audience,
		},
		Query:            query,
		TopK:             8,
		ScoreThreshold:   0.3,
		RerankTopN:       config.CurrentOrDefault().RAG.Normalized().RerankTopN,
		ContextMaxTokens: config.CurrentOrDefault().RAG.Normalized().ContextMaxTokens,
		Scene:            "diagnosis",
		SessionID:        session.ID,
		ConversationID:   session.ConversationRefID,
	}

	results, err := KnowledgeRetrievalService.Retrieve(ctx, req)
	if err != nil {
		slog.Error("rag retrieve failed", "query", query, "error", err)
		return nil, err
	}
	if results != nil && results.ResolvedScope.Context.TenantID > 0 {
		sqls.DB().Model(&models.DiagnosisSession{}).
			Where("tenant_id = ? AND id = ?", session.TenantID, session.ID).
			Update("knowledge_scope_snapshot", marshalDiagnosisScopeSnapshot(&results.ResolvedScope))
	}

	return results, nil
}

// buildRAGContextText 将RAG检索结果构造成 LLM 可读的上下文文本
func (s *diagnosisService) buildRAGContextText(results []rag.RetrieveResult) string {
	if len(results) == 0 {
		return ""
	}

	var parts []string
	for i, r := range results {
		if r.Content == "" {
			continue
		}
		label := fmt.Sprintf("[Knowledge %d]", i+1)
		if r.DocumentTitle != "" {
			label = fmt.Sprintf("[Source: %s]", r.DocumentTitle)
		}
		parts = append(parts, fmt.Sprintf("%s (score=%.4f):\n%s", label, r.Score, r.Content))
	}
	return strings.Join(parts, "\n\n")
}

// buildFaultTreeContext 将匹配的故障树节点构造成 LLM 可读的上下文文本
func (s *diagnosisService) buildFaultTreeContext(nodes []*models.FaultTreeNode) string {
	if len(nodes) == 0 {
		return ""
	}

	var parts []string
	for _, node := range nodes {
		if node == nil {
			continue
		}
		parts = append(parts, fmt.Sprintf("- [%s] %s: %s (risk: %s)",
			node.NodeType, node.Title, node.Description, node.RiskLevel))
	}
	return strings.Join(parts, "\n")
}

// callDiagnosisLLM 调用LLM生成诊断建议
func (s *diagnosisService) callDiagnosisLLM(ctx context.Context, language, userInput, ragContext, faultTreeContext string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	systemPrompt := s.buildDiagnosisSystemPrompt(language, ragContext, faultTreeContext)
	result, err := ai.LLM.Chat(ctx, systemPrompt, userInput)
	if err != nil {
		return "", err
	}

	return result.Content, nil
}

// buildDiagnosisSystemPrompt 构建诊断系统提示词
func (s *diagnosisService) buildDiagnosisSystemPrompt(language, ragContext, faultTreeContext string) string {
	prompt := "You are a technical diagnosis assistant for remote help desk support. "
	prompt += "Your role is to help diagnose equipment issues based on the user's description, knowledge base, and fault tree information.\n\n"

	if language != "" {
		prompt += fmt.Sprintf("Please respond in %s.\n\n", language)
	}

	if ragContext != "" {
		prompt += "## Relevant Knowledge Base References\n"
		prompt += ragContext + "\n\n"
	}

	if faultTreeContext != "" {
		prompt += "## Matched Fault Tree Nodes\n"
		prompt += faultTreeContext + "\n\n"
	}

	prompt += "## Instructions\n"
	prompt += "1. Analyze the user's problem description carefully.\n"
	prompt += "2. Use the provided knowledge and fault tree information to identify possible causes.\n"
	prompt += "3. Provide step-by-step diagnostic suggestions the user can follow.\n"
	prompt += "4. If the issue requires safety precautions, clearly state them.\n"
	prompt += "5. If you cannot determine the cause with confidence, suggest consulting a human engineer.\n"
	prompt += "6. Keep responses clear, structured, and actionable.\n"

	return prompt
}

// calculateConfidence 基于RAG结果、故障树匹配和LLM调用情况计算置信度
func (s *diagnosisService) calculateConfidence(ragResults []rag.RetrieveResult, matchedNodes []*models.FaultTreeNode, retrieveErr error, llmErr error) float64 {
	ragScore := 0.0
	if len(ragResults) > 0 {
		for _, r := range ragResults {
			if float64(r.Score) > ragScore {
				ragScore = float64(r.Score)
			}
		}
	}

	treeScore := 0.0
	if len(matchedNodes) > 0 {
		treeScore = 0.6
		if len(matchedNodes) >= 2 {
			treeScore = 0.8
		}
	}

	llmScore := 1.0
	if llmErr != nil {
		llmScore = 0.3
	}

	confidence := ragScore*0.4 + treeScore*0.35 + llmScore*0.25
	if retrieveErr != nil && confidence > 0.35 {
		confidence = 0.35
	}
	if confidence > 1.0 {
		confidence = 1.0
	}
	return confidence
}

// createDiagnosisStep 创建并持久化诊断步骤记录
func (s *diagnosisService) createDiagnosisStep(session *models.DiagnosisSession, input, output string, nodes []*models.FaultTreeNode, retrieval *KnowledgeRetrievalResult, confidence float64) *models.DiagnosisStep {
	nodeIDs := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if n != nil {
			nodeIDs = append(nodeIDs, n.ID)
		}
	}
	nodeIDsJSON, _ := json.Marshal(nodeIDs)

	citationsJSON, _ := json.Marshal(nil)
	if retrieval != nil && len(retrieval.Citations) > 0 {
		citationsJSON, _ = json.Marshal(retrieval.Citations)
	}

	step := &models.DiagnosisStep{
		ID:              utils.UUID(),
		SessionID:       session.ID,
		TenantID:        session.TenantID,
		SequenceNo:      session.TotalRounds,
		StepType:        "ai_response",
		InputContent:    input,
		OutputContent:   output,
		FaultNodeIDs:    string(nodeIDsJSON),
		RagCitations:    string(citationsJSON),
		ConfidenceScore: confidence,
		UserConfirmed:   "",
		CreatedAt:       time.Now(),
	}

	if err := sqls.DB().Create(step).Error; err != nil {
		slog.Error("create diagnosis step failed", "error", err)
		return nil
	}

	return step
}

// updateSessionStats 更新会话的检索统计和置信度
func (s *diagnosisService) updateSessionStats(session *models.DiagnosisSession, retrieval *KnowledgeRetrievalResult, confidence float64) {
	updates := map[string]any{
		"confidence_score": confidence,
	}

	if retrieval != nil && len(retrieval.RawHits) > 0 {
		updates["retrieve_count"] = session.RetrieveCount + 1
		updates["retrieve_hit_count"] = session.RetrieveHitCount + len(retrieval.RawHits)

		topScore := retrieval.TopScore
		if topScore > session.RetrieveTopScore {
			updates["retrieve_top_score"] = topScore
		}
	}

	sqls.DB().Model(&models.DiagnosisSession{}).Where("tenant_id = ? AND id = ?", session.TenantID, session.ID).Updates(updates)
}

// hasNegativeUserFeedback 检查最近步骤是否有否定反馈
func (s *diagnosisService) hasNegativeUserFeedback(sessionID string) bool {
	session := s.getSession(sessionID)
	if session == nil {
		return false
	}
	return s.hasNegativeUserFeedbackForTenant(session.TenantID, sessionID)
}

func (s *diagnosisService) hasNegativeUserFeedbackForTenant(tenantID int64, sessionID string) bool {
	var recentSteps []models.DiagnosisStep
	sqls.DB().Where("tenant_id = ? AND session_id = ? AND user_confirmed IN ?", tenantID, sessionID, []string{"rejected", "skipped"}).
		Order("sequence_no desc").
		Limit(3).
		Find(&recentSteps)

	rejectedCount := 0
	for _, step := range recentSteps {
		if step.UserConfirmed == "rejected" {
			rejectedCount++
		}
	}

	// 连续否定2次以上视为用户否定
	return rejectedCount >= 2
}

func parseDiagnosisInt64(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func formatDiagnosisLegacyID(value int64) string {
	if value <= 0 {
		return ""
	}
	return strconv.FormatInt(value, 10)
}

func marshalDiagnosisScopeSnapshot(scope *dto.ResolvedKnowledgeScope) string {
	if scope == nil {
		return ""
	}
	data, err := json.Marshal(scope)
	if err != nil {
		return ""
	}
	return string(data)
}

func retrievalHitsForDiagnosis(result *KnowledgeRetrievalResult) []rag.RetrieveResult {
	if result == nil {
		return nil
	}
	if len(result.RerankedHits) > 0 {
		return result.RerankedHits
	}
	return result.RawHits
}
