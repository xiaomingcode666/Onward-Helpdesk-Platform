package constants

const (
	RoleCodeSuperAdmin   = "super_admin"    // 超管
	RoleCodeAdmin        = "admin"          // 管理员
	RoleCodeCsTeamLeader = "cs_team_leader" // 客服组长
	RoleCodeCsUser       = "cs_user"        // 客服
)

const (
	AuthTokenPrefix = "ak_"
)

const (
	ClientTypeAdminWeb = "admin_web"
)

const (
	BootstrapAdminUsername = "admin"
	BootstrapAdminNickname = "Super Admin"
)

// Permission 权限结构体
type Permission struct {
	Name      string
	Code      string
	Type      string
	GroupName string
	Method    string
	APIPath   string
	SortNo    int
}

// 权限常量定义
var (
	// 用户相关权限
	PermissionUserView       = Permission{Name: "查看用户", Code: "user.view", Type: "api", GroupName: "user", Method: "ANY", APIPath: "/api/dashboard/user/list", SortNo: 10}
	PermissionUserCreate     = Permission{Name: "创建用户", Code: "user.create", Type: "api", GroupName: "user", Method: "POST", APIPath: "/api/dashboard/user/create", SortNo: 20}
	PermissionUserUpdate     = Permission{Name: "更新用户", Code: "user.update", Type: "api", GroupName: "user", Method: "POST", APIPath: "/api/dashboard/user/update", SortNo: 30}
	PermissionUserDelete     = Permission{Name: "删除用户", Code: "user.delete", Type: "api", GroupName: "user", Method: "POST", APIPath: "/api/dashboard/user/delete", SortNo: 40}
	PermissionUserAssignRole = Permission{Name: "分配用户角色", Code: "user.assignRole", Type: "api", GroupName: "user", Method: "POST", APIPath: "/api/dashboard/user/assign_role", SortNo: 50}

	// 角色相关权限
	PermissionRoleView             = Permission{Name: "查看角色", Code: "role.view", Type: "api", GroupName: "role", Method: "ANY", APIPath: "/api/dashboard/role/list", SortNo: 110}
	PermissionRoleCreate           = Permission{Name: "创建角色", Code: "role.create", Type: "api", GroupName: "role", Method: "POST", APIPath: "/api/dashboard/role/create", SortNo: 120}
	PermissionRoleUpdate           = Permission{Name: "更新角色", Code: "role.update", Type: "api", GroupName: "role", Method: "POST", APIPath: "/api/dashboard/role/update", SortNo: 130}
	PermissionRoleDelete           = Permission{Name: "删除角色", Code: "role.delete", Type: "api", GroupName: "role", Method: "POST", APIPath: "/api/dashboard/role/delete", SortNo: 140}
	PermissionRoleAssignPermission = Permission{Name: "分配角色权限", Code: "role.assignPermission", Type: "api", GroupName: "role", Method: "POST", APIPath: "/api/dashboard/role/assign_permission", SortNo: 150}

	// 权限相关权限
	PermissionPermissionView = Permission{Name: "查看权限", Code: "permission.view", Type: "api", GroupName: "permission", Method: "ANY", APIPath: "/api/dashboard/permission/list", SortNo: 210}
	PermissionPermissionSync = Permission{Name: "同步权限", Code: "permission.sync", Type: "api", GroupName: "permission", Method: "POST", APIPath: "/api/dashboard/permission/sync", SortNo: 220}

	// 会话相关权限
	PermissionSessionView   = Permission{Name: "查看会话", Code: "session.view", Type: "api", GroupName: "session", Method: "ANY", APIPath: "/api/dashboard/session/list", SortNo: 310}
	PermissionSessionRevoke = Permission{Name: "踢除会话", Code: "session.revoke", Type: "api", GroupName: "session", Method: "POST", APIPath: "/api/dashboard/session/revoke", SortNo: 320}

	// 客服会话相关权限
	PermissionConversationView         = Permission{Name: "查看会话", Code: "conversation.view", Type: "api", GroupName: "conversation", Method: "ANY", APIPath: "/api/dashboard/conversation/list", SortNo: 410}
	PermissionConversationAssign       = Permission{Name: "分配会话", Code: "conversation.assign", Type: "api", GroupName: "conversation", Method: "POST", APIPath: "/api/dashboard/conversation/assign", SortNo: 430}
	PermissionConversationTransfer     = Permission{Name: "转接会话", Code: "conversation.transfer", Type: "api", GroupName: "conversation", Method: "POST", APIPath: "/api/dashboard/conversation/transfer", SortNo: 440}
	PermissionConversationClose        = Permission{Name: "关闭会话", Code: "conversation.close", Type: "api", GroupName: "conversation", Method: "POST", APIPath: "/api/dashboard/conversation/close", SortNo: 450}
	PermissionConversationSend         = Permission{Name: "发送会话消息", Code: "conversation.send", Type: "api", GroupName: "conversation", Method: "POST", APIPath: "/api/dashboard/conversation/send_message", SortNo: 460}
	PermissionConversationTag          = Permission{Name: "管理会话标签", Code: "conversation.tag", Type: "api", GroupName: "conversation", Method: "POST", APIPath: "/api/dashboard/conversation/add_tag", SortNo: 470}
	PermissionConversationHandover     = Permission{Name: "处理会话交接", Code: "conversation.handover", Type: "api", GroupName: "conversation", Method: "ANY", APIPath: "/api/dashboard/conversation/handover_list", SortNo: 480}
	PermissionConversationRecycle      = Permission{Name: "回收会话", Code: "conversation.recycle", Type: "api", GroupName: "conversation", Method: "POST", APIPath: "/api/dashboard/conversation/recycle", SortNo: 490}
	PermissionConversationLinkCustomer = Permission{Name: "关联会话客户", Code: "conversation.linkCustomer", Type: "api", GroupName: "conversation", Method: "POST", APIPath: "/api/dashboard/conversation/link_customer", SortNo: 495}

	// 工单相关权限
	PermissionTicketView              = Permission{Name: "查看工单", Code: "ticket.view", Type: "api", GroupName: "ticket", Method: "ANY", APIPath: "/api/dashboard/ticket/list", SortNo: 500}
	PermissionTicketCreate            = Permission{Name: "创建工单", Code: "ticket.create", Type: "api", GroupName: "ticket", Method: "POST", APIPath: "/api/dashboard/ticket/create", SortNo: 510}
	PermissionTicketUpdate            = Permission{Name: "更新工单", Code: "ticket.update", Type: "api", GroupName: "ticket", Method: "POST", APIPath: "/api/dashboard/ticket/update", SortNo: 520}
	PermissionTicketAssign            = Permission{Name: "指派工单", Code: "ticket.assign", Type: "api", GroupName: "ticket", Method: "POST", APIPath: "/api/dashboard/ticket/assign", SortNo: 530}
	PermissionTicketChangeStatus      = Permission{Name: "变更工单状态", Code: "ticket.changeStatus", Type: "api", GroupName: "ticket", Method: "POST", APIPath: "/api/dashboard/ticket/change_status", SortNo: 540}
	PermissionTicketProgress          = Permission{Name: "更新工单进展", Code: "ticket.progress", Type: "api", GroupName: "ticket", Method: "POST", APIPath: "/api/dashboard/ticket/progress/create", SortNo: 550}
	PermissionTicketRepairHistoryView = Permission{Name: "查看维修历史", Code: "ticket.repairHistory.view", Type: "api", GroupName: "ticket", Method: "ANY", APIPath: "/api/dashboard/ticket/repair-history/list", SortNo: 555}

	// 通知相关权限
	PermissionNotificationView          = Permission{Name: "查看通知", Code: "notification.view", Type: "api", GroupName: "notification", Method: "ANY", APIPath: "/api/dashboard/notification/list", SortNo: 680}
	PermissionNotificationUpdate        = Permission{Name: "更新通知", Code: "notification.update", Type: "api", GroupName: "notification", Method: "POST", APIPath: "/api/dashboard/notification/mark_read", SortNo: 690}
	PermissionNotificationChannelManage = Permission{Name: "管理通知渠道", Code: "notification.channel.manage", Type: "api", GroupName: "notification", Method: "ANY", APIPath: "/api/enterprise/v1/notifications/mail-settings", SortNo: 695}

	// 快捷回复相关权限
	PermissionQuickReplyView   = Permission{Name: "查看快捷回复", Code: "quickReply.view", Type: "api", GroupName: "quickReply", Method: "ANY", APIPath: "/api/dashboard/quick-reply/list", SortNo: 610}
	PermissionQuickReplyCreate = Permission{Name: "创建快捷回复", Code: "quickReply.create", Type: "api", GroupName: "quickReply", Method: "POST", APIPath: "/api/dashboard/quick-reply/create", SortNo: 620}
	PermissionQuickReplyUpdate = Permission{Name: "更新快捷回复", Code: "quickReply.update", Type: "api", GroupName: "quickReply", Method: "POST", APIPath: "/api/dashboard/quick-reply/update", SortNo: 630}
	PermissionQuickReplyDelete = Permission{Name: "删除快捷回复", Code: "quickReply.delete", Type: "api", GroupName: "quickReply", Method: "POST", APIPath: "/api/dashboard/quick-reply/delete", SortNo: 640}

	// 标签相关权限
	PermissionTagView   = Permission{Name: "查看标签", Code: "tag.view", Type: "api", GroupName: "tag", Method: "ANY", APIPath: "/api/dashboard/tag/list", SortNo: 550}
	PermissionTagCreate = Permission{Name: "创建标签", Code: "tag.create", Type: "api", GroupName: "tag", Method: "POST", APIPath: "/api/dashboard/tag/create", SortNo: 560}
	PermissionTagUpdate = Permission{Name: "更新标签", Code: "tag.update", Type: "api", GroupName: "tag", Method: "POST", APIPath: "/api/dashboard/tag/update", SortNo: 570}
	PermissionTagDelete = Permission{Name: "删除标签", Code: "tag.delete", Type: "api", GroupName: "tag", Method: "POST", APIPath: "/api/dashboard/tag/delete", SortNo: 580}

	// 公司相关权限
	PermissionCompanyView   = Permission{Name: "查看公司", Code: "company.view", Type: "api", GroupName: "company", Method: "ANY", APIPath: "/api/dashboard/company/list", SortNo: 590}
	PermissionCompanyCreate = Permission{Name: "创建公司", Code: "company.create", Type: "api", GroupName: "company", Method: "POST", APIPath: "/api/dashboard/company/create", SortNo: 600}
	PermissionCompanyUpdate = Permission{Name: "更新公司", Code: "company.update", Type: "api", GroupName: "company", Method: "POST", APIPath: "/api/dashboard/company/update", SortNo: 610}
	PermissionCompanyDelete = Permission{Name: "删除公司", Code: "company.delete", Type: "api", GroupName: "company", Method: "POST", APIPath: "/api/dashboard/company/delete", SortNo: 620}

	// RemoteHelpDesk 产品中心相关权限
	PermissionProductView                    = Permission{Name: "查看产品", Code: "product.view", Type: "api", GroupName: "product", Method: "ANY", APIPath: "/api/dashboard/product/list", SortNo: 700}
	PermissionProductCreate                  = Permission{Name: "创建产品", Code: "product.create", Type: "api", GroupName: "product", Method: "POST", APIPath: "/api/dashboard/product/create", SortNo: 710}
	PermissionProductUpdate                  = Permission{Name: "更新产品", Code: "product.update", Type: "api", GroupName: "product", Method: "POST", APIPath: "/api/dashboard/product/update", SortNo: 720}
	PermissionProductDelete                  = Permission{Name: "删除产品", Code: "product.delete", Type: "api", GroupName: "product", Method: "POST", APIPath: "/api/dashboard/product/delete", SortNo: 730}
	PermissionProductServiceProfileView      = Permission{Name: "查看产品服务配置", Code: "productServiceProfile.view", Type: "api", GroupName: "productServiceProfile", Method: "ANY", APIPath: "/api/dashboard/product-service-profile/list", SortNo: 735}
	PermissionProductServiceProfileCreate    = Permission{Name: "创建产品服务配置", Code: "productServiceProfile.create", Type: "api", GroupName: "productServiceProfile", Method: "POST", APIPath: "/api/dashboard/product-service-profile/create", SortNo: 736}
	PermissionProductServiceProfileUpdate    = Permission{Name: "更新产品服务配置", Code: "productServiceProfile.update", Type: "api", GroupName: "productServiceProfile", Method: "POST", APIPath: "/api/dashboard/product-service-profile/update", SortNo: 737}
	PermissionProductServiceProfileDelete    = Permission{Name: "删除产品服务配置", Code: "productServiceProfile.delete", Type: "api", GroupName: "productServiceProfile", Method: "POST", APIPath: "/api/dashboard/product-service-profile/delete", SortNo: 738}
	PermissionProductKnowledgeBindingView    = Permission{Name: "查看产品知识库绑定", Code: "productKnowledgeBinding.view", Type: "api", GroupName: "productKnowledgeBinding", Method: "ANY", APIPath: "/api/dashboard/product-knowledge-binding/list", SortNo: 739}
	PermissionProductKnowledgeBindingCreate  = Permission{Name: "创建产品知识库绑定", Code: "productKnowledgeBinding.create", Type: "api", GroupName: "productKnowledgeBinding", Method: "POST", APIPath: "/api/dashboard/product-knowledge-binding/create", SortNo: 740}
	PermissionProductKnowledgeBindingUpdate  = Permission{Name: "更新产品知识库绑定", Code: "productKnowledgeBinding.update", Type: "api", GroupName: "productKnowledgeBinding", Method: "POST", APIPath: "/api/dashboard/product-knowledge-binding/update", SortNo: 741}
	PermissionProductKnowledgeBindingDelete  = Permission{Name: "删除产品知识库绑定", Code: "productKnowledgeBinding.delete", Type: "api", GroupName: "productKnowledgeBinding", Method: "POST", APIPath: "/api/dashboard/product-knowledge-binding/delete", SortNo: 742}
	PermissionProductKnowledgeBindingResolve = Permission{Name: "解析产品知识库", Code: "productKnowledgeBinding.resolve", Type: "api", GroupName: "productKnowledgeBinding", Method: "POST", APIPath: "/api/dashboard/product-knowledge-binding/resolve", SortNo: 743}
	PermissionTenantIntegrationConfigView    = Permission{Name: "查看租户集成配置", Code: "tenantIntegrationConfig.view", Type: "api", GroupName: "tenantIntegrationConfig", Method: "ANY", APIPath: "/api/dashboard/tenant-integration-config/list", SortNo: 739}
	PermissionTenantIntegrationConfigCreate  = Permission{Name: "创建租户集成配置", Code: "tenantIntegrationConfig.create", Type: "api", GroupName: "tenantIntegrationConfig", Method: "POST", APIPath: "/api/dashboard/tenant-integration-config/create", SortNo: 740}
	PermissionTenantIntegrationConfigUpdate  = Permission{Name: "更新租户集成配置", Code: "tenantIntegrationConfig.update", Type: "api", GroupName: "tenantIntegrationConfig", Method: "POST", APIPath: "/api/dashboard/tenant-integration-config/update", SortNo: 741}
	PermissionTenantIntegrationConfigDelete  = Permission{Name: "删除租户集成配置", Code: "tenantIntegrationConfig.delete", Type: "api", GroupName: "tenantIntegrationConfig", Method: "POST", APIPath: "/api/dashboard/tenant-integration-config/delete", SortNo: 742}
	PermissionProductAIUsageCredentialView   = Permission{Name: "查看产品 AI 用量凭证", Code: "productAIUsageCredential.view", Type: "api", GroupName: "productAIUsageCredential", Method: "ANY", APIPath: "/api/dashboard/product-ai-usage-credential/list", SortNo: 743}
	PermissionProductAIUsageCredentialCreate = Permission{Name: "创建产品 AI 用量凭证", Code: "productAIUsageCredential.create", Type: "api", GroupName: "productAIUsageCredential", Method: "POST", APIPath: "/api/dashboard/product-ai-usage-credential/create", SortNo: 744}
	PermissionProductAIUsageCredentialUpdate = Permission{Name: "更新产品 AI 用量凭证", Code: "productAIUsageCredential.update", Type: "api", GroupName: "productAIUsageCredential", Method: "POST", APIPath: "/api/dashboard/product-ai-usage-credential/update", SortNo: 745}
	PermissionProductAIUsageCredentialDelete = Permission{Name: "删除产品 AI 用量凭证", Code: "productAIUsageCredential.delete", Type: "api", GroupName: "productAIUsageCredential", Method: "POST", APIPath: "/api/dashboard/product-ai-usage-credential/delete", SortNo: 746}
	PermissionMeetingPreview                 = Permission{Name: "创建会议预览", Code: "meeting.preview", Type: "api", GroupName: "meeting", Method: "POST", APIPath: "/api/dashboard/meeting/preview-create", SortNo: 747}
	PermissionMeetingCreate                  = Permission{Name: "创建会议", Code: "meeting.create", Type: "api", GroupName: "meeting", Method: "POST", APIPath: "/api/dashboard/meeting/create", SortNo: 748}
	PermissionMeetingView                    = Permission{Name: "查看会议", Code: "meeting.view", Type: "api", GroupName: "meeting", Method: "ANY", APIPath: "/api/dashboard/meeting/*", SortNo: 749}
	PermissionMeetingUpdate                  = Permission{Name: "更新会议", Code: "meeting.update", Type: "api", GroupName: "meeting", Method: "POST", APIPath: "/api/dashboard/meeting/*/end", SortNo: 750}
	PermissionProductModelView               = Permission{Name: "查看产品型号", Code: "productModel.view", Type: "api", GroupName: "productModel", Method: "ANY", APIPath: "/api/dashboard/product-model/list", SortNo: 740}
	PermissionProductModelCreate             = Permission{Name: "创建产品型号", Code: "productModel.create", Type: "api", GroupName: "productModel", Method: "POST", APIPath: "/api/dashboard/product-model/create", SortNo: 750}
	PermissionProductModelUpdate             = Permission{Name: "更新产品型号", Code: "productModel.update", Type: "api", GroupName: "productModel", Method: "POST", APIPath: "/api/dashboard/product-model/update", SortNo: 760}
	PermissionProductModelDelete             = Permission{Name: "删除产品型号", Code: "productModel.delete", Type: "api", GroupName: "productModel", Method: "POST", APIPath: "/api/dashboard/product-model/delete", SortNo: 770}
	PermissionDeviceView                     = Permission{Name: "查看设备", Code: "device.view", Type: "api", GroupName: "device", Method: "ANY", APIPath: "/api/dashboard/device/list", SortNo: 780}
	PermissionDeviceCreate                   = Permission{Name: "创建设备", Code: "device.create", Type: "api", GroupName: "device", Method: "POST", APIPath: "/api/dashboard/device/create", SortNo: 790}
	PermissionDeviceUpdate                   = Permission{Name: "更新设备", Code: "device.update", Type: "api", GroupName: "device", Method: "POST", APIPath: "/api/dashboard/device/update", SortNo: 800}
	PermissionDeviceDelete                   = Permission{Name: "删除设备", Code: "device.delete", Type: "api", GroupName: "device", Method: "POST", APIPath: "/api/dashboard/device/delete", SortNo: 810}
	PermissionServiceCodeBatchView           = Permission{Name: "查看服务码批次", Code: "serviceCodeBatch.view", Type: "api", GroupName: "serviceCodeBatch", Method: "ANY", APIPath: "/api/dashboard/service-code-batch/list", SortNo: 820}
	PermissionServiceCodeBatchCreate         = Permission{Name: "创建服务码批次", Code: "serviceCodeBatch.create", Type: "api", GroupName: "serviceCodeBatch", Method: "POST", APIPath: "/api/dashboard/service-code-batch/create", SortNo: 830}
	PermissionServiceCodeBatchUpdate         = Permission{Name: "更新服务码批次", Code: "serviceCodeBatch.update", Type: "api", GroupName: "serviceCodeBatch", Method: "POST", APIPath: "/api/dashboard/service-code-batch/update_status", SortNo: 840}
	PermissionServiceCodeBatchDelete         = Permission{Name: "删除服务码批次", Code: "serviceCodeBatch.delete", Type: "api", GroupName: "serviceCodeBatch", Method: "POST", APIPath: "/api/dashboard/service-code-batch/delete", SortNo: 850}
	PermissionServiceCodeView                = Permission{Name: "查看服务码", Code: "serviceCode.view", Type: "api", GroupName: "serviceCode", Method: "ANY", APIPath: "/api/dashboard/service-code/list", SortNo: 860}
	PermissionServiceCodeGenerate            = Permission{Name: "生成服务码", Code: "serviceCode.generate", Type: "api", GroupName: "serviceCode", Method: "POST", APIPath: "/api/dashboard/service-code/generate", SortNo: 870}
	PermissionServiceCodeRevoke              = Permission{Name: "吊销服务码", Code: "serviceCode.revoke", Type: "api", GroupName: "serviceCode", Method: "POST", APIPath: "/api/dashboard/service-code/revoke", SortNo: 880}

	// 接入渠道相关权限
	PermissionChannelView   = Permission{Name: "查看接入渠道", Code: "channel.view", Type: "api", GroupName: "channel", Method: "ANY", APIPath: "/api/dashboard/channel/list", SortNo: 625}
	PermissionChannelCreate = Permission{Name: "创建接入渠道", Code: "channel.create", Type: "api", GroupName: "channel", Method: "POST", APIPath: "/api/dashboard/channel/create", SortNo: 626}
	PermissionChannelUpdate = Permission{Name: "更新接入渠道", Code: "channel.update", Type: "api", GroupName: "channel", Method: "POST", APIPath: "/api/dashboard/channel/update", SortNo: 627}
	PermissionChannelDelete = Permission{Name: "删除接入渠道", Code: "channel.delete", Type: "api", GroupName: "channel", Method: "POST", APIPath: "/api/dashboard/channel/delete", SortNo: 628}

	// 客户相关权限
	PermissionCustomerView   = Permission{Name: "查看客户", Code: "customer.view", Type: "api", GroupName: "customer", Method: "POST", APIPath: "/api/dashboard/customer/list", SortNo: 630}
	PermissionCustomerCreate = Permission{Name: "创建客户", Code: "customer.create", Type: "api", GroupName: "customer", Method: "POST", APIPath: "/api/dashboard/customer/create", SortNo: 640}
	PermissionCustomerUpdate = Permission{Name: "更新客户", Code: "customer.update", Type: "api", GroupName: "customer", Method: "POST", APIPath: "/api/dashboard/customer/update", SortNo: 650}
	PermissionCustomerDelete = Permission{Name: "删除客户", Code: "customer.delete", Type: "api", GroupName: "customer", Method: "POST", APIPath: "/api/dashboard/customer/delete", SortNo: 660}

	// 多域 IAM 与平台治理权限
	PermissionTenantView           = Permission{Name: "查看租户", Code: "tenant.view", Type: "api", GroupName: "tenant", Method: "GET", APIPath: "/api/platform/tenant/list", SortNo: 1800}
	PermissionTenantCreate         = Permission{Name: "创建租户", Code: "tenant.create", Type: "api", GroupName: "tenant", Method: "POST", APIPath: "/api/platform/tenant/create", SortNo: 1810}
	PermissionTenantUpdate         = Permission{Name: "更新租户", Code: "tenant.update", Type: "api", GroupName: "tenant", Method: "POST", APIPath: "/api/platform/tenant/update", SortNo: 1820}
	PermissionTenantDelete         = Permission{Name: "停用租户", Code: "tenant.delete", Type: "api", GroupName: "tenant", Method: "POST", APIPath: "/api/platform/tenant/decommission", SortNo: 1830}
	PermissionTenantExport         = Permission{Name: "导出租户数据", Code: "tenant.export", Type: "api", GroupName: "tenant", Method: "POST", APIPath: "/api/platform/tenant/data-export", SortNo: 1840}
	PermissionReportView           = Permission{Name: "查看报表", Code: "report.view", Type: "api", GroupName: "report", Method: "GET", APIPath: "/api/enterprise/v1/reports/overview", SortNo: 1850}
	PermissionReportExport         = Permission{Name: "导出报表", Code: "report.export", Type: "api", GroupName: "report", Method: "GET", APIPath: "/api/enterprise/v1/reports/export", SortNo: 1860}
	PermissionPartnerMemberView    = Permission{Name: "查看供应商成员", Code: "partnerMember.view", Type: "api", GroupName: "partnerMember", Method: "GET", APIPath: "/api/partner/v1/accounts", SortNo: 1870}
	PermissionPartnerMemberInvite  = Permission{Name: "邀请供应商成员", Code: "partnerMember.invite", Type: "api", GroupName: "partnerMember", Method: "POST", APIPath: "/api/partner/v1/accounts", SortNo: 1880}
	PermissionPartnerMemberUpdate  = Permission{Name: "更新供应商成员", Code: "partnerMember.update", Type: "api", GroupName: "partnerMember", Method: "PUT", APIPath: "/api/partner/v1/accounts/:accountId", SortNo: 1890}
	PermissionCustomerMemberView   = Permission{Name: "查看客户成员", Code: "customerMember.view", Type: "api", GroupName: "customerMember", Method: "GET", APIPath: "/api/customer/v1/members", SortNo: 1900}
	PermissionCustomerMemberInvite = Permission{Name: "邀请客户成员", Code: "customerMember.invite", Type: "api", GroupName: "customerMember", Method: "POST", APIPath: "/api/customer/v1/members", SortNo: 1910}
	PermissionCustomerMemberUpdate = Permission{Name: "更新客户成员", Code: "customerMember.update", Type: "api", GroupName: "customerMember", Method: "PUT", APIPath: "/api/customer/v1/members/:memberId", SortNo: 1920}
	PermissionFinanceView          = Permission{Name: "查看财务与计费", Code: "finance.view", Type: "api", GroupName: "finance", Method: "GET", APIPath: "/api/platform/usage", SortNo: 1930}
	PermissionFinanceManage        = Permission{Name: "管理充值、套餐与额度", Code: "finance.manage", Type: "api", GroupName: "finance", Method: "POST", APIPath: "/api/platform/models/tenant/recharge", SortNo: 1940}

	// 客服相关权限
	PermissionAgentView         = Permission{Name: "查看客服", Code: "agent.view", Type: "api", GroupName: "agent", Method: "ANY", APIPath: "/api/dashboard/agent/list", SortNo: 610}
	PermissionAgentCreate       = Permission{Name: "创建客服", Code: "agent.create", Type: "api", GroupName: "agent", Method: "POST", APIPath: "/api/dashboard/agent/create", SortNo: 620}
	PermissionAgentUpdate       = Permission{Name: "更新客服", Code: "agent.update", Type: "api", GroupName: "agent", Method: "POST", APIPath: "/api/dashboard/agent/update", SortNo: 630}
	PermissionAgentDelete       = Permission{Name: "删除客服", Code: "agent.delete", Type: "api", GroupName: "agent", Method: "POST", APIPath: "/api/dashboard/agent/delete", SortNo: 640}
	PermissionAgentUpdateStatus = Permission{Name: "更新客服状态", Code: "agent.updateStatus", Type: "api", GroupName: "agent", Method: "POST", APIPath: "/api/dashboard/agent/update_status", SortNo: 650}
	PermissionAgentConfig       = Permission{Name: "配置客服服务规则", Code: "agent.config", Type: "api", GroupName: "agent", Method: "POST", APIPath: "/api/dashboard/agent/update_service_config", SortNo: 660}

	// 客服组相关权限
	PermissionAgentTeamView   = Permission{Name: "查看客服组", Code: "agentTeam.view", Type: "api", GroupName: "agentTeam", Method: "ANY", APIPath: "/api/dashboard/agent-team/list", SortNo: 710}
	PermissionAgentTeamCreate = Permission{Name: "创建客服组", Code: "agentTeam.create", Type: "api", GroupName: "agentTeam", Method: "POST", APIPath: "/api/dashboard/agent-team/create", SortNo: 720}
	PermissionAgentTeamUpdate = Permission{Name: "更新客服组", Code: "agentTeam.update", Type: "api", GroupName: "agentTeam", Method: "POST", APIPath: "/api/dashboard/agent-team/update", SortNo: 730}
	PermissionAgentTeamDelete = Permission{Name: "删除客服组", Code: "agentTeam.delete", Type: "api", GroupName: "agentTeam", Method: "POST", APIPath: "/api/dashboard/agent-team/delete", SortNo: 740}

	// 客服组时间表相关权限。产品组时间表从成员可接单规则和请假生成，不再下发旧排班写入权限。
	PermissionAgentTeamScheduleView   = Permission{Name: "查看客服组时间表", Code: "agentTeamSchedule.view", Type: "api", GroupName: "agentTeamSchedule", Method: "ANY", APIPath: "/api/dashboard/agent-team-schedule/list", SortNo: 810}
	PermissionAgentTeamScheduleUpdate = Permission{Name: "管理可接单规则和请假", Code: "agentTeamSchedule.update", Type: "api", GroupName: "agentTeamSchedule", Method: "POST", APIPath: "/api/dashboard/agent-team-schedule/template/update", SortNo: 830}

	// 文件资源相关权限
	PermissionAssetView   = Permission{Name: "查看文件资源", Code: "asset.view", Type: "api", GroupName: "asset", Method: "ANY", APIPath: "/api/dashboard/asset/list", SortNo: 1210}
	PermissionAssetCreate = Permission{Name: "上传文件资源", Code: "asset.create", Type: "api", GroupName: "asset", Method: "POST", APIPath: "/api/dashboard/asset/create", SortNo: 1220}
	PermissionAssetDelete = Permission{Name: "删除文件资源", Code: "asset.delete", Type: "api", GroupName: "asset", Method: "POST", APIPath: "/api/dashboard/asset/delete", SortNo: 1230}

	// AI Agent 相关权限
	PermissionAIAgentView          = Permission{Name: "查看 AI Agent", Code: "aiAgent.view", Type: "api", GroupName: "aiAgent", Method: "ANY", APIPath: "/api/dashboard/ai-agent/list", SortNo: 1310}
	PermissionAIAgentCreate        = Permission{Name: "创建 AI Agent", Code: "aiAgent.create", Type: "api", GroupName: "aiAgent", Method: "POST", APIPath: "/api/dashboard/ai-agent/create", SortNo: 1320}
	PermissionAIAgentUpdate        = Permission{Name: "更新 AI Agent", Code: "aiAgent.update", Type: "api", GroupName: "aiAgent", Method: "POST", APIPath: "/api/dashboard/ai-agent/update", SortNo: 1330}
	PermissionAIAgentDelete        = Permission{Name: "删除 AI Agent", Code: "aiAgent.delete", Type: "api", GroupName: "aiAgent", Method: "POST", APIPath: "/api/dashboard/ai-agent/delete", SortNo: 1340}
	PermissionAIWorkflowView       = Permission{Name: "查看 AI Workflow", Code: "aiWorkflow.view", Type: "api", GroupName: "aiWorkflow", Method: "GET", APIPath: "/api/enterprise/v1/ai-workflows", SortNo: 1350}
	PermissionAIWorkflowUpdate     = Permission{Name: "编辑 AI Workflow", Code: "aiWorkflow.update", Type: "api", GroupName: "aiWorkflow", Method: "PATCH", APIPath: "/api/enterprise/v1/ai-workflows/:id/draft", SortNo: 1360}
	PermissionAIWorkflowPublish    = Permission{Name: "发布 AI Workflow", Code: "aiWorkflow.publish", Type: "api", GroupName: "aiWorkflow", Method: "POST", APIPath: "/api/enterprise/v1/ai-workflows/:id/_publish", SortNo: 1370}
	PermissionAIAgentReleaseCreate = Permission{Name: "生成机器人上线包", Code: "aiAgentRelease.create", Type: "api", GroupName: "aiAgentRelease", Method: "POST", APIPath: "/api/enterprise/v1/ai-agents/:id/releases", SortNo: 1371}
	PermissionAIAgentReleaseReview = Permission{Name: "审核机器人上线包", Code: "aiAgentRelease.review", Type: "api", GroupName: "aiAgentRelease", Method: "POST", APIPath: "/api/enterprise/v1/ai-agent-releases/:id/_approve", SortNo: 1372}
	PermissionAIAgentReleaseDeploy = Permission{Name: "部署机器人上线包", Code: "aiAgentRelease.deploy", Type: "api", GroupName: "aiAgentRelease", Method: "POST", APIPath: "/api/enterprise/v1/ai-agent-releases/:id/_deploy", SortNo: 1373}

	// AI 配置相关权限
	PermissionAIConfigView   = Permission{Name: "查看 AI 配置", Code: "aiConfig.view", Type: "api", GroupName: "aiConfig", Method: "ANY", APIPath: "/api/dashboard/ai-config/list", SortNo: 1390}
	PermissionAIConfigCreate = Permission{Name: "创建 AI 配置", Code: "aiConfig.create", Type: "api", GroupName: "aiConfig", Method: "POST", APIPath: "/api/dashboard/ai-config/create", SortNo: 1400}
	PermissionAIConfigUpdate = Permission{Name: "更新 AI 配置", Code: "aiConfig.update", Type: "api", GroupName: "aiConfig", Method: "POST", APIPath: "/api/dashboard/ai-config/update", SortNo: 1410}
	PermissionAIConfigDelete = Permission{Name: "删除 AI 配置", Code: "aiConfig.delete", Type: "api", GroupName: "aiConfig", Method: "POST", APIPath: "/api/dashboard/ai-config/delete", SortNo: 1420}

	// 知识库相关权限
	PermissionKnowledgeBaseView   = Permission{Name: "查看知识库", Code: "knowledgeBase.view", Type: "api", GroupName: "knowledgeBase", Method: "ANY", APIPath: "/api/dashboard/knowledge-base/list", SortNo: 1410}
	PermissionKnowledgeBaseCreate = Permission{Name: "创建知识库", Code: "knowledgeBase.create", Type: "api", GroupName: "knowledgeBase", Method: "POST", APIPath: "/api/dashboard/knowledge-base/create", SortNo: 1420}
	PermissionKnowledgeBaseUpdate = Permission{Name: "更新知识库", Code: "knowledgeBase.update", Type: "api", GroupName: "knowledgeBase", Method: "POST", APIPath: "/api/dashboard/knowledge-base/update", SortNo: 1430}
	PermissionKnowledgeBaseDelete = Permission{Name: "删除知识库", Code: "knowledgeBase.delete", Type: "api", GroupName: "knowledgeBase", Method: "POST", APIPath: "/api/dashboard/knowledge-base/delete", SortNo: 1440}

	// 知识文档相关权限
	PermissionKnowledgeDocumentView   = Permission{Name: "查看知识文档", Code: "knowledgeDocument.view", Type: "api", GroupName: "knowledgeDocument", Method: "ANY", APIPath: "/api/dashboard/knowledge-document/list", SortNo: 1510}
	PermissionKnowledgeDocumentCreate = Permission{Name: "创建知识文档", Code: "knowledgeDocument.create", Type: "api", GroupName: "knowledgeDocument", Method: "POST", APIPath: "/api/dashboard/knowledge-document/create", SortNo: 1520}
	PermissionKnowledgeDocumentUpdate = Permission{Name: "更新知识文档", Code: "knowledgeDocument.update", Type: "api", GroupName: "knowledgeDocument", Method: "POST", APIPath: "/api/dashboard/knowledge-document/update", SortNo: 1530}
	PermissionKnowledgeDocumentDelete = Permission{Name: "删除知识文档", Code: "knowledgeDocument.delete", Type: "api", GroupName: "knowledgeDocument", Method: "POST", APIPath: "/api/dashboard/knowledge-document/delete", SortNo: 1540}
	PermissionKnowledgeFAQView        = Permission{Name: "查看知识FAQ", Code: "knowledgeFAQ.view", Type: "api", GroupName: "knowledgeFAQ", Method: "ANY", APIPath: "/api/dashboard/knowledge-faq/list", SortNo: 1550}
	PermissionKnowledgeFAQCreate      = Permission{Name: "创建知识FAQ", Code: "knowledgeFAQ.create", Type: "api", GroupName: "knowledgeFAQ", Method: "POST", APIPath: "/api/dashboard/knowledge-faq/create", SortNo: 1560}
	PermissionKnowledgeFAQUpdate      = Permission{Name: "更新知识FAQ", Code: "knowledgeFAQ.update", Type: "api", GroupName: "knowledgeFAQ", Method: "POST", APIPath: "/api/dashboard/knowledge-faq/update", SortNo: 1570}
	PermissionKnowledgeFAQDelete      = Permission{Name: "删除知识FAQ", Code: "knowledgeFAQ.delete", Type: "api", GroupName: "knowledgeFAQ", Method: "POST", APIPath: "/api/dashboard/knowledge-faq/delete", SortNo: 1580}

	// Skill 定义相关权限
	PermissionSkillDefinitionView   = Permission{Name: "查看技能定义", Code: "skillDefinition.view", Type: "api", GroupName: "skillDefinition", Method: "ANY", APIPath: "/api/dashboard/skill-definition/list", SortNo: 1610}
	PermissionSkillDefinitionCreate = Permission{Name: "创建技能定义", Code: "skillDefinition.create", Type: "api", GroupName: "skillDefinition", Method: "POST", APIPath: "/api/dashboard/skill-definition/create", SortNo: 1620}
	PermissionSkillDefinitionUpdate = Permission{Name: "更新技能定义", Code: "skillDefinition.update", Type: "api", GroupName: "skillDefinition", Method: "POST", APIPath: "/api/dashboard/skill-definition/update", SortNo: 1630}
	PermissionSkillDefinitionDelete = Permission{Name: "删除技能定义", Code: "skillDefinition.delete", Type: "api", GroupName: "skillDefinition", Method: "POST", APIPath: "/api/dashboard/skill-definition/delete", SortNo: 1640}

	// MCP 调试相关权限
	PermissionMCPView = Permission{Name: "查看MCP调试信息", Code: "mcp.view", Type: "api", GroupName: "mcp", Method: "POST", APIPath: "/api/dashboard/mcp/list_tools", SortNo: 1710}
	PermissionMCPCall = Permission{Name: "调用MCP工具", Code: "mcp.call", Type: "api", GroupName: "mcp", Method: "POST", APIPath: "/api/dashboard/mcp/call_tool", SortNo: 1720}

	// 隐私与合规权限。数据导出、删除和泄露通知不得复用普通工单权限。
	PermissionPrivacyRequestView   = Permission{Name: "查看隐私请求", Code: "privacyRequest.view", Type: "api", GroupName: "privacy", Method: "GET", APIPath: "/api/enterprise/v1/gdpr/dsar/list", SortNo: 1730}
	PermissionPrivacyRequestManage = Permission{Name: "处理隐私请求", Code: "privacyRequest.manage", Type: "api", GroupName: "privacy", Method: "POST", APIPath: "/api/enterprise/v1/gdpr/dsar/:id/execute", SortNo: 1740}
	PermissionDataBreachView       = Permission{Name: "查看数据泄露事件", Code: "dataBreach.view", Type: "api", GroupName: "privacy", Method: "GET", APIPath: "/api/enterprise/v1/gdpr/breach/list", SortNo: 1750}
	PermissionDataBreachManage     = Permission{Name: "处理数据泄露事件", Code: "dataBreach.manage", Type: "api", GroupName: "privacy", Method: "POST", APIPath: "/api/enterprise/v1/gdpr/breach/report", SortNo: 1760}
	PermissionDataRetentionView    = Permission{Name: "查看数据保留策略", Code: "dataRetention.view", Type: "api", GroupName: "privacy", Method: "GET", APIPath: "/api/enterprise/v1/gdpr/retention-policy", SortNo: 1770}

	// 平台审计治理权限。
	PermissionAuditRetentionManage = Permission{Name: "管理审计留存策略", Code: "audit.retention.manage", Type: "api", GroupName: "audit", Method: "POST", APIPath: "/api/platform/audit/retention", SortNo: 1780}

	// 系统介绍文档权限（平台端，供客户门户访客预览）。
	PermissionSystemIntroView   = Permission{Name: "查看系统介绍文档", Code: "systemIntro.view", Type: "api", GroupName: "systemIntro", Method: "GET", APIPath: "/api/platform/system-intro/list", SortNo: 1950}
	PermissionSystemIntroCreate = Permission{Name: "上传系统介绍文档", Code: "systemIntro.create", Type: "api", GroupName: "systemIntro", Method: "POST", APIPath: "/api/platform/system-intro/create", SortNo: 1960}
	PermissionSystemIntroUpdate = Permission{Name: "编辑系统介绍文档", Code: "systemIntro.update", Type: "api", GroupName: "systemIntro", Method: "POST", APIPath: "/api/platform/system-intro/update", SortNo: 1970}
	PermissionSystemIntroDelete = Permission{Name: "删除系统介绍文档", Code: "systemIntro.delete", Type: "api", GroupName: "systemIntro", Method: "POST", APIPath: "/api/platform/system-intro/delete", SortNo: 1980}
)

