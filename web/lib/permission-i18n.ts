const PERMISSION_ACTION_LABELS: Record<string, string> = {
  view: "View",
  create: "Create",
  update: "Update",
  delete: "Delete",
  assignRole: "Assign roles to",
  assignPermission: "Assign permissions to",
  sync: "Sync",
  revoke: "Revoke",
  assign: "Assign",
  transfer: "Transfer",
  close: "Close",
  send: "Send",
  tag: "Manage tags for",
  handover: "Handle handoffs for",
  recycle: "Recycle",
  linkCustomer: "Link customers to",
  changeStatus: "Change status for",
  progress: "Update progress for",
  resetUserTokenSecret: "Reset user token secret for",
  updateStatus: "Update status for",
  config: "Configure service rules for",
  call: "Call",
  invite: "Invite",
  export: "Export",
  generate: "Generate",
  batchGenerate: "Batch generate",
  preview: "Preview",
  manage: "Manage",
  resolve: "Resolve",
  publish: "Publish",
  review: "Review",
  deploy: "Deploy",
  "channel.manage": "Manage channels for",
  "repairHistory.view": "View repair history for",
}

const PERMISSION_ACTION_LABELS_ZH: Record<string, string> = {
  view: "查看",
  create: "创建",
  update: "更新",
  delete: "删除",
  sync: "同步",
  revoke: "撤销",
  assign: "分配",
  transfer: "转接",
  close: "关闭",
  send: "发送",
  tag: "管理标签",
  handover: "转交",
  recycle: "回收",
  linkCustomer: "关联客户",
  changeStatus: "变更状态",
  progress: "更新进度",
  resetUserTokenSecret: "重置用户令牌密钥",
  updateStatus: "更新状态",
  config: "配置",
  call: "调用",
  invite: "邀请",
  export: "导出",
  generate: "生成",
  batchGenerate: "批量生成",
  preview: "预览",
  manage: "管理",
  resolve: "解析",
  publish: "发布",
  review: "审核",
  deploy: "部署",
}

const PERMISSION_ACTION_LABELS_ES: Record<string, string> = {
  view: "Ver",
  create: "Crear",
  update: "Actualizar",
  delete: "Eliminar",
  assignRole: "Asignar roles a",
  assignPermission: "Asignar permisos a",
  sync: "Sincronizar",
  revoke: "Revocar",
  assign: "Asignar",
  transfer: "Transferir",
  close: "Cerrar",
  send: "Enviar",
  tag: "Gestionar etiquetas de",
  handover: "Gestionar traspasos de",
  recycle: "Reciclar",
  linkCustomer: "Vincular clientes a",
  changeStatus: "Cambiar estado de",
  progress: "Actualizar progreso de",
  resetUserTokenSecret: "Restablecer secreto de token de usuario de",
  updateStatus: "Actualizar estado de",
  config: "Configurar reglas de servicio de",
  call: "Llamar",
  invite: "Invitar",
  export: "Exportar",
  generate: "Generar",
  batchGenerate: "Generar en lote",
  preview: "Vista previa",
  manage: "Gestionar",
  resolve: "Resolver",
  publish: "Publicar",
  review: "Revisar",
  deploy: "Desplegar",
  "channel.manage": "Gestionar canales de",
  "repairHistory.view": "Ver historial de reparaciones de",
}

