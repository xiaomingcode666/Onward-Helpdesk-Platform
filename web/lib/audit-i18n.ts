type AuditLocale = "zh-CN" | "en-US" | "es-ES"

const AUDIT_ACTION_LABELS: Record<string, string> = {
  "access.query.executed": "访问连接器查询",
  "auth_policy.saved": "权限策略已保存",
  "auth_role.created": "角色已创建",
  "auth_role.updated": "角色已更新",
  "ai_agent_release.approved": "接待配置上线包已通过审核",
  "ai_agent_release.created": "接待配置上线包已生成",
  "ai_agent_release.deployed": "接待配置上线包已部署",
  "ai_agent_release.rejected": "接待配置上线包已驳回",
  "ai_agent_release.rolled_back": "接待配置上线包已回滚",
  "ai_agent_release.submitted": "接待配置上线包已提交审核",
  "audit.exported": "审计记录已导出",
  "audit_retention.updated": "审计留存策略已更新",
  "access.connector.created": "系统连接器已创建",
  "access.connector.tested": "系统连接器连接已测试",
  "conversation.message_recalled": "聊天消息已撤回",
  "conversation.message_sent": "聊天消息已发送",
  "customer_user.authorized": "客户联系人已授权",
  "customer_user.support_session_started": "客户门户代访已开始",
  "department.created": "部门已创建",
  "data.exported": "数据已导出",
  "file.downloaded": "文件已下载",
  "knowledge.published": "知识已发布",
  "jitsi_runtime.updated": "Jitsi 运行配置已更新",
  "meeting.joined": "视频协作已加入",
  "meeting.created": "视频协作已创建",
  "meeting.ended": "视频协作已结束",
  "meeting.join_requested": "视频协作加入已请求",
  "meeting.left": "视频协作已离开",
  "meeting_ar_runtime.updated": "会议 AR 配置已更新",
  "notification.read": "通知已读",
  "notification.read_all": "个人通知已全部标为已读",
  "notification.mail_settings.updated": "通知邮箱配置已更新",
  "notification.preferences.updated": "个人通知偏好已更新",
  "notification.recipient_test.sent": "个人测试邮件已发送",
  "notification.test_email.sent": "通知测试邮件已发送",
  "partner_admin.invited": "供应商管理员已邀请",
  "permission.changed": "权限已变更",
  "plan.created": "套餐已创建",
  "plan.updated": "套餐已更新",
  "platform_staff.disabled": "平台人员已停用",
  "platform_staff.invited": "平台人员已邀请",
  "platform_staff.tenant_granted": "平台人员已授予租户访问权限",
  "platform_integration.connection_tested": "外部服务连接已测试",
  "platform_integration.updated": "外部服务配置已更新",
  "quota.override_created": "租户额度规则已创建",
  "quota.override_updated": "租户额度规则已更新",
  "secret.rotated": "密钥已轮换",
  "speech_runtime.updated": "语音服务配置已更新",
  "sub2api.account_bound": "模型服务账号已绑定",
  "sub2api.account_disabled": "模型服务账号已停用",
  "sub2api.account_enabled": "模型服务账号已启用",
  "sub2api.account_test": "模型服务账号连接测试",
  "sub2api.account_updated": "模型服务账号已更新",
  "subscription.assigned": "企业套餐已分配",
  "tenant.created": "租户已创建",
  "tenant.decommission_undone": "租户退役已撤销",
  "tenant.decommissioned": "租户已退役",
  "tenant.frozen": "租户已冻结",
  "tenant.restored": "租户已恢复",
  "tenant.support_session_started": "平台协助访问已开始",
  "tenant.updated": "租户信息已更新",
  "tenant_administrator.password_reset": "企业管理员密码已重置",
  "tenant_member.invited": "企业成员已邀请",
  "tenant_member.support_session_started": "员工门户代登录已开始",
  "ticket.assigned": "工单已分派",
  "ticket.closed": "工单已关闭",
  "ticket.transitioned": "工单状态已流转",
}

const AUDIT_TARGET_LABELS: Record<string, string> = {
  audit_log: "审计记录",
  audit_retention: "审计留存策略",
  access_connector: "系统连接器",
  ai_agent_release: "接待配置上线包",
  auth_role: "角色",
  auth_role_permission: "角色权限",
  customer_user: "客户联系人",
  department: "部门",
  conversation: "服务会话",
  meeting: "视频协作",
  message: "聊天消息",
  notification: "通知",
  notification_recipient_setting: "个人通知偏好",
  partner_account: "供应商账号",
  partner_company: "供应商企业",
  platform_staff: "平台人员",
  platform_integration: "外部服务",
  platform_tenant_grant: "跨租户授权",
  sub2api_account: "模型服务账号",
  speech_runtime: "语音服务配置",
  tenant: "租户",
  tenant_administrator: "企业管理员",
  tenant_member: "企业成员",
  tenant_plan: "租户套餐",
  tenant_quota_override: "租户额度规则",
  tenant_subscription: "企业套餐",
  tenant_mail_setting: "通知邮箱配置",
}