// Permissions 内置权限列表
var Permissions = []Permission{
	PermissionUserView,
	PermissionUserCreate,
	PermissionUserUpdate,
	PermissionUserDelete,
	PermissionUserAssignRole,
	PermissionRoleView,
	PermissionRoleCreate,
	PermissionRoleUpdate,
	PermissionRoleDelete,
	PermissionRoleAssignPermission,
	PermissionPermissionView,
	PermissionPermissionSync,
	PermissionSessionView,
	PermissionSessionRevoke,
	PermissionConversationView,
	PermissionConversationAssign,
	PermissionConversationTransfer,
	PermissionConversationClose,
	PermissionConversationSend,
	PermissionConversationTag,
	PermissionConversationHandover,
	PermissionConversationRecycle,
	PermissionConversationLinkCustomer,
	PermissionTicketView,
	PermissionTicketCreate,
	PermissionTicketUpdate,
	PermissionTicketAssign,
	PermissionTicketChangeStatus,
	PermissionTicketProgress,
	PermissionTicketRepairHistoryView,
	PermissionNotificationView,
	PermissionNotificationUpdate,
	PermissionNotificationChannelManage,
	PermissionQuickReplyView,
	PermissionQuickReplyCreate,
	PermissionQuickReplyUpdate,
	PermissionQuickReplyDelete,
	PermissionTagView,
	PermissionTagCreate,
	PermissionTagUpdate,
	PermissionTagDelete,
	PermissionCompanyView,
	PermissionCompanyCreate,
	PermissionCompanyUpdate,
	PermissionCompanyDelete,
	PermissionProductView,
	PermissionProductCreate,
	PermissionProductUpdate,
	PermissionProductDelete,
	PermissionProductServiceProfileView,
	PermissionProductServiceProfileCreate,
	PermissionProductServiceProfileUpdate,
	PermissionProductServiceProfileDelete,
	PermissionProductKnowledgeBindingView,
	PermissionProductKnowledgeBindingCreate,
	PermissionProductKnowledgeBindingUpdate,
	PermissionProductKnowledgeBindingDelete,
	PermissionProductKnowledgeBindingResolve,
	PermissionTenantIntegrationConfigView, PermissionTenantIntegrationConfigCreate, PermissionTenantIntegrationConfigUpdate, PermissionTenantIntegrationConfigDelete,
	PermissionProductAIUsageCredentialView, PermissionProductAIUsageCredentialCreate, PermissionProductAIUsageCredentialUpdate, PermissionProductAIUsageCredentialDelete,
	PermissionMeetingPreview,
	PermissionMeetingCreate,
	PermissionMeetingView,
	PermissionMeetingUpdate,
	PermissionProductModelView,
	PermissionProductModelCreate,
	PermissionProductModelUpdate,
	PermissionProductModelDelete,
	PermissionDeviceView,
	PermissionDeviceCreate,
	PermissionDeviceUpdate,
	PermissionDeviceDelete,
	PermissionServiceCodeBatchView,
	PermissionServiceCodeBatchCreate,
	PermissionServiceCodeBatchUpdate,
	PermissionServiceCodeBatchDelete,
	PermissionServiceCodeView,
	PermissionServiceCodeGenerate,
	PermissionServiceCodeRevoke,
	PermissionChannelView,
	PermissionChannelCreate,
	PermissionChannelUpdate,
	PermissionChannelDelete,
	PermissionCustomerView,
	PermissionCustomerCreate,
	PermissionCustomerUpdate,
	PermissionCustomerDelete,
	PermissionTenantView,
	PermissionTenantCreate,
	PermissionTenantUpdate,
	PermissionTenantDelete,
	PermissionTenantExport,
	PermissionReportView,
	PermissionReportExport,
	PermissionPartnerMemberView,
	PermissionPartnerMemberInvite,
	PermissionPartnerMemberUpdate,
	PermissionCustomerMemberView,
	PermissionCustomerMemberInvite,
	PermissionCustomerMemberUpdate,
	PermissionFinanceView,
	PermissionFinanceManage,
	PermissionAgentView,
	PermissionAgentCreate,
	PermissionAgentUpdate,
	PermissionAgentDelete,
	PermissionAgentUpdateStatus,
	PermissionAgentConfig,
	PermissionAgentTeamView,
	PermissionAgentTeamCreate,
	PermissionAgentTeamUpdate,
	PermissionAgentTeamDelete,
	PermissionAgentTeamScheduleView,
	PermissionAgentTeamScheduleUpdate,
	PermissionAssetView,
	PermissionAssetCreate,
	PermissionAssetDelete,
	PermissionAIAgentView,
	PermissionAIAgentCreate,
	PermissionAIAgentUpdate,
	PermissionAIAgentDelete,
	PermissionAIWorkflowView,
	PermissionAIWorkflowUpdate,
	PermissionAIWorkflowPublish,
	PermissionAIAgentReleaseCreate,
	PermissionAIAgentReleaseReview,
	PermissionAIAgentReleaseDeploy,
	PermissionAIConfigView,
	PermissionAIConfigCreate,
	PermissionAIConfigUpdate,
	PermissionAIConfigDelete,
	PermissionKnowledgeBaseView,
	PermissionKnowledgeBaseCreate,
	PermissionKnowledgeBaseUpdate,
	PermissionKnowledgeBaseDelete,
	PermissionKnowledgeDocumentView,
	PermissionKnowledgeDocumentCreate,
	PermissionKnowledgeDocumentUpdate,
	PermissionKnowledgeDocumentDelete,
	PermissionKnowledgeFAQView,
	PermissionKnowledgeFAQCreate,
	PermissionKnowledgeFAQUpdate,
	PermissionKnowledgeFAQDelete,
	PermissionSkillDefinitionView,
	PermissionSkillDefinitionCreate,
	PermissionSkillDefinitionUpdate,
	PermissionSkillDefinitionDelete,
	PermissionMCPView,
	PermissionMCPCall,
	PermissionPrivacyRequestView,
	PermissionPrivacyRequestManage,
	PermissionDataBreachView,
	PermissionDataBreachManage,
	PermissionDataRetentionView,
	PermissionAuditRetentionManage,
	PermissionSystemIntroView,
	PermissionSystemIntroCreate,
	PermissionSystemIntroUpdate,
	PermissionSystemIntroDelete,
}