const PERMISSION_RESOURCE_LABELS: Record<string, { singular: string; plural: string }> = {
  user: { singular: "employee account", plural: "employee accounts" },
  role: { singular: "role", plural: "roles" },
  permission: { singular: "permission catalog item", plural: "permission catalog" },
  session: { singular: "session", plural: "sessions" },
  audit: { singular: "audit policy", plural: "audit policies" },
  conversation: { singular: "conversation", plural: "conversations" },
  ticket: { singular: "ticket", plural: "tickets" },
  notification: { singular: "notification", plural: "notifications" },
  quickReply: { singular: "quick reply", plural: "quick replies" },
  tag: { singular: "tag", plural: "tags" },
  company: { singular: "company", plural: "companies" },
  channel: { singular: "channel", plural: "channels" },
  customer: { singular: "customer", plural: "customers" },
  tenant: { singular: "tenant", plural: "tenants" },
  product: { singular: "product", plural: "products" },
  productModel: { singular: "product model", plural: "product models" },
  productServiceProfile: { singular: "product service profile", plural: "product service profiles" },
  productKnowledgeBinding: { singular: "product knowledge binding", plural: "product knowledge bindings" },
  productAIUsageCredential: { singular: "product model usage credential", plural: "product model usage credentials" },
  finance: { singular: "finance and billing", plural: "finance and billing" },
  device: { singular: "device", plural: "devices" },
  serviceCode: { singular: "device service code", plural: "device service codes" },
  serviceCodeBatch: { singular: "service-code batch", plural: "service-code batches" },
  meeting: { singular: "meeting", plural: "meetings" },
  report: { singular: "report", plural: "reports" },
  partnerMember: { singular: "supplier account", plural: "supplier accounts" },
  customerMember: { singular: "customer account", plural: "customer accounts" },
  agent: { singular: "engineer", plural: "engineers" },
  agentTeam: { singular: "engineer team", plural: "engineer teams" },
  agentTeamSchedule: { singular: "engineer schedule", plural: "engineer schedules" },
  asset: { singular: "file asset", plural: "file assets" },
  aiAgent: { singular: "reception configuration", plural: "reception configurations" },
  aiWorkflow: { singular: "workflow", plural: "workflows" },
  aiAgentRelease: { singular: "reception configuration release", plural: "reception configuration releases" },
  aiConfig: { singular: "model configuration", plural: "model configurations" },
  knowledgeBase: { singular: "knowledge base", plural: "knowledge bases" },
  knowledgeDocument: { singular: "knowledge document", plural: "knowledge documents" },
  knowledgeFAQ: { singular: "knowledge FAQ", plural: "knowledge FAQs" },
  skillDefinition: { singular: "skill definition", plural: "skill definitions" },
  mcp: { singular: "MCP tool", plural: "MCP tools" },
  tenantIntegrationConfig: { singular: "integration connector", plural: "integration connectors" },
  privacy: { singular: "privacy and compliance", plural: "privacy and compliance" },
  privacyRequest: { singular: "privacy request", plural: "privacy requests" },
  dataBreach: { singular: "data breach", plural: "data breaches" },
  dataRetention: { singular: "data retention policy", plural: "data retention policies" },
}

const PERMISSION_RESOURCE_LABELS_ZH: Record<string, string> = {
  user: "员工账号",
  role: "角色",
  permission: "权限目录",
  session: "登录会话",
  audit: "审计治理",
  conversation: "服务会话",
  ticket: "工单",
  notification: "消息通知",
  quickReply: "快捷回复",
  tag: "标签",
  company: "企业资料",
  channel: "渠道",
  customer: "客户",
  tenant: "租户",
  product: "产品",
  productModel: "产品型号",
  productServiceProfile: "产品服务配置",
  productKnowledgeBinding: "产品知识绑定",
  productAIUsageCredential: "产品模型用量凭证",
  finance: "财务与计费",
  device: "设备",
  serviceCode: "设备服务码",
  serviceCodeBatch: "服务码批次",
  meeting: "视频协作",
  report: "报表",
  partnerMember: "供应商账号",
  customerMember: "客户账号",
  agent: "工程师",
  agentTeam: "工程师团队",
  agentTeamSchedule: "工程师排班",
  asset: "文件资产",
  aiAgent: "接待配置",
  aiWorkflow: "会话流程",
  aiAgentRelease: "接待配置上线包",
  aiConfig: "模型配置",
  knowledgeBase: "知识库",
  knowledgeDocument: "知识文档",
  knowledgeFAQ: "知识 FAQ",
  skillDefinition: "技能定义",
  mcp: "MCP 工具",
  tenantIntegrationConfig: "企业系统接入",
  privacyRequest: "隐私请求",
  dataBreach: "数据泄露事件",
  dataRetention: "数据保留策略",
  privacy: "隐私与合规",
}

