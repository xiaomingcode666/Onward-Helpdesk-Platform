package bootstrap

import (
	"remotehelpdesk/internal/handlers/api"
	"remotehelpdesk/internal/handlers/dashboard"
	"remotehelpdesk/internal/handlers/enterprise"
	partnerhandler "remotehelpdesk/internal/handlers/partner"
	"remotehelpdesk/internal/handlers/platform"
	"remotehelpdesk/internal/handlers/third"
	"remotehelpdesk/internal/middleware"
	"remotehelpdesk/internal/pkg/constants"

	"github.com/gin-gonic/gin"
)

func registerApiAuthRoutes(group *gin.RouterGroup) {
	publicLimit := middleware.AuthenticationRateLimitMiddleware()
	group.POST("/login", publicLimit, api.Login)
	group.POST("/verification-code/send", publicLimit, api.AuthVerificationCodeSend)
	group.POST("/password-reset", publicLimit, api.PasswordReset)
	group.POST("/enterprise-registration/register", publicLimit, api.EnterpriseRegistrationRegister)
	group.POST("/customer-registration/verify", publicLimit, api.CustomerRegistrationVerify)
	group.POST("/customer-registration/register", publicLimit, api.CustomerRegistrationRegister)
	group.POST("/portal-invitation/verify", publicLimit, api.PortalInvitationVerify)
	group.POST("/portal-invitation/register", publicLimit, api.PortalInvitationRegister)
	group.POST("/portal-invitation/bind", publicLimit, api.PortalInvitationBind)
	group.GET("/wxwork_callback", publicLimit, api.WxWorkCallback)
	group.POST("/wxwork_exchange", publicLimit, api.WxWorkExchange)
	group.GET("/wxwork_login", publicLimit, api.WxWorkLogin)
	group.GET("/wxwork_qr_login", publicLimit, api.WxWorkQRLogin)
	group.GET("/oidc_callback", publicLimit, api.OIDCCallback)
	group.POST("/oidc_exchange", publicLimit, api.OIDCExchange)
	group.GET("/oidc_login", publicLimit, api.OIDCLogin)

	authenticated := group.Group("",
		middleware.AuthenticatedEndpointPreAuthRateLimitMiddleware(),
		middleware.AuthMiddleware,
		middleware.TenantContextMiddleware(),
		middleware.AuthenticatedSessionRateLimitMiddleware(),
	)
	authenticated.POST("/logout", api.Logout)
	authenticated.GET("/profile", api.Profile)
	authenticated.POST("/profile/update", api.UpdateProfile)
}

func registerApiChannelRoutes(group *gin.RouterGroup) {
	group.Any("/config", api.ChannelAnyConfig)
}

func registerApiCustomerRoutes(group *gin.RouterGroup) {
	group.GET("/service-code/resolve", api.CustomerGetService_code_resolve)
	group.GET("/service-code/qr-image", api.CustomerGetService_code_qr_image)
	group.POST("/session_exchange", api.CustomerPostSession_exchange)
}

func registerApiConversationRoutes(group *gin.RouterGroup) {
	group.GET("/:id", middleware.CustomerConversationReadRateLimitMiddleware(), api.ConversationGetBy)
	group.POST("/close", middleware.CustomerConversationMutationRateLimitMiddleware(), api.ConversationPostClose)
	group.POST("/create_or_match", middleware.CustomerConversationMutationRateLimitMiddleware(), api.ConversationPostCreate_or_match)
	group.POST("/request_human", middleware.CustomerConversationMutationRateLimitMiddleware(), api.ConversationPostRequest_human)
}

func registerApiMessageRoutes(group *gin.RouterGroup) {
	group.GET("/media/:assetId", middleware.CustomerConversationReadRateLimitMiddleware(), api.ConversationMediaGetForCustomer)
	group.Any("/list", middleware.CustomerConversationReadRateLimitMiddleware(), api.MessageAnyList)
	group.POST("/read", middleware.CustomerConversationReadRateLimitMiddleware(), api.MessagePostRead)
	group.POST("/send", middleware.CustomerMessageRateLimitMiddleware(), api.MessagePostSend)
	group.POST("/upload_attachment", middleware.CustomerMessageRateLimitMiddleware(), api.MessagePostUpload_attachment)
	group.POST("/upload_image", middleware.CustomerMessageRateLimitMiddleware(), api.MessagePostUpload_image)
	group.POST("/upload_audio", middleware.CustomerMessageRateLimitMiddleware(), api.MessagePostUpload_audio)
}

func registerDashboardDashboardRoutes(group *gin.RouterGroup) {
	group.GET("/overview", dashboard.DashboardGetOverview)
}

func registerDashboardUserRoutes(group *gin.RouterGroup) {
	group.GET("/:id", dashboard.UserGetBy)
	group.POST("/assign_role", dashboard.UserPostAssign_role)
	group.POST("/change_password", dashboard.UserPostChange_password)
	group.POST("/create", dashboard.UserPostCreate)
	group.POST("/delete", dashboard.UserPostDelete)
	group.Any("/list", dashboard.UserAnyList)
	group.Any("/list_all", dashboard.UserAnyList_all)
	group.POST("/reset_password", dashboard.UserPostReset_password)
	group.POST("/update", dashboard.UserPostUpdate)
	group.POST("/update_status", dashboard.UserPostUpdate_status)
}

func registerDashboardCompanyRoutes(group *gin.RouterGroup) {
	group.GET("/:id", dashboard.CompanyGetBy)
	group.POST("/create", dashboard.CompanyPostCreate)
	group.POST("/delete", dashboard.CompanyPostDelete)
	group.Any("/list", dashboard.CompanyAnyList)
	group.POST("/update", dashboard.CompanyPostUpdate)
	group.POST("/update_status", dashboard.CompanyPostUpdate_status)
}

func registerDashboardProductRoutes(group *gin.RouterGroup) {
	group.GET("/:id", dashboard.ProductGetBy)
	group.POST("/create", dashboard.ProductPostCreate)
	group.POST("/delete", dashboard.ProductPostDelete)
	group.Any("/list", dashboard.ProductAnyList)
	group.GET("/list_all", dashboard.ProductGetList_all)
	group.POST("/update", dashboard.ProductPostUpdate)
	group.POST("/update_status", dashboard.ProductPostUpdate_status)
}

func registerDashboardProductServiceProfileRoutes(group *gin.RouterGroup) {
	group.GET("/:id", dashboard.ProductServiceProfileGetBy)
	group.POST("/create", dashboard.ProductServiceProfilePostCreate)
	group.POST("/delete", dashboard.ProductServiceProfilePostDelete)
	group.Any("/list", dashboard.ProductServiceProfileAnyList)
	group.POST("/update", dashboard.ProductServiceProfilePostUpdate)
	group.POST("/update_status", dashboard.ProductServiceProfilePostUpdate_status)
}

func registerDashboardProductKnowledgeBindingRoutes(group *gin.RouterGroup) {
	group.GET("/:id", dashboard.ProductKnowledgeBindingGetBy)
	group.Any("/list", dashboard.ProductKnowledgeBindingAnyList)
	group.POST("/create", dashboard.ProductKnowledgeBindingPostCreate)
	group.POST("/update", dashboard.ProductKnowledgeBindingPostUpdate)
	group.POST("/delete", dashboard.ProductKnowledgeBindingPostDelete)
	group.POST("/update_status", dashboard.ProductKnowledgeBindingPostUpdate_status)
	group.POST("/resolve", dashboard.ProductKnowledgeBindingPostResolve)
}

func registerDashboardTenantIntegrationConfigRoutes(group *gin.RouterGroup) {
	group.GET("/:id", dashboard.TenantIntegrationConfigGetBy)
	group.POST("/create", dashboard.TenantIntegrationConfigPostCreate)
	group.POST("/delete", dashboard.TenantIntegrationConfigPostDelete)
	group.Any("/list", dashboard.TenantIntegrationConfigList)
	group.POST("/update", dashboard.TenantIntegrationConfigPostUpdate)
	group.POST("/update_status", dashboard.TenantIntegrationConfigPostUpdate_status)
}

func registerDashboardProductAIUsageCredentialRoutes(group *gin.RouterGroup) {
	group.GET("/:id", dashboard.ProductAIUsageCredentialGetBy)
	group.POST("/create", dashboard.ProductAIUsageCredentialPostCreate)
	group.POST("/delete", dashboard.ProductAIUsageCredentialPostDelete)
	group.Any("/list", dashboard.ProductAIUsageCredentialList)
	group.POST("/update", dashboard.ProductAIUsageCredentialPostUpdate)
	group.POST("/update_status", dashboard.ProductAIUsageCredentialPostUpdate_status)
}

func registerDashboardMeetingRoutes(group *gin.RouterGroup) {
	group.POST("/preview-create", dashboard.MeetingPostPreview_create)
	group.POST("/create", dashboard.MeetingPostCreate)
	group.POST("/:id/join", dashboard.MeetingPostJoin)
	group.POST("/:id/end", dashboard.MeetingPostEnd)
	group.GET("/:id/status", dashboard.MeetingGetStatus)
	group.GET("/ticket/:ticketId/meetings", dashboard.MeetingGetByTicket)
}

func registerDashboardProductModelRoutes(group *gin.RouterGroup) {
	group.GET("/:id", dashboard.ProductModelGetBy)
	group.POST("/create", dashboard.ProductModelPostCreate)
	group.POST("/delete", dashboard.ProductModelPostDelete)
	group.Any("/list", dashboard.ProductModelAnyList)
	group.GET("/list_all", dashboard.ProductModelGetList_all)
	group.POST("/update", dashboard.ProductModelPostUpdate)
	group.POST("/update_status", dashboard.ProductModelPostUpdate_status)
}

func registerDashboardDeviceRoutes(group *gin.RouterGroup) {
	group.GET("/:id", dashboard.DeviceGetBy)
	group.POST("/create", dashboard.DevicePostCreate)
	group.POST("/delete", dashboard.DevicePostDelete)
	group.Any("/list", dashboard.DeviceAnyList)
	group.POST("/update", dashboard.DevicePostUpdate)
	group.POST("/update_status", dashboard.DevicePostUpdate_status)
}

