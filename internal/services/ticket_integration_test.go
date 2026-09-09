package services_test

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"remotehelpdesk/internal/bootstrap"
	"remotehelpdesk/internal/events"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/pkg/providers"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTicketIntegrationDB(t *testing.T) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "ticket-integration-test.db")
	db, err := bootstrap.InitDB(config.DBConfig{
		Type:         "sqlite",
		DSN:          "file:" + dbPath + "?_busy_timeout=5000",
		MaxIdleConns: 1,
		MaxOpenConns: 0,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		waitTicketIntegrationEvents()
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, bootstrap.InitMigrations())
}

func waitTicketIntegrationEvents() {
	eventbus.WaitAsync[events.TicketCreatedEvent]()
	eventbus.WaitAsync[events.TicketAssignedEvent]()
	eventbus.WaitAsync[events.ConversationAssignedEvent]()
	eventbus.WaitAsync[events.MeetingEndedEvent]()
	eventbus.WaitAsync[events.KnowledgeCandidateCreatedEvent]()
	eventbus.WaitAsync[events.TicketClosedEvent]()
	eventbus.WaitAsync[events.TicketReopenedEvent]()
}

type ticketIntegrationFixture struct {
	Operator     *dto.AuthPrincipal
	Tenant       *models.Tenant
	Product      *models.Product
	ProductModel *models.ProductModel
	Device       *models.Device
	ServiceCode  *models.ServiceCode
	CustomerID   int64
}

func createTicketIntegrationFixture(t *testing.T, prefix string) *ticketIntegrationFixture {
	t.Helper()
	now := time.Now()
	f := &ticketIntegrationFixture{}

	f.Tenant = &models.Tenant{Name: prefix + "-tenant", Status: enums.StatusOk}
	require.NoError(t, sqls.DB().Create(f.Tenant).Error)

	user := &models.User{
		Username: fmt.Sprintf("%s-op-%d", prefix, now.UnixNano()),
		Nickname: prefix + "-operator",
		Status:   enums.StatusOk,
	}
	require.NoError(t, sqls.DB().Create(user).Error)
	f.Operator = &dto.AuthPrincipal{UserID: user.ID, Username: user.Username, TenantID: f.Tenant.ID}

	f.Product = &models.Product{TenantID: f.Tenant.ID, Code: prefix + "-prod", Name: prefix + " Product", Status: enums.StatusOk}
	require.NoError(t, repositories.ProductRepository.Create(sqls.DB(), f.Product))

	f.ProductModel = &models.ProductModel{TenantID: f.Tenant.ID, ProductID: f.Product.ID, ModelCode: prefix + "-model", Name: prefix + " Model", Status: enums.StatusOk}
	require.NoError(t, repositories.ProductModelRepository.Create(sqls.DB(), f.ProductModel))

	f.Device = &models.Device{TenantID: f.Tenant.ID, ProductID: f.Product.ID, ProductModelID: f.ProductModel.ID, DeviceNo: prefix + "-device", Status: enums.StatusOk}
	require.NoError(t, repositories.DeviceRepository.Create(sqls.DB(), f.Device))

	f.ServiceCode = &models.ServiceCode{TenantID: f.Tenant.ID, ServiceCode: prefix + "-sc", DeviceID: f.Device.ID, ProductID: f.Product.ID, ProductModelID: f.ProductModel.ID, Status: enums.ServiceCodeStatusActive}
	require.NoError(t, repositories.ServiceCodeRepository.Create(sqls.DB(), f.ServiceCode))

	customer := &models.Customer{
		Name:   prefix + "-customer",
		Status: enums.StatusOk,
		AuditFields: models.AuditFields{
			CreatedAt: now, CreateUserID: user.ID, CreateUserName: user.Username,
			UpdatedAt: now, UpdateUserID: user.ID, UpdateUserName: user.Username,
		},
	}
	require.NoError(t, repositories.CustomerRepository.Create(sqls.DB(), customer))
	f.CustomerID = customer.ID

	return f
}