const PERMISSION_RESOURCE_LABELS_ES: Record<string, { singular: string; plural: string }> = {
  user: { singular: "cuenta de empleado", plural: "cuentas de empleado" },
  role: { singular: "rol", plural: "roles" },
  permission: { singular: "elemento del catalogo de permisos", plural: "catalogo de permisos" },
  session: { singular: "sesion", plural: "sesiones" },
  audit: { singular: "politica de auditoria", plural: "politicas de auditoria" },
  conversation: { singular: "conversacion", plural: "conversaciones" },
  ticket: { singular: "ticket", plural: "tickets" },
  notification: { singular: "notificacion", plural: "notificaciones" },
  quickReply: { singular: "respuesta rapida", plural: "respuestas rapidas" },
  tag: { singular: "etiqueta", plural: "etiquetas" },
  company: { singular: "empresa", plural: "empresas" },
  channel: { singular: "canal", plural: "canales" },
  customer: { singular: "cliente", plural: "clientes" },
  tenant: { singular: "tenant", plural: "tenants" },
  product: { singular: "producto", plural: "productos" },
  productModel: { singular: "modelo de producto", plural: "modelos de producto" },
  productServiceProfile: { singular: "perfil de servicio de producto", plural: "perfiles de servicio de producto" },
  productKnowledgeBinding: { singular: "vinculacion de conocimiento de producto", plural: "vinculaciones de conocimiento de producto" },
  productAIUsageCredential: { singular: "credencial de uso de modelo de producto", plural: "credenciales de uso de modelo de producto" },
  finance: { singular: "finanzas y facturacion", plural: "finanzas y facturacion" },
  device: { singular: "dispositivo", plural: "dispositivos" },
  serviceCode: { singular: "codigo de servicio de dispositivo", plural: "codigos de servicio de dispositivo" },
  serviceCodeBatch: { singular: "lote de codigos de servicio", plural: "lotes de codigos de servicio" },
  meeting: { singular: "colaboracion de video", plural: "colaboraciones de video" },
  report: { singular: "informe", plural: "informes" },
  partnerMember: { singular: "cuenta de proveedor", plural: "cuentas de proveedor" },
  customerMember: { singular: "cuenta de cliente", plural: "cuentas de cliente" },
  agent: { singular: "ingeniero", plural: "ingenieros" },
  agentTeam: { singular: "equipo de ingenieros", plural: "equipos de ingenieros" },
  agentTeamSchedule: { singular: "horario de ingenieros", plural: "horarios de ingenieros" },
  asset: { singular: "archivo", plural: "archivos" },
  aiAgent: { singular: "configuracion de recepcion", plural: "configuraciones de recepcion" },
  aiWorkflow: { singular: "flujo de trabajo", plural: "flujos de trabajo" },
  aiAgentRelease: { singular: "paquete de recepcion", plural: "paquetes de recepcion" },
  aiConfig: { singular: "configuracion de modelo", plural: "configuraciones de modelo" },
  knowledgeBase: { singular: "base de conocimiento", plural: "bases de conocimiento" },
  knowledgeDocument: { singular: "documento de conocimiento", plural: "documentos de conocimiento" },
  knowledgeFAQ: { singular: "FAQ de conocimiento", plural: "FAQ de conocimiento" },
  skillDefinition: { singular: "definicion de habilidad", plural: "definiciones de habilidad" },
  mcp: { singular: "herramienta MCP", plural: "herramientas MCP" },
  tenantIntegrationConfig: { singular: "conector de integracion", plural: "conectores de integracion" },
  privacy: { singular: "privacidad y cumplimiento", plural: "privacidad y cumplimiento" },
  privacyRequest: { singular: "solicitud de privacidad", plural: "solicitudes de privacidad" },
  dataBreach: { singular: "fuga de datos", plural: "fugas de datos" },
  dataRetention: { singular: "politica de retencion de datos", plural: "politicas de retencion de datos" },
}