// PermissionMap 权限映射，用于通过 Code 查找 Permission
var PermissionMap = make(map[string]Permission)

// init 初始化 PermissionMap
func init() {
	normalizeBuiltinPermissionNames()
	for _, permission := range Permissions {
		PermissionMap[permission.Code] = permission
	}
}

func normalizeBuiltinPermissionNames() {
	for i := range Permissions {
		Permissions[i].Name = builtinPermissionName(Permissions[i].Code, Permissions[i].Name)
	}
}

func builtinPermissionName(code string, fallback string) string {
	if name, ok := builtinPermissionNameOverrides[code]; ok {
		return name
	}
	resourceKey, actionKey, ok := splitPermissionCode(code)
	if !ok {
		return fallback
	}
	action, ok := builtinPermissionActionLabels[actionKey]
	if !ok {
		return fallback
	}
	resource, ok := builtinPermissionResourceLabels[resourceKey]
	if !ok {
		return fallback
	}
	return action + " " + resource
}

func splitPermissionCode(code string) (string, string, bool) {
	for i := 0; i < len(code); i++ {
		if code[i] == '.' {
			return code[:i], code[i+1:], i > 0 && i < len(code)-1
		}
	}
	return "", "", false
}

var builtinPermissionActionLabels = map[string]string{
	"view":             "View",
	"create":           "Create",
	"update":           "Update",
	"delete":           "Delete",
	"assignRole":       "Assign roles to",
	"assignPermission": "Assign permissions to",
	"sync":             "Sync",
	"revoke":           "Revoke",
	"assign":           "Assign",
	"transfer":         "Transfer",
	"close":            "Close",
	"send":             "Send",
	"tag":              "Manage tags for",
	"handover":         "Handle handoffs for",
	"recycle":          "Recycle",
	"linkCustomer":     "Link customers to",
	"changeStatus":     "Change status for",
	"progress":         "Update progress for",
	"updateStatus":     "Update status for",
	"config":           "Configure service rules for",
	"batchGenerate":    "Batch generate",
	"call":             "Call",
	"invite":           "Invite",
	"export":           "Export",
	"manage":           "Manage",
}