func registerDashboardServiceCodeBatchRoutes(group *gin.RouterGroup) {
	group.GET("/:id", dashboard.ServiceCodeBatchGetBy)
	group.POST("/create", dashboard.ServiceCodeBatchPostCreate)
	group.POST("/delete", dashboard.ServiceCodeBatchPostDelete)
	group.Any("/list", dashboard.ServiceCodeBatchAnyList)
	group.POST("/update_status", dashboard.ServiceCodeBatchPostUpdate_status)
}

func registerDashboardServiceCodeRoutes(group *gin.RouterGroup) {
	group.GET("/:id", dashboard.ServiceCodeGetBy)
	group.Any("/list", dashboard.ServiceCodeAnyList)
	group.POST("/generate", dashboard.ServiceCodePostGenerate)
	group.POST("/revoke", dashboard.ServiceCodePostRevoke)
}

func registerDashboardCustomerRoutes(group *gin.RouterGroup) {
	group.GET("/:id", dashboard.CustomerGetBy)
	group.POST("/create", dashboard.CustomerPostCreate)
	group.POST("/delete", dashboard.CustomerPostDelete)
	group.POST("/list", dashboard.CustomerPostList)
	group.POST("/save_profile", dashboard.CustomerPostSave_profile)
	group.POST("/update", dashboard.CustomerPostUpdate)
	group.POST("/update_status", dashboard.CustomerPostUpdate_status)
}

func registerDashboardCustomerContactRoutes(group *gin.RouterGroup) {
	group.POST("/create", dashboard.CustomerContactPostCreate)
	group.POST("/delete", dashboard.CustomerContactPostDelete)
	group.Any("/list", dashboard.CustomerContactAnyList)
	group.POST("/update", dashboard.CustomerContactPostUpdate)
}

func registerDashboardRoleRoutes(group *gin.RouterGroup) {
	group.GET("/:id", dashboard.RoleGetBy)
	group.POST("/assign_permission", dashboard.RolePostAssign_permission)
	group.POST("/create", dashboard.RolePostCreate)
	group.POST("/delete", dashboard.RolePostDelete)
	group.Any("/list", dashboard.RoleAnyList)
	group.GET("/list_all", dashboard.RoleGetList_all)
	group.POST("/update", dashboard.RolePostUpdate)
	group.POST("/update_sort", dashboard.RolePostUpdate_sort)
	group.POST("/update_status", dashboard.RolePostUpdate_status)
}

func registerDashboardPermissionRoutes(group *gin.RouterGroup) {
	group.GET("/:id", dashboard.PermissionGetBy)
	group.Any("/list", dashboard.PermissionAnyList)
}

func registerDashboardSessionRoutes(group *gin.RouterGroup) {
	group.Any("/list", dashboard.SessionAnyList)
	group.POST("/revoke", dashboard.SessionPostRevoke)
	group.POST("/revoke/by/user", dashboard.SessionPostRevokeByUser)
}

func registerDashboardTagRoutes(group *gin.RouterGroup) {
	group.GET("/:id", dashboard.TagGetBy)
	group.POST("/create", dashboard.TagPostCreate)
	group.POST("/delete", dashboard.TagPostDelete)
	group.Any("/list", dashboard.TagAnyList)
	group.GET("/list_all", dashboard.TagGetList_all)
	group.POST("/update", dashboard.TagPostUpdate)
	group.POST("/update_sort", dashboard.TagPostUpdate_sort)
	group.POST("/update_status", dashboard.TagPostUpdate_status)
}

func registerDashboardConversationRoutes(group *gin.RouterGroup) {
	group.GET("/:id", dashboard.ConversationGetBy)
	group.POST("/add_tag", dashboard.ConversationPostAdd_tag)
	group.POST("/assign", dashboard.ConversationPostAssign)
	group.POST("/close", dashboard.ConversationPostClose)
	group.Any("/conversations", dashboard.ConversationAnyConversations)
	group.POST("/dispatch", dashboard.ConversationPostDispatch)
	group.POST("/link_customer", dashboard.ConversationPostLink_customer)
	group.Any("/list", dashboard.ConversationAnyList)
	group.Any("/message_list", dashboard.ConversationAnyMessage_list)
	group.POST("/read", dashboard.ConversationPostRead)
	group.POST("/recall_message", dashboard.ConversationPostRecall_message)
	group.POST("/remove_tag", dashboard.ConversationPostRemove_tag)
	group.POST("/send_message", dashboard.ConversationPostSend_message)
	group.POST("/forward_image", dashboard.ConversationPostForward_image)
	group.POST("/transfer", dashboard.ConversationPostTransfer)
	group.POST("/upload_attachment", dashboard.ConversationPostUpload_attachment)
	group.POST("/upload_image", dashboard.ConversationPostUpload_image)
	group.POST("/upload_audio", dashboard.ConversationPostUpload_audio)
}

func registerDashboardTicketRoutes(group *gin.RouterGroup) {
	group.Any("/repair-history/list", dashboard.TicketAnyRepairHistoryList)
	group.GET("/:id", dashboard.TicketGetBy)
	group.POST("/assign", dashboard.TicketPostAssign)
	group.POST("/change_status", dashboard.TicketPostChange_status)
	group.POST("/create", dashboard.TicketPostCreate)
	group.POST("/create_from_conversation", dashboard.TicketPostCreate_from_conversation)
	group.POST("/delete_view", dashboard.TicketPostDelete_view)
	group.POST("/link_customer", dashboard.TicketPostLink_customer)
	group.Any("/list", dashboard.TicketAnyList)
	group.POST("/progress/create", dashboard.TicketPostProgressCreate)
	group.Any("/progress/list", dashboard.TicketAnyProgressList)
	group.POST("/save_view", dashboard.TicketPostSave_view)
	group.Any("/summary", dashboard.TicketAnySummary)
	group.POST("/update", dashboard.TicketPostUpdate)
	group.Any("/view_list", dashboard.TicketAnyView_list)
	group.POST("/:id/accept", dashboard.TicketPostAccept)
	group.POST("/:id/takeover", dashboard.TicketPostTakeover)
	group.POST("/:id/transfer", dashboard.TicketPostTransfer)
	group.POST("/:id/escalate", dashboard.TicketPostEscalate)
	group.POST("/:id/close", dashboard.TicketPostClose)
	group.POST("/:id/repair", dashboard.TicketPostRepair)
	group.POST("/:id/reopen", dashboard.TicketPostReopen)
}

func registerDashboardTicketAliasRoutes(group *gin.RouterGroup) {
	group.POST("/:id/takeover", dashboard.TicketPostTakeover)
}

func registerDashboardNotificationRoutes(group *gin.RouterGroup) {
	group.Any("/list", dashboard.NotificationAnyList)
	group.POST("/mark_all_read", dashboard.NotificationPostMark_all_read)
	group.POST("/mark_read", dashboard.NotificationPostMark_read)
	group.GET("/unread_count", dashboard.NotificationGetUnread_count)
}

func registerDashboardQuickReplyRoutes(group *gin.RouterGroup) {
	group.GET("/:id", dashboard.QuickReplyGetBy)
	group.POST("/create", dashboard.QuickReplyPostCreate)
	group.POST("/delete", dashboard.QuickReplyPostDelete)
	group.Any("/list", dashboard.QuickReplyAnyList)
	group.GET("/list_all", dashboard.QuickReplyGetList_all)
	group.POST("/update", dashboard.QuickReplyPostUpdate)
}

func registerDashboardChannelRoutes(group *gin.RouterGroup) {
	group.GET("/:id", dashboard.ChannelGetBy)
	group.POST("/create", dashboard.ChannelPostCreate)
	group.POST("/delete", dashboard.ChannelPostDelete)
	group.Any("/list", dashboard.ChannelAnyList)
	group.POST("/reset_user_token_secret", dashboard.ChannelPostReset_user_token_secret)
	group.POST("/update", dashboard.ChannelPostUpdate)
	group.POST("/update_status", dashboard.ChannelPostUpdate_status)
	group.Any("/wxwork/kf/accounts", dashboard.ChannelAnyWxworkKfAccounts)
}

func registerDashboardAgentRoutes(group *gin.RouterGroup) {
	group.GET("/self/briefing", dashboard.AgentSelfGetBriefing)
	group.GET("/self/work-schedule", dashboard.AgentSelfGetWorkSchedule)
	group.POST("/self/leave/create", dashboard.AgentSelfPostLeaveCreate)
	group.POST("/self/leave/cancel", dashboard.AgentSelfPostLeaveCancel)
	group.POST("/self/work-status", dashboard.AgentSelfPostWorkStatus)
	group.GET("/dispatch-candidates", dashboard.AgentGetDispatchCandidates)
	group.GET("/:id", dashboard.AgentGetBy)
	group.POST("/create", dashboard.AgentPostCreate)
	group.POST("/delete", dashboard.AgentPostDelete)
	group.Any("/list", dashboard.AgentAnyList)
	group.GET("/list_all", dashboard.AgentGetList_all)
	group.POST("/update", dashboard.AgentPostUpdate)
}

func registerDashboardAgentTeamRoutes(group *gin.RouterGroup) {
	group.GET("/:id", dashboard.AgentTeamGetBy)
	group.POST("/create", dashboard.AgentTeamPostCreate)
	group.POST("/delete", dashboard.AgentTeamPostDelete)
	group.Any("/list", dashboard.AgentTeamAnyList)
	group.GET("/list_all", dashboard.AgentTeamGetList_all)
	group.POST("/member/delete", dashboard.AgentTeamMemberPostDelete)
	group.POST("/member/upsert", dashboard.AgentTeamMemberPostUpsert)
	group.POST("/update", dashboard.AgentTeamPostUpdate)
}