func createTicketIntegrationEngineer(t *testing.T, f *ticketIntegrationFixture, prefix string) *dto.AuthPrincipal {
	t.Helper()
	if f == nil || f.Tenant == nil || f.Product == nil {
		t.Fatal("ticket integration fixture is incomplete")
	}
	operator := createTestOperator(t, prefix)
	operator.TenantID = f.Tenant.ID
	operator.Roles = []string{services.EnterpriseRoleEngineer}
	team := repositories.AgentTeamRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("tenant_id", f.Tenant.ID).
		Eq("product_id", f.Product.ID).
		Eq("team_type", services.AgentTeamTypeProductRepair).
		NotEq("status", enums.StatusDeleted))
	if team == nil {
		team = &models.AgentTeam{
			TenantID:  f.Tenant.ID,
			ProductID: f.Product.ID,
			Name:      prefix + "-repair-team",
			TeamType:  services.AgentTeamTypeProductRepair,
			Status:    enums.StatusOk,
		}
		require.NoError(t, repositories.AgentTeamRepository.Create(sqls.DB(), team))
	}
	now := time.Now()
	require.NoError(t, sqls.DB().Create(&models.AgentProfile{
		TenantID:           f.Tenant.ID,
		UserID:             operator.UserID,
		TeamID:             team.ID,
		AgentCode:          fmt.Sprintf("%s-eng-%d", prefix, operator.UserID),
		DisplayName:        prefix + "-engineer",
		ServiceStatus:      enums.ServiceStatusIdle,
		MaxConcurrentCount: 5,
		AutoAssignEnabled:  true,
		LastOnlineAt:       &now,
		Status:             enums.StatusOk,
		AuditFields:        models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error)
	require.NoError(t, sqls.DB().Create(&models.AgentTeamMember{
		TenantID:        f.Tenant.ID,
		TeamID:          team.ID,
		UserID:          operator.UserID,
		DispatchEnabled: true,
		DispatchWeight:  1,
		Status:          enums.StatusOk,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error)
	require.NoError(t, sqls.DB().Create(&models.AgentWorkStatus{
		TenantID:        f.Tenant.ID,
		UserID:          operator.UserID,
		Status:          services.AgentWorkStatusAvailable,
		ConfirmedAt:     now,
		StatusChangedAt: now,
		AuditFields:     models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error)
	return operator
}

// ---- TestTicketLifecycle ----
// 工单从创建到关闭的完整生命周期测试

func TestTicketLifecycle(t *testing.T) {
	setupTicketIntegrationDB(t)
	f := createTicketIntegrationFixture(t, "lifecycle")

	// 注入一个已配置的 Jitsi 客户端，使测试环境可生成会议 token
	providers.DefaultJitsiClient = providers.NewJitsiClient(&config.JitsiConfig{AppID: "test-app", AppSecret: "test-secret"})
	providers.DefaultJitsiProvider = providers.DefaultJitsiClient

	// 1. 创建工单（创建者即当前受理人）
	ensureTestProductRepairEngineer(t, f.Tenant.ID, f.Product.ID, f.Operator.UserID)
	created, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title:             "设备无法正常启动",
		Description:       "客户反馈按下开机键后无任何响应",
		TenantID:          f.Tenant.ID,
		ProductID:         f.Product.ID,
		ProductModelID:    f.ProductModel.ID,
		DeviceID:          f.Device.ID,
		ServiceCodeID:     f.ServiceCode.ID,
		CustomerID:        f.CustomerID,
		CurrentAssigneeID: f.Operator.UserID,
		ServiceRegion:     "CN",
		FaultCode:         "PWR-001",
		SymptomSummary:    "按下开机键后无响应",
	}, f.Operator)
	require.NoError(t, err)
	waitTicketIntegrationEvents()
	assert.NotEmpty(t, created.TicketNo, "工单号应自动生成")
	assert.Contains(t, created.TicketNo, "TK", "工单号应以TK开头")
	assert.Equal(t, enums.TicketStatusPendingAssigneeAccept, created.Status, "初始状态应为 pending_assignee_accept")
	assert.Equal(t, f.Operator.UserID, created.CurrentAssigneeID, "创建者应为当前受理人")

	// 验证时间线记录（progress）
	progressList := services.TicketProgressService.Find(sqls.NewCnd().Eq("ticket_id", created.ID))
	assert.Len(t, progressList, 1, "创建工单应产生一条progress记录")
	assert.Equal(t, "Created ticket", progressList[0].Content)

	// 2. 受理工单（走 lifecycle，写时间线）
	require.NoError(t, services.TicketLifecycleService.Accept(created.ID, 0, f.Operator))
	accepted := services.TicketService.Get(created.ID)
	require.NotNil(t, accepted)
	assert.Equal(t, enums.TicketStatusProcessing, accepted.Status)

	// 验证受理后的progress
	progressList = services.TicketProgressService.Find(sqls.NewCnd().Eq("ticket_id", created.ID).Asc("id"))
	assert.Len(t, progressList, 2, "受理工单应产生第二条progress记录")

	// 3. 派单（分配）
	nextAssignee := createTicketIntegrationEngineer(t, f, "lifecycle-assignee")
	require.NoError(t, services.TicketService.AssignTicket(request.AssignTicketRequest{
		TicketID: created.ID,
		ToUserID: nextAssignee.UserID,
		Reason:   "需要二线工程师跟进硬件问题",
	}, f.Operator))
	waitTicketIntegrationEvents()
	assigned := services.TicketService.Get(created.ID)
	require.NotNil(t, assigned)
	assert.Equal(t, nextAssignee.UserID, assigned.CurrentAssigneeID)
	assert.Equal(t, enums.TicketStatusPendingAssigneeAccept, assigned.Status)

	// 验证派单记录
	assignProgress := services.TicketProgressService.Find(sqls.NewCnd().Eq("ticket_id", created.ID).Asc("id"))
	assert.Len(t, assignProgress, 3)
	lastProgress := assignProgress[len(assignProgress)-1]
	assert.Contains(t, lastProgress.Content, "分配工单")
	assert.Contains(t, lastProgress.Content, "需要二线工程师跟进硬件问题")

	// 4. 工程师接单后处理（添加维修记录）
	require.NoError(t, services.TicketLifecycleService.Accept(created.ID, nextAssignee.UserID, nextAssignee))

	progress, err := services.TicketService.AddProgress(request.CreateTicketProgressRequest{
		TicketID: created.ID,
		Content:  "已检查电源模块，发现电源指示灯不亮，怀疑电源板故障",
	}, nextAssignee)
	require.NoError(t, err)
	assert.NotZero(t, progress.ID)
	assert.Equal(t, nextAssignee.UserID, progress.AuthorID)

	// 5. 创建会议（视频支持）
	_, err = services.MeetingService.CreateMeetingRoomForOperator(nil, created.ID, f.Operator)
	require.Error(t, err, "非当前处理人不得创建视频协作")
	meetingConfig, err := services.MeetingService.CreateMeetingRoomForOperator(nil, created.ID, nextAssignee)
	require.NoError(t, err, "创建视频协作应成功")
	retriedMeetingConfig, err := services.MeetingService.CreateMeetingRoom(
		nil,
		fmt.Sprintf("%d", created.ID),
		fmt.Sprintf("%d", nextAssignee.UserID),
		nextAssignee.Username,
		f.Tenant.ID,
	)
	require.NoError(t, err, "重复创建视频协作应复用活动协作")
	assert.Equal(t, meetingConfig.MeetingID, retriedMeetingConfig.MeetingID)
	var openMeetingCount int64
	require.NoError(t, sqls.DB().Model(&models.MeetingRoomJitsi{}).
		Where("tenant_id = ? AND ticket_id = ? AND status IN ?", f.Tenant.ID, fmt.Sprintf("%d", created.ID), []string{"waiting", "scheduled", "active"}).
		Count(&openMeetingCount).Error)
	assert.EqualValues(t, 1, openMeetingCount)
	var meetingStartedProgressCount int64
	require.NoError(t, sqls.DB().Model(&models.TicketProgress{}).
		Where("ticket_id = ? AND content LIKE ?", created.ID, "发起视频协作%").
		Count(&meetingStartedProgressCount).Error)
	assert.EqualValues(t, 1, meetingStartedProgressCount)
	_, err = services.MeetingService.JoinMeeting(nil, meetingConfig.MeetingID, fmt.Sprintf("%d", nextAssignee.UserID), nextAssignee.Username, "member", f.Tenant.ID)
	require.NoError(t, err, "工程师加入视频协作应成功")
	var participantCount int64
	require.NoError(t, sqls.DB().Model(&models.MeetingParticipant{}).
		Where("meeting_id = ? AND user_id = ?", meetingConfig.MeetingID, fmt.Sprintf("%d", nextAssignee.UserID)).
		Count(&participantCount).Error)
	assert.EqualValues(t, 1, participantCount, "创建后立即入会不应重复记录主持人")
	now := time.Now()
	collaboration := &models.TicketSupplierCollaboration{
		TenantID: f.Tenant.ID, TicketID: created.ID, ProductID: f.Product.ID, ProductModuleID: 1,
		PartnerCompanyID: 1, PartnerAccountID: 42, Status: services.SupplierCollaborationProcessing,
		VisibilityJSON: `[]`, InvitedAt: now, RecordStatus: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, repositories.TicketSupplierCollaborationRepository.Create(sqls.DB(), collaboration))
	authorization := &models.PartnerAuthorizationScope{
		TenantID: f.Tenant.ID, PartnerAccountID: 42, ResourceType: "ticket", ResourceID: fmt.Sprint(created.ID),
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, repositories.TicketSupplierCollaborationRepository.CreateAuthorizationScope(sqls.DB(), authorization))

	// 6. 保存维修记录后关闭工单（关闭必须走 TicketLifecycleService.Close 且需有维修记录）
	_, err = services.TicketService.CreateRepairRecord(request.CreateTicketRepairRecordRequest{
		TicketID:     created.ID,
		Conclusion:   "更换电源板后设备正常启动",
		RootCause:    "电源板故障",
		RepairMethod: "更换电源板",
	}, nextAssignee)
	require.NoError(t, err)
	_, err = services.TicketService.Transition(request.TransitionTicketRequest{
		TicketID: created.ID,
		Status:   string(enums.TicketStatusResolved),
		Remark:   "维修记录已提交",
	}, nextAssignee)
	require.NoError(t, err)
	require.NoError(t, services.TicketLifecycleService.Close(created.ID, "更换电源板后设备正常启动", nextAssignee))
	done := services.TicketService.Get(created.ID)
	require.NotNil(t, done)
	assert.Equal(t, enums.TicketStatusClosed, done.Status)
	assert.NotNil(t, done.HandledAt, "关闭时 handled_at 应设置")
	var endedMeeting models.MeetingRoomJitsi
	require.NoError(t, sqls.DB().First(&endedMeeting, "id = ?", meetingConfig.MeetingID).Error)
	assert.Equal(t, "ended", endedMeeting.Status)
	assert.NotNil(t, endedMeeting.EndedAt)
	_, err = services.MeetingService.CreateMeetingRoomForOperator(nil, created.ID, nextAssignee)
	require.ErrorContains(t, err, "工单已结束，不能发起视频协作")
	_, err = services.MeetingService.CreateMeetingRoom(
		nil,
		fmt.Sprintf("%d", created.ID),
		fmt.Sprintf("%d", nextAssignee.UserID),
		nextAssignee.Username,
		f.Tenant.ID,
	)
	require.ErrorContains(t, err, "工单已结束，不能发起视频协作")
	var leftParticipant models.MeetingParticipant
	require.NoError(t, sqls.DB().First(&leftParticipant, "meeting_id = ?", meetingConfig.MeetingID).Error)
	assert.NotNil(t, leftParticipant.LeftAt)
	closedCollaboration := repositories.TicketSupplierCollaborationRepository.Get(sqls.DB(), collaboration.ID)
	require.NotNil(t, closedCollaboration)
	assert.Equal(t, services.SupplierCollaborationResolved, closedCollaboration.Status)
	assert.NotNil(t, closedCollaboration.ResolvedAt)
	var revokedScope models.PartnerAuthorizationScope
	require.NoError(t, sqls.DB().First(&revokedScope, authorization.ID).Error)
	assert.Equal(t, enums.StatusDisabled, revokedScope.Status)
}

func TestMeetingServiceConcurrentCreateReusesActiveTicketMeeting(t *testing.T) {
	setupTicketIntegrationDB(t)
	rawDB, err := sqls.DB().DB()
	require.NoError(t, err)
	rawDB.SetMaxOpenConns(1)
	f := createTicketIntegrationFixture(t, "meeting-concurrent")
	ensureTestProductRepairEngineer(t, f.Tenant.ID, f.Product.ID, f.Operator.UserID)

	providers.DefaultJitsiClient = providers.NewJitsiClient(&config.JitsiConfig{AppID: "test-app", AppSecret: "test-secret"})
	providers.DefaultJitsiProvider = providers.DefaultJitsiClient
	ticket, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title:             "并发视频协作",
		Description:       "验证重复创建会议只产生一个活动会议",
		TenantID:          f.Tenant.ID,
		ProductID:         f.Product.ID,
		ProductModelID:    f.ProductModel.ID,
		DeviceID:          f.Device.ID,
		ServiceCodeID:     f.ServiceCode.ID,
		CustomerID:        f.CustomerID,
		CurrentAssigneeID: f.Operator.UserID,
	}, f.Operator)
	require.NoError(t, err)
	require.NoError(t, services.TicketLifecycleService.Accept(ticket.ID, 0, f.Operator))

	const requestCount = 4
	configs := make([]*services.JoinConfig, requestCount)
	errs := make([]error, requestCount)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			configs[index], errs[index] = services.MeetingService.CreateMeetingRoomForOperator(nil, ticket.ID, f.Operator)
		}(i)
	}
	close(start)
	wg.Wait()

	for i := range errs {
		require.NoError(t, errs[i])
		require.NotNil(t, configs[i])
		assert.Equal(t, configs[0].MeetingID, configs[i].MeetingID)
	}
	var openMeetingCount int64
	require.NoError(t, sqls.DB().Model(&models.MeetingRoomJitsi{}).
		Where("tenant_id = ? AND ticket_id = ? AND status IN ?", f.Tenant.ID, fmt.Sprint(ticket.ID), []string{"waiting", "scheduled", "active"}).
		Count(&openMeetingCount).Error)
	assert.EqualValues(t, 1, openMeetingCount)
	var createdMeeting models.MeetingRoomJitsi
	require.NoError(t, sqls.DB().First(&createdMeeting, "id = ?", configs[0].MeetingID).Error)
	assert.Equal(t, "waiting", createdMeeting.Status)
	assert.Nil(t, createdMeeting.StartedAt, "issuing a join token must not start meeting attendance")
	var progressCount int64
	require.NoError(t, sqls.DB().Model(&models.TicketProgress{}).
		Where("ticket_id = ? AND content LIKE ?", ticket.ID, "发起视频协作%").
		Count(&progressCount).Error)
	assert.EqualValues(t, 1, progressCount)
	var participantCount int64
	require.NoError(t, sqls.DB().Model(&models.MeetingParticipant{}).
		Where("meeting_id = ? AND user_id = ? AND user_type = ?", configs[0].MeetingID, fmt.Sprint(f.Operator.UserID), "enterprise").
		Count(&participantCount).Error)
	assert.EqualValues(t, 1, participantCount)
}

