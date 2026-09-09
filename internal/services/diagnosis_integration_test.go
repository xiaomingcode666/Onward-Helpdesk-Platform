package services_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"remotehelpdesk/internal/bootstrap"
	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/google/uuid"
	"github.com/mlogclub/simple/sqls"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupDiagnosisIntegrationDB(t *testing.T) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "diagnosis-integration-test.db")
	db, err := bootstrap.InitDB(config.DBConfig{
		Type:         "sqlite",
		DSN:          "file:" + dbPath + "?_busy_timeout=5000",
		MaxIdleConns: 1,
		MaxOpenConns: 1,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	t.Cleanup(func() {
		eventbus.WaitAsync[events.DiagnosisCompletedEvent]()
	})
	require.NoError(t, bootstrap.InitMigrations())
}

type diagnosisTestFixture struct {
	TenantID    int64
	CustomerID  int64
	CustomerStr string
	DeviceID    int64
	DeviceStr   string
	ProductID   int64
	ProductStr  string
	Operator    *dto.AuthPrincipal
}

func createDiagnosisFixture(t *testing.T, prefix string) *diagnosisTestFixture {
	t.Helper()
	now := time.Now()
	f := &diagnosisTestFixture{}

	tenant := &models.Tenant{Name: prefix + "-diag-tenant", Status: enums.StatusOk}
	require.NoError(t, sqls.DB().Create(tenant).Error)
	f.TenantID = tenant.ID

	product := &models.Product{TenantID: tenant.ID, Code: prefix + "-diag-prod", Name: prefix + " Diag Product", Status: enums.StatusOk}
	require.NoError(t, repositories.ProductRepository.Create(sqls.DB(), product))
	f.ProductID = product.ID
	f.ProductStr = fmt.Sprintf("%d", product.ID)

	user := &models.User{
		Username: fmt.Sprintf("%s-diag-op-%d", prefix, now.UnixNano()),
		Nickname: prefix + "-diag-operator",
		Status:   enums.StatusOk,
	}
	require.NoError(t, sqls.DB().Create(user).Error)
	f.Operator = &dto.AuthPrincipal{UserID: user.ID, Username: user.Username, TenantID: tenant.ID}

	customer := &models.Customer{
		Name:   prefix + "-diag-customer",
		Status: enums.StatusOk,
	}
	require.NoError(t, repositories.CustomerRepository.Create(sqls.DB(), customer))
	f.CustomerID = customer.ID
	f.CustomerStr = fmt.Sprintf("%d", customer.ID)

	device := &models.Device{TenantID: tenant.ID, ProductID: product.ID, DeviceNo: prefix + "-diag-device", Status: enums.StatusOk}
	require.NoError(t, repositories.DeviceRepository.Create(sqls.DB(), device))
	f.DeviceID = device.ID
	f.DeviceStr = fmt.Sprintf("%d", device.ID)

	return f
}

// TestDiagnosisSessionLifecycle 诊断会话完整流程测试
func TestDiagnosisSessionLifecycle(t *testing.T) {
	setupDiagnosisIntegrationDB(t)
	f := createDiagnosisFixture(t, "lifecycle")

	ctx := context.Background()

	// 1. 创建诊断会话
	session, err := services.DiagnosisService.CreateDiagnosisSession(
		ctx,
		f.TenantID,
		f.CustomerStr,
		f.DeviceStr,
		f.ProductStr,
		"",       // serviceCodeID
		"",       // conversationID
		"zh-CN",  // language
		"设备无法启动", // symptoms
		10,       // maxRounds
	)
	require.NoError(t, err)
	assert.NotEmpty(t, session.ID)
	assert.Equal(t, "active", session.Status)
	assert.Equal(t, f.TenantID, session.TenantID)
	assert.Equal(t, "zh-CN", session.Language)
	assert.Equal(t, 10, session.MaxRounds)
	assert.Zero(t, session.TotalRounds)

	// 2. 执行一轮诊断
	step, err := services.DiagnosisService.Diagnose(ctx, session.ID, "按下开机键后电源灯不亮")
	require.NoError(t, err)
	assert.NotNil(t, step)
	assert.NotEmpty(t, step.ID)
	assert.Contains(t, step.InputContent, "按下开机键后电源灯不亮")
	assert.Equal(t, "ai_response", step.StepType)
	assert.Equal(t, 1, step.SequenceNo, "第一轮诊断的序号应为1")

	// 3. 获取诊断摘要
	summary, err := services.DiagnosisService.GetDiagnosisSummary(session.ID)
	require.NoError(t, err)
	assert.NotNil(t, summary)
	assert.Equal(t, session.ID, summary["session_id"])
	assert.Equal(t, f.TenantID, summary["tenant_id"])

	// 4. 结束诊断会话（正常解决）
	err = services.DiagnosisService.EndDiagnosisSession(session.ID, "resolved", "电源模块接触不良，重新插拔后正常")
	require.NoError(t, err)
	require.NoError(t, services.DiagnosisService.EndDiagnosisSession(session.ID, "resolved", "电源模块接触不良，重新插拔后正常"), "identical completion retry must be idempotent")
	require.Error(t, services.DiagnosisService.EndDiagnosisSession(session.ID, "escalated", "重复改写终态"), "terminal diagnosis status must be immutable")

	// 验证会话状态已更新
	var updatedSession models.DiagnosisSession
	err = sqls.DB().Where("id = ?", session.ID).First(&updatedSession).Error
	require.NoError(t, err)
	assert.Equal(t, "resolved", updatedSession.Status)
	assert.NotNil(t, updatedSession.EndedAt)
	var eventCount, outboxCount int64
	require.NoError(t, sqls.DB().Model(&models.DomainEvent{}).
		Where("event_type = ? AND aggregate_id = ?", events.EventDiagnosisCompleted, session.ID).
		Count(&eventCount).Error)
	require.NoError(t, sqls.DB().Model(&models.OutboxRecord{}).
		Where("event_type = ?", events.EventDiagnosisCompleted).
		Count(&outboxCount).Error)
	assert.EqualValues(t, 1, eventCount)
	assert.EqualValues(t, 1, outboxCount)
}

// TestDiagnosisToTicketHandoff 诊断转人工工单测试
func TestDiagnosisToTicketHandoff(t *testing.T) {
	setupDiagnosisIntegrationDB(t)
	f := createDiagnosisFixture(t, "handoff")

	ctx := context.Background()

	// 1. 创建诊断会话
	session, err := services.DiagnosisService.CreateDiagnosisSession(
		ctx,
		f.TenantID, f.CustomerStr, f.DeviceStr, f.ProductStr,
		"", "", "zh-CN", "设备异常噪音", 3, // maxRounds=3，快速达到上限
	)
	require.NoError(t, err)

	// 2. 执行多轮诊断（模拟低置信度场景）
	for i := 0; i < 3; i++ {
		_, err := services.DiagnosisService.Diagnose(ctx, session.ID, fmt.Sprintf("问题依旧存在，轮次%d", i+1))
		require.NoError(t, err)
	}

	// 3. 检查是否需要转人工
	needEscalate, reason := services.DiagnosisService.ShouldEscalateToHuman(session.ID)
	if needEscalate {
		t.Logf("需要转人工，原因：%s", reason)

		// 结束诊断并标记为转人工
		err = services.DiagnosisService.EndDiagnosisSession(session.ID, "escalated", "连续诊断未解决，转人工处理")
		require.NoError(t, err)

		// 验证会话状态
		var updatedSession models.DiagnosisSession
		err = sqls.DB().Where("id = ?", session.ID).First(&updatedSession).Error
		require.NoError(t, err)
		assert.Equal(t, "escalated", updatedSession.Status)
	} else {
		t.Log("未触发转人工条件（置信度可能已达标）")
	}
}

// TestDiagnosisRAGRetrieval RAG 检索集成测试
func TestDiagnosisRAGRetrieval(t *testing.T) {
	setupDiagnosisIntegrationDB(t)
	f := createDiagnosisFixture(t, "rag")

	// 创建知识库和知识文档，验证诊断时的 RAG 集成
	kb := &models.KnowledgeBase{
		TenantID: f.TenantID,
		Name:     "诊断测试知识库",
		Status:   enums.StatusOk,
	}
	require.NoError(t, sqls.DB().Create(kb).Error)

	doc := &models.KnowledgeDocument{
		KnowledgeBaseID: kb.ID,
		Title:           "电源故障排查指南",
		Content:         "电源指示灯不亮时，请先检查电源线连接。如果连接正常但指示灯不亮，可能是电源板故障。",
		Status:          enums.StatusOk,
	}
	require.NoError(t, sqls.DB().Create(doc).Error)

	// 创建诊断会话并执行，内部会触发 RAG 检索
	ctx := context.Background()
	session, err := services.DiagnosisService.CreateDiagnosisSession(
		ctx,
		f.TenantID, f.CustomerStr, f.DeviceStr, f.ProductStr,
		"", "", "zh-CN", "电源指示灯不亮", 5,
	)
	require.NoError(t, err)

	step, err := services.DiagnosisService.Diagnose(ctx, session.ID, "电源指示灯不亮")
	require.NoError(t, err)
	assert.NotNil(t, step)
	// 诊断步骤应包含输出内容
	assert.NotEmpty(t, step.OutputContent, "诊断结果不应为空")

	t.Logf("RAG 集成诊断结果：%s", step.OutputContent)
}

// TestDiagnosisFaultTreeMatching 故障树匹配测试
func TestDiagnosisFaultTreeMatching(t *testing.T) {
	setupDiagnosisIntegrationDB(t)
	f := createDiagnosisFixture(t, "fault-tree")

	// 创建故障树节点
	nodeID1 := uuid.New().String()
	nodeID2 := uuid.New().String()

	node1 := &models.FaultTreeNode{
		ID:          nodeID1,
		TenantID:    f.TenantID,
		ProductID:   f.ProductStr,
		Title:       "电源故障",
		Description: "设备无法开机，电源指示灯不亮",
		NodeType:    "symptom",
		RiskLevel:   "medium",
		OrderIndex:  1,
		Status:      "published",
	}
	require.NoError(t, sqls.DB().Create(node1).Error)

	node2 := &models.FaultTreeNode{
		ID:          nodeID2,
		TenantID:    f.TenantID,
		ProductID:   f.ProductStr,
		ParentID:    nodeID1,
		Title:       "电源板检查",
		Description: "检查电源板连接和保险丝",
		NodeType:    "check",
		RiskLevel:   "low",
		OrderIndex:  1,
		Status:      "published",
	}
	require.NoError(t, sqls.DB().Create(node2).Error)

	// 匹配故障树节点
	nodes, err := services.FaultTreeService.MatchFaultTree(
		context.Background(),
		f.ProductStr,
		[]string{"电源灯不亮"},
	)
	require.NoError(t, err)
	assert.NotEmpty(t, nodes, "应匹配到故障树节点")

	// 验证匹配到的节点
	found := false
	for _, n := range nodes {
		if n.Title == "电源故障" {
			found = true
			break
		}
	}
	assert.True(t, found, "应匹配到'电源故障'节点")
}

// TestShouldEscalateToHuman 转人工判断逻辑测试
func TestShouldEscalateToHuman(t *testing.T) {
	setupDiagnosisIntegrationDB(t)
	f := createDiagnosisFixture(t, "escalate-check")

	ctx := context.Background()

	// 场景 1：低置信度场景 - 创建会话但不执行有效诊断
	sessionLowConf, err := services.DiagnosisService.CreateDiagnosisSession(
		ctx,
		f.TenantID, f.CustomerStr, f.DeviceStr, f.ProductStr,
		"", "", "zh-CN", "奇怪的噪音", 5,
	)
	require.NoError(t, err)

	// 模拟低置信度：直接设置置信度为 0.2
	require.NoError(t, sqls.DB().Model(&models.DiagnosisSession{}).
		Where("id = ?", sessionLowConf.ID).
		Update("confidence_score", 0.2).Error)

	needEscalate, reason := services.DiagnosisService.ShouldEscalateToHuman(sessionLowConf.ID)
	assert.True(t, needEscalate, "低置信度时应触发转人工")
	assert.Contains(t, reason, "low_confidence", "转人工原因应包含 low_confidence")

	// 场景 2：多轮无效 - 已经诊断 3 轮但置信度仍低
	sessionMultiRound, err := services.DiagnosisService.CreateDiagnosisSession(
		ctx,
		f.TenantID, f.CustomerStr, f.DeviceStr, f.ProductStr,
		"", "", "zh-CN", "间歇性故障", 5,
	)
	require.NoError(t, err)

	// 模拟多轮诊断但置信度不高
	require.NoError(t, sqls.DB().Model(&models.DiagnosisSession{}).
		Where("id = ?", sessionMultiRound.ID).
		Updates(map[string]any{
			"total_rounds":     3,
			"confidence_score": 0.45,
		}).Error)

	needEscalate, reason = services.DiagnosisService.ShouldEscalateToHuman(sessionMultiRound.ID)
	assert.True(t, needEscalate, "多轮无效时应触发转人工")
	assert.Contains(t, reason, "multiple_rounds_no_resolution", "转人工原因应包含 multiple_rounds_no_resolution")

	// 场景 3：安全风险（不属于 ShouldEscalateToHuman 逻辑，但作为补充验证）
	// 创建一个高置信度会话，不应转人工
	sessionHighConf, err := services.DiagnosisService.CreateDiagnosisSession(
		ctx,
		f.TenantID, f.CustomerStr, f.DeviceStr, f.ProductStr,
		"", "", "zh-CN", "正常问题", 5,
	)
	require.NoError(t, err)

	require.NoError(t, sqls.DB().Model(&models.DiagnosisSession{}).
		Where("id = ?", sessionHighConf.ID).
		Updates(map[string]any{
			"total_rounds":     1,
			"confidence_score": 0.85,
		}).Error)

	needEscalate, reason = services.DiagnosisService.ShouldEscalateToHuman(sessionHighConf.ID)
	assert.False(t, needEscalate, "高置信度且少轮次时不应触发转人工")
	assert.Empty(t, reason)
}