func registerDashboardAgentTeamScheduleRoutes(group *gin.RouterGroup) {
	group.GET("/template", dashboard.AgentTeamScheduleGetTemplate)
	group.POST("/template/update", dashboard.AgentTeamSchedulePostUpdate_template)
	group.GET("/leave/list", dashboard.AgentTeamScheduleAnyLeaveList)
	group.GET("/member-availability", dashboard.AgentTeamScheduleAnyMemberAvailability)
	group.POST("/leave/review", dashboard.AgentTeamSchedulePostLeaveReview)
	group.GET("/:id", dashboard.AgentTeamScheduleGetBy)
	group.POST("/batch_generate", dashboard.AgentTeamSchedulePostBatch_generate)
	group.POST("/batch_preview", dashboard.AgentTeamSchedulePostBatch_preview)
	group.Any("/calendar", dashboard.AgentTeamScheduleAnyCalendar)
	group.POST("/create", dashboard.AgentTeamSchedulePostCreate)
	group.POST("/delete", dashboard.AgentTeamSchedulePostDelete)
	group.POST("/disable", dashboard.AgentTeamSchedulePostDisable)
	group.Any("/list", dashboard.AgentTeamScheduleAnyList)
	group.POST("/prepare_draft", dashboard.AgentTeamSchedulePostPrepare_draft)
	group.POST("/publish", dashboard.AgentTeamSchedulePostPublish)
	group.POST("/rollback", dashboard.AgentTeamSchedulePostRollback)
	group.POST("/update", dashboard.AgentTeamSchedulePostUpdate)
}

func registerDashboardAIAgentRoutes(group *gin.RouterGroup) {
	group.GET("/capabilities", dashboard.AIAgentGetCapabilities)
	group.Any("/list", dashboard.AIAgentAnyList)
	group.GET("/list_all", dashboard.AIAgentGetList_all)
	group.GET("/:id/workflow", dashboard.AIWorkflowGetByAgent)
	group.POST("/workflow/save", dashboard.AIWorkflowPostSaveAgent)
	group.POST("/workflow/validate", dashboard.AIWorkflowPostValidate)
	group.POST("/workflow/publish", dashboard.AIWorkflowPostPublishAgent)
	group.GET("/:id", dashboard.AIAgentGetBy)
	group.POST("/create", dashboard.AIAgentPostCreate)
	group.POST("/delete", dashboard.AIAgentPostDelete)
	group.POST("/update", dashboard.AIAgentPostUpdate)
	group.POST("/update_sort", dashboard.AIAgentPostUpdate_sort)
	group.POST("/update_status", dashboard.AIAgentPostUpdate_status)
}

func registerDashboardAIWorkflowRoutes(group *gin.RouterGroup) {
	group.GET("/node-spec/list", dashboard.AIWorkflowGetNodeSpecList)
	group.GET("/default-definition", dashboard.AIWorkflowGetDefaultDefinition)
	group.POST("/validate", dashboard.AIWorkflowPostValidate)
	group.Any("/run/list", dashboard.AIWorkflowAnyRunList)
	group.GET("/run/:id", dashboard.AIWorkflowGetRunBy)
	group.Any("/version/list", dashboard.AIWorkflowAnyVersionList)
	group.GET("/version/:id", dashboard.AIWorkflowGetVersionBy)
}

func registerDashboardAIConfigRoutes(group *gin.RouterGroup) {
	group.GET("/speech_runtime", dashboard.SpeechRuntimeGet)
	group.POST("/speech_runtime/update", dashboard.SpeechRuntimePostUpdate)
	group.GET("/:id", dashboard.AIConfigGetBy)
	group.POST("/create", dashboard.AIConfigPostCreate)
	group.POST("/delete", dashboard.AIConfigPostDelete)
	group.Any("/list", dashboard.AIConfigAnyList)
	group.Any("/list_all", dashboard.AIConfigAnyList_all)
	group.POST("/update", dashboard.AIConfigPostUpdate)
	group.POST("/update_sort", dashboard.AIConfigPostUpdateSort)
	group.POST("/update_status", dashboard.AIConfigPostUpdate_status)
}

func registerDashboardAssetRoutes(group *gin.RouterGroup) {
	group.GET("/:id", dashboard.AssetGetBy)
	group.POST("/create", dashboard.AssetPostCreate)
	group.POST("/delete", dashboard.AssetPostDelete)
	group.Any("/list", dashboard.AssetAnyList)
}

func registerDashboardKnowledgeBaseRoutes(group *gin.RouterGroup) {
	group.GET("/:id", dashboard.KnowledgeBaseGetBy)
	group.POST("/create", dashboard.KnowledgeBasePostCreate)
	group.POST("/delete", dashboard.KnowledgeBasePostDelete)
	group.Any("/list", dashboard.KnowledgeBaseAnyList)
	group.Any("/list_all", dashboard.KnowledgeBaseAnyList_all)
	group.POST("/rebuild_index", dashboard.KnowledgeBasePostRebuild_index)
	group.POST("/update", dashboard.KnowledgeBasePostUpdate)
	group.POST("/update_sort", dashboard.KnowledgeBasePostUpdate_sort)
}

func registerDashboardKnowledgeDirectoryRoutes(group *gin.RouterGroup) {
	group.GET("/list_all", dashboard.KnowledgeDirectoryGetList_all)
	group.POST("/create", dashboard.KnowledgeDirectoryPostCreate)
	group.POST("/delete", dashboard.KnowledgeDirectoryPostDelete)
	group.POST("/update", dashboard.KnowledgeDirectoryPostUpdate)
	group.POST("/update_sort", dashboard.KnowledgeDirectoryPostUpdate_sort)
}

func registerDashboardKnowledgeDocumentRoutes(group *gin.RouterGroup) {
	group.GET("/:id", dashboard.KnowledgeDocumentGetBy)
	group.POST("/batch_delete", dashboard.KnowledgeDocumentPostBatch_delete)
	group.POST("/batch_move", dashboard.KnowledgeDocumentPostBatch_move)
	group.POST("/create", dashboard.KnowledgeDocumentPostCreate)
	group.POST("/delete", dashboard.KnowledgeDocumentPostDelete)
	group.Any("/list", dashboard.KnowledgeDocumentAnyList)
	group.POST("/update", dashboard.KnowledgeDocumentPostUpdate)
}

func registerDashboardKnowledgeFAQRoutes(group *gin.RouterGroup) {
	group.GET("/import_template", dashboard.KnowledgeFAQGetImport_template)
	group.GET("/export", dashboard.KnowledgeFAQGetExport)
	group.GET("/:id", dashboard.KnowledgeFAQGetBy)
	group.POST("/batch_delete", dashboard.KnowledgeFAQPostBatch_delete)
	group.POST("/batch_move", dashboard.KnowledgeFAQPostBatch_move)
	group.POST("/create", dashboard.KnowledgeFAQPostCreate)
	group.POST("/delete", dashboard.KnowledgeFAQPostDelete)
	group.POST("/import", dashboard.KnowledgeFAQPostImport)
	group.Any("/list", dashboard.KnowledgeFAQAnyList)
	group.POST("/update", dashboard.KnowledgeFAQPostUpdate)
}

func registerDashboardKnowledgeRetrieveRoutes(group *gin.RouterGroup) {
	group.POST("/build", dashboard.KnowledgeRetrievePostBuild)
	group.POST("/debug/answer", dashboard.KnowledgeRetrievePostDebugAnswer)
	group.POST("/debug/search", dashboard.KnowledgeRetrievePostDebugSearch)
}

func registerDashboardKnowledgeRetrieveLogRoutes(group *gin.RouterGroup) {
	group.GET("/:id", dashboard.KnowledgeRetrieveLogGetBy)
	group.Any("/list", dashboard.KnowledgeRetrieveLogAnyList)
}

func registerDashboardKnowledgeIndexGenerationRoutes(group *gin.RouterGroup) {
	group.GET("/active", dashboard.KnowledgeIndexGenerationGetActive)
	group.Any("/list", dashboard.KnowledgeIndexGenerationAnyList)
	group.POST("/ensure_active", dashboard.KnowledgeIndexGenerationPostEnsureActive)
	group.POST("/prepare", dashboard.KnowledgeIndexGenerationPostPrepare)
	group.POST("/activate", dashboard.KnowledgeIndexGenerationPostActivate)
	group.POST("/backfill", dashboard.KnowledgeIndexGenerationPostBackfill)
}

func registerDashboardSkillDefinitionRoutes(group *gin.RouterGroup) {
	group.GET("/:id", dashboard.SkillDefinitionGetBy)
	group.POST("/create", dashboard.SkillDefinitionPostCreate)
	group.POST("/debug_resume", dashboard.SkillDefinitionPostDebug_resume)
	group.POST("/debug_run", dashboard.SkillDefinitionPostDebug_run)
	group.POST("/delete", dashboard.SkillDefinitionPostDelete)
	group.Any("/list", dashboard.SkillDefinitionAnyList)
	group.GET("/list_all", dashboard.SkillDefinitionGetList_all)
	group.POST("/restore", dashboard.SkillDefinitionPostRestore)
	group.POST("/update", dashboard.SkillDefinitionPostUpdate)
	group.POST("/update_status", dashboard.SkillDefinitionPostUpdate_status)
}

func registerDashboardMCPRoutes(group *gin.RouterGroup) {
	group.POST("/call_tool", dashboard.MCPPostCall_tool)
	group.Any("/catalog", dashboard.MCPAnyCatalog)
	group.Any("/list_servers", dashboard.MCPAnyList_servers)
	group.POST("/list_tools", dashboard.MCPPostList_tools)
	group.POST("/test_connection", dashboard.MCPPostTest_connection)
}

func registerThirdWechatRoutes(group *gin.RouterGroup) {
	group.GET("/callback", third.WechatGetCallback)
	group.POST("/callback", third.WechatPostCallback)
}

func registerThirdJitsiRoutes(group *gin.RouterGroup) {
	group.POST("/webhook", third.JitsiPostWebhook)
	group.GET("/transcription/ws/:accessToken/:connectionId", third.JigasiTranscriptionWebSocket)
	group.POST("/transcription/events/:accessToken", third.JigasiTranscriptionPostEvent)
}

// ------ Enterprise 路由（SLA、升级） ------