func TestReopenedTicketAcceptanceActivatesConversationForHumanReply(t *testing.T) {
	setupTicketIntegrationDB(t)
	f := createTicketIntegrationFixture(t, "ticket-conversation-reopen")
	ensureTestProductRepairEngineer(t, f.Tenant.ID, f.Product.ID, f.Operator.UserID)
	now := time.Now()
	conversation := &models.Conversation{
		TenantID:          f.Tenant.ID,
		ProductID:         f.Product.ID,
		ProductModelID:    f.ProductModel.ID,
		DeviceID:          f.Device.ID,
		ServiceCodeID:     f.ServiceCode.ID,
		Status:            enums.IMConversationStatusActive,
		ServiceMode:       enums.IMConversationServiceModeAIFirst,
		CurrentAssigneeID: f.Operator.UserID,
		LastMessageAt:     now,
		LastActiveAt:      now,
		AuditFields:       models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, repositories.ConversationRepository.Create(sqls.DB(), conversation))
	ticket := &models.Ticket{
		TicketNo:          fmt.Sprintf("TK-CONV-%d", now.UnixNano()),
		Title:             "客户需要人工确认",
		TenantID:          f.Tenant.ID,
		ProductID:         f.Product.ID,
		ProductModelID:    f.ProductModel.ID,
		DeviceID:          f.Device.ID,
		ServiceCodeID:     f.ServiceCode.ID,
		ConversationID:    conversation.ID,
		CurrentAssigneeID: f.Operator.UserID,
		FaultCode:         "customer-confirmed",
		Status:            enums.TicketStatusResolved,
		AuditFields:       models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, repositories.TicketRepository.Create(sqls.DB(), ticket))
	require.NoError(t, sqls.DB().Create(&models.TicketRepairRecord{
		TenantID: ticket.TenantID, TicketID: ticket.ID, ProductID: ticket.ProductID,
		Conclusion: "问题已解决", RootCause: "配置异常", Solution: "恢复配置",
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error)

	require.NoError(t, services.TicketLifecycleService.Close(ticket.ID, "客户确认问题已解决", f.Operator))
	eventbus.WaitAsync[events.TicketClosedEvent]()
	closedConversation := repositories.ConversationRepository.Get(sqls.DB(), conversation.ID)
	require.NotNil(t, closedConversation)
	assert.Equal(t, enums.IMConversationStatusClosed, closedConversation.Status)
	assert.NotNil(t, closedConversation.ClosedAt)

	require.NoError(t, services.TicketLifecycleService.Reopen(ticket.ID, "需要工程师继续答复", f.Operator))
	eventbus.WaitAsync[events.TicketReopenedEvent]()
	reopenedConversation := repositories.ConversationRepository.Get(sqls.DB(), conversation.ID)
	require.NotNil(t, reopenedConversation)
	assert.Equal(t, enums.IMConversationStatusPending, reopenedConversation.Status)
	assert.Zero(t, reopenedConversation.CurrentAssigneeID)

	require.NoError(t, services.TicketLifecycleService.Accept(ticket.ID, f.Operator.UserID, f.Operator))
	activeConversation := repositories.ConversationRepository.Get(sqls.DB(), conversation.ID)
	require.NotNil(t, activeConversation)
	assert.Equal(t, enums.IMConversationStatusActive, activeConversation.Status)
	assert.Equal(t, f.Operator.UserID, activeConversation.CurrentAssigneeID)

	message, err := services.MessageService.SendAgentMessage(conversation.ID, 0, "human-reply", enums.IMMessageTypeText, "工程师人工回复", "", f.Operator)
	require.NoError(t, err)
	require.NotNil(t, message)
	visible, _, _ := services.MessageService.FindCustomerVisibleByConversationIDCursor(conversation.ID, 0, 20, "", "")
	require.Len(t, visible, 2)
	assert.Equal(t, enums.IMSenderTypeAgent, visible[0].SenderType)
	assert.Contains(t, visible[0].Content, "需要工程师继续答复")
	assert.Contains(t, visible[0].Payload, "ticket_reopen")
	assert.Equal(t, enums.IMSenderTypeAgent, visible[1].SenderType)
	assert.Equal(t, "工程师人工回复", visible[1].Content)
}

func TestCancelAssignedTicketClosesLinkedConversationIdempotently(t *testing.T) {
	setupTicketIntegrationDB(t)
	f := createTicketIntegrationFixture(t, "ticket-cancel")
	now := time.Now()
	conversation := &models.Conversation{
		TenantID: f.Tenant.ID, ProductID: f.Product.ID, Status: enums.IMConversationStatusPending,
		CurrentAssigneeID: f.Operator.UserID, LastMessageAt: now, LastActiveAt: now,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, repositories.ConversationRepository.Create(sqls.DB(), conversation))
	ticket := &models.Ticket{
		TicketNo: fmt.Sprintf("TK-CANCEL-%d", now.UnixNano()), Title: "误建的重复工单",
		TenantID: f.Tenant.ID, ProductID: f.Product.ID, ConversationID: conversation.ID,
		CurrentAssigneeID: f.Operator.UserID, Status: enums.TicketStatusAssigned,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, repositories.TicketRepository.Create(sqls.DB(), ticket))

	require.Error(t, services.TicketLifecycleService.Cancel(ticket.ID, " ", f.Operator))
	require.NoError(t, services.TicketLifecycleService.Cancel(ticket.ID, "否定意图误判导致误建", f.Operator))
	require.NoError(t, services.TicketLifecycleService.Cancel(ticket.ID, "网络重试", f.Operator))
	require.NoError(t, services.TicketService.SyncConversationDispatchTx(
		sqls.DB(), conversation.ID, 999, f.Operator.UserID+999, "迟到的自动派单", f.Operator,
	))

	cancelled := repositories.TicketRepository.Get(sqls.DB(), ticket.ID)
	require.NotNil(t, cancelled)
	assert.Equal(t, enums.TicketStatusCancelled, cancelled.Status)
	assert.Zero(t, cancelled.CurrentTeamID)
	assert.Equal(t, f.Operator.UserID, cancelled.CurrentAssigneeID)
	assert.NotNil(t, cancelled.HandledAt)
	closedConversation := repositories.ConversationRepository.Get(sqls.DB(), conversation.ID)
	require.NotNil(t, closedConversation)
	assert.Equal(t, enums.IMConversationStatusClosed, closedConversation.Status)
	assert.Equal(t, "关联工单已取消：否定意图误判导致误建", closedConversation.CloseReason)
	progress := repositories.TicketProgressRepository.Find(sqls.DB(), sqls.NewCnd().Eq("ticket_id", ticket.ID))
	require.Len(t, progress, 1)
	assert.Contains(t, progress[0].Content, "取消工单")
}

func TestMeetingEndIsIdempotentAndClosesParticipants(t *testing.T) {
	setupTicketIntegrationDB(t)
	f := createTicketIntegrationFixture(t, "meeting-end")
	now := time.Now()
	ticket := &models.Ticket{
		TicketNo:          fmt.Sprintf("TK-MEETING-%d", now.UnixNano()),
		Title:             "视频协作状态回写",
		TenantID:          f.Tenant.ID,
		ProductID:         f.Product.ID,
		ProductModelID:    f.ProductModel.ID,
		DeviceID:          f.Device.ID,
		ServiceCodeID:     f.ServiceCode.ID,
		CurrentAssigneeID: f.Operator.UserID,
		Status:            enums.TicketStatusVideoSupport,
		AuditFields:       models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, repositories.TicketRepository.Create(sqls.DB(), ticket))

	meeting, err := services.MeetingService.CreateMeetingRoomForOperator(nil, ticket.ID, f.Operator)
	require.NoError(t, err)
	_, err = services.MeetingService.JoinMeeting(nil, meeting.MeetingID, "customer-1", "客户", "customer", f.Tenant.ID)
	require.NoError(t, err)

	require.NoError(t, services.MeetingService.EndMeeting(nil, meeting.MeetingID, f.Tenant.ID))
	eventbus.WaitAsync[events.MeetingEndedEvent]()
	require.NoError(t, services.MeetingService.EndMeeting(nil, meeting.MeetingID, f.Tenant.ID), "重复结束会议应幂等")

	var ended models.MeetingRoomJitsi
	require.NoError(t, sqls.DB().First(&ended, "id = ?", meeting.MeetingID).Error)
	assert.Equal(t, "ended", ended.Status)
	assert.NotNil(t, ended.EndedAt)
	var activeParticipants int64
	require.NoError(t, sqls.DB().Model(&models.MeetingParticipant{}).
		Where("meeting_id = ? AND left_at IS NULL", meeting.MeetingID).
		Count(&activeParticipants).Error)
	assert.Zero(t, activeParticipants)
	currentTicket := repositories.TicketRepository.Get(sqls.DB(), ticket.ID)
	require.NotNil(t, currentTicket)
	assert.Equal(t, enums.TicketStatusProcessing, currentTicket.Status)
	var endedProgressCount int64
	require.NoError(t, sqls.DB().Model(&models.TicketProgress{}).
		Where("ticket_id = ? AND event_type = ?", ticket.ID, enums.TicketProgressEventMeetingEnded).
		Count(&endedProgressCount).Error)
	assert.EqualValues(t, 1, endedProgressCount)
	var transcriptCount int64
	require.NoError(t, sqls.DB().Model(&models.MeetingTranscriptSegment{}).
		Where("tenant_id = ? AND meeting_id = ?", f.Tenant.ID, meeting.MeetingID).
		Count(&transcriptCount).Error)
	assert.EqualValues(t, 1, transcriptCount)
	var closureTranscript models.MeetingTranscriptSegment
	require.NoError(t, sqls.DB().Where("meeting_id = ?", meeting.MeetingID).First(&closureTranscript).Error)
	assert.Equal(t, "system", closureTranscript.Provider)
	assert.Equal(t, "system", closureTranscript.IngestSource)
	assert.Contains(t, closureTranscript.Text, "未收到实时语音转写片段")
}

func TestEndMeetingKeepsExistingSpeechTranscriptAsAuthoritative(t *testing.T) {
	setupTicketIntegrationDB(t)
	f := createTicketIntegrationFixture(t, "meeting-transcript-authoritative")
	providers.DefaultJitsiClient = providers.NewJitsiClient(&config.JitsiConfig{AppID: "test-app", AppSecret: "test-secret"})
	providers.DefaultJitsiProvider = providers.DefaultJitsiClient
	now := time.Now()
	ticket := &models.Ticket{
		Title:             "视频协作已有字幕",
		TenantID:          f.Tenant.ID,
		ProductID:         f.Product.ID,
		ProductModelID:    f.ProductModel.ID,
		DeviceID:          f.Device.ID,
		ServiceCodeID:     f.ServiceCode.ID,
		CurrentAssigneeID: f.Operator.UserID,
		Status:            enums.TicketStatusVideoSupport,
		AuditFields:       models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, repositories.TicketRepository.Create(sqls.DB(), ticket))
	meeting, err := services.MeetingService.CreateMeetingRoomForOperator(nil, ticket.ID, f.Operator)
	require.NoError(t, err)
	_, err = services.MeetingService.JoinMeeting(nil, meeting.MeetingID, "customer-1", "客户", "customer", f.Tenant.ID)
	require.NoError(t, err)
	require.NoError(t, sqls.DB().Create(&models.MeetingTranscriptSegment{
		ID: "authoritative-transcript", TenantID: f.Tenant.ID, MeetingID: meeting.MeetingID,
		ParticipantID: "customer-1", SpeakerName: "客户", Provider: "jitsi_caption",
		ProviderEventID: "speech-1", IngestSource: "jigasi", Language: "zh-CN",
		Text: "设备运行时有异响", IsFinal: true, StartedAtMS: 1000, EndedAtMS: 1800,
		Confidence: 0.93, RawJSON: "{}", BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now},
	}).Error)

	require.NoError(t, services.MeetingService.EndMeeting(nil, meeting.MeetingID, f.Tenant.ID))
	eventbus.WaitAsync[events.MeetingEndedEvent]()

	var transcripts []models.MeetingTranscriptSegment
	require.NoError(t, sqls.DB().Where("meeting_id = ?", meeting.MeetingID).Order("created_at ASC").Find(&transcripts).Error)
	require.Len(t, transcripts, 1)
	assert.Equal(t, "jitsi_caption", transcripts[0].Provider)
	assert.Equal(t, "设备运行时有异响", transcripts[0].Text)
}

func TestExpireStaleMeetingsClosesOnlyOverdueSessions(t *testing.T) {
	setupTicketIntegrationDB(t)
	f := createTicketIntegrationFixture(t, "meeting-expiry")
	staleStartedAt := time.Now().Add(-13 * time.Hour)
	currentStartedAt := time.Now().Add(-time.Hour)
	stale := models.MeetingRoomJitsi{
		ID: "stale-meeting", TenantID: f.Tenant.ID, TicketID: "101", RoomName: "stale-room",
		Status: "active", CreatedBy: fmt.Sprint(f.Operator.UserID), StartedAt: &staleStartedAt,
		BaseModel: models.BaseModel{CreatedAt: staleStartedAt, UpdatedAt: staleStartedAt},
	}
	current := models.MeetingRoomJitsi{
		ID: "current-meeting", TenantID: f.Tenant.ID, TicketID: "102", RoomName: "current-room",
		Status: "active", CreatedBy: fmt.Sprint(f.Operator.UserID), StartedAt: &currentStartedAt,
		BaseModel: models.BaseModel{CreatedAt: currentStartedAt, UpdatedAt: currentStartedAt},
	}
	require.NoError(t, sqls.DB().Create(&stale).Error)
	require.NoError(t, sqls.DB().Create(&current).Error)
	require.NoError(t, sqls.DB().Create(&models.MeetingParticipant{
		ID: "stale-participant", MeetingID: stale.ID, UserID: "customer", UserType: "customer",
		JoinedAt: &staleStartedAt, BaseModel: models.BaseModel{CreatedAt: staleStartedAt, UpdatedAt: staleStartedAt},
	}).Error)

	require.Equal(t, 1, services.MeetingService.ExpireStaleMeetings(10))
	eventbus.WaitAsync[events.MeetingEndedEvent]()

	var expired models.MeetingRoomJitsi
	require.NoError(t, sqls.DB().First(&expired, "id = ?", stale.ID).Error)
	require.Equal(t, "ended", expired.Status)
	require.NotNil(t, expired.EndedAt)
	require.WithinDuration(t, staleStartedAt.Add(12*time.Hour), *expired.EndedAt, time.Second)
	var active models.MeetingRoomJitsi
	require.NoError(t, sqls.DB().First(&active, "id = ?", current.ID).Error)
	require.Equal(t, "active", active.Status)
	var participant models.MeetingParticipant
	require.NoError(t, sqls.DB().First(&participant, "id = ?", "stale-participant").Error)
	require.NotNil(t, participant.LeftAt)
	require.EqualValues(t, 12*time.Hour/time.Second, participant.Duration)
}

// ---- TestTicketSLATracking ----

func TestTicketSLATracking(t *testing.T) {
	setupTicketIntegrationDB(t)
	f := createTicketIntegrationFixture(t, "sla")

	// 1. 创建工单（SLA 截止时间通过 SLADueAt 字段传入）
	slaDue := time.Now().Add(4 * time.Hour)
	created, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title:          "SLA测试工单",
		Description:    "验证SLA跟踪",
		TenantID:       f.Tenant.ID,
		ProductID:      f.Product.ID,
		ProductModelID: f.ProductModel.ID,
		DeviceID:       f.Device.ID,
		ServiceCodeID:  f.ServiceCode.ID,
		CustomerID:     f.CustomerID,
		SLADueAt:       &slaDue,
	}, f.Operator)
	require.NoError(t, err)
	waitTicketIntegrationEvents()
	assert.NotNil(t, created.SLADueAt, "SLA截止时间应被保存")
	assert.WithinDuration(t, slaDue, *created.SLADueAt, time.Second)

	// 2. 暂停 SLA
	ticketIDStr := fmt.Sprintf("%d", created.ID)
	pauseRecord, err := services.SLAService.PauseSLA(ticketIDStr, "waiting_customer")
	require.NoError(t, err)
	assert.NotNil(t, pauseRecord)
	assert.Equal(t, ticketIDStr, pauseRecord.TicketID)
	assert.Equal(t, "waiting_customer", pauseRecord.Reason)
	assert.NotNil(t, pauseRecord.PausedAt)

	// 验证暂停记录
	pausedTicket := services.TicketService.Get(created.ID)
	require.NotNil(t, pausedTicket)

	// 3. 恢复 SLA（Duration 以整秒计，需跨越至少 1 秒才能得到 >0 的暂停时长）
	time.Sleep(1100 * time.Millisecond)
	resumeRecord, err := services.SLAService.ResumeSLA(ticketIDStr)
	require.NoError(t, err)
	assert.NotNil(t, resumeRecord)
	assert.NotNil(t, resumeRecord.ResumedAt)
	assert.Greater(t, resumeRecord.Duration, int64(0), "暂停时长应 > 0")

	// 4. 创建 SLA 策略并手动触发违规检查
	_, err = services.SLAService.CreateSLAPolicy(services.CreateSLAPolicyInput{
		TenantID:          fmt.Sprintf("%d", f.Tenant.ID),
		Name:              "标准SLA",
		Priority:          "p2",
		FRTMinutes:        60,
		AssignmentMinutes: 120,
		ResolutionMinutes: 1440,
	})
	require.NoError(t, err)

	// 触发全局 SLA 违规检查
	violations, err := services.SLAService.CheckSLAViolations()
	require.NoError(t, err)
	_ = violations
}

