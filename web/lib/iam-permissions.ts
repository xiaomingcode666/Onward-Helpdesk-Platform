import { getPermissionDisplayName, getPermissionGroupName } from "@/lib/permission-i18n"

export type IAMPermissionCatalogItem = {
  apiPath: string
  code: string
  groupName: string
  method: string
  name: string
  sortNo?: number
  type: string
}

export type IAMPermissionDetail = IAMPermissionCatalogItem & {
  action: string
}

export type IAMPermissionGroup = {
  key: string
  label: string
  permissions: IAMPermissionDetail[]
}

const permissionGroupLabels: Record<string, string> = {
  platform: "平台端",
  tenant: "租户治理",
  user: "人员与账号",
  role: "角色管理",
  permission: "权限目录",
  session: "登录会话与审计",
  audit: "审计治理",
  product: "产品档案",
  productModel: "产品型号",
  productServiceProfile: "产品服务配置",
  productKnowledgeBinding: "产品知识绑定",
  productAIUsageCredential: "产品模型用量",
  finance: "财务与计费",
  device: "设备管理",
  serviceCode: "设备服务码",
  serviceCodeBatch: "服务码批次",
  conversation: "服务会话",
  ticket: "工单与维修",
  meeting: "视频协作",
  notification: "消息通知",
  report: "报表分析",
  customer: "客户管理",
  customerMember: "客户账号",
  partnerMember: "供应商账号",
  knowledgeBase: "知识库",
  knowledgeDocument: "知识文档",
  knowledgeFAQ: "知识 FAQ",
  aiAgent: "接待配置",
  aiWorkflow: "会话流程",
  aiAgentRelease: "接待配置上线包",
  aiConfig: "模型配置",
  asset: "文件资产",
  tenantIntegrationConfig: "企业系统接入",
  channel: "渠道接入",
  company: "企业资料",
  agent: "工程师",
  agentTeam: "工程师团队",
  agentTeamSchedule: "工程师排班",
  quickReply: "快捷回复",
  tag: "标签管理",
  skillDefinition: "技能定义",
  mcp: "MCP 工具",
  privacy: "隐私与合规",
  privacyRequest: "隐私请求",
  dataBreach: "数据泄露事件",
  dataRetention: "数据保留策略",
}

const permissionGroupLabelsEn: Record<string, string> = {
  platform: "Platform",
  tenant: "Tenant governance",
  user: "Employees and accounts",
  role: "Role management",
  permission: "Permission catalog",
  session: "Sessions and audit",
  audit: "Audit governance",
  product: "Products",
  productModel: "Product models",
  productServiceProfile: "Product service profiles",
  productKnowledgeBinding: "Product knowledge bindings",
  productAIUsageCredential: "Product model usage credentials",
  finance: "Finance and billing",
  device: "Devices",
  serviceCode: "Device service codes",
  serviceCodeBatch: "Service-code batches",
  conversation: "Service conversations",
  ticket: "Tickets and repair",
  meeting: "Video collaboration",
  notification: "Notifications",
  report: "Reports",
  customer: "Customers",
  customerMember: "Customer accounts",
  partnerMember: "Supplier accounts",
  knowledgeBase: "Knowledge bases",
  knowledgeDocument: "Knowledge documents",
  knowledgeFAQ: "Knowledge FAQs",
  aiAgent: "Reception profiles",
  aiWorkflow: "Workflows",
  aiAgentRelease: "Reception profile releases",
  aiConfig: "Provider configurations",
  asset: "File assets",
  tenantIntegrationConfig: "Integration connectors",
  channel: "Channels",
  company: "Company profile",
  agent: "Engineers",
  agentTeam: "Engineer teams",
  agentTeamSchedule: "Engineer schedules",
  quickReply: "Quick replies",
  tag: "Tags",
  skillDefinition: "Skill definitions",
  mcp: "MCP tools",
  privacy: "Privacy and compliance",
  privacyRequest: "Privacy requests",
  dataBreach: "Data breaches",
  dataRetention: "Data retention policies",
}