func registerEnterpriseSLARoutes(group *gin.RouterGroup) {
	require := middleware.RequirePermissionMiddleware
	group.POST("/sla/policy/create", require(constants.PermissionTicketUpdate), enterprise.SlaPostPolicyCreate)
	group.GET("/sla/policies", require(constants.PermissionTicketView), enterprise.SlaGetPolicies)
	group.PATCH("/sla/policy/:id", require(constants.PermissionTicketUpdate), enterprise.SlaPatchPolicyUpdate)
	group.PATCH("/sla/policy/:id/_toggle", require(constants.PermissionTicketUpdate), enterprise.SlaPatchPolicyToggle)
	group.POST("/sla/pause", require(constants.PermissionTicketChangeStatus), enterprise.SlaPostPause)
	group.POST("/sla/resume", require(constants.PermissionTicketChangeStatus), enterprise.SlaPostResume)
	group.GET("/ticket/:id/sla-timeline", require(constants.PermissionTicketView), enterprise.SlaGetTicketTimeline)
	group.GET("/sla/violations", require(constants.PermissionTicketView), enterprise.SlaGetViolations)
	group.POST("/escalation/rule/create", require(constants.PermissionTicketUpdate), enterprise.SlaPostEscalationRuleCreate)
	group.GET("/escalation/rules", require(constants.PermissionTicketView), enterprise.SlaGetEscalationRules)
}

func registerEnterpriseDiagnosisRoutes(group *gin.RouterGroup) {
	require := middleware.RequirePermissionMiddleware
	requireAI := middleware.RequireTenantAICapabilityMiddleware()
	group.GET("/diagnosis/fault-tree-nodes", require(constants.PermissionProductView), requireAI, enterprise.DiagnosisFaultTreeNodeList)
	group.POST("/diagnosis/fault-tree-nodes", require(constants.PermissionProductUpdate), requireAI, enterprise.DiagnosisFaultTreeNodeCreate)
	group.PATCH("/diagnosis/fault-tree-nodes/:nodeId", require(constants.PermissionProductUpdate), requireAI, enterprise.DiagnosisFaultTreeNodeUpdate)
	group.POST("/diagnosis/start", require(constants.PermissionTicketProgress), requireAI, enterprise.DiagnosisPostStart)
	group.POST("/diagnosis/:id/step", require(constants.PermissionTicketProgress), requireAI, enterprise.DiagnosisPostStep)
	group.GET("/diagnosis/:id/summary", require(constants.PermissionTicketView), requireAI, enterprise.DiagnosisGetSummary)
	group.GET("/ticket/:id/diagnosis", require(constants.PermissionTicketView), requireAI, enterprise.DiagnosisGetByTicket)
}

func registerEnterpriseReportRoutes(group *gin.RouterGroup) {
	require := middleware.RequirePermissionMiddleware
	group.GET("/workbench/overview", enterprise.WorkbenchGetOverview)
	group.GET("/workbench/core", enterprise.WorkbenchGetCore)
	group.GET("/workbench/collaboration", enterprise.WorkbenchGetCollaboration)
	group.GET("/workbench/notifications", enterprise.WorkbenchGetNotifications)
	group.GET("/workbench/resources", enterprise.WorkbenchGetResources)
	group.GET("/workbench/queue", enterprise.WorkbenchGetQueue)
	group.GET("/reminders/poll", require(constants.PermissionTicketView), enterprise.ReminderPoll)
	group.GET("/reports/overview", require(constants.PermissionReportView), enterprise.ReportGetOverview)
	group.GET("/reports/products/:id", require(constants.PermissionReportView), enterprise.ReportGetProductReport)
	group.GET("/reports/trends", require(constants.PermissionReportView), enterprise.ReportGetTrends)
	group.GET("/reports/top-failures", require(constants.PermissionReportView), enterprise.ReportGetTopFailures)
	group.GET("/reports/team-performance", require(constants.PermissionReportView), enterprise.ReportGetTeamPerformance)
	group.GET("/reports/supplier-performance", require(constants.PermissionReportView), enterprise.ReportGetSupplierPerformance)
	group.GET("/reports/export", require(constants.PermissionReportExport), enterprise.ReportGetExport)
}

func registerEnterpriseConversationRoutes(group *gin.RouterGroup) {
	group.POST("/conversations/:id/_translate", middleware.RequirePermissionMiddleware(constants.PermissionConversationView), enterprise.ConversationMessageTranslate)
}

func registerEnterpriseWebhookRoutes(group *gin.RouterGroup) {
	require := middleware.RequirePermissionMiddleware
	group.POST("/webhooks/register", require(constants.PermissionTenantIntegrationConfigCreate), enterprise.WebhookPostRegister)
	group.GET("/webhooks/list", require(constants.PermissionTenantIntegrationConfigView), enterprise.WebhookGetList)
	group.POST("/webhooks/:webhookId/toggle", require(constants.PermissionTenantIntegrationConfigUpdate), enterprise.WebhookPostToggle)
}

