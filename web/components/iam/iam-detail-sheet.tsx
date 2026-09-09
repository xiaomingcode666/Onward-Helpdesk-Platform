"use client"

import { useMemo, useState, type ReactNode } from "react"
import {
  CheckCircle2Icon,
  ChevronDownIcon,
  CircleUserRoundIcon,
  Code2Icon,
  KeyRoundIcon,
  Layers3Icon,
  ScrollTextIcon,
  ShieldCheckIcon,
  WrenchIcon,
} from "lucide-react"
import { DetailDrawer, SearchField, StatusTag } from "@railops/ui"

import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible"
import type { AdminPermission } from "@/lib/api/admin"
import { useAppLocale, useI18n } from "@/i18n/provider"
import type {
  EnterpriseIAMCustomerUser,
  EnterpriseIAMMember,
  EnterpriseIAMPartner,
  IAMAuditLog,
  IAMRole,
} from "@/lib/api/platform-iam"
import {
  getAuditActionLabel,
  getAuditStatusLabel,
  getAuditSubjectLabel,
  getAuditTargetLabel,
} from "@/lib/audit-i18n"
import { groupIAMPermissions } from "@/lib/iam-permissions"
import {
  getRemoteHelpdeskPermissionCategories,
  getRemoteHelpdeskPermissionGroupOrder,
} from "@/lib/navigation-remote-helpdesk"
import { translateCurrentMessage } from "@/i18n/messages"

const iamTitleClass = "text-base font-semibold text-foreground"
const iamMutedTextClass = "text-muted-foreground"
const iamPrimaryIconClass = "flex size-9 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary"
const iamSectionClass = "mt-5 border-y border-border py-4"
const iamPanelClass = "rounded-md border border-border bg-muted/50"
const iamDividerClass = "border-border"

function tr(key: string, values?: Record<string, string | number>) {
  return translateCurrentMessage(`iamExtract.${key}`, values)
}

export type IAMWorkspaceDetail =
  | { id: string; kind: "role"; role: IAMRole }
  | { id: string; kind: "enterprise-member"; member: EnterpriseIAMMember }
  | { id: string; kind: "enterprise-customer-user"; customerUser: EnterpriseIAMCustomerUser }
  | { id: string; kind: "enterprise-partner"; partner: EnterpriseIAMPartner }
  | { id: string; kind: "audit-log"; audit: IAMAuditLog; scope: "enterprise" | "platform" }

export function IAMDetailSheet({
  catalog,
  detail,
  onOpenChange,
  open,
}: {
  catalog: AdminPermission[]
  detail: IAMWorkspaceDetail | null
  onOpenChange: (open: boolean) => void
  open: boolean
}) {
  return (
    <DetailDrawer
      open={open}
      onClose={() => onOpenChange(false)}
      title={detail ? <IAMDetailHeader detail={detail} /> : undefined}
      styles={{ body: { padding: 0 } }}
    >
      {detail?.kind === "role" ? (
        <RoleDetail key={detail.id} catalog={catalog} role={detail.role} />
      ) : detail?.kind === "enterprise-member" ? (
        <MemberDetail member={detail.member} />
      ) : detail?.kind === "enterprise-customer-user" ? (
        <CustomerUserDetail customerUser={detail.customerUser} />
      ) : detail?.kind === "enterprise-partner" ? (
        <PartnerDetail partner={detail.partner} />
      ) : detail?.kind === "audit-log" ? (
        <AuditDetail key={detail.id} audit={detail.audit} scope={detail.scope} />
      ) : null}
    </DetailDrawer>
  )
}