var builtinPermissionResourceLabels = map[string]string{
	"user":                     "users",
	"role":                     "roles",
	"permission":               "permissions",
	"session":                  "sessions",
	"audit":                    "audit policies",
	"conversation":             "conversations",
	"ticket":                   "tickets",
	"notification":             "notifications",
	"quickReply":               "quick replies",
	"tag":                      "tags",
	"company":                  "companies",
	"product":                  "products",
	"productServiceProfile":    "product service profiles",
	"tenantIntegrationConfig":  "tenant integration configurations",
	"productAIUsageCredential": "product AI usage credentials",
	"meeting":                  "meetings",
	"productModel":             "product models",
	"device":                   "devices",
	"serviceCodeBatch":         "service code batches",
	"serviceCode":              "service codes",
	"channel":                  "channels",
	"customer":                 "customers",
	"tenant":                   "tenants",
	"report":                   "reports",
	"partnerMember":            "partner members",
	"customerMember":           "customer members",
	"finance":                  "finance and billing",
	"agent":                    "agents",
	"agentTeam":                "agent teams",
	"agentTeamSchedule":        "agent team schedules",
	"asset":                    "file assets",
	"aiAgent":                  "AI Agents",
	"aiConfig":                 "AI configurations",
	"knowledgeBase":            "knowledge bases",
	"knowledgeDocument":        "knowledge documents",
	"knowledgeFAQ":             "knowledge FAQs",
	"skillDefinition":          "Skill definitions",
	"mcp":                      "MCP tools",
}