func registerEnterpriseProductRoutes(group *gin.RouterGroup) {
	require := middleware.RequirePermissionMiddleware
	requireAI := middleware.RequireTenantAICapabilityMiddleware()
	group.GET("/industry-solution-packs", require(constants.PermissionProductView), enterprise.IndustrySolutionPackList)
	group.POST("/industry-solution-packs/import", require(constants.PermissionProductUpdate), enterprise.IndustrySolutionPackImport)
	group.PUT("/industry-solution-packs/:packId/draft", require(constants.PermissionProductUpdate), enterprise.IndustrySolutionPackDraftUpdate)
	group.POST("/industry-solution-packs/:packId/_validate", require(constants.PermissionProductView), enterprise.IndustrySolutionPackValidate)
	group.POST("/industry-solution-packs/:packId/_publish", require(constants.PermissionProductUpdate), enterprise.IndustrySolutionPackPublish)
	group.POST("/industry-solution-packs/:packId/_dry-run", require(constants.PermissionProductView), enterprise.IndustrySolutionPackDryRun)
	group.POST("/industry-solution-packs/:packId/_apply", require(constants.PermissionProductUpdate), enterprise.IndustrySolutionPackApply)
	group.GET("/industry-solution-packs/applications/:applicationId", require(constants.PermissionProductView), enterprise.IndustrySolutionPackApplicationGet)
	group.GET("/models/capabilities", require(constants.PermissionAIConfigView), requireAI, enterprise.AICapabilities)
	group.PUT("/models/default-llm-model", require(constants.PermissionAIConfigUpdate), requireAI, enterprise.AIModelDefaultLLMUpdate)
	group.GET("/models/workspace", require(constants.PermissionAIConfigView), requireAI, enterprise.AIModelWorkspace)
	group.PUT("/models/keys/:keyId/quota", require(constants.PermissionAIConfigUpdate), requireAI, enterprise.AIModelKeyUpdateQuota)
	group.POST("/models/keys/:keyId/reset-quota", require(constants.PermissionAIConfigUpdate), requireAI, enterprise.AIModelKeyResetQuota)
	group.GET("/ai-agents/summary", require(constants.PermissionAIAgentView), requireAI, enterprise.EnterpriseAIAgentSummary)
	group.GET("/ai-agents", require(constants.PermissionAIAgentView), requireAI, enterprise.EnterpriseAIAgentList)
	group.POST("/ai-agents/_provision-products", require(constants.PermissionAIAgentCreate), requireAI, enterprise.ProductAIAgentsProvision)
	group.POST("/ai-agents/_ensure-tenant-default", require(constants.PermissionAIAgentCreate), requireAI, enterprise.TenantDefaultAIAgentEnsure)
	group.POST("/ai-agents/_ensure-product", require(constants.PermissionAIAgentCreate), requireAI, enterprise.ProductAIAgentEnsure)
	group.POST("/ai-agents/:id/_submit-review", require(constants.PermissionAIAgentUpdate), requireAI, enterprise.ProductAIAgentSubmitReview)
	group.POST("/ai-agents/:id/_approve", require(constants.PermissionAIAgentUpdate), requireAI, enterprise.ProductAIAgentApprove)
	group.POST("/ai-agents/:id/_reject", require(constants.PermissionAIAgentUpdate), requireAI, enterprise.ProductAIAgentReject)
	group.GET("/ai-agents/:id/releases", require(constants.PermissionAIWorkflowView), requireAI, enterprise.AIAgentReleaseList)
	group.POST("/ai-agents/:id/releases", require(constants.PermissionAIAgentReleaseCreate), requireAI, enterprise.AIAgentReleaseCreate)
	group.GET("/ai-agent-releases/:releaseId", require(constants.PermissionAIWorkflowView), requireAI, enterprise.AIAgentReleaseGet)
	group.POST("/ai-agent-releases/:releaseId/_submit-review", require(constants.PermissionAIAgentReleaseCreate), requireAI, enterprise.AIAgentReleaseSubmitReview)
	group.POST("/ai-agent-releases/:releaseId/_approve", require(constants.PermissionAIAgentReleaseReview), requireAI, enterprise.AIAgentReleaseApprove)
	group.POST("/ai-agent-releases/:releaseId/_reject", require(constants.PermissionAIAgentReleaseReview), requireAI, enterprise.AIAgentReleaseReject)
	group.POST("/ai-agent-releases/:releaseId/_deploy", require(constants.PermissionAIAgentReleaseDeploy), requireAI, enterprise.AIAgentReleaseDeploy)
	group.POST("/ai-agent-releases/:releaseId/_rollback", require(constants.PermissionAIAgentReleaseDeploy), requireAI, enterprise.AIAgentReleaseRollback)

	group.GET("/products", require(constants.PermissionProductView), enterprise.ProductList)
	group.GET("/product-count", require(constants.PermissionProductView), enterprise.ProductCount)
	group.POST("/products", require(constants.PermissionProductCreate), enterprise.ProductCreate)
	group.PATCH("/products/:id", require(constants.PermissionProductUpdate), enterprise.ProductUpdate)
	group.GET("/products/:id", require(constants.PermissionProductView), enterprise.ProductGet)
	group.GET("/products/:id/profile", require(constants.PermissionProductView), enterprise.ProductProfile)
	group.GET("/products/:id/resources", require(constants.PermissionProductAIUsageCredentialView), requireAI, enterprise.ProductResources)
	group.POST("/products/:id/resources/_retry", require(constants.PermissionProductAIUsageCredentialUpdate), requireAI, enterprise.ProductResourcesRetry)
	group.PATCH("/products/:id/ai-quota", require(constants.PermissionProductAIUsageCredentialUpdate), requireAI, enterprise.ProductAIQuotaUpdate)
	group.POST("/products/:id/ai-quota/_reset", require(constants.PermissionProductAIUsageCredentialUpdate), requireAI, enterprise.ProductAIQuotaReset)
	group.POST("/products/:id/knowledge-base", require(constants.PermissionKnowledgeBaseCreate), requireAI, enterprise.ProductKnowledgeBaseCreate)
	group.GET("/products/:id/devices", require(constants.PermissionDeviceView), enterprise.ProductDevices)
	group.GET("/products/:id/service-codes", require(constants.PermissionServiceCodeView), enterprise.ProductServiceCodes)
	group.GET("/products/:id/tickets", require(constants.PermissionTicketView), enterprise.ProductTickets)
	group.GET("/products/:id/conversations", require(constants.PermissionConversationView), enterprise.ProductConversations)
	group.GET("/products/:id/repair-history", require(constants.PermissionTicketRepairHistoryView), enterprise.ProductRepairHistory)
	group.GET("/products/:id/knowledge-coverage", require(constants.PermissionKnowledgeBaseView), requireAI, enterprise.ProductKnowledgeCoverage)
	group.GET("/products/:id/quality-signals", require(constants.PermissionProductView), enterprise.ProductQualitySignals)
	group.POST("/products/:id/quality-signals", require(constants.PermissionProductUpdate), enterprise.ProductQualitySignalCreate)
	group.PATCH("/products/:id/quality-signals/:signalId", require(constants.PermissionProductUpdate), enterprise.ProductQualitySignalUpdate)
	group.POST("/products/:id/quality-signals/:signalId/_resolve", require(constants.PermissionProductUpdate), enterprise.ProductQualitySignalResolve)
	group.GET("/products/:id/usage", require(constants.PermissionProductAIUsageCredentialView), requireAI, enterprise.ProductUsage)
	group.GET("/products/:id/models", require(constants.PermissionProductModelView), enterprise.ProductModels)
	group.POST("/products/:id/models", require(constants.PermissionProductModelCreate), enterprise.ProductModelCreate)
	group.PATCH("/products/:id/models/:modelId", require(constants.PermissionProductModelUpdate), enterprise.ProductModelUpdate)
	group.POST("/products/:id/models/:modelId/_enable", require(constants.PermissionProductModelUpdate), enterprise.ProductModelEnable)
	group.POST("/products/:id/models/:modelId/_disable", require(constants.PermissionProductModelUpdate), enterprise.ProductModelDisable)
	group.GET("/products/:id/modules", require(constants.PermissionProductModelView), enterprise.ProductModules)
	group.POST("/products/:id/modules", require(constants.PermissionProductModelCreate), enterprise.ProductModuleCreate)
	group.PATCH("/products/:id/modules/:moduleId", require(constants.PermissionProductModelUpdate), enterprise.ProductModuleUpdate)
	group.POST("/products/:id/modules/:moduleId/_enable", require(constants.PermissionProductModelUpdate), enterprise.ProductModuleEnable)
	group.POST("/products/:id/modules/:moduleId/_disable", require(constants.PermissionProductModelUpdate), enterprise.ProductModuleDisable)
	group.GET("/products/:id/modules/:moduleId/models", require(constants.PermissionProductModelView), enterprise.ProductModuleModels)
	group.PUT("/products/:id/modules/:moduleId/models", require(constants.PermissionProductModelUpdate), enterprise.ProductModuleModelsReplace)
	group.GET("/products/:id/fault-stats", require(constants.PermissionProductView), enterprise.ProductFaultStats)
	group.POST("/products/:id/fault-stats/_rebuild", require(constants.PermissionProductUpdate), enterprise.ProductFaultStatsRebuild)
	group.GET("/products/:id/fault-stats/jobs/:jobId", require(constants.PermissionProductView), enterprise.ProductFaultStatsJob)
	group.GET("/products/:id/manual-files", require(constants.PermissionProductView), enterprise.ProductManualFiles)
	group.GET("/products/:id/manual-files/:manualFileId/content", require(constants.PermissionProductView), enterprise.ProductManualFileContent)
	group.POST("/products/:id/manual-files/_upload", require(constants.PermissionProductUpdate), enterprise.ProductManualFileUpload)
	group.PATCH("/products/:id/manual-files/:manualFileId", require(constants.PermissionProductUpdate), enterprise.ProductManualFileUpdate)
	group.DELETE("/products/:id/manual-files/:manualFileId", require(constants.PermissionProductUpdate), enterprise.ProductManualFileDelete)
	group.GET("/products/:id/knowledge-documents", require(constants.PermissionKnowledgeDocumentView), requireAI, enterprise.ProductKnowledgeDocuments)
	group.POST("/products/:id/knowledge-documents/_upload", require(constants.PermissionKnowledgeDocumentCreate), requireAI, enterprise.ProductKnowledgeDocumentUpload)
	group.POST("/products/:id/knowledge-documents/:documentId/_reprocess", require(constants.PermissionKnowledgeDocumentUpdate), requireAI, enterprise.ProductKnowledgeDocumentReprocess)
	group.DELETE("/products/:id/knowledge-documents/:documentId", require(constants.PermissionKnowledgeDocumentDelete), requireAI, enterprise.ProductKnowledgeDocumentDelete)
	group.GET("/products/:id/manuals", require(constants.PermissionProductKnowledgeBindingView), requireAI, enterprise.ProductManuals)
	group.POST("/products/:id/manuals/_link", require(constants.PermissionProductKnowledgeBindingCreate), requireAI, enterprise.ProductManualLinkCreate)
	group.PATCH("/products/:id/manuals/:linkId", require(constants.PermissionProductKnowledgeBindingUpdate), requireAI, enterprise.ProductManualUpdate)
	group.DELETE("/products/:id/manuals/:linkId", require(constants.PermissionProductKnowledgeBindingDelete), requireAI, enterprise.ProductManualDelete)
	group.POST("/products/:id/manuals/:linkId/_publish", require(constants.PermissionProductKnowledgeBindingUpdate), requireAI, enterprise.ProductManualPublish)
	group.POST("/products/:id/manuals/:linkId/_deprecate", require(constants.PermissionProductKnowledgeBindingUpdate), requireAI, enterprise.ProductManualDeprecate)
	group.POST("/products/:id/manuals/:linkId/_reindex", require(constants.PermissionProductKnowledgeBindingResolve), requireAI, enterprise.ProductManualReindex)
	group.GET("/products/:id/knowledge-links", require(constants.PermissionProductKnowledgeBindingView), requireAI, enterprise.ProductKnowledgeLinks)
	group.POST("/products/:id/knowledge-links", require(constants.PermissionProductKnowledgeBindingCreate), requireAI, enterprise.ProductKnowledgeLinkCreate)
	group.DELETE("/products/:id/knowledge-links/:linkId", require(constants.PermissionProductKnowledgeBindingDelete), requireAI, enterprise.ProductKnowledgeLinkDelete)
}

func registerEnterpriseAIWorkflowRoutes(group *gin.RouterGroup) {
	require := middleware.RequirePermissionMiddleware
	requireAI := middleware.RequireTenantAICapabilityMiddleware()
	group.GET("/ai-workflows/summary", require(constants.PermissionAIWorkflowView), requireAI, enterprise.AIWorkflowSummary)
	group.GET("/ai-workflows/adoption", require(constants.PermissionAIWorkflowView), requireAI, enterprise.AIWorkflowAdoption)
	group.GET("/ai-workflows", require(constants.PermissionAIWorkflowView), requireAI, enterprise.AIWorkflowList)
	group.GET("/ai-workflows/:id", require(constants.PermissionAIWorkflowView), requireAI, enterprise.AIWorkflowGet)
	group.GET("/ai-workflows/:id/versions", require(constants.PermissionAIWorkflowView), requireAI, enterprise.AIWorkflowVersionList)
	group.POST("/ai-workflows", require(constants.PermissionAIWorkflowUpdate), requireAI, enterprise.AIWorkflowTemplateCreate)
	group.PATCH("/ai-workflows/:id/draft", require(constants.PermissionAIWorkflowUpdate), requireAI, enterprise.AIWorkflowDraftUpdate)
	group.DELETE("/ai-workflows/:id", require(constants.PermissionAIWorkflowUpdate), requireAI, enterprise.AIWorkflowDelete)
	group.POST("/ai-workflows/:id/_test", require(constants.PermissionAIWorkflowUpdate), requireAI, enterprise.AIWorkflowTestRun)
	group.POST("/ai-workflows/:id/_publish", require(constants.PermissionAIWorkflowPublish), requireAI, enterprise.AIWorkflowPublish)
	group.POST("/ai-workflows/:id/versions/:versionId/_rollback", require(constants.PermissionAIWorkflowPublish), requireAI, enterprise.AIWorkflowVersionRollback)
	group.PATCH("/ai-agents/:id/workflow-binding", require(constants.PermissionAIAgentUpdate), requireAI, enterprise.AIAgentWorkflowBindingUpdate)
}