function IAMDetailHeader({ detail }: { detail: IAMWorkspaceDetail }) {
  switch (detail.kind) {
    case "role":
      return (
        <div className="flex items-start gap-3">
          <div className={iamPrimaryIconClass}>
            <KeyRoundIcon className="size-4" />
          </div>
          <div className="min-w-0">
            <div className={iamTitleClass}>{detail.role.name}</div>
            <div className={`mt-1 break-all font-mono text-xs ${iamMutedTextClass}`}>
              {detail.role.code}
            </div>
          </div>
        </div>
      )
    case "enterprise-member":
      return (
        <div className="flex items-start gap-3">
          <div className={iamPrimaryIconClass}>
            <CircleUserRoundIcon className="size-4" />
          </div>
          <div className="min-w-0">
            <div className={iamTitleClass}>
              {detail.member.display_name || detail.member.username}
            </div>
            <div className={`mt-1 break-all text-xs ${iamMutedTextClass}`}>
              {detail.member.username || detail.member.member_no || `member:${detail.member.id}`}
            </div>
          </div>
        </div>
      )
    case "enterprise-customer-user":
      return (
        <div className="flex items-start gap-3">
          <div className={iamPrimaryIconClass}>
            <CircleUserRoundIcon className="size-4" />
          </div>
          <div className="min-w-0">
            <div className={iamTitleClass}>
              {detail.customerUser.display_name || detail.customerUser.email || tr("workspace.detail.customerUserWithId", { id: detail.customerUser.id })}
            </div>
            <div className={`mt-1 break-all text-xs ${iamMutedTextClass}`}>
              {detail.customerUser.email || detail.customerUser.phone || `customer:${detail.customerUser.id}`}
            </div>
          </div>
        </div>
      )
    case "enterprise-partner":
      return (
        <div className="flex items-start gap-3">
          <div className={iamPrimaryIconClass}>
            <ShieldCheckIcon className="size-4" />
          </div>
          <div className="min-w-0">
            <div className={iamTitleClass}>
              {detail.partner.name || detail.partner.partner_no || tr("workspace.detail.supplierWithId", { id: detail.partner.id })}
            </div>
            <div className={`mt-1 break-all font-mono text-xs ${iamMutedTextClass}`}>
              {detail.partner.partner_no || `partner:${detail.partner.id}`}
            </div>
          </div>
        </div>
      )
    case "audit-log":
      return (
        <div className="flex items-start gap-3">
          <div className={iamPrimaryIconClass}>
            <ScrollTextIcon className="size-4" />
          </div>
          <div className="min-w-0">
            <div className={iamTitleClass}>
              {getAuditActionLabel(detail.audit.action)}
            </div>
            <div className={`mt-1 text-xs ${iamMutedTextClass}`}>
              {detail.audit.id < 0 ? tr("common.businessAudit", { id: Math.abs(detail.audit.id) }) : tr("common.authAudit", { id: detail.audit.id })} · {detail.audit.occurredAt || tr("common.timeUnknown")}
            </div>
          </div>
        </div>
      )
  }
}