const AUDIT_SUBJECT_LABELS: Record<string, string> = {
  ai_agent: "接待配置",
  customer_user: "客户联系人",
  enterprise: "企业账号",
  enterprise_member: "企业成员",
  member: "企业成员",
  partner_account: "供应商账号",
  platform: "平台账号",
  platform_staff: "平台人员",
  service_account: "系统服务账号",
  system: "系统",
  temp_visitor: "临时协作身份",
  user: "用户",
}

const AUDIT_RISK_LABELS: Record<string, string> = {
  critical: "严重",
  high: "高风险",
  low: "低风险",
  medium: "中风险",
}

const AUDIT_STATUS_LABELS: Record<string, string> = {
  blocked: "已阻断",
  failure: "失败",
  failed: "失败",
  success: "成功",
}

const ACTION_VERB_LABELS: Record<string, string> = {
  authorized: "已授权",
  approved: "已通过审核",
  assigned: "已分配",
  bound: "已绑定",
  changed: "已变更",
  closed: "已关闭",
  created: "已创建",
  decommission_undone: "退役已撤销",
  decommissioned: "已退役",
  disabled: "已停用",
  downloaded: "已下载",
  enabled: "已启用",
  executed: "已执行",
  frozen: "已冻结",
  granted: "已授权",
  invited: "已邀请",
  joined: "已加入",
  published: "已发布",
  rejected: "已驳回",
  password_reset: "密码已重置",
  restored: "已恢复",
  rolled_back: "已回滚",
  rotated: "已轮换",
  saved: "已保存",
  support_session_started: "协助访问已开始",
  submitted: "已提交审核",
  test: "连接测试",
  transitioned: "状态已流转",
  updated: "已更新",
}

const ACTION_RESOURCE_LABELS: Record<string, string> = {
  access: "访问连接器",
  account: "账号",
  ai_agent_release: "接待配置上线包",
  audit: "审计",
  auth: "权限",
  customer: "客户",
  customer_user: "客户联系人",
  conversation: "服务会话",
  data: "数据",
  department: "部门",
  file: "文件",
  knowledge: "知识",
  jitsi_runtime: "Jitsi 运行配置",
  meeting: "视频协作",
  message: "聊天消息",
  meeting_ar_runtime: "会议 AR 配置",
  notification: "通知",
  partner_admin: "供应商管理员",
  permission: "权限",
  plan: "套餐",
  platform_staff: "平台人员",
  platform_integration: "外部服务",
  quota: "额度",
  secret: "密钥",
  sub2api: "模型服务",
  speech_runtime: "语音服务配置",
  subscription: "企业套餐",
  tenant: "租户",
  tenant_administrator: "企业管理员",
  tenant_member: "企业成员",
  ticket: "工单",
}

const ACTION_VERB_LABELS_EN: Record<string, string> = {
  authorized: "authorized",
  approved: "approved",
  assigned: "assigned",
  bound: "bound",
  changed: "changed",
  closed: "closed",
  created: "created",
  decommission_undone: "decommission undone",
  decommissioned: "decommissioned",
  disabled: "disabled",
  downloaded: "downloaded",
  enabled: "enabled",
  executed: "executed",
  frozen: "frozen",
  granted: "granted",
  invited: "invited",
  joined: "joined",
  published: "published",
  rejected: "rejected",
  password_reset: "password reset",
  restored: "restored",
  rolled_back: "rolled back",
  rotated: "rotated",
  saved: "saved",
  support_session_started: "support session started",
  submitted: "submitted for review",
  test: "connection tested",
  tested: "tested",
  transitioned: "transitioned",
  updated: "updated",
}

const ACTION_VERB_LABELS_ES: Record<string, string> = {
  authorized: "autorizado",
  approved: "aprobado",
  assigned: "asignado",
  bound: "vinculado",
  changed: "modificado",
  closed: "cerrado",
  created: "creado",
  decommission_undone: "retiro revertido",
  decommissioned: "retirado",
  disabled: "desactivado",
  downloaded: "descargado",
  enabled: "activado",
  executed: "ejecutado",
  frozen: "congelado",
  granted: "autorizado",
  invited: "invitado",
  joined: "unido",
  published: "publicado",
  rejected: "rechazado",
  password_reset: "contrasena restablecida",
  restored: "restaurado",
  rolled_back: "revertido",
  rotated: "rotado",
  saved: "guardado",
  support_session_started: "sesion de soporte iniciada",
  submitted: "enviado a revision",
  test: "conexion probada",
  tested: "probado",
  transitioned: "estado actualizado",
  updated: "actualizado",
}