var builtinPermissionNameOverrides = map[string]string{
	"user.assignRole":           "Assign user roles",
	"role.assignPermission":     "Assign role permissions",
	"session.revoke":            "Revoke sessions",
	"audit.retention.manage":    "Manage audit retention",
	"conversation.send":         "Send conversation messages",
	"conversation.linkCustomer": "Link conversation customer",
	"ticket.changeStatus":       "Change ticket status",
	"ticket.progress":           "Update ticket progress",
	"agent.config":              "Configure agent service rules",
	"agentTeamSchedule.view":    "View team availability",
	"agentTeamSchedule.update":  "Manage availability and leave",
	"serviceCode.generate":      "Generate service codes",
	"serviceCode.revoke":        "Revoke service codes",
	"mcp.view":                  "View MCP debug information",
	"mcp.call":                  "Call MCP tools",
	"finance.view":              "View finance and billing",
	"finance.manage":            "Manage recharge, plans, and quotas",
}

type RoleSpec struct {
	Name   string
	Code   string
	SortNo int
}

var Roles = []RoleSpec{
	{Name: "Super Admin", Code: RoleCodeSuperAdmin, SortNo: 1},
	{Name: "Admin", Code: RoleCodeAdmin, SortNo: 2},
	{Name: "Support Team Lead", Code: RoleCodeCsTeamLeader, SortNo: 3},
	{Name: "Support Agent", Code: RoleCodeCsUser, SortNo: 4},
}

