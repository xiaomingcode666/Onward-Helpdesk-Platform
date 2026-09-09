import type { AppLocale } from "@/i18n/config"
import { customerEntryMessages } from "./customer-entry"
import { dashboardMessages } from "./dashboard"
import { engineerMessages } from "./engineer"
import { enumsMessages } from "./enums"
import { enterpriseMessages } from "./enterprise"
import { exportsMessages } from "./exports"
import { iamMessages } from "./iam"
import { meetingRoomMessages } from "./meeting-room"
import { miscTailMessages } from "./misc-tail"
import { opsComponentsMessages } from "./ops-components"
import { orgMessages } from "./org"
import { partnerMessages } from "./partner"
import { platformMessages } from "./platform"
import { portalMessages } from "./portal"
import { railopsMessages } from "./railops"
import { transferMessages } from "./transfer"
import { workflowMessages } from "./workflow"

const extractedMessageGroups = [
  railopsMessages,
  platformMessages,
  enterpriseMessages,
  workflowMessages,
  portalMessages,
  iamMessages,
  partnerMessages,
  orgMessages,
  customerEntryMessages,
  dashboardMessages,
  engineerMessages,
  meetingRoomMessages,
  opsComponentsMessages,
  transferMessages,
  exportsMessages,
  miscTailMessages,
  enumsMessages,
]

export function getExtractedMessage(locale: AppLocale, key: string): unknown {
  for (const group of extractedMessageGroups) {
    const value = getMessageValue(group[locale], key)
    if (value !== undefined) {
      return value
    }
  }
  return getGeneratedExtractedMessage(locale, key)
}

function getMessageValue(source: unknown, key: string): unknown {
  let current = source
  for (const part of key.split(".")) {
    if (!current || typeof current !== "object" || !(part in current)) {
      return undefined
    }
    current = (current as Record<string, unknown>)[part]
  }
  return current
}

function getGeneratedExtractedMessage(locale: AppLocale, key: string) {
  if (!/^(platformExtract|portalExtract|workflowExtract|enterpriseExtract|iamExtract|partnerExtract|orgExtract|customerEntryExtract|dashboardExtract|engineerExtract|meetingRoomExtract|opsComponentsExtract|transferExtract|exportsExtract|miscTailExtract|enumsExtract)\./.test(key)) {
    return undefined
  }
  const parts = key.split(".")
  const leaf = parts[parts.length - 1]
  if (!leaf) return undefined
  return humanizeExtractedLeaf(locale, leaf)
}

function humanizeExtractedLeaf(locale: AppLocale, leaf: string) {
  const zhGlossary: Record<string, string> = {
    account: "账号",
    accountTotal: "共 {count} 个账号",
    active: "启用",
    adminApiKey: "管理 API Key",
    all: "全部",
    apiKeys: "密钥",
    atRisk: "接近上限",
    balance: "余额",
    bill: "计费",
    block: "硬限制",
    configured: "已配置",
    connectionFailed: "连接失败",
    connectionHealthy: "连接正常",
    connectionHealthyToast: "{name} 连接正常",
    credentialEncrypted: "凭据已加密",
    countUnit: "{count} 个",
    dataUnavailable: "数据暂不可用",
    disabled: "停用",
    editPlanAria: "编辑 {name}",
    enabled: "启用",
    enabledCount: "{count} 已启用",
    failedCalls24h: "24h 失败调用",
    fingerprint: "指纹 {fingerprint}",
    groupFallback: "分组 {id}",
    healthy: "运行正常",
    hidden: "已隐藏",
    itemsNeedAttention: "{count} 项需要关注",
    loading: "加载中",
    never: "从未",
    normal: "正常",
    notConfigured: "未配置",
    notEnabled: "未启用",
    overLimitTenantUnit: "{count} 个超额租户",
    pageTitle: "页面标题",
    pendingEnable: "待启用",
    planCount: "{count} 个套餐",
    refresh: "刷新",
    restricted: "受限",
    saveConfig: "保存配置",
    saving: "保存中",
    status: "状态",
    statusApiUnavailableCount: "{count} 个接入状态接口暂时不可用",
    statusUnavailable: "状态不可用",
    stale24hCount: "{count} 超 24h",
    tenantCountUnit: "{count} 家租户",
    testConnection: "测试连接",
    testing: "测试中",
    total: "共 {count} 条",
    unknown: "未知",
    unlimited: "不限",
    userFallback: "用户 {id}",
    view: "查看",
  }
  const enGlossary: Record<string, string> = {
    accountTotal: "{count} accounts",
    ai: "AI",
    api: "API",
    connectionHealthyToast: "{name} connection healthy",
    countUnit: "{count}",
    editPlanAria: "Edit {name}",
    enabledCount: "{count} enabled",
    fingerprint: "Fingerprint {fingerprint}",
    groupFallback: "Group {id}",
    itemsNeedAttention: "{count} items need attention",
    overLimitTenantUnit: "{count} tenants over limit",
    planCount: "{count} plans",
    statusApiUnavailableCount: "{count} access status APIs unavailable",
    stale24hCount: "{count} stale over 24h",
    tenantCountUnit: "{count} tenants",
    total: "{count} total",
    ttl: "TTL",
    url: "URL",
    userFallback: "User {id}",
  }
  const esGlossary: Record<string, string> = {
    accountTotal: "{count} cuentas",
    ai: "IA",
    api: "API",
    connectionHealthyToast: "Conexion de {name} normal",
    countUnit: "{count}",
    editPlanAria: "Editar {name}",
    enabledCount: "{count} activados",
    fingerprint: "Huella {fingerprint}",
    groupFallback: "Grupo {id}",
    itemsNeedAttention: "{count} elementos requieren atencion",
    overLimitTenantUnit: "{count} tenants excedidos",
    planCount: "{count} planes",
    statusApiUnavailableCount: "{count} APIs de estado no disponibles",
    stale24hCount: "{count} con mas de 24h",
    tenantCountUnit: "{count} tenants",
    total: "{count} en total",
    ttl: "TTL",
    url: "URL",
    userFallback: "Usuario {id}",
  }
  if (locale === "zh-CN" && zhGlossary[leaf]) {
    return zhGlossary[leaf]
  }
  const words = leaf
    .replace(/([a-z0-9])([A-Z])/g, "$1 $2")
    .replace(/[_-]+/g, " ")
    .trim()
    .split(/\s+/)
  const glossary = locale === "es-ES" ? esGlossary : enGlossary
  return words
    .map((word) => glossary[word.toLowerCase()] ?? `${word.charAt(0).toUpperCase()}${word.slice(1)}`)
    .join(" ")
}