const ACTION_RESOURCE_LABELS_EN: Record<string, string> = {
  access: "Access connector",
  account: "Account",
  ai_agent_release: "Reception release",
  audit: "Audit",
  auth: "Permission",
  customer: "Customer",
  customer_user: "Customer contact",
  conversation: "Conversation",
  data: "Data",
  department: "Department",
  file: "File",
  knowledge: "Knowledge",
  jitsi_runtime: "Jitsi runtime configuration",
  meeting: "Video collaboration",
  message: "Message",
  meeting_ar_runtime: "Meeting AR configuration",
  notification: "Notification",
  partner_admin: "Supplier admin",
  permission: "Permission",
  plan: "Plan",
  platform_staff: "Platform staff",
  platform_integration: "External service",
  quota: "Quota",
  secret: "Secret",
  sub2api: "Model service",
  speech_runtime: "Speech service configuration",
  subscription: "Enterprise plan",
  tenant: "Tenant",
  tenant_administrator: "Enterprise admin",
  tenant_member: "Enterprise member",
  ticket: "Ticket",
}

const ACTION_RESOURCE_LABELS_ES: Record<string, string> = {
  access: "Conector de acceso",
  account: "Cuenta",
  ai_agent_release: "Paquete de recepcion",
  audit: "Auditoria",
  auth: "Permisos",
  customer: "Cliente",
  customer_user: "Contacto de cliente",
  conversation: "Conversacion",
  data: "Datos",
  department: "Departamento",
  file: "Archivo",
  knowledge: "Conocimiento",
  jitsi_runtime: "Configuracion de Jitsi",
  meeting: "Colaboracion de video",
  message: "Mensaje",
  meeting_ar_runtime: "Configuracion AR de reunion",
  notification: "Notificacion",
  partner_admin: "Admin de proveedor",
  permission: "Permiso",
  plan: "Plan",
  platform_staff: "Personal de plataforma",
  platform_integration: "Servicio externo",
  quota: "Cuota",
  secret: "Secreto",
  sub2api: "Servicio de modelos",
  speech_runtime: "Configuracion de voz",
  subscription: "Plan empresarial",
  tenant: "Tenant",
  tenant_administrator: "Admin empresarial",
  tenant_member: "Miembro empresarial",
  ticket: "Ticket",
}

const AUDIT_RISK_LABELS_EN: Record<string, string> = {
  critical: "Critical",
  high: "High risk",
  low: "Low risk",
  medium: "Medium risk",
}

const AUDIT_RISK_LABELS_ES: Record<string, string> = {
  critical: "Critico",
  high: "Riesgo alto",
  low: "Riesgo bajo",
  medium: "Riesgo medio",
}

const AUDIT_STATUS_LABELS_EN: Record<string, string> = {
  blocked: "Blocked",
  failure: "Failed",
  failed: "Failed",
  success: "Success",
}

const AUDIT_STATUS_LABELS_ES: Record<string, string> = {
  blocked: "Bloqueado",
  failure: "Fallido",
  failed: "Fallido",
  success: "Correcto",
}

const ROUTINE_SUCCESS_AUDIT_ACTIONS = new Set([
  "auth_policy.saved",
  "auth_role.created",
  "auth_role.updated",
  "customer_user.authorized",
  "partner_admin.invited",
  "platform_staff.invited",
  "subscription.assigned",
  "tenant.created",
  "tenant_member.invited",
])

export function getAuditActionLabel(action: string, locale: AuditLocale = readCurrentAuditLocale()) {
  const normalized = action.trim()
  if (!normalized) {
    return "-"
  }
  const appLocale = normalizeAuditLocale(locale)
  if (appLocale === "zh-CN") {
    const explicit = AUDIT_ACTION_LABELS[normalized]
    if (explicit) {
      return explicit
    }
  }

  const parts = normalized.split(".").filter(Boolean)
  if (parts.length >= 2) {
    const verb = parts.at(-1) ?? ""
    const resourceKey = parts.slice(0, -1).join(".")
    if (appLocale === "zh-CN") {
      const resource =
        ACTION_RESOURCE_LABELS[resourceKey] ??
        ACTION_RESOURCE_LABELS[parts[0]] ??
        resourceKey.replaceAll("_", " ")
      const verbLabel = ACTION_VERB_LABELS[verb] ?? verb.replaceAll("_", " ")
      return `${resource}${verbLabel}`
    }

    const resourceMap = appLocale === "es-ES" ? ACTION_RESOURCE_LABELS_ES : ACTION_RESOURCE_LABELS_EN
    const verbMap = appLocale === "es-ES" ? ACTION_VERB_LABELS_ES : ACTION_VERB_LABELS_EN
    const resource =
      resourceMap[resourceKey] ??
      resourceMap[parts[0]] ??
      sentenceCaseAudit(resourceKey)
    const verbLabel = verbMap[verb] ?? humanizeAuditIdentifier(verb)
    return `${resource} ${verbLabel}`
  }

  return appLocale === "zh-CN" ? normalized : sentenceCaseAudit(normalized)
}