var RolePermissions = map[string][]Permission{
	RoleCodeSuperAdmin: Permissions,
	RoleCodeAdmin: {
		PermissionUserView, PermissionUserCreate, PermissionUserUpdate, PermissionUserAssignRole,
		PermissionRoleView, PermissionRoleCreate, PermissionRoleUpdate, PermissionRoleAssignPermission,
		PermissionPermissionView, PermissionPermissionSync,
		PermissionSessionView, PermissionSessionRevoke,
		PermissionConversationView, PermissionConversationAssign, PermissionConversationTransfer, PermissionConversationClose, PermissionConversationSend, PermissionConversationTag, PermissionConversationHandover, PermissionConversationRecycle, PermissionConversationLinkCustomer,
		PermissionTicketView, PermissionTicketCreate, PermissionTicketUpdate, PermissionTicketAssign, PermissionTicketChangeStatus, PermissionTicketProgress, PermissionTicketRepairHistoryView,
		PermissionMeetingPreview,
		PermissionMeetingCreate,
		PermissionMeetingView,
		PermissionMeetingUpdate,
		PermissionNotificationView, PermissionNotificationUpdate, PermissionNotificationChannelManage,
		PermissionReportView, PermissionReportExport,
		PermissionQuickReplyView, PermissionQuickReplyCreate, PermissionQuickReplyUpdate, PermissionQuickReplyDelete,
		PermissionTagView, PermissionTagCreate, PermissionTagUpdate, PermissionTagDelete,
		PermissionCompanyView, PermissionCompanyCreate, PermissionCompanyUpdate, PermissionCompanyDelete,
		PermissionProductView, PermissionProductCreate, PermissionProductUpdate, PermissionProductDelete,
		PermissionProductServiceProfileView, PermissionProductServiceProfileCreate, PermissionProductServiceProfileUpdate, PermissionProductServiceProfileDelete,
		PermissionTenantIntegrationConfigView, PermissionTenantIntegrationConfigCreate, PermissionTenantIntegrationConfigUpdate, PermissionTenantIntegrationConfigDelete,
		PermissionProductAIUsageCredentialView, PermissionProductAIUsageCredentialCreate, PermissionProductAIUsageCredentialUpdate, PermissionProductAIUsageCredentialDelete,
		PermissionMeetingPreview,
		PermissionMeetingCreate,
		PermissionMeetingView,
		PermissionMeetingUpdate,
		PermissionProductKnowledgeBindingView, PermissionProductKnowledgeBindingCreate, PermissionProductKnowledgeBindingUpdate, PermissionProductKnowledgeBindingDelete, PermissionProductKnowledgeBindingResolve,
		PermissionProductModelView, PermissionProductModelCreate, PermissionProductModelUpdate, PermissionProductModelDelete,
		PermissionDeviceView, PermissionDeviceCreate, PermissionDeviceUpdate, PermissionDeviceDelete,
		PermissionServiceCodeBatchView, PermissionServiceCodeBatchCreate, PermissionServiceCodeBatchUpdate, PermissionServiceCodeBatchDelete,
		PermissionServiceCodeView, PermissionServiceCodeGenerate, PermissionServiceCodeRevoke,
		PermissionChannelView, PermissionChannelCreate, PermissionChannelUpdate, PermissionChannelDelete,
		PermissionCustomerView, PermissionCustomerCreate, PermissionCustomerUpdate, PermissionCustomerDelete,
		PermissionAgentView, PermissionAgentCreate, PermissionAgentUpdate, PermissionAgentDelete, PermissionAgentUpdateStatus, PermissionAgentConfig,
		PermissionAgentTeamView, PermissionAgentTeamCreate, PermissionAgentTeamUpdate, PermissionAgentTeamDelete,
		PermissionAgentTeamScheduleView, PermissionAgentTeamScheduleUpdate,
		PermissionAssetView, PermissionAssetCreate, PermissionAssetDelete,
		PermissionAIAgentView, PermissionAIAgentCreate, PermissionAIAgentUpdate, PermissionAIAgentDelete,
		PermissionAIWorkflowView, PermissionAIWorkflowUpdate, PermissionAIWorkflowPublish,
		PermissionAIAgentReleaseCreate, PermissionAIAgentReleaseReview, PermissionAIAgentReleaseDeploy,
		PermissionAIConfigView, PermissionAIConfigCreate, PermissionAIConfigUpdate, PermissionAIConfigDelete,
		PermissionKnowledgeBaseView, PermissionKnowledgeBaseCreate, PermissionKnowledgeBaseUpdate, PermissionKnowledgeBaseDelete,
		PermissionKnowledgeDocumentView, PermissionKnowledgeDocumentCreate, PermissionKnowledgeDocumentUpdate, PermissionKnowledgeDocumentDelete,
		PermissionKnowledgeFAQView, PermissionKnowledgeFAQCreate, PermissionKnowledgeFAQUpdate, PermissionKnowledgeFAQDelete,
		PermissionSkillDefinitionView, PermissionSkillDefinitionCreate, PermissionSkillDefinitionUpdate, PermissionSkillDefinitionDelete,
		PermissionPrivacyRequestView, PermissionPrivacyRequestManage,
		PermissionDataBreachView, PermissionDataBreachManage, PermissionDataRetentionView,
	},
	RoleCodeCsTeamLeader: {
		PermissionUserView,
		PermissionRoleView,
		PermissionPermissionView,
		PermissionSessionView,
		PermissionConversationView, PermissionConversationClose, PermissionConversationSend, PermissionConversationTag, PermissionConversationHandover, PermissionConversationRecycle, PermissionConversationLinkCustomer,
		PermissionTicketView, PermissionTicketCreate, PermissionTicketUpdate, PermissionTicketAssign, PermissionTicketChangeStatus, PermissionTicketProgress, PermissionTicketRepairHistoryView,
		PermissionMeetingPreview,
		PermissionMeetingCreate,
		PermissionMeetingView,
		PermissionMeetingUpdate,
		PermissionNotificationView, PermissionNotificationUpdate, PermissionNotificationChannelManage,
		PermissionReportView,
		PermissionQuickReplyView, PermissionQuickReplyCreate, PermissionQuickReplyUpdate, PermissionQuickReplyDelete,
		PermissionTagView, PermissionTagCreate, PermissionTagUpdate, PermissionTagDelete,
		PermissionCompanyView,
		PermissionProductView,
		PermissionProductServiceProfileView,
		PermissionProductModelView,
		PermissionDeviceView,
		PermissionServiceCodeBatchView, PermissionServiceCodeBatchCreate, PermissionServiceCodeBatchUpdate,
		PermissionServiceCodeView, PermissionServiceCodeGenerate, PermissionServiceCodeRevoke,
		PermissionChannelView, PermissionChannelCreate, PermissionChannelUpdate,
		PermissionCustomerView, PermissionCustomerCreate, PermissionCustomerUpdate,
		PermissionAgentView, PermissionAgentUpdate,
		PermissionAgentTeamView,
		PermissionAgentTeamScheduleView, PermissionAgentTeamScheduleUpdate,
		PermissionAssetView, PermissionAssetCreate, PermissionAssetDelete,
		PermissionAIAgentView, PermissionAIAgentCreate, PermissionAIAgentUpdate,
		PermissionAIWorkflowView, PermissionAIWorkflowUpdate, PermissionAIWorkflowPublish,
		PermissionAIAgentReleaseCreate, PermissionAIAgentReleaseReview, PermissionAIAgentReleaseDeploy,
		PermissionAIConfigView,
		PermissionSkillDefinitionView, PermissionSkillDefinitionCreate, PermissionSkillDefinitionUpdate,
	},
	RoleCodeCsUser: {
		PermissionUserView,
		PermissionRoleView,
		PermissionPermissionView,
		PermissionConversationView,
		PermissionTicketView, PermissionTicketCreate, PermissionTicketAssign, PermissionTicketChangeStatus, PermissionTicketProgress, PermissionTicketRepairHistoryView,
		PermissionMeetingPreview,
		PermissionMeetingCreate,
		PermissionMeetingView,
		PermissionMeetingUpdate,
		PermissionNotificationView, PermissionNotificationUpdate,
		PermissionQuickReplyView,
		PermissionTagView,
		PermissionCompanyView,
		PermissionProductView,
		PermissionProductServiceProfileView,
		PermissionProductModelView,
		PermissionDeviceView,
		PermissionServiceCodeBatchView,
		PermissionServiceCodeView,
		PermissionChannelView,
		PermissionCustomerView,
		PermissionAssetView,
		PermissionAgentView,
		PermissionAgentTeamView,
		PermissionAgentTeamScheduleView,
		PermissionAIAgentView,
		PermissionAIWorkflowView,
		PermissionAIConfigView,
		PermissionSkillDefinitionView,
	},
}

func PermissionCodes() []string {
	ret := make([]string, 0, len(Permissions))
	for _, permission := range Permissions {
		ret = append(ret, permission.Code)
	}
	return ret
}