function AuditDetail({ audit, scope }: { audit: IAMAuditLog; scope: "enterprise" | "platform" }) {
  const t = useI18n()
  const [technicalOpen, setTechnicalOpen] = useState(false)
  const actor = getAuditSubjectLabel(audit.actorSubjectType || "")
  const target = getAuditTargetLabel(audit.targetType || "")
  const currentEnterprise = scope === "enterprise" && audit.targetType === "tenant" && String(audit.tenantId) === audit.targetId
  const failed = ["blocked", "error", "failed", "failure"].includes(audit.status.toLowerCase())
  const chatFields = chatAuditDetailFields(audit)

  return (
    <>
      <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
        <div className="flex flex-wrap gap-2">
          <StatusTag tone={failed ? "error" : "neutral"}>{getAuditStatusLabel(audit.status)}</StatusTag>
          <StatusTag tone="neutral">{audit.domainType === "platform" ? t("iamExtract.detail.domainPlatform") : audit.domainType === "partner" ? t("iamExtract.detail.domainPartner") : t("iamExtract.detail.domainEnterprise")}</StatusTag>
          {audit.supportGrantId > 0 ? <StatusTag tone="neutral">{t("iamExtract.workspace.metrics.assistedAccess")}</StatusTag> : null}
        </div>

        <section className={iamSectionClass}>
          <h3 className="text-sm font-semibold text-foreground">{t("iamExtract.detail.operationInfo")}</h3>
          <dl className="mt-3 grid gap-x-5 gap-y-4 sm:grid-cols-2">
            <DetailTerm label={t("iamExtract.detail.actor")} value={`${actor}${audit.actorSubjectId ? ` #${audit.actorSubjectId}` : ""}`} />
            <DetailTerm label={t("iamExtract.detail.actorAccountId")} value={String(audit.actorUserId || "-")} mono />
            <DetailTerm label={t("iamExtract.detail.target")} value={currentEnterprise ? t("iamExtract.common.currentEnterprise") : `${target}${audit.targetId ? ` #${audit.targetId}` : ""}`} />
            <DetailTerm label={t("iamExtract.detail.enterpriseId")} value={String(audit.tenantId || "-")} mono />
            <DetailTerm label={t("iamExtract.detail.eventTime")} value={audit.occurredAt || "-"} />
            <DetailTerm label={t("iamExtract.detail.auditResult")} value={getAuditStatusLabel(audit.status)} />
            {audit.supportGrantId > 0 ? <DetailTerm label={t("iamExtract.detail.supportGrant")} value={`#${audit.supportGrantId}`} mono /> : null}
            {audit.requestId ? <DetailTerm label={t("iamExtract.common.requestId")} value={audit.requestId} mono /> : null}
            {audit.ipAddress ? <DetailTerm label={t("iamExtract.detail.ipAddress")} value={audit.ipAddress} mono /> : null}
            {audit.userAgent ? <DetailTerm label={t("iamExtract.detail.userAgent")} value={audit.userAgent} /> : null}
          </dl>
        </section>

        {chatFields.length ? (
          <section className={iamSectionClass}>
            <h3 className="text-sm font-semibold text-foreground">{t("iamExtract.detail.chatRecords")}</h3>
            <dl className="mt-3 grid gap-x-5 gap-y-4 sm:grid-cols-2">
              {chatFields.map((field) => (
                <DetailTerm key={field.label} label={field.label} value={field.value} mono={field.mono} />
              ))}
            </dl>
          </section>
        ) : null}

        <Collapsible
          className="py-2"
          open={technicalOpen}
          onOpenChange={setTechnicalOpen}
        >
          <CollapsibleTrigger className="flex w-full items-center justify-between gap-3 py-2 text-left">
            <span className="flex items-center gap-2 text-sm font-semibold text-foreground">
              <Code2Icon className={`size-4 ${iamMutedTextClass}`} />
              {t("iamExtract.detail.technicalInfo")}
            </span>
            <ChevronDownIcon className={`size-4 ${iamMutedTextClass} transition-transform ${technicalOpen ? "rotate-180" : ""}`} />
          </CollapsibleTrigger>
          <CollapsibleContent className="pb-2">
            <div className={`mt-3 px-3 py-2 ${iamPanelClass}`}>
              <div className={`text-xs font-medium ${iamMutedTextClass}`}>{t("iamExtract.detail.actionCode")}</div>
              <code className="mt-1 block break-all text-xs text-foreground">{audit.action}</code>
            </div>
          </CollapsibleContent>
        </Collapsible>
      </div>
    </>
  )
}

function chatAuditDetailFields(audit: IAMAuditLog) {
  if (!isChatAudit(audit)) {
    return []
  }
  const metadata = audit.metadata ?? {}
  const fields = [
    { label: tr("detail.serviceSession"), value: metadata.conversationId ? `#${metadata.conversationId}` : "", mono: true },
    { label: tr("detail.messageId"), value: metadata.messageId || audit.targetId ? `#${metadata.messageId || audit.targetId}` : "", mono: true },
    { label: tr("detail.sender"), value: chatSenderLabel(metadata.senderType, metadata.senderName, metadata.senderId) },
    { label: tr("detail.messageType"), value: chatMessageTypeLabel(metadata.messageType) },
    { label: tr("detail.contentSummary"), value: metadata.contentSummary || audit.summary || "" },
    { label: tr("detail.clientMessageId"), value: metadata.clientMsgId || "", mono: true },
    { label: tr("detail.workflowRun"), value: metadata.workflowRunId ? `#${metadata.workflowRunId}` : "", mono: true },
    { label: tr("detail.collaboration"), value: metadata.collaborationId ? `#${metadata.collaborationId}` : "", mono: true },
    { label: tr("detail.ticket"), value: metadata.ticketId ? `#${metadata.ticketId}` : "", mono: true },
  ]
  return fields.filter((field) => field.value)
}

function isChatAudit(audit: IAMAuditLog) {
  return audit.targetType === "message" ||
    audit.targetType === "conversation_message" ||
    audit.action.startsWith("conversation.message_") ||
    audit.action.startsWith("message.")
}

function chatSenderLabel(senderType?: string, senderName?: string, senderId?: string) {
  const typeLabel = senderType === "customer"
    ? tr("detail.senderCustomer")
    : senderType === "agent"
      ? tr("detail.senderAgent")
      : senderType === "ai"
        ? "AI"
        : senderType === "partner"
          ? tr("detail.senderPartner")
          : senderType === "system"
            ? tr("detail.senderSystem")
            : senderType || tr("common.unknown")
  const name = senderName?.trim()
  const id = senderId?.trim()
  if (name && id && id !== "0") return `${typeLabel} · ${name} #${id}`
  if (name) return `${typeLabel} · ${name}`
  if (id && id !== "0") return `${typeLabel} #${id}`
  return typeLabel
}

function chatMessageTypeLabel(messageType?: string) {
  switch (messageType) {
    case "text":
      return tr("detail.chatMessageType.text")
    case "image":
      return tr("detail.chatMessageType.image")
    case "file":
      return tr("detail.chatMessageType.file")
    case "audio":
      return tr("detail.chatMessageType.audio")
    case "system":
      return tr("detail.chatMessageType.system")
    default:
      return messageType || tr("common.unknown")
  }
}

function RoleDetail({ catalog, role }: { catalog: AdminPermission[]; role: IAMRole }) {
  const t = useI18n()
  const { locale } = useAppLocale()
  const [search, setSearch] = useState("")
  const permissionDomain = role.domainType === "platform" ? "platform" : "enterprise"
  const menuCategories = useMemo(
    () => getRemoteHelpdeskPermissionCategories(permissionDomain),
    [permissionDomain]
  )
  const menuGroupOrder = useMemo(
    () => getRemoteHelpdeskPermissionGroupOrder(permissionDomain),
    [permissionDomain]
  )
  const groups = useMemo(
    () => groupIAMPermissions(role.permissions ?? [], catalog, menuGroupOrder, locale),
    [catalog, locale, menuGroupOrder, role.permissions]
  )
  const categories = useMemo(() => {
    const groupByKey = new Map(groups.map((group) => [group.key, group]))
    const assigned = new Set<string>()
    const menuBacked = menuCategories.map((category) => ({
      key: category.key,
      label: t(category.labelKey),
      groups: category.groupKeys.flatMap((key) => {
        const group = groupByKey.get(key)
        if (!group) return []
        assigned.add(key)
        return [group]
      }),
    })).filter((category) => category.groups.length > 0)
    const remaining = groups.filter((group) => !assigned.has(group.key))
    if (!remaining.length) return menuBacked
    return [
      ...menuBacked,
      {
        key: "other-capabilities",
        label: t("remoteSidebar.otherCapabilities"),
        groups: remaining,
      },
    ]
  }, [groups, menuCategories, t])
  const [openCategoryKey, setOpenCategoryKey] = useState<string | null>(categories[0]?.key ?? null)
  const [openGroupKey, setOpenGroupKey] = useState<string | null>(categories[0]?.groups[0]?.key ?? null)
  const filteredCategories = useMemo(() => {
    const keyword = search.trim().toLowerCase()
    if (!keyword) return categories
    return categories
      .map((category) => ({
        ...category,
        groups: category.groups
          .map((group) => ({
            ...group,
            permissions: group.permissions.filter((permission) =>
              `${category.label} ${group.label} ${permission.name} ${permission.code} ${permission.method} ${permission.apiPath}`
                .toLowerCase()
                .includes(keyword)
            ),
          }))
          .filter((group) => group.permissions.length > 0),
      }))
      .filter((category) => category.groups.length > 0)
  }, [categories, search])
  const isSearching = Boolean(search.trim())

  return (
    <>
      <div className="min-h-0 flex-1 overflow-y-auto">
        <section className="border-b border-border px-5 py-4">
          <div className="flex flex-wrap gap-2">
            <StatusTag tone={role.isBuiltin ? "blue" : "neutral"}>
              {role.isBuiltin ? t("iamExtract.workspace.detail.systemRole") : t("iamExtract.workspace.detail.customRole")}
            </StatusTag>
            <StatusTag tone="neutral">{role.domainType === "platform" ? t("iamExtract.detail.platformDomain") : t("iamExtract.detail.enterpriseTenant", { id: role.tenantId })}</StatusTag>
            <StatusTag tone={role.status === 0 ? "neutral" : "error"}>
              {role.status === 0 ? t("iamExtract.detail.enabledStatus") : t("iamExtract.detail.disabledStatus")}
            </StatusTag>
          </div>
          <dl className="mt-4 grid grid-cols-3 gap-3 border-y border-border py-3 text-sm">
            <SummaryTerm label={t("iamExtract.detail.permission.permissionCode")} value={String(role.permissions?.length ?? 0)} />
            <SummaryTerm label={t("iamExtract.detail.permission.abilityDomain")} value={String(groups.length)} />
            <SummaryTerm label={t("iamExtract.detail.permission.sort")} value={String(role.sortNo)} />
          </dl>
        </section>

        <section className="px-5 py-4">
          <div>
            <SearchField
              allowClear
              aria-label={t("iamExtract.detail.permission.searchAria")}
              placeholder={t("iamExtract.detail.permission.searchPlaceholder")}
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
          </div>

          <div className="mt-4 space-y-3">
            {filteredCategories.length ? filteredCategories.map((category) => {
              const categoryOpen = isSearching || openCategoryKey === category.key
              const permissionCount = category.groups.reduce(
                (sum, group) => sum + group.permissions.length,
                0
              )
              return (
                <Collapsible
                  key={category.key}
                  open={categoryOpen}
                  onOpenChange={(open) => {
                    setOpenCategoryKey(open ? category.key : null)
                    if (open && !category.groups.some((group) => group.key === openGroupKey)) {
                      setOpenGroupKey(category.groups[0]?.key ?? null)
                    }
                  }}
                >
                  <CollapsibleTrigger className={`flex min-h-12 w-full items-center justify-between gap-3 border border-border px-3 py-3 text-left transition hover:bg-muted ${categoryOpen ? "rounded-t-md bg-muted" : "rounded-md bg-muted/50"}`}>
                    <span className="flex min-w-0 items-center gap-2 text-sm font-semibold text-foreground">
                      <Layers3Icon className={`size-4 shrink-0 ${iamMutedTextClass}`} />
                      <span className="truncate">{category.label}</span>
                    </span>
                    <span className="flex shrink-0 items-center gap-2">
                      <span className={`text-xs tabular-nums ${iamMutedTextClass}`}>
                        {t("iamExtract.detail.permission.abilitiesCount", { count: category.groups.length })} · {t("iamExtract.detail.permission.permissionItemsCount", { count: permissionCount })}
                      </span>
                      <ChevronDownIcon className={`size-4 ${iamMutedTextClass} transition-transform ${categoryOpen ? "rotate-180" : ""}`} />
                    </span>
                  </CollapsibleTrigger>
                  <CollapsibleContent className="space-y-2 rounded-b-md border border-t-0 border-border bg-muted/30 p-2">
                    {category.groups.map((group) => {
                      const groupOpen = isSearching || openGroupKey === group.key
                      return (
                        <Collapsible
                          key={group.key}
                          open={groupOpen}
                          onOpenChange={(open) => setOpenGroupKey(open ? group.key : null)}
                        >
                          <CollapsibleTrigger className={`flex min-h-11 w-full items-center justify-between gap-3 border px-3 py-2.5 text-left transition ${groupOpen ? "rounded-t-md border-primary/20 bg-primary/10" : "rounded-md border-border bg-card hover:bg-muted/50"}`}>
                            <span className="flex min-w-0 items-center gap-2 text-sm font-semibold text-foreground">
                              <ShieldCheckIcon className="size-4 shrink-0 text-primary" />
                              <span className="truncate">{group.label}</span>
                            </span>
                            <span className="flex shrink-0 items-center gap-2">
                              <span className={`text-xs tabular-nums ${iamMutedTextClass}`}>{t("iamExtract.common.itemCount", { count: group.permissions.length })}</span>
                              <ChevronDownIcon className={`size-4 ${iamMutedTextClass} transition-transform ${groupOpen ? "rotate-180" : ""}`} />
                            </span>
                          </CollapsibleTrigger>
                          <CollapsibleContent className="overflow-hidden rounded-b-md border border-t-0 border-primary/20 bg-card">
                            {group.permissions.map((permission, index) => (
                              <div
                                key={permission.code}
                                className={`grid gap-1 px-3 py-3 ${index > 0 ? "border-t border-border" : ""}`}
                              >
                                <div className="flex min-w-0 items-start justify-between gap-3">
                                  <div className="min-w-0 text-sm font-medium text-foreground">{permission.name}</div>
                                  {permission.method ? (
                                    <span className={`shrink-0 rounded border border-border bg-muted px-1.5 py-0.5 font-mono text-rhd-xs ${iamMutedTextClass}`}>
                                      {permission.method}
                                    </span>
                                  ) : null}
                                </div>
                                <code className="break-all text-xs text-primary">{permission.code}</code>
                                {permission.apiPath ? (
                                  <div className={`flex items-start gap-1.5 text-xs ${iamMutedTextClass}`}>
                                    <Code2Icon className="mt-0.5 size-3.5 shrink-0" />
                                    <code className="break-all">{permission.apiPath}</code>
                                  </div>
                                ) : null}
                              </div>
                            ))}
                          </CollapsibleContent>
                        </Collapsible>
                      )
                    })}
                  </CollapsibleContent>
                </Collapsible>
              )
            }) : (
              <div className={`py-10 text-center text-sm ${iamMutedTextClass}`}>{t("iamExtract.detail.noMatchedPermission")}</div>
            )}
          </div>
        </section>
      </div>
    </>
  )
}

function MemberDetail({ member }: { member: EnterpriseIAMMember }) {
  const t = useI18n()
  const skills = parseDetailList(member.skill_tags_json)
  const languages = parseDetailList(member.languages_json)
  const regions = parseDetailList(member.service_regions_json)
  const productGroups = (member.product_groups || []).map((group) =>
    group.product_name || group.team_name || t("iamExtract.workspace.detail.productGroupWithId", { id: group.team_id }),
  )

  return (
    <>
      <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
        <div className="flex flex-wrap gap-2">
          <StatusTag tone={member.status === 0 ? "neutral" : "error"}>
            {member.status === 0 ? t("iamExtract.detail.accountEnabled") : t("iamExtract.detail.accountDisabled")}
          </StatusTag>
          <StatusTag tone={member.dispatch_enabled ? "blue" : "neutral"}>
            {member.dispatch_enabled ? t("iamExtract.detail.participantDispatch") : t("iamExtract.detail.unavailableDispatch")}
          </StatusTag>
          <StatusTag tone="neutral">{member.member_type || "employee"}</StatusTag>
        </div>

        <section className={iamSectionClass}>
          <h3 className="text-sm font-semibold text-foreground">{t("iamExtract.detail.accountAndOrg")}</h3>
          <dl className="mt-3 grid gap-x-5 gap-y-4 sm:grid-cols-2">
            <DetailTerm label={t("iamExtract.detail.memberNo")} value={member.member_no || "-"} mono />
            <DetailTerm label={t("iamExtract.detail.userId")} value={String(member.user_id || "-")} mono />
            <DetailTerm label={t("iamExtract.detail.email")} value={member.email || t("iamExtract.common.unfilled")} />
            <DetailTerm label={t("iamExtract.detail.department")} value={member.department_name || t("iamExtract.common.unassignedDepartment")} />
            <DetailTerm label={t("iamExtract.dialog.jobTitle")} value={member.job_title || t("iamExtract.common.unsetJobTitle")} />
            <DetailTerm label={t("iamExtract.detail.joinTime")} value={member.joined_at || "-"} />
            <DetailTerm label={t("iamExtract.detail.updatedAt")} value={member.updated_at || "-"} />
          </dl>
        </section>

        <DetailBadgeSection icon={<KeyRoundIcon className="size-4" />} title={t("iamExtract.detail.assignedRoles")} values={member.role_names || member.roles} empty={t("iamExtract.detail.noRole")} />
        <DetailBadgeSection icon={<WrenchIcon className="size-4" />} title={t("iamExtract.detail.productGroup")} values={productGroups} empty={t("iamExtract.detail.noProductGroup")} />
        <DetailBadgeSection icon={<WrenchIcon className="size-4" />} title={t("iamExtract.detail.engineerSkills")} values={skills} empty={t("iamExtract.detail.noSkill")} />
        <DetailBadgeSection icon={<CheckCircle2Icon className="size-4" />} title={t("iamExtract.detail.serviceLanguage")} values={languages} empty={t("iamExtract.detail.noServiceLanguage")} />
        <DetailBadgeSection icon={<ShieldCheckIcon className="size-4" />} title={t("iamExtract.detail.serviceRegion")} values={regions} empty={t("iamExtract.detail.noServiceRegion")} />
      </div>
    </>
  )
}

function CustomerUserDetail({ customerUser }: { customerUser: EnterpriseIAMCustomerUser }) {
  const t = useI18n()
  return (
    <>
      <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
        <div className="flex flex-wrap gap-2">
          <StatusTag tone={customerUser.status === 0 ? "neutral" : "error"}>
            {customerUser.status === 0 ? t("iamExtract.detail.accountEnabled") : t("iamExtract.detail.accountDisabled")}
          </StatusTag>
          <StatusTag tone={customerUser.customer_org_id > 0 ? "blue" : "neutral"}>
            {customerUser.customer_org_id > 0 ? t("iamExtract.workspace.tabs.boundOrganization") : t("iamExtract.detail.unboundOrg")}
          </StatusTag>
        </div>

        <section className={iamSectionClass}>
          <h3 className="text-sm font-semibold text-foreground">{t("iamExtract.detail.customerAccount")}</h3>
          <dl className="mt-3 grid gap-x-5 gap-y-4 sm:grid-cols-2">
            <DetailTerm label={t("iamExtract.detail.customerUserId")} value={String(customerUser.id || "-")} mono />
            <DetailTerm label={t("iamExtract.detail.userId")} value={String(customerUser.user_id || "-")} mono />
            <DetailTerm label={t("iamExtract.detail.email")} value={customerUser.email || t("iamExtract.common.unfilled")} />
            <DetailTerm label={t("iamExtract.detail.phone")} value={customerUser.phone || t("iamExtract.common.unfilled")} />
            <DetailTerm label={t("iamExtract.detail.customerOrg")} value={customerUser.customer_org_name || t("iamExtract.workspace.detail.customerOrgWithId", { id: customerUser.customer_org_id || "-" })} />
            <DetailTerm label={t("iamExtract.detail.languageTimezone")} value={`${customerUser.locale || "-"} / ${customerUser.timezone || "-"}`} />
            <DetailTerm label={t("iamExtract.detail.lastSeen")} value={customerUser.last_seen_at || "-"} />
            <DetailTerm label={t("iamExtract.detail.updatedAt")} value={customerUser.updated_at || "-"} />
          </dl>
        </section>
      </div>
    </>
  )
}

function PartnerDetail({ partner }: { partner: EnterpriseIAMPartner }) {
  const t = useI18n()
  return (
    <>
      <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
        <div className="flex flex-wrap gap-2">
          <StatusTag tone={partner.status === 0 ? "neutral" : "error"}>
            {partner.status === 0 ? t("iamExtract.detail.enabledStatus") : t("iamExtract.detail.disabledStatus")}
          </StatusTag>
          <StatusTag tone="neutral">{partner.partner_type || t("iamExtract.detail.unsetType")}</StatusTag>
        </div>

        <section className={iamSectionClass}>
          <h3 className="text-sm font-semibold text-foreground">{t("iamExtract.detail.supplierInfo")}</h3>
          <dl className="mt-3 grid gap-x-5 gap-y-4 sm:grid-cols-2">
            <DetailTerm label={t("iamExtract.detail.supplierId")} value={String(partner.id || "-")} mono />
            <DetailTerm label={t("iamExtract.detail.supplierNo")} value={partner.partner_no || "-"} mono />
            <DetailTerm label={t("iamExtract.detail.serviceRegion")} value={partner.country_region || "-"} />
            <DetailTerm label={t("iamExtract.detail.contact")} value={partner.contact_name || t("iamExtract.common.unset")} />
            <DetailTerm label={t("iamExtract.detail.accountQuantity")} value={t("iamExtract.workspace.detail.accountCount", { count: partner.account_count })} />
            <DetailTerm label={t("iamExtract.detail.contractQuantity")} value={t("iamExtract.workspace.detail.contractCount", { count: partner.contract_count })} />
            <DetailTerm label={t("iamExtract.detail.updatedAt")} value={partner.updated_at || "-"} />
          </dl>
        </section>
      </div>
    </>
  )
}

function SummaryTerm({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0 text-center">
      <dt className={`text-xs ${iamMutedTextClass}`}>{label}</dt>
      <dd className="mt-1 truncate text-base font-semibold tabular-nums text-foreground">{value}</dd>
    </div>
  )
}

function DetailTerm({ label, mono, value }: { label: string; mono?: boolean; value: string }) {
  return (
    <div className="min-w-0">
      <dt className={`text-xs ${iamMutedTextClass}`}>{label}</dt>
      <dd className={`mt-1 break-words text-sm font-medium text-foreground ${mono ? "font-mono" : ""}`}>{value}</dd>
    </div>
  )
}

function DetailBadgeSection({
  empty,
  icon,
  title,
  values,
}: {
  empty: string
  icon: ReactNode
  title: string
  values: string[]
}) {
  return (
    <section className={`border-b py-4 last:border-b-0 ${iamDividerClass}`}>
      <h3 className="flex items-center gap-2 text-sm font-semibold text-foreground">{icon}{title}</h3>
      <div className="mt-3 flex flex-wrap gap-2">
        {values.length ? values.map((value) => <StatusTag key={value} tone="neutral">{value}</StatusTag>) : (
          <span className={`text-sm ${iamMutedTextClass}`}>{empty}</span>
        )}
      </div>
    </section>
  )
}

function parseDetailList(raw: string) {
  if (!raw.trim()) return []
  try {
    const parsed = JSON.parse(raw) as unknown
    if (Array.isArray(parsed)) {
      return parsed.map((item) => String(item).trim()).filter(Boolean)
    }
  } catch {
    return raw.split(",").map((item) => item.trim()).filter(Boolean)
  }
  return []
}