const PERMISSION_NAME_OVERRIDES: Record<string, string> = {
  "user.assignRole": "Assign employee roles",
  "permission.sync": "Sync permission catalog",
  "role.assignPermission": "Configure role permissions",
  "session.revoke": "Revoke sessions",
  "audit.retention.manage": "Manage audit retention",
  "tenant.delete": "Decommission tenants",
  "tenant.export": "Export tenant data",
  "conversation.handover": "Manage conversation handoffs",
  "conversation.linkCustomer": "Link customer to conversation",
  "ticket.changeStatus": "Change ticket status",
  "ticket.progress": "Update ticket progress",
  "meeting.update": "End meetings",
  "meeting.preview": "Preview meeting creation",
  "channel.resetUserTokenSecret": "Reset channel user token secret",
  "asset.create": "Upload file assets",
  "serviceCodeBatch.update": "Update service-code batch status",
  "agent.config": "Configure engineer service rules",
  "agent.updateStatus": "Update engineer status",
  "agentTeamSchedule.view": "View engineer schedules",
  "agentTeamSchedule.update": "Update engineer schedules",
  "agentTeamSchedule.batchGenerate": "Batch generate engineer schedules",
  "aiWorkflow.update": "Edit workflow draft",
  "aiWorkflow.publish": "Publish workflow",
  "aiAgentRelease.create": "Create reception configuration release",
  "aiAgentRelease.review": "Review reception configuration release",
  "aiAgentRelease.deploy": "Deploy reception configuration release",
  "mcp.view": "View MCP debug information",
  "mcp.call": "Call MCP tools",
  "finance.view": "View finance and billing",
  "finance.manage": "Manage recharge, plans, and quotas",
}

const PERMISSION_NAME_OVERRIDES_ZH: Record<string, string> = {
  "user.assignRole": "分配员工角色",
  "permission.sync": "同步权限目录",
  "role.assignPermission": "配置角色权限",
  "session.revoke": "下线登录会话",
  "audit.retention.manage": "管理审计留存策略",
  "tenant.delete": "租户退租处理",
  "tenant.export": "导出租户数据",
  "conversation.send": "发送会话消息",
  "conversation.tag": "管理会话标签",
  "conversation.handover": "处理会话转人工",
  "conversation.linkCustomer": "关联会话客户",
  "ticket.changeStatus": "变更工单状态",
  "ticket.progress": "更新工单处理进展",
  "meeting.update": "结束视频协作",
  "meeting.preview": "预览视频协作创建",
  "ticket.repairHistory.view": "查看工单维修历史",
  "notification.channel.manage": "管理消息通知渠道",
  "channel.resetUserTokenSecret": "重置渠道用户令牌密钥",
  "agent.config": "配置工程师服务规则",
  "agent.updateStatus": "更新工程师状态",
  "agentTeamSchedule.view": "查看工程师排班",
  "agentTeamSchedule.update": "更新工程师排班",
  "agentTeamSchedule.batchGenerate": "批量生成工程师排班",
  "asset.create": "上传文件资产",
  "serviceCode.generate": "生成设备服务码",
  "serviceCode.revoke": "撤销设备服务码",
  "aiAgent.view": "查看接待配置",
  "aiAgent.create": "创建接待配置",
  "aiAgent.update": "更新接待配置",
  "aiAgent.delete": "删除接待配置",
  "aiWorkflow.update": "编辑会话流程",
  "aiWorkflow.publish": "发布会话流程",
  "aiAgentRelease.create": "生成接待配置上线包",
  "aiAgentRelease.review": "审核接待配置上线包",
  "aiAgentRelease.deploy": "部署接待配置上线包",
  "mcp.view": "查看 MCP 调试信息",
  "mcp.call": "调用 MCP 工具",
  "finance.view": "查看财务与计费",
  "finance.manage": "管理充值、套餐与额度",
}

