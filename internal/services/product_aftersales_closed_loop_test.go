package services_test

import (
	"fmt"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/openidentity"
	"remotehelpdesk/internal/pkg/providers"
	"remotehelpdesk/internal/repositories"
	"remotehelpdesk/internal/services"

	"github.com/mlogclub/simple/sqls"
	"github.com/stretchr/testify/require"
)

func TestProductAfterSalesClosedLoop(t *testing.T) {
	setupTicketIntegrationDB(t)
	f := createTicketIntegrationFixture(t, "product-closed-loop")
	now := time.Now()

	previousJitsiClient := providers.DefaultJitsiClient
	previousJitsiProvider := providers.DefaultJitsiProvider
	providers.DefaultJitsiClient = providers.NewJitsiClient(&config.JitsiConfig{AppID: "test-app", AppSecret: "test-secret"})
	providers.DefaultJitsiProvider = providers.DefaultJitsiClient
	t.Cleanup(func() {
		providers.DefaultJitsiClient = previousJitsiClient
		providers.DefaultJitsiProvider = previousJitsiProvider
	})

	team := &models.AgentTeam{
		TenantID: f.Tenant.ID, ProductID: f.Product.ID, TeamType: services.AgentTeamTypeProductRepair,
		Name: f.Product.Name + "维修组", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, sqls.DB().Create(team).Error)
	require.NoError(t, sqls.DB().Create(&models.AgentTeamSchedule{
		TenantID: f.Tenant.ID, TeamID: team.ID, StartAt: now.Add(-time.Hour), EndAt: now.Add(time.Hour),
		Status: enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error)
	require.NoError(t, sqls.DB().Create(&models.AgentProfile{
		TenantID: f.Tenant.ID, UserID: f.Operator.UserID, TeamID: team.ID,
		AgentCode: fmt.Sprintf("CLOSED-LOOP-%d", f.Operator.UserID), DisplayName: "闭环工程师",
		ServiceStatus: enums.ServiceStatusIdle, MaxConcurrentCount: 5, AutoAssignEnabled: true,
		LastOnlineAt: &now,
		Status:       enums.StatusOk, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error)
	require.NoError(t, sqls.DB().Create(&models.AgentTeamMember{
		TenantID: f.Tenant.ID, TeamID: team.ID, UserID: f.Operator.UserID,
		DispatchEnabled: true, DispatchWeight: 1, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error)
	require.NoError(t, sqls.DB().Create(&models.AgentWorkStatus{
		TenantID: f.Tenant.ID, UserID: f.Operator.UserID, Status: services.AgentWorkStatusAvailable,
		ConfirmedAt: now, StatusChangedAt: now, AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error)
	ensureTestEnterpriseWorkTime(t, f.Tenant.ID, now)

	aiAgent := &models.AIAgent{
		TenantID: f.Tenant.ID, ProductID: f.Product.ID, Name: "产品售后机器人",
		ServiceMode: enums.IMConversationServiceModeAIFirst, TeamIDs: fmt.Sprint(team.ID),
		HandoffMode: enums.AIAgentHandoffModeDefaultTeamPool, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, sqls.DB().Create(aiAgent).Error)

	external := openidentity.ExternalUser{
		ExternalSource: enums.ExternalSourceGuest,
		ExternalID:     "closed-loop-visitor",
		ExternalName:   "闭环测试客户",
	}
	require.NoError(t, sqls.DB().Create(&models.CustomerIdentity{
		CustomerID: f.CustomerID, ExternalSource: external.ExternalSource, ExternalID: external.ExternalID,
	}).Error)
	entrySession := &models.CustomerEntrySession{
		TenantID: f.Tenant.ID, ProductID: f.Product.ID, ProductModelID: f.ProductModel.ID,
		EntryType: "service_code", ServiceCodeID: f.ServiceCode.ID, DeviceID: f.Device.ID,
		VisitorID: external.ExternalID, Locale: "zh-CN", State: "active",
		VisitorTokenHash: "closed-loop-token", CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, sqls.DB().Create(entrySession).Error)
	conversation := &models.Conversation{
		AIAgentID: aiAgent.ID, CustomerID: f.CustomerID, CustomerName: external.ExternalName,
		TenantID: f.Tenant.ID, ProductID: f.Product.ID, ProductModelID: f.ProductModel.ID,
		DeviceID: f.Device.ID, ServiceCodeID: f.ServiceCode.ID, CustomerEntrySessionID: entrySession.ID,
		Status: enums.IMConversationStatusAIServing, ServiceMode: enums.IMConversationServiceModeAIFirst,
		LastMessageAt: now, LastActiveAt: now,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, sqls.DB().Create(conversation).Error)

	require.NoError(t, services.ConversationHumanDispatchService.RequestByCustomer(
		conversation.ID, external, "客户点击转人工：设备运行十分钟后自动停机", "closed-loop-handoff",
	))
	// 网络重试不得创建第二张工单或重复转人工提示。
	require.NoError(t, services.ConversationHumanDispatchService.RequestByCustomer(
		conversation.ID, external, "重复点击转人工", "closed-loop-handoff-retry",
	))
	waitTicketIntegrationEvents()
	ticket := repositories.TicketRepository.FindOne(sqls.DB(), sqls.NewCnd().Eq("conversation_id", conversation.ID))
	require.NotNil(t, ticket)
	require.Equal(t, f.Tenant.ID, ticket.TenantID)
	require.Equal(t, f.Product.ID, ticket.ProductID)
	require.Equal(t, f.Device.ID, ticket.DeviceID)
	require.Equal(t, team.ID, ticket.CurrentTeamID)
	require.Equal(t, f.Operator.UserID, ticket.CurrentAssigneeID)
	require.EqualValues(t, 1, repositories.TicketRepository.Count(sqls.DB(), sqls.NewCnd().Eq("conversation_id", conversation.ID)))

	var acceptErr error
	require.Eventually(t, func() bool {
		acceptErr = services.TicketLifecycleService.Accept(ticket.ID, f.Operator.UserID, f.Operator)
		return acceptErr == nil
	}, 2*time.Second, 50*time.Millisecond, "accept handoff ticket: %v", acceptErr)
	conversation = services.ConversationService.Get(conversation.ID)
	require.NotNil(t, conversation)
	require.Equal(t, enums.IMConversationStatusActive, conversation.Status)
	require.Equal(t, f.Operator.UserID, conversation.CurrentAssigneeID)

	engineerMessage, err := services.MessageService.SendAgentMessage(
		conversation.ID, f.Operator.UserID, "closed-loop-engineer-message", enums.IMMessageTypeText,
		"我已接单，请确认停机前控制器是否显示告警。", "", f.Operator,
	)
	require.NoError(t, err)
	require.Equal(t, enums.IMSenderTypeAgent, engineerMessage.SenderType)
	customerMessage, err := services.MessageService.SendCustomerMessage(
		conversation.ID, "closed-loop-customer-message", enums.IMMessageTypeText,
		"控制器显示 PWR-001，复位后仍会停机。", "", external,
	)
	require.NoError(t, err)
	require.Equal(t, enums.IMSenderTypeCustomer, customerMessage.SenderType)

	company := &models.PartnerCompany{
		TenantID: f.Tenant.ID, PartnerNo: "CLOSED-LOOP-SUPPLIER", Name: "电源模块供应商",
		PartnerType: "module_supplier", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, sqls.DB().Create(company).Error)
	partnerUserID := createTestUser(t, "closed-loop-supplier")
	partnerAccount := &models.PartnerAccount{
		TenantID: f.Tenant.ID, PartnerCompanyID: company.ID, UserID: partnerUserID,
		DisplayName: "供应商工程师", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, sqls.DB().Create(partnerAccount).Error)
	module := &models.ProductModule{
		TenantID: f.Tenant.ID, ProductID: f.Product.ID, ModuleCode: "PWR", Name: "电源模块",
		DefaultSupplierID: company.ID, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, repositories.ProductModuleRepository.Create(sqls.DB(), module))

	collaboration, err := services.TicketSupplierCollaborationService.Invite(ticket.ID, dto.TicketSupplierInviteRequest{
		ProductModuleID: module.ID, Reason: "需要原厂确认 PWR-001 电源板故障",
		Visibility: []string{"ticket_summary", "diagnosis", "repair_progress", "meeting"}, AccessDays: 7,
	}, f.Operator)
	require.NoError(t, err)
	require.NotNil(t, collaboration)
	partner := &dto.AuthPrincipal{
		UserID: partnerUserID, TenantID: f.Tenant.ID, DomainType: models.DomainTypePartner,
		SubjectType: models.SubjectTypePartnerAccount, PartnerAccountID: partnerAccount.ID,
		Permissions: []string{constants.PermissionPartnerMemberView.Code, constants.PermissionPartnerMemberUpdate.Code},
	}
	_, err = services.TicketSupplierCollaborationService.Accept(collaboration.Collaboration.ID, partner)
	require.NoError(t, err)
	_, err = services.TicketSupplierCollaborationService.AddPartnerProgress(
		collaboration.Collaboration.ID, "已远程核对，建议更换电源板并复测三十分钟。", partner,
	)
	require.NoError(t, err)
	partnerMessage := repositories.MessageRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("conversation_id", conversation.ID).Eq("sender_type", enums.IMSenderTypePartner).Desc("id"))
	require.NotNil(t, partnerMessage)
	require.Equal(t, partnerUserID, partnerMessage.SenderID)

	meeting, err := services.MeetingService.CreateMeetingRoomForOperator(nil, ticket.ID, f.Operator)
	require.NoError(t, err)
	secondMeeting, err := services.MeetingService.CreateMeetingRoomForOperator(nil, ticket.ID, f.Operator)
	require.NoError(t, err)
	require.Equal(t, meeting.MeetingID, secondMeeting.MeetingID)
	foreignMeeting := &models.MeetingRoomJitsi{
		ID: "closed-loop-foreign-meeting", TenantID: f.Tenant.ID, TicketID: "999999",
		RoomName: "closed-loop-foreign-room", Status: "active",
		BaseModel: models.BaseModel{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, sqls.DB().Create(foreignMeeting).Error)
	_, err = services.TicketSupplierCollaborationService.JoinMeeting(nil, collaboration.Collaboration.ID, foreignMeeting.ID, partner)
	require.Error(t, err, "supplier collaboration must not grant access to another ticket meeting")
	_, err = services.MeetingService.JoinMeeting(nil, meeting.MeetingID, external.ExternalID, external.ExternalName, "customer", f.Tenant.ID)
	require.NoError(t, err)
	_, err = services.TicketSupplierCollaborationService.JoinMeeting(nil, collaboration.Collaboration.ID, meeting.MeetingID, partner)
	require.NoError(t, err)
	_, err = services.TicketSupplierCollaborationService.JoinMeetingForTicket(nil, ticket.ID, meeting.MeetingID, partner)
	require.NoError(t, err, "ticket-scoped supplier meeting join should resolve its collaboration")
	require.NoError(t, services.TicketSupplierCollaborationService.ConfirmMeetingJoined(
		nil, collaboration.Collaboration.ID, meeting.MeetingID, partner,
	))
	// Browser retries must not create another participant row or reset attendance.
	require.NoError(t, services.TicketSupplierCollaborationService.ConfirmMeetingJoined(
		nil, collaboration.Collaboration.ID, meeting.MeetingID, partner,
	))
	require.NoError(t, services.TicketSupplierCollaborationService.HeartbeatMeeting(
		nil, collaboration.Collaboration.ID, meeting.MeetingID, partner,
	))
	var partnerParticipant models.MeetingParticipant
	require.NoError(t, sqls.DB().Where(
		"meeting_id = ? AND user_id = ? AND user_type = ?",
		meeting.MeetingID, fmt.Sprint(partnerUserID), "partner",
	).First(&partnerParticipant).Error)
	require.NotNil(t, partnerParticipant.JoinedAt)
	require.Nil(t, partnerParticipant.LeftAt)
	require.NoError(t, services.TicketSupplierCollaborationService.ConfirmMeetingLeft(
		nil, collaboration.Collaboration.ID, meeting.MeetingID, partner,
	))
	require.NoError(t, services.TicketSupplierCollaborationService.ConfirmMeetingLeft(
		nil, collaboration.Collaboration.ID, meeting.MeetingID, partner,
	))
	require.NoError(t, sqls.DB().First(&partnerParticipant, "id = ?", partnerParticipant.ID).Error)
	require.NotNil(t, partnerParticipant.LeftAt)
	require.NoError(t, services.MeetingService.EndMeeting(nil, meeting.MeetingID, f.Tenant.ID))
	require.NoError(t, services.MeetingService.EndMeeting(nil, meeting.MeetingID, f.Tenant.ID))
	waitTicketIntegrationEvents()

	_, err = services.TicketSupplierCollaborationService.Resolve(
		collaboration.Collaboration.ID, "确认电源板老化，替换后参数恢复正常。", partner,
	)
	require.NoError(t, err)
	_, err = services.TicketSupplierCollaborationService.JoinMeeting(nil, collaboration.Collaboration.ID, meeting.MeetingID, partner)
	require.Error(t, err, "resolved supplier collaboration must not rejoin a meeting")
	meetingStatus, err := services.TicketSupplierCollaborationService.GetMeetingStatus(nil, collaboration.Collaboration.ID, meeting.MeetingID, partner)
	require.NoError(t, err, "resolved supplier collaboration should retain read-only meeting audit access")
	require.Equal(t, "ended", meetingStatus.Status)
	transcripts, err := services.TicketSupplierCollaborationService.ListMeetingTranscripts(collaboration.Collaboration.ID, meeting.MeetingID, partner)
	require.NoError(t, err)
	require.NotEmpty(t, transcripts, "resolved supplier collaboration should retain meeting transcript history")
	ticket = services.TicketService.Get(ticket.ID)
	require.Equal(t, enums.TicketStatusProcessing, ticket.Status)

	_, err = services.TicketService.CreateRepairRecord(request.CreateTicketRepairRecordRequest{
		TicketID: ticket.ID, FaultCode: "PWR-001", Conclusion: "更换电源板后连续运行三十分钟正常",
		Solution: "更换电源板并恢复推荐参数", RootCause: "电源板老化导致供电不稳定",
		RepairMethod: "更换电源板", ServiceMethod: "video_remote", TestResult: "passed",
		WarrantyCovered: true, RemoteResolved: true, VisibleToCustomer: true, MarkResolved: true,
	}, f.Operator)
	require.NoError(t, err)
	ticket = services.TicketService.Get(ticket.ID)
	require.Equal(t, enums.TicketStatusResolved, ticket.Status)
	candidate := repositories.KnowledgeCandidateRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("ticket_id", ticket.ID).Eq("source_type", "ticket_repair"))
	require.NotNil(t, candidate)
	require.Equal(t, f.Product.ID, candidate.ProductID)
	require.Equal(t, "pending", candidate.ReviewStatus)

	customerPrincipal := &dto.AuthPrincipal{
		TenantID: f.Tenant.ID, Username: external.ExternalName,
		DomainType: models.DomainTypeCustomer, SubjectType: models.SubjectTypeTempVisitor,
	}
	feedbackRequest := request.SubmitTicketFeedbackRequest{
		TicketID: ticket.ID, Rating: 5, Tags: []string{"响应及时", "解决专业"}, Comment: "问题已经解决",
	}
	feedback, err := services.CustomerTicketActionService.SubmitFeedback(ticket, 0, feedbackRequest, customerPrincipal)
	require.NoError(t, err)
	feedbackRetry, err := services.CustomerTicketActionService.SubmitFeedback(ticket, 0, feedbackRequest, customerPrincipal)
	require.NoError(t, err)
	require.Equal(t, feedback.ID, feedbackRetry.ID)
	require.NoError(t, services.CustomerTicketActionService.ConfirmResolved(ticket, customerPrincipal))
	waitTicketIntegrationEvents()

	ticket = services.TicketService.Get(ticket.ID)
	require.Equal(t, enums.TicketStatusClosed, ticket.Status)
	conversation = services.ConversationService.Get(conversation.ID)
	require.Equal(t, enums.IMConversationStatusClosed, conversation.Status)
	closedCollaboration := repositories.TicketSupplierCollaborationRepository.Get(sqls.DB(), collaboration.Collaboration.ID)
	require.Equal(t, services.SupplierCollaborationResolved, closedCollaboration.Status)
	var scope models.PartnerAuthorizationScope
	require.NoError(t, sqls.DB().Where(
		"tenant_id = ? AND partner_account_id = ? AND resource_type = ? AND resource_id = ?",
		f.Tenant.ID, partnerAccount.ID, "ticket", fmt.Sprint(ticket.ID),
	).First(&scope).Error)
	require.Equal(t, enums.StatusDisabled, scope.Status)

	knowledgeBase := &models.KnowledgeBase{
		TenantID: f.Tenant.ID, Name: "产品维修知识库", KnowledgeType: "document", Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}
	require.NoError(t, sqls.DB().Create(knowledgeBase).Error)
	require.NoError(t, sqls.DB().Create(&models.ProductServiceProfile{
		TenantID: f.Tenant.ID, ProductID: f.Product.ID, DefaultKnowledgeBaseID: knowledgeBase.ID,
		MeetingEnabled: true, Status: enums.StatusOk,
		AuditFields: models.AuditFields{CreatedAt: now, UpdatedAt: now},
	}).Error)
	candidate = repositories.KnowledgeCandidateRepository.FindOne(sqls.DB(), sqls.NewCnd().
		Eq("ticket_id", ticket.ID).Eq("source_type", "ticket_repair"))
	require.NotNil(t, candidate)
	require.Equal(t, f.Product.ID, candidate.ProductID)
	require.Equal(t, "pending", candidate.ReviewStatus)

	approved, err := services.KnowledgeCandidateReviewService.ApproveCandidate(
		f.Tenant.ID, candidate.ID,
		dto.EnterpriseKnowledgeCandidateApproveRequest{Language: "zh-CN", Visibility: "public", Publish: true},
		f.Operator,
	)
	require.NoError(t, err)
	require.Equal(t, "approved", approved.ReviewStatus)
	var document models.KnowledgeDocument
	require.NoError(t, sqls.DB().Where(
		"tenant_id = ? AND source_type = ? AND source_reference_id = ?",
		f.Tenant.ID, "knowledge_candidate", candidate.ID,
	).First(&document).Error)
	require.Equal(t, "published", document.ReviewStatus)
	var link models.ProductKnowledgeLink
	require.NoError(t, sqls.DB().Where(
		"tenant_id = ? AND product_id = ? AND knowledge_entry_id = ? AND status = ?",
		f.Tenant.ID, f.Product.ID, document.ID, enums.StatusOk,
	).First(&link).Error)
	require.Equal(t, "public", link.Visibility)
	require.Equal(t, "published", link.PublishStatus)

	entries, err := services.EnterpriseKnowledgeService.ListEntries(f.Tenant.ID, services.EnterpriseKnowledgeQuery{
		Page: 1, PageSize: 20, ProductID: f.Product.ID,
	})
	require.NoError(t, err)
	require.Len(t, entries.Items, 1)
	require.Equal(t, "published", entries.Items[0].Status)
}