// ---- TestTicketEscalation ----

func TestTicketEscalation(t *testing.T) {
	setupTicketIntegrationDB(t)
	f := createTicketIntegrationFixture(t, "escalation")

	// 1. 创建升级规则
	rule, err := services.EscalationService.CreateEscalationRule(services.CreateEscalationRuleInput{
		TenantID:    fmt.Sprintf("%d", f.Tenant.ID),
		Name:        "闲置超时升级",
		Priority:    "p2",
		HoursIdle:   1, // 1小时闲置即升级
		TargetLevel: 2,
		NotifyRoles: []string{"role-supervisor"},
		AutoAssign:  true,
	})
	require.NoError(t, err)
	assert.NotNil(t, rule)
	assert.Equal(t, "闲置超时升级", rule.Name)
	assert.Equal(t, 2, rule.TargetLevel)

	// 2. 创建工单并使其闲置超时
	ticket, err := services.TicketService.CreateTicket(request.CreateTicketRequest{
		Title:       "升级测试工单",
		Description: "验证闲置超时升级",
		TenantID:    f.Tenant.ID,
		ProductID:   f.Product.ID,
		CustomerID:  f.CustomerID,
	}, f.Operator)
	require.NoError(t, err)
	waitTicketIntegrationEvents()

	// 强制将工单 updated_at 设置为超过 idle 阈值
	oldTime := time.Now().Add(-2 * time.Hour)
	require.NoError(t, repositories.TicketRepository.Updates(sqls.DB(), ticket.ID, map[string]any{
		"updated_at": oldTime,
	}))

	// 3. 触发升级检查
	escalations, err := services.EscalationService.CheckAndEscalate()
	require.NoError(t, err)

	// 验证是否有针对该工单的升级记录
	found := false
	for _, e := range escalations {
		if e.TicketID == fmt.Sprintf("%d", ticket.ID) {
			found = true
			assert.Equal(t, 2, e.ToLevel)
			assert.Equal(t, "idle_timeout", e.TriggerType)
			assert.Equal(t, "pending", e.Status)
			break
		}
	}
	assert.True(t, found, "工单应产生升级记录")
}
