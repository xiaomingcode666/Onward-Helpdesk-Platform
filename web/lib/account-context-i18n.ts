type AccountContextLabels = {
  domains: Record<string, string>
  subjects: Record<string, string>
  roles: Record<string, string>
}

const ACCOUNT_CONTEXT_LABELS: Record<"zh-CN" | "en-US" | "es-ES", AccountContextLabels> = {
  "zh-CN": {
    domains: {
      platform: "平台端",
      enterprise: "企业端",
      customer: "客户端",
      partner: "供应商端",
      service_account: "系统服务",
    },
    subjects: {
      platform_staff: "平台人员",
      enterprise_member: "企业成员",
      tenant_member: "企业成员",
      customer_user: "客户用户",
      partner_account: "供应商账号",
      service_account: "系统服务账号",
      temp_visitor: "临时协作身份",
      system: "系统",
      user: "用户",
    },
    roles: {
      super_admin: "超级管理员",
      admin: "系统管理员",
      cs_team_leader: "客服组长",
      cs_user: "客服",
      platform_admin: "平台管理员",
      platform_operations: "平台运营",
      platform_auditor: "平台审计员",
      platform_staff: "平台人员",
      tenant_owner: "租户所有者",
      tenant_admin: "企业管理员",
      tenant_admin_seed: "企业管理员",
      service_manager: "服务负责人",
      service_engineer: "服务工程师",
      knowledge_manager: "知识管理员",
      enterprise_viewer: "企业只读成员",
      partner_admin: "供应商管理员",
      partner_engineer: "供应商工程师",
      customer_admin: "客户管理员",
      customer_user: "客户用户",
      service_account: "系统服务账号",
    },
  },
  "en-US": {
    domains: {
      platform: "Platform portal",
      enterprise: "Enterprise portal",
      customer: "Customer portal",
      partner: "Partner portal",
      service_account: "System service",
    },
    subjects: {
      platform_staff: "Platform staff",
      enterprise_member: "Enterprise member",
      tenant_member: "Enterprise member",
      customer_user: "Customer user",
      partner_account: "Partner account",
      service_account: "System service account",
      temp_visitor: "Temporary visitor",
      system: "System",
      user: "User",
    },
    roles: {
      super_admin: "Super admin",
      admin: "System administrator",
      cs_team_leader: "Support team lead",
      cs_user: "Support agent",
      platform_admin: "Platform administrator",
      platform_operations: "Platform operations",
      platform_auditor: "Platform auditor",
      platform_staff: "Platform staff",
      tenant_owner: "Tenant owner",
      tenant_admin: "Enterprise administrator",
      tenant_admin_seed: "Enterprise administrator",
      service_manager: "Service manager",
      service_engineer: "Service engineer",
      knowledge_manager: "Knowledge manager",
      enterprise_viewer: "Enterprise viewer",
      partner_admin: "Partner administrator",
      partner_engineer: "Partner engineer",
      customer_admin: "Customer administrator",
      customer_user: "Customer user",
      service_account: "System service account",
    },
  },
  "es-ES": {
    domains: {
      platform: "Portal de plataforma",
      enterprise: "Portal de empresa",
      customer: "Portal de cliente",
      partner: "Portal de proveedor",
      service_account: "Servicio del sistema",
    },
    subjects: {
      platform_staff: "Personal de plataforma",
      enterprise_member: "Miembro de empresa",
      tenant_member: "Miembro de empresa",
      customer_user: "Usuario cliente",
      partner_account: "Cuenta de proveedor",
      service_account: "Cuenta de servicio del sistema",
      temp_visitor: "Visitante temporal",
      system: "Sistema",
      user: "Usuario",
    },
    roles: {
      super_admin: "Super administrador",
      admin: "Administrador del sistema",
      cs_team_leader: "Lider del equipo de soporte",
      cs_user: "Agente de soporte",
      platform_admin: "Administrador de plataforma",
      platform_operations: "Operaciones de plataforma",
      platform_auditor: "Auditor de plataforma",
      platform_staff: "Personal de plataforma",
      tenant_owner: "Propietario del tenant",
      tenant_admin: "Administrador de empresa",
      tenant_admin_seed: "Administrador de empresa",
      service_manager: "Responsable de servicio",
      service_engineer: "Ingeniero de servicio",
      knowledge_manager: "Administrador de conocimiento",
      enterprise_viewer: "Miembro de solo lectura",
      partner_admin: "Administrador de proveedor",
      partner_engineer: "Ingeniero de proveedor",
      customer_admin: "Administrador de cliente",
      customer_user: "Usuario cliente",
      service_account: "Cuenta de servicio del sistema",
    },
  },
}

function labelsFor(locale: string) {
  if (locale === "en-US") {
    return ACCOUNT_CONTEXT_LABELS["en-US"]
  }
  if (locale === "es-ES") {
    return ACCOUNT_CONTEXT_LABELS["es-ES"]
  }
  return ACCOUNT_CONTEXT_LABELS["zh-CN"]
}

function normalize(value: string | null | undefined) {
  return value?.trim() ?? ""
}

export function getAccountDomainLabel(value: string | null | undefined, locale: string) {
  const normalized = normalize(value)
  if (!normalized) return "-"
  return labelsFor(locale).domains[normalized] ?? normalized
}

export function getAccountSubjectLabel(
  subjectType: string | null | undefined,
  subjectId: number | null | undefined,
  locale: string
) {
  const normalized = normalize(subjectType)
  if (!normalized) return "-"
  const label = labelsFor(locale).subjects[normalized] ?? normalized
  return subjectId && subjectId > 0 ? `${label} #${subjectId}` : label
}

export function getAccountRoleLabel(value: string | null | undefined, locale: string) {
  const normalized = normalize(value)
  if (!normalized) return "-"
  return labelsFor(locale).roles[normalized] ?? normalized
}