func registerEnterpriseTicketRoutes(group *gin.RouterGroup) {
	require := middleware.RequirePermissionMiddleware
	group.GET("/ticket-settings/intake", require(constants.PermissionTicketCreate), enterprise.TicketIntakePolicyGet)
	group.PUT("/ticket-settings/intake", require(constants.PermissionTicketUpdate), enterprise.TicketIntakePolicyUpdate)
	group.PATCH("/tickets/:id/intake", require(constants.PermissionTicketUpdate), enterprise.TicketIntakeComplete)
	group.GET("/ticket-settings/auto-close", require(constants.PermissionTicketView), enterprise.TicketAutoClosePolicyGet)
	group.PUT("/ticket-settings/auto-close", require(constants.PermissionTicketUpdate), enterprise.TicketAutoClosePolicyUpdate)
	group.GET("/tickets", require(constants.PermissionTicketView), enterprise.TicketList)
	group.POST("/tickets", require(constants.PermissionTicketCreate), enterprise.TicketCreate)
	group.GET("/tickets/customer-options", require(constants.PermissionTicketCreate), enterprise.TicketCustomerOptions)
	group.POST("/tickets/customer-invitation-drafts", require(constants.PermissionTicketCreate), enterprise.TicketCustomerInvitationDraftCreate)
	group.GET("/tickets/summary", require(constants.PermissionTicketView), enterprise.TicketSummary)
	group.GET("/tickets/:id", require(constants.PermissionTicketView), enterprise.TicketGet)
	group.GET("/tickets/:id/actions", require(constants.PermissionTicketView), enterprise.TicketActions)
	group.POST("/tickets/:id/progress", require(constants.PermissionTicketProgress), enterprise.TicketProgressCreate)
	group.POST("/tickets/:id/repair", require(constants.PermissionTicketUpdate), enterprise.TicketRepairCreate)
	group.GET("/tickets/:id/knowledge-candidates", require(constants.PermissionTicketView), enterprise.TicketKnowledgeCandidateList)
	group.POST("/tickets/:id/knowledge-candidates", require(constants.PermissionTicketUpdate), enterprise.TicketKnowledgeCandidateCreate)
	group.POST("/tickets/:id/_accept", require(constants.PermissionTicketChangeStatus), enterprise.TicketAccept)
	group.POST("/tickets/:id/_takeover", require(constants.PermissionTicketChangeStatus), enterprise.TicketTakeover)
	group.POST("/tickets/:id/takeover", require(constants.PermissionTicketChangeStatus), enterprise.TicketTakeover)
	group.POST("/tickets/:id/_assign", require(constants.PermissionTicketAssign), enterprise.TicketAssign)
	group.POST("/tickets/:id/_transfer", require(constants.PermissionTicketAssign), enterprise.TicketTransfer)
	group.POST("/tickets/:id/_close", require(constants.PermissionTicketChangeStatus), enterprise.TicketClose)
	group.POST("/tickets/:id/_cancel", require(constants.PermissionTicketChangeStatus), enterprise.TicketCancel)
	group.POST("/tickets/:id/_reopen", require(constants.PermissionTicketChangeStatus), enterprise.TicketReopen)
	group.GET("/tickets/:id/supplier-options", require(constants.PermissionTicketProgress), enterprise.TicketSupplierOptionList)
	group.GET("/tickets/:id/supplier-collaborations", require(constants.PermissionTicketView), enterprise.TicketSupplierCollaborationList)
	group.POST("/tickets/:id/supplier-collaborations", require(constants.PermissionTicketProgress), enterprise.TicketSupplierCollaborationInvite)
	group.GET("/tickets/:id/supplier-collaborations/:collaborationId", require(constants.PermissionTicketView), enterprise.TicketSupplierCollaborationDetail)
	group.POST("/tickets/:id/supplier-collaborations/:collaborationId/progress", require(constants.PermissionTicketProgress), enterprise.TicketSupplierCollaborationProgress)
	group.POST("/tickets/:id/supplier-collaborations/:collaborationId/_resolve", require(constants.PermissionTicketChangeStatus), enterprise.TicketSupplierCollaborationResolve)
}

func registerPartnerRoutes(group *gin.RouterGroup) {
	require := middleware.RequirePermissionMiddleware
	group.GET("/profile", partnerhandler.Profile)
	group.GET("/accounts", require(constants.PermissionPartnerMemberView), partnerhandler.AccountList)
	group.POST("/accounts", require(constants.PermissionPartnerMemberInvite), partnerhandler.AccountCreate)
	group.PUT("/accounts/:accountId", require(constants.PermissionPartnerMemberUpdate), partnerhandler.AccountUpdate)
	group.GET("/meetings", require(constants.PermissionMeetingView), partnerhandler.MeetingList)
	group.GET("/conversations", require(constants.PermissionTicketView), partnerhandler.ConversationList)
	group.GET("/tickets", require(constants.PermissionTicketView), partnerhandler.TicketList)
	group.GET("/tickets/:id", require(constants.PermissionTicketView), partnerhandler.TicketGet)
	group.POST("/tickets/:id/progress", require(constants.PermissionTicketProgress), partnerhandler.TicketProgressCreate)
	group.POST("/tickets/:id/upload_image", require(constants.PermissionTicketProgress), partnerhandler.TicketUploadImage)
	group.POST("/tickets/:id/upload_audio", require(constants.PermissionTicketProgress), partnerhandler.TicketUploadAudio)
	group.POST("/tickets/:id/upload_attachment", require(constants.PermissionTicketProgress), partnerhandler.TicketUploadAttachment)
	group.POST("/tickets/:id/_accept", require(constants.PermissionTicketChangeStatus), partnerhandler.TicketAccept)
	group.POST("/tickets/:id/_assign", require(constants.PermissionTicketAssign), partnerhandler.TicketAssign)
	group.POST("/tickets/:id/participants", require(constants.PermissionTicketAssign), partnerhandler.TicketParticipantAdd)
	group.POST("/tickets/:id/participants/:accountId/_remove", require(constants.PermissionTicketAssign), partnerhandler.TicketParticipantRemove)
	group.POST("/tickets/:id/_resolve", require(constants.PermissionTicketChangeStatus), partnerhandler.TicketResolve)
	group.GET("/tickets/:id/meetings", require(constants.PermissionMeetingView), partnerhandler.TicketMeetings)
	group.GET("/supplier-collaborations/:id/meetings/:meetingId/join", require(constants.PermissionMeetingView), partnerhandler.MeetingJoin)
	group.GET("/tickets/:id/meetings/:meetingId/join", require(constants.PermissionMeetingView), partnerhandler.MeetingJoinByTicket)
	group.POST("/supplier-collaborations/:id/meetings/:meetingId/_joined", require(constants.PermissionMeetingView), partnerhandler.MeetingJoined)
	group.POST("/supplier-collaborations/:id/meetings/:meetingId/_left", require(constants.PermissionMeetingView), partnerhandler.MeetingLeft)
	group.POST("/supplier-collaborations/:id/meetings/:meetingId/_heartbeat", require(constants.PermissionMeetingView), partnerhandler.MeetingHeartbeat)
	group.GET("/supplier-collaborations/:id/meetings/:meetingId/status", require(constants.PermissionMeetingView), partnerhandler.MeetingStatus)
	group.GET("/supplier-collaborations/:id/meetings/:meetingId/transcripts", require(constants.PermissionMeetingView), partnerhandler.MeetingTranscripts)
	group.GET("/supplier-collaborations/:id/meetings/:meetingId/transcripts/page", require(constants.PermissionMeetingView), partnerhandler.MeetingTranscriptPage)
	group.POST("/supplier-collaborations/:id/meetings/:meetingId/transcripts", require(constants.PermissionMeetingView), partnerhandler.MeetingTranscriptCreate)
}

func registerEnterpriseDeviceRoutes(group *gin.RouterGroup) {
	require := middleware.RequirePermissionMiddleware
	group.GET("/devices", require(constants.PermissionDeviceView), enterprise.DeviceList)
	group.POST("/devices", require(constants.PermissionDeviceCreate), enterprise.DeviceCreate)
	group.POST("/devices/_batch-import", require(constants.PermissionDeviceCreate), enterprise.DeviceBatchImport)
	group.GET("/devices/:id", require(constants.PermissionDeviceView), enterprise.DeviceGet)
	group.POST("/devices/:id", require(constants.PermissionDeviceUpdate), enterprise.DeviceUpdate)
}

func registerEnterpriseServiceCodeRoutes(group *gin.RouterGroup) {
	require := middleware.RequirePermissionMiddleware
	group.GET("/service-codes", require(constants.PermissionServiceCodeView), enterprise.ServiceCodeList)
	group.GET("/service-code-batches", require(constants.PermissionServiceCodeBatchView), enterprise.ServiceCodeBatchList)
}

func registerEnterpriseKnowledgeRoutes(group *gin.RouterGroup) {
	require := middleware.RequirePermissionMiddleware
	requireAI := middleware.RequireTenantAICapabilityMiddleware()
	group.GET("/knowledge-bases/:knowledgeBaseId/documents", require(constants.PermissionKnowledgeDocumentView), requireAI, enterprise.TenantKnowledgeDocuments)
	group.POST("/knowledge-bases/:knowledgeBaseId/documents/_upload", require(constants.PermissionKnowledgeDocumentCreate), requireAI, enterprise.TenantKnowledgeDocumentUpload)
	group.POST("/knowledge-bases/:knowledgeBaseId/documents/:documentId/_reprocess", require(constants.PermissionKnowledgeDocumentUpdate), requireAI, enterprise.TenantKnowledgeDocumentReprocess)
	group.DELETE("/knowledge-bases/:knowledgeBaseId/documents/:documentId", require(constants.PermissionKnowledgeDocumentDelete), requireAI, enterprise.TenantKnowledgeDocumentDelete)
	group.GET("/knowledge/upload-quota", require(constants.PermissionKnowledgeDocumentView), requireAI, enterprise.KnowledgeUploadQuota)
	group.GET("/knowledge/entries", require(constants.PermissionKnowledgeBaseView), requireAI, enterprise.KnowledgeEntryList)
	group.GET("/knowledge/stats", require(constants.PermissionKnowledgeBaseView), requireAI, enterprise.KnowledgeStats)
	group.POST("/knowledge/entries", require(constants.PermissionKnowledgeBaseCreate), requireAI, enterprise.KnowledgeEntryCreate)
	group.GET("/knowledge/entries/:id", require(constants.PermissionKnowledgeBaseView), requireAI, enterprise.KnowledgeEntryGet)
	group.PATCH("/knowledge/entries/:id", require(constants.PermissionKnowledgeBaseUpdate), requireAI, enterprise.KnowledgeEntryUpdate)
	group.POST("/knowledge/entries/:id/_submit", require(constants.PermissionKnowledgeBaseUpdate), requireAI, enterprise.KnowledgeEntrySubmit)
	group.POST("/knowledge/entries/:id/_publish", require(constants.PermissionKnowledgeBaseUpdate), requireAI, enterprise.KnowledgeEntryPublish)
	group.POST("/knowledge/entries/:id/_deprecate", require(constants.PermissionKnowledgeBaseUpdate), requireAI, enterprise.KnowledgeEntryDeprecate)
	group.GET("/knowledge/entries/:id/versions", require(constants.PermissionKnowledgeBaseView), requireAI, enterprise.KnowledgeEntryVersions)
	group.GET("/knowledge/entries/:id/quality", require(constants.PermissionKnowledgeBaseView), requireAI, enterprise.KnowledgeEntryQuality)
	group.GET("/knowledge/entries/:id/translations", require(constants.PermissionKnowledgeBaseView), requireAI, enterprise.KnowledgeEntryTranslations)
	group.GET("/knowledge/candidates", require(constants.PermissionKnowledgeBaseView), requireAI, enterprise.KnowledgeCandidateList)
	group.GET("/knowledge/candidates/page", require(constants.PermissionKnowledgeBaseView), requireAI, enterprise.KnowledgeCandidatePage)
	group.GET("/knowledge/candidates/:id", require(constants.PermissionKnowledgeBaseView), requireAI, enterprise.KnowledgeCandidateGet)
	group.POST("/knowledge/candidates/:id/_enrich", require(constants.PermissionKnowledgeBaseUpdate), requireAI, enterprise.KnowledgeCandidateEnrich)
	group.POST("/knowledge/candidates/:id/_detach-duplicate", require(constants.PermissionKnowledgeBaseUpdate), requireAI, enterprise.KnowledgeCandidateDetachDuplicate)
	group.POST("/knowledge/candidates/:id/_approve", require(constants.PermissionKnowledgeBaseUpdate), requireAI, enterprise.KnowledgeCandidateApprove)
	group.POST("/knowledge/candidates/:id/_reject", require(constants.PermissionKnowledgeBaseUpdate), requireAI, enterprise.KnowledgeCandidateReject)
	group.POST("/knowledge/candidates/:id/_merge", require(constants.PermissionKnowledgeBaseUpdate), requireAI, enterprise.KnowledgeCandidateMerge)
	group.GET("/knowledge/index-tasks", require(constants.PermissionKnowledgeBaseView), requireAI, enterprise.KnowledgeIndexTaskList)
	group.POST("/knowledge/index-tasks/:id/_retry", require(constants.PermissionKnowledgeBaseUpdate), requireAI, enterprise.KnowledgeIndexTaskRetry)
}