const permissionGroupLabelsEs: Record<string, string> = {
  platform: "Plataforma",
  tenant: "Gobernanza de tenants",
  user: "Empleados y cuentas",
  role: "Gestion de roles",
  permission: "Catalogo de permisos",
  session: "Sesiones y auditoria",
  audit: "Gobernanza de auditoria",
  product: "Productos",
  productModel: "Modelos de producto",
  productServiceProfile: "Perfiles de servicio de producto",
  productKnowledgeBinding: "Vinculaciones de conocimiento de producto",
  productAIUsageCredential: "Credenciales de uso de modelo de producto",
  finance: "Finanzas y facturacion",
  device: "Dispositivos",
  serviceCode: "Codigos de servicio de dispositivo",
  serviceCodeBatch: "Lotes de codigos de servicio",
  conversation: "Conversaciones de servicio",
  ticket: "Tickets y reparaciones",
  meeting: "Colaboracion de video",
  notification: "Notificaciones",
  report: "Informes",
  customer: "Clientes",
  customerMember: "Cuentas de cliente",
  partnerMember: "Cuentas de proveedor",
  knowledgeBase: "Bases de conocimiento",
  knowledgeDocument: "Documentos de conocimiento",
  knowledgeFAQ: "FAQ de conocimiento",
  aiAgent: "Perfiles de recepcion",
  aiWorkflow: "Flujos de trabajo",
  aiAgentRelease: "Paquetes de recepcion",
  aiConfig: "Configuraciones de proveedor",
  asset: "Archivos",
  tenantIntegrationConfig: "Conectores de integracion",
  channel: "Canales",
  company: "Perfil de empresa",
  agent: "Ingenieros",
  agentTeam: "Equipos de ingenieros",
  agentTeamSchedule: "Horarios de ingenieros",
  quickReply: "Respuestas rapidas",
  tag: "Etiquetas",
  skillDefinition: "Definiciones de habilidad",
  mcp: "Herramientas MCP",
  privacy: "Privacidad y cumplimiento",
  privacyRequest: "Solicitudes de privacidad",
  dataBreach: "Fugas de datos",
  dataRetention: "Politicas de retencion de datos",
}

const permissionCodeLabels: Record<string, string> = {
  "platform.audit.view": "审计日志",
  "platform.staff.manage": "平台人员",
  "platform.tenant.view": "租户管理",
}

const preferredGroupOrder = Object.keys(permissionGroupLabels)
const menuPermissionCodeOrder = [
  "platform.staff.manage",
  "platform.tenant.view",
  "platform.audit.view",
]

export function getIAMPermissionGroupLabel(groupName: string, locale = "zh-CN") {
  if (locale === "en-US") {
    return permissionGroupLabelsEn[groupName] || getPermissionGroupName(groupName, locale) || groupName || "Other permissions"
  }
  if (locale === "es-ES") {
    return permissionGroupLabelsEs[groupName] || getPermissionGroupName(groupName, locale) || groupName || "Otros permisos"
  }
  return permissionGroupLabels[groupName] || getPermissionGroupName(groupName, locale) || groupName || "其他权限"
}

export function groupIAMPermissions(
  permissionCodes: readonly string[],
  catalog: readonly IAMPermissionCatalogItem[],
  menuGroupOrder: readonly string[] = [],
  locale = "zh-CN"
): IAMPermissionGroup[] {
  const catalogByCode = new Map(catalog.map((item) => [item.code, item]))
  const groups = new Map<string, IAMPermissionDetail[]>()
  const orderedCodes = [...new Set(permissionCodes)].sort((left, right) => {
    const leftMenuIndex = menuPermissionCodeOrder.indexOf(left)
    const rightMenuIndex = menuPermissionCodeOrder.indexOf(right)
    if (leftMenuIndex !== -1 || rightMenuIndex !== -1) {
      if (leftMenuIndex === -1) return 1
      if (rightMenuIndex === -1) return -1
      return leftMenuIndex - rightMenuIndex
    }
    const leftSortNo = catalogByCode.get(left)?.sortNo
    const rightSortNo = catalogByCode.get(right)?.sortNo
    if (leftSortNo !== undefined || rightSortNo !== undefined) {
      if (leftSortNo === undefined) return 1
      if (rightSortNo === undefined) return -1
      if (leftSortNo !== rightSortNo) return leftSortNo - rightSortNo
    }
    return left.localeCompare(right)
  })

  for (const code of orderedCodes) {
    const catalogItem = catalogByCode.get(code)
    const groupName = catalogItem?.groupName || code.split(".")[0] || "other"
    const action = code.includes(".") ? code.slice(code.indexOf(".") + 1) : code
    const detail: IAMPermissionDetail = {
      apiPath: catalogItem?.apiPath ?? "",
      code,
      groupName,
      method: catalogItem?.method ?? "",
      name: permissionCodeLabels[code]
        ?? getPermissionDisplayName(code, catalogItem?.name ?? code, locale),
      type: catalogItem?.type ?? "api",
      action,
    }
    groups.set(groupName, [...(groups.get(groupName) ?? []), detail])
  }

  return [...groups.entries()]
    .sort(([left], [right]) => {
      const leftMenuIndex = menuGroupOrder.indexOf(left)
      const rightMenuIndex = menuGroupOrder.indexOf(right)
      if (leftMenuIndex !== -1 || rightMenuIndex !== -1) {
        if (leftMenuIndex === -1) return 1
        if (rightMenuIndex === -1) return -1
        return leftMenuIndex - rightMenuIndex
      }
      const leftFallbackIndex = preferredGroupOrder.indexOf(left)
      const rightFallbackIndex = preferredGroupOrder.indexOf(right)
      if (leftFallbackIndex === -1 && rightFallbackIndex === -1) return left.localeCompare(right)
      if (leftFallbackIndex === -1) return 1
      if (rightFallbackIndex === -1) return -1
      return leftFallbackIndex - rightFallbackIndex
    })
    .map(([key, permissions]) => ({
      key,
      label: getIAMPermissionGroupLabel(key, locale),
      permissions,
    }))
}