export function getAuditTargetLabel(targetType: string, locale: AuditLocale = readCurrentAuditLocale()) {
  const normalized = targetType.trim()
  if (!normalized) {
    return "-"
  }
  const appLocale = normalizeAuditLocale(locale)
  if (appLocale === "zh-CN") {
    return AUDIT_TARGET_LABELS[normalized] ?? normalized.replaceAll("_", " ")
  }
  const resourceMap = appLocale === "es-ES" ? ACTION_RESOURCE_LABELS_ES : ACTION_RESOURCE_LABELS_EN
  return resourceMap[normalized] ?? sentenceCaseAudit(normalized)
}

export function getAuditSubjectLabel(subjectType: string, locale: AuditLocale = readCurrentAuditLocale()) {
  const normalized = subjectType.trim()
  if (!normalized) {
    return "-"
  }
  const appLocale = normalizeAuditLocale(locale)
  if (appLocale === "zh-CN") {
    return AUDIT_SUBJECT_LABELS[normalized] ?? normalized.replaceAll("_", " ")
  }
  return sentenceCaseAudit(normalized)
}

export function getAuditRiskLabel(risk: string, locale: AuditLocale = readCurrentAuditLocale()) {
  const normalized = risk.trim()
  const appLocale = normalizeAuditLocale(locale)
  if (appLocale === "zh-CN") {
    return AUDIT_RISK_LABELS[normalized] ?? (normalized || "-")
  }
  const labels = appLocale === "es-ES" ? AUDIT_RISK_LABELS_ES : AUDIT_RISK_LABELS_EN
  return labels[normalized] ?? (normalized ? sentenceCaseAudit(normalized) : "-")
}

export function getAuditStatusLabel(status: string, locale: AuditLocale = readCurrentAuditLocale()) {
  const normalized = status.trim()
  const appLocale = normalizeAuditLocale(locale)
  if (appLocale === "zh-CN") {
    return AUDIT_STATUS_LABELS[normalized] ?? (normalized || "-")
  }
  const labels = appLocale === "es-ES" ? AUDIT_STATUS_LABELS_ES : AUDIT_STATUS_LABELS_EN
  return labels[normalized] ?? (normalized ? sentenceCaseAudit(normalized) : "-")
}

function normalizeAuditLocale(locale: string | undefined): AuditLocale {
  return locale === "en-US" || locale === "es-ES" || locale === "zh-CN" ? locale : "zh-CN"
}

function readCurrentAuditLocale(): AuditLocale {
  if (typeof document !== "undefined") {
    const documentLocale = normalizeAuditLocale(document.documentElement.lang)
    if (documentLocale !== "zh-CN" || document.documentElement.lang === "zh-CN") {
      return documentLocale
    }
  }
  if (typeof window !== "undefined") {
    return normalizeAuditLocale(window.localStorage.getItem("remote-helpdesk-locale") ?? undefined)
  }
  return "zh-CN"
}

function humanizeAuditIdentifier(value: string) {
  return value.replaceAll("_", " ").replaceAll(".", " ")
}

function sentenceCaseAudit(value: string) {
  const humanized = humanizeAuditIdentifier(value)
  return humanized.charAt(0).toUpperCase() + humanized.slice(1)
}

export function getAuditDisplayRiskLevel(event: {
  action?: string
  riskLevel?: string
  status?: string
}) {
  const action = (event.action ?? "").trim()
  const riskLevel = (event.riskLevel ?? "").trim()
  const status = (event.status ?? "").trim().toLowerCase()

  if (status === "success" && ROUTINE_SUCCESS_AUDIT_ACTIONS.has(action)) {
    return "medium"
  }
  return riskLevel
}

export function isPermissionAuditAction(action: string) {
  const normalized = action.trim()
  return (
    normalized.includes("auth") ||
    normalized.includes("permission") ||
    normalized.includes("role") ||
    normalized.includes("policy") ||
    normalized.includes("grant")
  )
}

export function isAuditAlertEvent(event: {
  action?: string
  riskLevel?: string
  status?: string
}) {
  const action = (event.action ?? "").trim().toLowerCase()
  const riskLevel = (event.riskLevel ?? "").trim().toLowerCase()
  const status = (event.status ?? "").trim().toLowerCase()

  if (
    action.startsWith("security.alert") ||
    action.startsWith("alert.") ||
    action === "audit.log.tampered"
  ) {
    return true
  }
  if (["blocked", "failure", "failed", "error"].includes(status)) {
    return true
  }
  return ["critical", "high"].includes(riskLevel) && status !== "success"
}