const PERMISSION_NAME_OVERRIDES_ES: Record<string, string> = {
  "user.assignRole": "Asignar roles de empleados",
  "permission.sync": "Sincronizar catalogo de permisos",
  "role.assignPermission": "Configurar permisos de roles",
  "session.revoke": "Revocar sesiones",
  "audit.retention.manage": "Gestionar retencion de auditoria",
  "tenant.delete": "Retirar tenants",
  "tenant.export": "Exportar datos de tenant",
  "conversation.handover": "Gestionar traspasos de conversacion",
  "conversation.linkCustomer": "Vincular cliente a conversacion",
  "ticket.changeStatus": "Cambiar estado del ticket",
  "ticket.progress": "Actualizar progreso del ticket",
  "meeting.update": "Finalizar colaboraciones de video",
  "meeting.preview": "Vista previa de creacion de colaboracion de video",
  "channel.resetUserTokenSecret": "Restablecer secreto de token de usuario del canal",
  "asset.create": "Subir archivos",
  "serviceCodeBatch.update": "Actualizar estado del lote de codigos de servicio",
  "agent.config": "Configurar reglas de servicio de ingenieros",
  "agent.updateStatus": "Actualizar estado del ingeniero",
  "agentTeamSchedule.view": "Ver horarios de ingenieros",
  "agentTeamSchedule.update": "Actualizar horarios de ingenieros",
  "agentTeamSchedule.batchGenerate": "Generar horarios de ingenieros en lote",
  "aiWorkflow.update": "Editar borrador del flujo de trabajo",
  "aiWorkflow.publish": "Publicar flujo de trabajo",
  "aiAgentRelease.create": "Crear paquete de configuracion de recepcion",
  "aiAgentRelease.review": "Revisar paquete de configuracion de recepcion",
  "aiAgentRelease.deploy": "Desplegar paquete de configuracion de recepcion",
  "mcp.view": "Ver informacion de depuracion MCP",
  "mcp.call": "Llamar herramientas MCP",
  "finance.view": "Ver finanzas y facturacion",
  "finance.manage": "Gestionar recargas, planes y cuotas",
}

export function getPermissionDisplayName(
  code: string | undefined,
  fallbackName: string,
  locale: string
) {
  const normalizedCode = code?.trim() ?? ""
  if (!normalizedCode) {
    return fallbackName
  }
  const isChinese = locale === "zh-CN"
  const isSpanish = locale === "es-ES"
  const override = isChinese
    ? PERMISSION_NAME_OVERRIDES_ZH[normalizedCode]
    : isSpanish
      ? PERMISSION_NAME_OVERRIDES_ES[normalizedCode]
      : PERMISSION_NAME_OVERRIDES[normalizedCode]
  if (override) {
    return override
  }
  const separatorIndex = normalizedCode.indexOf(".")
  if (separatorIndex <= 0 || separatorIndex === normalizedCode.length - 1) {
    return isChinese ? fallbackName : normalizedCode
  }
  const resourceKey = normalizedCode.slice(0, separatorIndex)
  const actionKey = normalizedCode.slice(separatorIndex + 1)
  if (isChinese) {
    const resource = PERMISSION_RESOURCE_LABELS_ZH[resourceKey]
    const action = PERMISSION_ACTION_LABELS_ZH[actionKey]
    return resource && action ? joinChineseActionResource(action, resource) : fallbackName
  }
  const resource = isSpanish
    ? PERMISSION_RESOURCE_LABELS_ES[resourceKey]
    : PERMISSION_RESOURCE_LABELS[resourceKey]
  const action = isSpanish
    ? PERMISSION_ACTION_LABELS_ES[actionKey]
    : PERMISSION_ACTION_LABELS[actionKey]
  if (!resource || !action) {
    return containsChinese(fallbackName) ? normalizedCode : fallbackName
  }
  return `${action} ${resource.plural}`
}

export function getPermissionGroupName(groupName: string | undefined, locale: string) {
  const normalizedGroupName = groupName?.trim() ?? ""
  if (locale === "zh-CN") {
    return PERMISSION_RESOURCE_LABELS_ZH[normalizedGroupName] ?? normalizedGroupName
  }
  const resource = (locale === "es-ES" ? PERMISSION_RESOURCE_LABELS_ES : PERMISSION_RESOURCE_LABELS)[
    normalizedGroupName
  ]
  if (!resource) {
    return normalizedGroupName
  }
  return sentenceCase(resource.plural)
}

function containsChinese(value: string) {
  return /[\u3400-\u9fff]/.test(value)
}

function sentenceCase(value: string) {
  if (value.startsWith("MCP ")) {
    return value
  }
  return value.charAt(0).toUpperCase() + value.slice(1)
}

function joinChineseActionResource(action: string, resource: string) {
  return /^[A-Za-z0-9]/.test(resource) ? `${action} ${resource}` : `${action}${resource}`
}