func registerEnterpriseSearchRoutes(group *gin.RouterGroup) {
	group.GET("/search/global", enterprise.GlobalSearch)
}

func registerEnterpriseMeetingRoutes(group *gin.RouterGroup) {
	require := middleware.RequirePermissionMiddleware
	group.GET("/meetings", require(constants.PermissionMeetingView), enterprise.MeetingGetList)
	group.GET("/tickets/:id/meetings", require(constants.PermissionMeetingView), enterprise.MeetingGetByTicket)
	group.POST("/tickets/:id/meetings", require(constants.PermissionMeetingCreate), enterprise.MeetingPostCreate)
	group.GET("/meetings/:id/join", require(constants.PermissionMeetingView), enterprise.MeetingGetJoin)
	group.POST("/meetings/:id/_joined", require(constants.PermissionMeetingView), enterprise.MeetingPostJoined)
	group.POST("/meetings/:id/_left", require(constants.PermissionMeetingView), enterprise.MeetingPostLeft)
	group.POST("/meetings/:id/_heartbeat", require(constants.PermissionMeetingView), enterprise.MeetingPostHeartbeat)
	group.GET("/meetings/:id/status", require(constants.PermissionMeetingView), enterprise.MeetingGetStatus)
	group.GET("/meetings/:id/transcripts", require(constants.PermissionMeetingView), enterprise.MeetingGetTranscripts)
	group.GET("/meetings/:id/transcripts/page", require(constants.PermissionMeetingView), enterprise.MeetingGetTranscriptPage)
	group.POST("/meetings/:id/transcripts", require(constants.PermissionMeetingUpdate), enterprise.MeetingPostTranscript)
	group.GET("/meetings/:id/annotations", require(constants.PermissionMeetingView), enterprise.MeetingGetAnnotations)
	group.POST("/meetings/:id/annotations", require(constants.PermissionMeetingUpdate), enterprise.MeetingPostAnnotation)
	group.POST("/meetings/:id/frames", require(constants.PermissionMeetingUpdate), enterprise.MeetingPostFrame)
	group.POST("/meetings/:id/frames/:frameId/_detect", require(constants.PermissionMeetingUpdate), enterprise.MeetingPostFrameDetect)
	group.POST("/meetings/:id/_end", require(constants.PermissionMeetingUpdate), enterprise.MeetingPostEnd)
}

func registerEnterpriseNotificationRoutes(group *gin.RouterGroup) {
	require := middleware.RequirePermissionMiddleware
	group.GET("/notifications", require(constants.PermissionNotificationView), enterprise.NotificationList)
	group.POST("/notifications/:id/_read", require(constants.PermissionNotificationUpdate), enterprise.NotificationMarkRead)
	group.POST("/notifications/_mark_all_read", require(constants.PermissionNotificationUpdate), enterprise.NotificationMarkAllRead)
	group.GET("/notifications/preferences", require(constants.PermissionNotificationView), enterprise.NotificationRecipientSettingGet)
	group.POST("/notifications/preferences", require(constants.PermissionNotificationUpdate), enterprise.NotificationRecipientSettingSave)
	group.POST("/notifications/preferences/_test", require(constants.PermissionNotificationUpdate), enterprise.NotificationRecipientSettingTest)
	group.POST("/notifications/push-tokens", require(constants.PermissionNotificationUpdate), enterprise.MobilePushTokenRegister)
	group.POST("/notifications/push-tokens/:id/_revoke", require(constants.PermissionNotificationUpdate), enterprise.MobilePushTokenRevoke)
	group.GET("/notifications/mail-settings", require(constants.PermissionNotificationChannelManage), enterprise.NotificationMailSettingGet)
	group.POST("/notifications/mail-settings", require(constants.PermissionNotificationChannelManage), enterprise.NotificationMailSettingSave)
	group.POST("/notifications/mail-settings/_test", require(constants.PermissionNotificationChannelManage), enterprise.NotificationMailSettingTest)
}

func registerEnterpriseIAMRoutes(group *gin.RouterGroup) {
	group.GET("/iam/members", enterprise.IAMMemberList)
	group.POST("/iam/members/invite", enterprise.IAMMemberInvite)
	group.POST("/iam/members/:id/update", enterprise.IAMMemberUpdate)
	group.POST("/iam/members/:id/_status", enterprise.IAMMemberStatus)
	group.POST("/iam/members/:id/portal-session", enterprise.IAMMemberPortalSession)
	group.GET("/iam/customer-users", enterprise.IAMCustomerUserList)
	group.POST("/iam/customer-users/authorize", enterprise.IAMCustomerUserAuthorize)
	group.POST("/iam/customer-users/invite", enterprise.IAMCustomerUserInvite)
	group.POST("/iam/customer-users/:id/update", enterprise.IAMCustomerUserUpdate)
	group.POST("/iam/customer-users/:id/_status", enterprise.IAMCustomerUserStatus)
	group.POST("/iam/customer-users/:id/portal-session", enterprise.IAMCustomerUserPortalSession)
	group.GET("/iam/partners", enterprise.IAMPartnerList)
	group.POST("/iam/partners/invite-admin", enterprise.IAMPartnerAdminInvite)
	group.POST("/iam/partners/:id/update", enterprise.IAMPartnerUpdate)
	group.POST("/iam/partners/:id/_status", enterprise.IAMPartnerStatus)
	group.GET("/iam/departments", enterprise.IAMDepartmentList)
	group.POST("/iam/departments/create", enterprise.IAMDepartmentCreate)
	group.GET("/iam/roles", enterprise.IAMRoleList)
	group.POST("/iam/roles/save", enterprise.IAMRoleSave)
	group.POST("/iam/policy/save", enterprise.IAMPolicySave)
	group.GET("/iam/audit", enterprise.IAMAuditList)
	group.GET("/iam/audit/export", enterprise.IAMAuditExport)
}

func registerEnterpriseGDPRRoutes(group *gin.RouterGroup) {
	require := middleware.RequirePermissionMiddleware
	group.POST("/gdpr/dsar/submit", require(constants.PermissionPrivacyRequestManage), enterprise.GdprPostDsarSubmit)
	group.POST("/gdpr/dsar/:id/verify", require(constants.PermissionPrivacyRequestManage), enterprise.GdprPostDsarVerify)
	group.GET("/gdpr/dsar/:id/status", require(constants.PermissionPrivacyRequestView), enterprise.GdprGetDsarStatus)
	group.POST("/gdpr/dsar/:id/execute", require(constants.PermissionPrivacyRequestManage), enterprise.GdprPostDsarExecute)
	group.GET("/gdpr/dsar/list", require(constants.PermissionPrivacyRequestView), enterprise.GdprGetDsarList)
	group.POST("/gdpr/breach/report", require(constants.PermissionDataBreachManage), enterprise.GdprPostBreachReport)
	group.POST("/gdpr/breach/:id/notify-authority", require(constants.PermissionDataBreachManage), enterprise.GdprPostBreachNotifyAuthority)
	group.POST("/gdpr/breach/:id/notify-users", require(constants.PermissionDataBreachManage), enterprise.GdprPostBreachNotifyUsers)
	group.GET("/gdpr/breach/:id/status", require(constants.PermissionDataBreachView), enterprise.GdprGetBreachStatus)
	group.GET("/gdpr/breach/list", require(constants.PermissionDataBreachView), enterprise.GdprGetBreachList)
	group.GET("/gdpr/retention-policy", require(constants.PermissionDataRetentionView), enterprise.GdprGetRetentionPolicy)
}

func registerEnterpriseAccessConnectorRoutes(group *gin.RouterGroup) {
	require := middleware.RequirePermissionMiddleware
	group.POST("/access/connector/create", require(constants.PermissionTenantIntegrationConfigCreate), enterprise.AccessConnectorPostCreate)
	group.POST("/access/connector/update", require(constants.PermissionTenantIntegrationConfigUpdate), enterprise.AccessConnectorPostUpdate)
	group.POST("/access/connector/:id/test", require(constants.PermissionTenantIntegrationConfigUpdate), enterprise.AccessConnectorPostTest)
	group.POST("/access/connector/:id/call", require(constants.PermissionTenantIntegrationConfigUpdate), enterprise.AccessConnectorPostCall)
	group.GET("/access/connectors", require(constants.PermissionTenantIntegrationConfigView), enterprise.AccessConnectorGetList)
	group.GET("/access/connector/:id/status", require(constants.PermissionTenantIntegrationConfigView), enterprise.AccessConnectorGetStatus)
	group.GET("/access/connector/:id/logs", require(constants.PermissionTenantIntegrationConfigView), enterprise.AccessConnectorGetLogs)
	group.GET("/access/connector/templates", require(constants.PermissionTenantIntegrationConfigView), enterprise.AccessConnectorGetTemplates)
}

func registerPlatformRoutes(group *gin.RouterGroup) {
	require := middleware.RequirePermissionMiddleware
	requireAny := middleware.RequireAnyPermissionMiddleware
	group.GET("/overview", require(constants.PermissionTenantView), platform.GetPlatformOverview)
	group.GET("/overview/metrics", require(constants.PermissionTenantView), platform.GetPlatformOverviewMetrics)
	group.GET("/overview/top-tenants", require(constants.PermissionTenantView), platform.GetPlatformOverviewTopTenants)
	group.GET("/overview/usage-trend", require(constants.PermissionTenantView), platform.GetPlatformOverviewUsageTrend)
	group.GET("/overview/events", require(constants.PermissionTenantView), platform.GetPlatformOverviewEvents)
	group.GET("/tenants", require(constants.PermissionTenantView), platform.GetPlatformTenants)
	group.GET("/globalization", require(constants.PermissionTenantView), platform.GetPlatformGlobalization)
	group.GET("/usage", requireAny(constants.PermissionReportView, constants.PermissionFinanceView), platform.GetPlatformUsage)
	group.GET("/usage/overview", requireAny(constants.PermissionReportView, constants.PermissionFinanceView), platform.GetPlatformUsage)
	group.GET("/usage/sub2api/users", requireAny(constants.PermissionProductAIUsageCredentialView, constants.PermissionFinanceView), platform.GetPlatformSub2APIUsers)
	group.GET("/usage/sub2api/users/:userId/api-keys", requireAny(constants.PermissionProductAIUsageCredentialView, constants.PermissionFinanceView), platform.GetPlatformSub2APIUserKeys)
	group.GET("/ops/overview", require(constants.PermissionTenantView), platform.GetPlatformOpsOverview)
	group.GET("/ops/runtime", require(constants.PermissionTenantView), platform.GetPlatformOpsRuntime)
	group.GET("/ops/database", require(constants.PermissionTenantView), platform.GetPlatformOpsDatabase)
	group.GET("/ops/infrastructure", require(constants.PermissionTenantView), platform.GetPlatformOpsInfrastructure)
	group.GET("/ops/jitsi", require(constants.PermissionTenantView), platform.GetPlatformOpsJitsi)
	group.GET("/ops/speech", require(constants.PermissionTenantView), platform.GetPlatformOpsSpeech)
	group.GET("/ops/translation", require(constants.PermissionTenantView), platform.GetPlatformOpsTranslation)
	group.GET("/ops/ar", require(constants.PermissionTenantView), platform.GetPlatformOpsAR)
	group.GET("/ops/sub2api", require(constants.PermissionTenantView), platform.GetPlatformOpsSub2API)
	group.GET("/ops/access", require(constants.PermissionTenantView), platform.GetPlatformOpsAccess)
	group.GET("/ops/sync-queue", require(constants.PermissionTenantView), platform.GetPlatformOpsPipeline)
	group.GET("/ops/knowledge-queue", require(constants.PermissionTenantView), platform.GetPlatformOpsKnowledgeQueue)
	group.GET("/ops/notification-queue", require(constants.PermissionTenantView), platform.GetPlatformOpsNotificationQueue)
	group.GET("/integrations/config", require(constants.PermissionAIConfigView), platform.GetPlatformIntegrationSettings)
	group.POST("/integrations/update", require(constants.PermissionAIConfigUpdate), platform.PostPlatformIntegrationUpdate)
	group.POST("/integrations/test", require(constants.PermissionAIConfigUpdate), platform.PostPlatformIntegrationTest)
	group.GET("/system-config/policy", require(constants.PermissionTenantView), platform.GetPlatformSystemPolicy)
	group.POST("/system-config/policy", require(constants.PermissionTenantUpdate), platform.PostPlatformSystemPolicy)

	models := group.Group("/models")
	models.GET("/workspace", requireAny(constants.PermissionAIConfigView, constants.PermissionFinanceView), platform.GetPlatformAIModelWorkspace)
	models.POST("/provider/save", require(constants.PermissionAIConfigUpdate), platform.PostPlatformAIProviderSave)
	models.POST("/tenant/provision", require(constants.PermissionAIConfigCreate), platform.PostPlatformAITenantProvision)
	models.GET("/tenant/billing", requireAny(constants.PermissionProductAIUsageCredentialView, constants.PermissionFinanceView), platform.GetPlatformAITenantBilling)
	models.POST("/tenant/recharge", requireAny(constants.PermissionProductAIUsageCredentialUpdate, constants.PermissionFinanceManage), platform.PostPlatformAITenantRecharge)
	models.GET("/tenant/usage", requireAny(constants.PermissionProductAIUsageCredentialView, constants.PermissionFinanceView), platform.GetPlatformAITenantUsage)

	tenant := group.Group("/tenant")
	tenant.GET("/list", require(constants.PermissionTenantView), platform.GetPlatformTenantList)
	tenant.POST("/create", require(constants.PermissionTenantCreate), platform.PostPlatformTenantCreate)
	tenant.POST("/update", require(constants.PermissionTenantUpdate), platform.PostPlatformTenantUpdate)
	tenant.POST("/logo/upload", requireAny(constants.PermissionTenantCreate, constants.PermissionTenantUpdate), platform.PostPlatformTenantLogoUpload)
	tenant.POST("/freeze", require(constants.PermissionTenantUpdate), platform.PostPlatformTenantFreeze)
	tenant.POST("/unfreeze", require(constants.PermissionTenantUpdate), platform.PostPlatformTenantUnfreeze)
	tenant.POST("/enter", require(constants.PermissionTenantUpdate), platform.PostPlatformTenantEnter)
	tenant.POST("/admin-password", require(constants.PermissionTenantUpdate), platform.PostPlatformTenantAdminPassword)
	tenant.POST("/decommission", require(constants.PermissionTenantDelete), platform.PostPlatformTenantDecommission)
	tenant.POST("/delete", require(constants.PermissionTenantDelete), platform.PostPlatformTenantDelete)
	tenant.POST("/undo-decommission", require(constants.PermissionTenantDelete), platform.PostPlatformTenantUndoDecommission)
	tenant.POST("/data-export", require(constants.PermissionTenantExport), platform.PostPlatformTenantDataExport)
	tenant.POST("/right-to-erasure", require(constants.PermissionTenantDelete), platform.PostPlatformTenantRightToErasure)

	plan := group.Group("/plan")
	plan.GET("/list", require(constants.PermissionTenantView), platform.GetPlatformPlanList)
	plan.POST("/create", requireAny(constants.PermissionTenantUpdate, constants.PermissionFinanceManage), platform.PostPlatformPlanCreate)
	plan.POST("/update", requireAny(constants.PermissionTenantUpdate, constants.PermissionFinanceManage), platform.PostPlatformPlanUpdate)

	subscription := group.Group("/subscription")
	subscription.POST("/assign", requireAny(constants.PermissionTenantUpdate, constants.PermissionFinanceManage), platform.PostPlatformSubscriptionAssign)
	subscription.POST("/change-plan", requireAny(constants.PermissionTenantUpdate, constants.PermissionFinanceManage), platform.PostPlatformSubscriptionChangePlan)

	group.POST("/quota/override", requireAny(constants.PermissionTenantUpdate, constants.PermissionFinanceManage), platform.PostPlatformQuotaOverride)

	sub2api := group.Group("/sub2api/account")
	sub2api.GET("/list", require(constants.PermissionProductAIUsageCredentialView), platform.GetPlatformSub2APIAccountList)
	sub2api.POST("/bind", require(constants.PermissionProductAIUsageCredentialUpdate), platform.PostPlatformSub2APIAccountBind)
	sub2api.POST("/test", require(constants.PermissionProductAIUsageCredentialUpdate), platform.PostPlatformSub2APIAccountTest)
	sub2api.POST("/sync", require(constants.PermissionProductAIUsageCredentialUpdate), platform.PostPlatformSub2APIAccountSync)

	staff := group.Group("/staff")
	staff.GET("/list", platform.GetPlatformStaffList)
	staff.POST("/invite", platform.PostPlatformStaffInvite)
	staff.POST("/update", platform.PostPlatformStaffUpdate)
	staff.POST("/grant-tenant", platform.PostPlatformStaffGrantTenant)
	staff.POST("/disable", platform.PostPlatformStaffDisable)
	staff.POST("/delete", platform.PostPlatformStaffDelete)

	permission := group.Group("/permission")
	permission.GET("/role/list", platform.GetPlatformPermissionRoleList)
	permission.POST("/role/save", platform.PostPlatformPermissionRoleSave)
	permission.POST("/role/delete", platform.PostPlatformPermissionRoleDelete)
	permission.POST("/policy/save", platform.PostPlatformPermissionPolicySave)
	permission.POST("/check", platform.PostPlatformPermissionCheck)

	audit := group.Group("/audit")
	audit.GET("/list", platform.GetPlatformAuditList)
	audit.GET("/export", require(constants.PermissionSessionView), platform.PostPlatformAuditExport)
	audit.POST("/export", require(constants.PermissionSessionView), platform.PostPlatformAuditExport)
	audit.GET("/retention", require(constants.PermissionSessionView), platform.GetPlatformAuditRetention)
	audit.POST("/retention", require(constants.PermissionAuditRetentionManage), platform.PostPlatformAuditRetention)

	systemIntro := group.Group("/system-intro")
	systemIntro.GET("/list", require(constants.PermissionSystemIntroView), platform.GetPlatformSystemIntroList)
	systemIntro.POST("/create", require(constants.PermissionSystemIntroCreate), platform.PostPlatformSystemIntroCreate)
	systemIntro.POST("/update", require(constants.PermissionSystemIntroUpdate), platform.PostPlatformSystemIntroUpdate)
	systemIntro.POST("/delete", require(constants.PermissionSystemIntroDelete), platform.PostPlatformSystemIntroDelete)
}
