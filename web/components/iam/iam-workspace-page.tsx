"use client"

import { Loader2Icon, LogInIcon, PencilIcon, RotateCcwIcon, Settings2Icon, Trash2Icon, UserRoundIcon } from "lucide-react"
import { useEffect, useState, type ReactNode } from "react"
import { useSearchParams } from "next/navigation"
import { toast } from "sonner"

import { IconButton, RailopsButton, SelectField, StandardModal } from "@railops/ui"
import { useAuth } from "@/components/auth-provider"
import { useConfirm } from "@/components/confirm-provider"
import { IAMDetailSheet, type IAMWorkspaceDetail } from "@/components/iam/iam-detail-sheet"
import { IAMEnterpriseEditDialog, type IAMEnterpriseEditTarget } from "@/components/iam/iam-enterprise-edit-dialog"
import { IAMInviteDialog, type IAMInviteKind } from "@/components/iam/iam-invite-dialog"
import { IAMPlatformStaffDialog } from "@/components/iam/iam-platform-staff-dialog"
import { IAMRoleDialog } from "@/components/iam/iam-role-dialog"
import { CanUseButton } from "@/components/layout/permission-guard"
import { RailopsDateRangeFilter } from "@/components/railops/date-range-filter"
import { Input } from "antd"
import {
  OpsWorkspacePage,
  StatusPill,
  type WorkspaceAction,
  type WorkspaceColumn,
  type WorkspaceMetric,
  type WorkspacePanel,
  type WorkspaceRow,
} from "@/components/layout/ops-workspace-page"
import { fetchPermissionCatalog, type AdminPermission } from "@/lib/api/admin"
import {
  fetchEnterpriseIAMAudit,
  fetchEnterpriseIAMCustomerUsers,
  fetchEnterpriseIAMDepartments,
  fetchEnterpriseIAMMembers,
  fetchEnterpriseIAMPartners,
  fetchEnterpriseIAMRoles,
  fetchIAMRoles,
  fetchPlatformAudit,
  fetchPlatformAuditRetention,
  fetchPlatformStaff,
  deleteIAMRole,
  deletePlatformStaff,
  downloadEnterpriseIAMAudit,
  downloadPlatformAudit,
  startEnterpriseCustomerPortalSession,
  startEnterpriseMemberPortalSession,
  updateEnterpriseCustomerUserStatus,
  updateEnterpriseMemberStatus,
  updateEnterprisePartnerStatus,
  updatePlatformAuditRetention,
  type EnterpriseIAMCustomerUser,
  type EnterpriseIAMDepartment,
  type EnterpriseIAMMember,
  type EnterpriseIAMPartner,
  type IAMAuditLog,
  type IAMRole,
  type PlatformAuditRetentionSettings,
  type PlatformStaff,
} from "@/lib/api/platform-iam"
import { stashDelegatedReturnSession, writeSession } from "@/lib/auth"
import { resolveSessionDestination } from "@/lib/login-portals"
import {
  getAuditActionLabel,
  getAuditStatusLabel,
  getAuditSubjectLabel,
  getAuditTargetLabel,
} from "@/lib/audit-i18n"
import { translateCurrentMessage } from "@/i18n/messages"
import { useI18n } from "@/i18n/provider"

function tr(key: string, values?: Record<string, string | number>) {
  return translateCurrentMessage(`iamExtract.${key}`, values)
}

export type IAMWorkspaceKind =
  | "enterprise-people"
  | "enterprise-customer-users"
  | "enterprise-partners"
  | "enterprise-org"
  | "enterprise-permissions"
  | "enterprise-audit"
  | "platform-staff"
  | "platform-permissions"
  | "platform-audit"

type IAMWorkspaceModel = {
  actions?: WorkspaceAction[]
  columns: WorkspaceColumn[]
  details: IAMWorkspaceDetail[]
  emptyMessage: string
  metrics: WorkspaceMetric[]
  panels: WorkspacePanel[]
  permissionCatalog: AdminPermission[]
  roleCatalog: IAMRole[]
  rows: IAMWorkspaceRow[]
  searchPlaceholder: string
  tabs: string[]
  title: string
}

type IAMWorkspaceRow = WorkspaceRow & {
  customerUser?: EnterpriseIAMCustomerUser
  enterpriseMember?: EnterpriseIAMMember
  partner?: EnterpriseIAMPartner
  platformStaff?: PlatformStaff
  role?: IAMRole
  subjectStatus?: number
  subjectUserId?: number
}

type IAMWorkspaceLoadState = {
  error: string
  key: string
  model: IAMWorkspaceModel
}

type AuditWorkspaceFilters = {
  period: "7d" | "14d" | "30d" | "90d" | "180d"
  source: "auth" | "business"
  status: "all" | "success" | "failed" | "blocked"
  targetType: string
}

const AUDIT_PERIOD_ITEMS = [
  { value: "7d", count: 7 },
  { value: "14d", count: 14 },
  { value: "30d", count: 30 },
  { value: "90d", count: 90 },
  { value: "180d", count: 180 },
]

const identityColumns: WorkspaceColumn[] = [
  { key: "subject", label: tr("workspace.columns.subject") },
  { key: "identity", label: tr("workspace.columns.identity") },
  { key: "roles", label: tr("workspace.columns.roles") },
  { key: "scope", label: tr("workspace.columns.scope") },
  { key: "status", label: tr("workspace.columns.status") },
  { key: "updated", label: tr("workspace.columns.updated") },
]

const platformStaffColumns: WorkspaceColumn[] = [
  { key: "subject", label: tr("workspace.columns.subject") },
  { key: "email", label: tr("workspace.columns.email") },
  { key: "roles", label: tr("workspace.columns.roles") },
  { key: "scope", label: tr("workspace.columns.scope") },
  { key: "status", label: tr("workspace.columns.status") },
  { key: "updated", label: tr("workspace.columns.updated") },
]

// 客户用户无角色概念,不展示角色列
const customerUserColumns: WorkspaceColumn[] = [
  { key: "subject", label: tr("workspace.columns.subject") },
  { key: "identity", label: tr("workspace.columns.identity") },
  { key: "scope", label: tr("workspace.columns.scope") },
  { key: "status", label: tr("workspace.columns.status") },
  { key: "updated", label: tr("workspace.columns.updated") },
]

const roleColumns: WorkspaceColumn[] = [
  { key: "role", label: tr("workspace.columns.role") },
  { key: "domain", label: tr("workspace.columns.domain") },
  { key: "permissions", label: tr("workspace.columns.permissions") },
  { key: "builtin", label: tr("workspace.columns.builtin") },
  { key: "status", label: tr("workspace.columns.status") },
  { key: "updated", label: tr("workspace.columns.updated") },
]

const auditColumns: WorkspaceColumn[] = [
  { key: "action", label: tr("workspace.columns.action") },
  { key: "actor", label: tr("workspace.columns.actor") },
  { key: "target", label: tr("workspace.columns.target") },
  { key: "status", label: tr("workspace.columns.result") },
  { key: "occurred", label: tr("workspace.columns.occurred") },
]

const baseCopy: Record<IAMWorkspaceKind, Pick<IAMWorkspaceModel, "emptyMessage" | "searchPlaceholder" | "title"> & { actions?: WorkspaceAction[]; tabs?: string[] }> = {
  "enterprise-people": {
    title: tr("workspace.copy.enterprise-people.title"),
    tabs: [tr("workspace.tabs.allMembers"), tr("workspace.tabs.dispatchable"), tr("workspace.tabs.assignedRole"), tr("workspace.tabs.disabled")],
    searchPlaceholder: tr("workspace.copy.enterprise-people.searchPlaceholder"),
    emptyMessage: tr("workspace.copy.enterprise-people.emptyMessage"),
    actions: [{ label: tr("workspace.actions.inviteEnterpriseMember"), primary: true }, { href: "/enterprise/permissions", label: tr("workspace.actions.roleAuthorization") }],
  },
  "enterprise-customer-users": {
    title: tr("workspace.copy.enterprise-customer-users.title"),
    tabs: [tr("workspace.tabs.allCustomers"), tr("workspace.tabs.boundOrganization"), tr("workspace.tabs.disabled")],
    searchPlaceholder: tr("workspace.copy.enterprise-customer-users.searchPlaceholder"),
    emptyMessage: tr("workspace.copy.enterprise-customer-users.emptyMessage"),
    actions: [{ label: tr("workspace.actions.inviteCustomerUser"), primary: true }],
  },
  "enterprise-partners": {
    title: tr("workspace.copy.enterprise-partners.title"),
    tabs: [],
    searchPlaceholder: tr("workspace.copy.enterprise-partners.searchPlaceholder"),
    emptyMessage: tr("workspace.copy.enterprise-partners.emptyMessage"),
    actions: [{ label: tr("workspace.actions.invitePartner"), primary: true }],
  },
  "enterprise-org": {
    title: tr("workspace.copy.enterprise-org.title"),
    tabs: [tr("workspace.tabs.allOrganizations"), tr("workspace.tabs.hasMembers"), tr("workspace.tabs.hasLeader"), tr("workspace.tabs.disabled")],
    searchPlaceholder: tr("workspace.copy.enterprise-org.searchPlaceholder"),
    emptyMessage: tr("workspace.copy.enterprise-org.emptyMessage"),
    actions: [{ label: tr("workspace.actions.addOrganization"), primary: true }],
  },
  "enterprise-permissions": {
    title: tr("workspace.copy.enterprise-permissions.title"),
    tabs: [tr("workspace.tabs.allRoles"), tr("workspace.tabs.builtinRole"), tr("workspace.tabs.customRole"), tr("workspace.tabs.disabled")],
    searchPlaceholder: tr("workspace.copy.enterprise-permissions.searchPlaceholder"),
    emptyMessage: tr("workspace.copy.enterprise-permissions.emptyMessage"),
    actions: [{ label: tr("workspace.actions.addCustomRole"), primary: true }, { href: "/enterprise/audit", label: tr("workspace.actions.viewAudit") }],
  },
  "enterprise-audit": {
    title: tr("workspace.copy.enterprise-audit.title"),
    searchPlaceholder: tr("workspace.copy.enterprise-audit.searchPlaceholder"),
    emptyMessage: tr("workspace.copy.enterprise-audit.emptyMessage"),
    actions: [{ label: tr("workspace.actions.exportLogs"), primary: true }],
  },
  "platform-staff": {
    title: tr("workspace.copy.platform-staff.title"),
    tabs: [tr("workspace.tabs.allStaff"), tr("workspace.tabs.platformOperations"), tr("workspace.tabs.platformAudit"), tr("workspace.tabs.disabled")],
    searchPlaceholder: tr("workspace.copy.platform-staff.searchPlaceholder"),
    emptyMessage: tr("workspace.copy.platform-staff.emptyMessage"),
    actions: [{ label: tr("workspace.actions.invitePlatformStaff"), primary: true }, { href: "/platform/permissions", label: tr("workspace.actions.platformAuthorization") }],
  },
  "platform-permissions": {
    title: tr("workspace.copy.platform-permissions.title"),
    tabs: [tr("workspace.tabs.allRoles"), tr("workspace.tabs.builtinRole"), tr("workspace.tabs.customRole"), tr("workspace.tabs.disabled")],
    searchPlaceholder: tr("workspace.copy.platform-permissions.searchPlaceholder"),
    emptyMessage: tr("workspace.copy.platform-permissions.emptyMessage"),
    actions: [{ label: tr("workspace.actions.addRole"), primary: true }, { href: "/platform/audit", label: tr("workspace.actions.viewAudit") }],
  },
  "platform-audit": {
    title: tr("workspace.copy.platform-audit.title"),
    searchPlaceholder: tr("workspace.copy.platform-audit.searchPlaceholder"),
    emptyMessage: tr("workspace.copy.platform-audit.emptyMessage"),
    actions: [{ label: tr("workspace.actions.exportLogs"), primary: true }, { label: tr("workspace.actions.retentionSettings") }],
  },
}

const defaultState: IAMWorkspaceModel = {
  actions: [],
  columns: identityColumns,
  details: [],
  emptyMessage: tr("workspace.emptyRecord"),
  metrics: [
    { label: tr("workspace.metrics.recordCount"), value: "0", meta: tr("workspace.metrics.realtime"), tone: "slate" },
    { label: tr("workspace.metrics.enabled"), value: "0", meta: tr("workspace.metrics.status"), tone: "green" },
    { label: tr("workspace.metrics.assignedRoles"), value: "0", meta: tr("workspace.metrics.permission"), tone: "blue" },
    { label: tr("workspace.metrics.risk"), value: "0", meta: tr("workspace.metrics.audit"), tone: "slate" },
  ],
  panels: [],
  permissionCatalog: [],
  roleCatalog: [],
  rows: [],
  searchPlaceholder: tr("workspace.searchPlaceholder"),
  tabs: [],
  title: "",
}

export function IAMWorkspacePage({ kind }: { kind: IAMWorkspaceKind }) {
  const t = useI18n()
  const { ready, session } = useAuth()
  const confirm = useConfirm()
  const searchParams = useSearchParams()
  const tenantId = resolveIAMTenantId(searchParams.get("tenantId"), session?.tenantId)
  const [loadState, setLoadState] = useState<IAMWorkspaceLoadState | null>(null)
  const [inviteOpen, setInviteOpen] = useState(false)
  const [roleOpen, setRoleOpen] = useState(false)
  const [editingEnterpriseTarget, setEditingEnterpriseTarget] = useState<IAMEnterpriseEditTarget | null>(null)
  const [editingPlatformStaff, setEditingPlatformStaff] = useState<PlatformStaff | null>(null)
  const [editingRole, setEditingRole] = useState<IAMRole | null>(null)
  const [selectedDetailId, setSelectedDetailId] = useState<string | null>(null)
  const [refreshVersion, setRefreshVersion] = useState(0)
  const [deletingKey, setDeletingKey] = useState("")
  const [enteringMemberId, setEnteringMemberId] = useState<number | null>(null)
  const [enteringCustomerUserId, setEnteringCustomerUserId] = useState<number | null>(null)
  const [auditFilters, setAuditFilters] = useState<AuditWorkspaceFilters>({ period: "90d", source: "auth", status: "all", targetType: "all" })
  const [auditSearch, setAuditSearch] = useState("")
  const [exportingAudit, setExportingAudit] = useState(false)
  const [retentionOpen, setRetentionOpen] = useState(false)
  const auditFilterKey = isAuditWorkspace(kind) ? JSON.stringify({ ...auditFilters, search: auditSearch }) : ""
  const requestKey = `${kind}:${tenantId}:${session?.user.id ?? "anonymous"}:${session?.accessToken ?? ""}:${refreshVersion}:${auditFilterKey}`

  useEffect(() => {
    let cancelled = false
    if (!ready || !session) {
      return
    }

    loadIAMWorkspace(kind, tenantId, auditFilters, auditSearch)
      .then((next) => {
        if (!cancelled) {
          setLoadState({ error: "", key: requestKey, model: next })
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setLoadState({
            error: err instanceof Error ? err.message : t("iamExtract.workspace.loadingFailed"),
            key: requestKey,
            model: mergeBase(kind, defaultState),
          })
        }
      })

    return () => {
      cancelled = true
    }
  }, [auditFilters, kind, ready, requestKey, session, t, tenantId])

  const fallbackModel = mergeBase(kind, defaultState)
  const currentState = loadState?.key === requestKey ? loadState : null
  const page = !ready || (session && !currentState)
    ? {
        ...fallbackModel,
        emptyMessage: t("iamExtract.workspace.loadingRows"),
        rows: [],
      }
    : !session
      ? {
          ...fallbackModel,
          emptyMessage: t("iamExtract.workspace.sessionExpired"),
          rows: [],
        }
      : currentState?.error
        ? {
            ...currentState.model,
            emptyMessage: currentState.error,
            rows: [],
          }
        : currentState?.model ?? fallbackModel

  const inviteKind = resolveInviteKind(kind)
  const roleDomain = kind === "platform-permissions" ? "platform" : kind === "enterprise-permissions" ? "enterprise" : null
  const canEnterEmployeePortal = kind === "enterprise-people" && CanUseButton("user.update", session?.permissions)
  const canEnterCustomerPortal = kind === "enterprise-customer-users" && CanUseButton("customer.view", session?.permissions)
  const canUpdateEnterpriseMember = kind === "enterprise-people" && CanUseButton("user.update", session?.permissions)
  const canUpdateEnterpriseCustomer = kind === "enterprise-customer-users" && CanUseButton("customer.update", session?.permissions)
  const canUpdateEnterprisePartner = kind === "enterprise-partners" && CanUseButton("user.update", session?.permissions)
  const canUpdatePlatformStaff = kind === "platform-staff" && CanUseButton("user.update", session?.permissions)
  const canDeletePlatformStaff = kind === "platform-staff" && CanUseButton("user.delete", session?.permissions)
  const canUpdatePlatformRole = kind === "platform-permissions" && (CanUseButton("role.update", session?.permissions) || CanUseButton("role.assignPermission", session?.permissions))
  const canDeletePlatformRole = kind === "platform-permissions" && CanUseButton("role.delete", session?.permissions)

  function refreshWorkspace() {
    setRefreshVersion((value) => value + 1)
  }

  async function removePlatformStaff(staff: PlatformStaff) {
    if (deletingKey) return
    const confirmed = await confirm({
      title: t("iamExtract.workspace.confirm.deletePlatformStaff"),
      description: staff.displayName || staff.username,
      confirmText: t("iamExtract.common.delete"),
      variant: "destructive",
    })
    if (!confirmed) return
    setDeletingKey(`platform-staff:${staff.id}`)
    try {
      await deletePlatformStaff(staff.id)
      toast.success(t("iamExtract.workspace.platformStaffDeleted"))
      refreshWorkspace()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("iamExtract.workspace.platformStaffDeleteFailed"))
    } finally {
      setDeletingKey("")
    }
  }

  async function removePlatformRole(role: IAMRole) {
    if (deletingKey) return
    const confirmed = await confirm({
      title: t("iamExtract.workspace.confirm.deletePlatformRole"),
      description: role.name || role.code,
      confirmText: t("iamExtract.common.delete"),
      variant: "destructive",
    })
    if (!confirmed) return
    setDeletingKey(`platform-role:${role.id}`)
    try {
      await deleteIAMRole(role.id)
      toast.success(t("iamExtract.workspace.platformRoleDeleted"))
      refreshWorkspace()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("iamExtract.workspace.platformRoleDeleteFailed"))
    } finally {
      setDeletingKey("")
    }
  }

  async function toggleEnterpriseMember(member: EnterpriseIAMMember) {
    if (deletingKey) return
    const nextStatus = member.status === 0 ? 1 : 0
    const action = nextStatus === 0 ? t("iamExtract.common.enabled") : t("iamExtract.common.disabled")
    if (nextStatus !== 0) {
      const confirmed = await confirm({
        title: t("iamExtract.workspace.confirm.disableEnterpriseMember"),
        description: member.display_name || member.username,
        confirmText: t("iamExtract.common.disabled"),
        variant: "destructive",
      })
      if (!confirmed) return
    }
    setDeletingKey(`enterprise-member:${member.id}`)
    try {
      await updateEnterpriseMemberStatus(member.id, nextStatus, tenantId)
      toast.success(t("iamExtract.workspace.entityStatusUpdated", { entity: t("iamExtract.workspace.metrics.enterpriseMember"), action }))
      refreshWorkspace()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("iamExtract.workspace.entityStatusUpdateFailed", { entity: t("iamExtract.workspace.metrics.enterpriseMember"), action }))
    } finally {
      setDeletingKey("")
    }
  }

  async function toggleEnterpriseCustomerUser(customerUser: EnterpriseIAMCustomerUser) {
    if (deletingKey) return
    const nextStatus = customerUser.status === 0 ? 1 : 0
    const action = nextStatus === 0 ? t("iamExtract.common.enabled") : t("iamExtract.common.disabled")
    if (nextStatus !== 0) {
      const confirmed = await confirm({
        title: t("iamExtract.workspace.confirm.disableCustomerUser"),
        description: customerUser.display_name || customerUser.email,
        confirmText: t("iamExtract.common.disabled"),
        variant: "destructive",
      })
      if (!confirmed) return
    }
    setDeletingKey(`enterprise-customer:${customerUser.id}`)
    try {
      await updateEnterpriseCustomerUserStatus(customerUser.id, nextStatus, tenantId)
      toast.success(t("iamExtract.workspace.entityStatusUpdated", { entity: t("iamExtract.workspace.metrics.customerUsers"), action }))
      refreshWorkspace()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("iamExtract.workspace.entityStatusUpdateFailed", { entity: t("iamExtract.workspace.metrics.customerUsers"), action }))
    } finally {
      setDeletingKey("")
    }
  }

  async function toggleEnterprisePartner(partner: EnterpriseIAMPartner) {
    if (deletingKey) return
    const nextStatus = partner.status === 0 ? 1 : 0
    const action = nextStatus === 0 ? t("iamExtract.common.enabled") : t("iamExtract.common.disabled")
    if (nextStatus !== 0) {
      const confirmed = await confirm({
        title: t("iamExtract.workspace.confirm.disablePartner"),
        description: partner.name || partner.partner_no,
        confirmText: t("iamExtract.common.disabled"),
        variant: "destructive",
      })
      if (!confirmed) return
    }
    setDeletingKey(`enterprise-partner:${partner.id}`)
    try {
      await updateEnterprisePartnerStatus(partner.id, nextStatus, tenantId)
      toast.success(t("iamExtract.workspace.entityStatusUpdated", { entity: t("iamExtract.workspace.detail.supplier"), action }))
      refreshWorkspace()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("iamExtract.workspace.entityStatusUpdateFailed", { entity: t("iamExtract.workspace.detail.supplier"), action }))
    } finally {
      setDeletingKey("")
    }
  }

  async function enterEmployeePortal(memberId: number) {
    if (!session || memberId <= 0) return
    setEnteringMemberId(memberId)
    try {
      const employeeSession = await startEnterpriseMemberPortalSession(memberId, tenantId)
      stashDelegatedReturnSession(session)
      writeSession(employeeSession)
      toast.success(t("iamExtract.workspace.enterEmployeePortalDone"))
      window.location.assign(resolveSessionDestination(employeeSession, null))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("iamExtract.workspace.enterEmployeePortalFailed"))
      setEnteringMemberId(null)
    }
  }

  async function enterCustomerPortal(customerUserId: number) {
    if (!session || customerUserId <= 0) return
    setEnteringCustomerUserId(customerUserId)
    try {
      const customerSession = await startEnterpriseCustomerPortalSession(customerUserId, tenantId)
      stashDelegatedReturnSession(session)
      writeSession(customerSession)
      toast.success(t("iamExtract.workspace.enterCustomerPortalDone"))
      window.location.assign("/customer")
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("iamExtract.workspace.enterCustomerPortalFailed"))
      setEnteringCustomerUserId(null)
    }
  }

  async function exportAudit() {
    if (!isAuditWorkspace(kind) || exportingAudit) return
    setExportingAudit(true)
    try {
      const result = kind === "platform-audit"
        ? await downloadPlatformAudit(auditFiltersToQuery(auditFilters, auditSearch, { includeSource: false }))
        : await downloadEnterpriseIAMAudit(auditFiltersToQuery(auditFilters, auditSearch), tenantId)
      const url = URL.createObjectURL(result.blob)
      const link = document.createElement("a")
      link.href = url
      link.download = result.filename || `${kind === "platform-audit" ? "platform" : "enterprise"}-audit-${new Date().toISOString().slice(0, 10)}.csv`
      link.click()
      URL.revokeObjectURL(url)
      toast.success(t("iamExtract.workspace.auditExported"))
      setRefreshVersion((value) => value + 1)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("iamExtract.workspace.auditExportFailed"))
    } finally {
      setExportingAudit(false)
    }
  }
  const actions = page.actions
    ?.filter((action) => CanUseButton(requiredWorkspaceActionPermission(kind, action), session?.permissions))
    .map((action) => (
      action.primary && isAuditWorkspace(kind)
        ? { ...action, label: exportingAudit ? t("iamExtract.workspace.auditExporting") : action.label, disabled: exportingAudit, onClick: () => void exportAudit() }
        : kind === "platform-audit" && action.label === t("iamExtract.workspace.actions.retentionSettings")
          ? { ...action, onClick: () => setRetentionOpen(true) }
        : action.primary && inviteKind
        ? { ...action, onClick: () => setInviteOpen(true) }
        : action.primary && roleDomain
          ? { ...action, onClick: () => setRoleOpen(true) }
          : action
    ))
  const selectedRowId = selectedDetailId?.startsWith(`${requestKey}:`)
    ? selectedDetailId.slice(requestKey.length + 1)
    : null
  const selectedDetail = page.details.find((detail) => detail.id === selectedRowId) ?? null
  const detailIds = new Set(page.details.map((detail) => detail.id))
  const rows = page.rows.map((row) => {
    const nextRow = detailIds.has(row.id) ? {
      ...row,
      onSelect: () => setSelectedDetailId(`${requestKey}:${row.id}`),
      selected: row.id === selectedRowId,
    } : row
    const rowActions: NonNullable<IAMWorkspaceRow["actions"]> = []
    if (row.platformStaff) {
      if (canUpdatePlatformStaff) {
        rowActions.push({
          icon: <PencilIcon className="size-3.5" />,
          label: t("iamExtract.common.edit"),
          onClick: () => setEditingPlatformStaff(row.platformStaff ?? null),
        })
      }
      if (canDeletePlatformStaff) {
        rowActions.push({
          disabled: Boolean(deletingKey),
          icon: <Trash2Icon className="size-3.5" />,
          label: deletingKey === `platform-staff:${row.platformStaff.id}` ? t("iamExtract.common.deleting") : t("iamExtract.common.delete"),
          onClick: () => void removePlatformStaff(row.platformStaff as PlatformStaff),
          tone: "danger" as const,
        })
      }
    }
    if (row.enterpriseMember && canUpdateEnterpriseMember) {
      rowActions.push({
        icon: <PencilIcon className="size-3.5" />,
        label: t("iamExtract.common.edit"),
        onClick: () => setEditingEnterpriseTarget({ kind: "member", item: row.enterpriseMember as EnterpriseIAMMember }),
      })
      const self = row.enterpriseMember.user_id === session?.user.id
      rowActions.push({
        disabled: Boolean(deletingKey) || self,
        icon: row.enterpriseMember.status === 0 ? <Trash2Icon className="size-3.5" /> : <RotateCcwIcon className="size-3.5" />,
        label: deletingKey === `enterprise-member:${row.enterpriseMember.id}` ? t("iamExtract.common.processing") : row.enterpriseMember.status === 0 ? t("iamExtract.common.disabled") : t("iamExtract.common.enabled"),
        onClick: () => void toggleEnterpriseMember(row.enterpriseMember as EnterpriseIAMMember),
        tone: row.enterpriseMember.status === 0 ? "danger" as const : "default" as const,
      })
    }
    if (row.customerUser && canUpdateEnterpriseCustomer) {
      rowActions.push({
        icon: <PencilIcon className="size-3.5" />,
        label: t("iamExtract.common.edit"),
        onClick: () => setEditingEnterpriseTarget({ kind: "customer", item: row.customerUser as EnterpriseIAMCustomerUser }),
      })
      rowActions.push({
        disabled: Boolean(deletingKey),
        icon: row.customerUser.status === 0 ? <Trash2Icon className="size-3.5" /> : <RotateCcwIcon className="size-3.5" />,
        label: deletingKey === `enterprise-customer:${row.customerUser.id}` ? t("iamExtract.common.processing") : row.customerUser.status === 0 ? t("iamExtract.common.disabled") : t("iamExtract.common.enabled"),
        onClick: () => void toggleEnterpriseCustomerUser(row.customerUser as EnterpriseIAMCustomerUser),
        tone: row.customerUser.status === 0 ? "danger" as const : "default" as const,
      })
    }
    if (row.partner && canUpdateEnterprisePartner) {
      rowActions.push({
        icon: <PencilIcon className="size-3.5" />,
        label: t("iamExtract.common.edit"),
        onClick: () => setEditingEnterpriseTarget({ kind: "partner", item: row.partner as EnterpriseIAMPartner }),
      })
      rowActions.push({
        disabled: Boolean(deletingKey),
        icon: row.partner.status === 0 ? <Trash2Icon className="size-3.5" /> : <RotateCcwIcon className="size-3.5" />,
        label: deletingKey === `enterprise-partner:${row.partner.id}` ? t("iamExtract.common.processing") : row.partner.status === 0 ? t("iamExtract.common.disabled") : t("iamExtract.common.enabled"),
        onClick: () => void toggleEnterprisePartner(row.partner as EnterpriseIAMPartner),
        tone: row.partner.status === 0 ? "danger" as const : "default" as const,
      })
    }
    if (row.role && kind === "platform-permissions") {
      if (canUpdatePlatformRole) {
        rowActions.push({
          icon: <PencilIcon className="size-3.5" />,
          label: t("iamExtract.common.edit"),
          onClick: () => setEditingRole(row.role ?? null),
        })
      }
      if (canDeletePlatformRole && !row.role.isBuiltin) {
        rowActions.push({
          disabled: Boolean(deletingKey),
          icon: <Trash2Icon className="size-3.5" />,
          label: deletingKey === `platform-role:${row.role.id}` ? t("iamExtract.common.deleting") : t("iamExtract.common.delete"),
          onClick: () => void removePlatformRole(row.role as IAMRole),
          tone: "danger" as const,
        })
      }
    }
    const actionableRow = rowActions.length ? { ...nextRow, actions: rowActions } : nextRow
    if (canEnterEmployeePortal && row.subjectStatus === 0 && row.subjectUserId !== session?.user.id) {
      const memberId = Number(row.id)
      const entering = enteringMemberId === memberId
      return {
        ...actionableRow,
        cells: {
          ...actionableRow.cells,
          status: (
            <div className="flex flex-wrap items-center gap-2">
              {actionableRow.cells.status}
              <button
                type="button"
                className="inline-flex h-7 items-center rounded-md border border-primary/20 bg-primary/10 px-2 text-xs font-medium text-primary transition hover:border-primary/30 hover:bg-primary/15 disabled:cursor-not-allowed disabled:opacity-60"
                disabled={enteringMemberId !== null}
                onClick={(event) => {
                  event.stopPropagation()
                  void enterEmployeePortal(memberId)
                }}
              >
                {entering ? t("iamExtract.workspace.entering") : t("iamExtract.workspace.enterEmployeePortal")}
              </button>
            </div>
          ),
        },
      }
    }
    if (!canEnterCustomerPortal) {
      return actionableRow
    }
    const customerUserId = Number(row.id)
    const entering = enteringCustomerUserId === customerUserId
    return {
      ...nextRow,
      actions: actionableRow.actions,
      cells: {
        ...actionableRow.cells,
        status: (
          <div className="flex flex-wrap items-center gap-2">
            {actionableRow.cells.status}
            <RailopsButton
              size="small"
              disabled={enteringCustomerUserId !== null}
              onClick={(event) => {
                event.stopPropagation()
                void enterCustomerPortal(customerUserId)
              }}
            >
              {entering ? <Loader2Icon className="size-3.5 animate-spin" /> : <LogInIcon className="size-3.5" />}
              {entering ? t("iamExtract.workspace.entering") : t("iamExtract.workspace.enterCustomerPortal")}
            </RailopsButton>
          </div>
        ),
      },
    }
  })

  return (
    <>
      <OpsWorkspacePage
        {...page}
        actions={actions}
        className={`rhd-railops-iam-workspace rhd-railops-iam-${kind}`}
        inlineSearch={isAuditWorkspace(kind)}
        searchValue={isAuditWorkspace(kind) ? auditSearch : undefined}
        onSearchChange={isAuditWorkspace(kind) ? setAuditSearch : undefined}
        rows={rows}
        showMetrics={!shouldHideIAMMetrics(kind)}
        toolbar={isAuditWorkspace(kind) ? <AuditWorkspaceToolbar filters={auditFilters} showSourceFilter={kind === "enterprise-audit"} onChange={setAuditFilters} /> : undefined}
      />
      <IAMDetailSheet
        catalog={page.permissionCatalog}
        detail={selectedDetail}
        open={Boolean(selectedDetail)}
        onOpenChange={(open) => {
          if (!open) setSelectedDetailId(null)
        }}
      />
      {inviteKind ? (
        <IAMInviteDialog
          kind={inviteKind}
          tenantId={tenantId}
          open={inviteOpen}
          onOpenChange={setInviteOpen}
          onSaved={refreshWorkspace}
        />
      ) : null}
      {roleDomain ? (
        <IAMRoleDialog
          domainType={(editingRole?.domainType as "platform" | "enterprise" | undefined) ?? roleDomain}
          tenantId={editingRole?.tenantId ?? (roleDomain === "platform" ? 0 : tenantId)}
          open={roleOpen || Boolean(editingRole)}
          onOpenChange={(open) => {
            if (!open) {
              setRoleOpen(false)
              setEditingRole(null)
            } else {
              setRoleOpen(true)
            }
          }}
          onSaved={refreshWorkspace}
          permissionCatalog={page.permissionCatalog}
          role={editingRole}
        />
      ) : null}
      <IAMPlatformStaffDialog
        staff={editingPlatformStaff}
        open={Boolean(editingPlatformStaff)}
        onOpenChange={(open) => {
          if (!open) setEditingPlatformStaff(null)
        }}
        onSaved={refreshWorkspace}
      />
      <IAMEnterpriseEditDialog
        roles={page.roleCatalog}
        target={editingEnterpriseTarget}
        tenantId={tenantId}
        open={Boolean(editingEnterpriseTarget)}
        onOpenChange={(open) => {
          if (!open) setEditingEnterpriseTarget(null)
        }}
        onSaved={refreshWorkspace}
      />
      <PlatformAuditRetentionDialog
        open={kind === "platform-audit" && retentionOpen}
        onOpenChange={setRetentionOpen}
      />
    </>
  )
}

function AuditWorkspaceToolbar({
  filters,
  showSourceFilter = false,
  onChange,
}: {
  filters: AuditWorkspaceFilters
  showSourceFilter?: boolean
  onChange: (filters: AuditWorkspaceFilters) => void
}) {
  const t = useI18n()
  const auditPeriodItems = AUDIT_PERIOD_ITEMS.map((item) => ({
    label: t("iamExtract.workspace.filters.lastDays", { count: item.count }),
    value: item.value,
  }))
  function update<K extends keyof AuditWorkspaceFilters>(key: K, value: AuditWorkspaceFilters[K]) {
    onChange({ ...filters, [key]: value })
  }
  return (
    <>
      <RailopsDateRangeFilter
        ariaLabel={t("iamExtract.workspace.filters.occurredAt")}
        className="rhd-railops-audit-period-filter"
        items={auditPeriodItems}
        value={filters.period}
        onChange={(value) => update("period", value)}
      />
      {showSourceFilter ? (
        <SelectField
          className="w-40 shrink-0"
          style={{ marginBottom: 0 }}
          selectProps={{
            "aria-label": t("iamExtract.workspace.filters.logSource"),
            value: filters.source,
            onChange: (value) => update("source", (value ?? "auth") as AuditWorkspaceFilters["source"]),
            options: [
              { value: "auth", label: t("iamExtract.common.authLog") },
              { value: "business", label: t("iamExtract.common.businessLog") },
            ],
            style: { width: "100%" },
          }}
        />
      ) : null}
      <SelectField
        className="w-32 shrink-0"
        style={{ marginBottom: 0 }}
        selectProps={{
          "aria-label": t("iamExtract.workspace.filters.actionResult"),
          value: filters.status,
          onChange: (value) => update("status", (value ?? "all") as AuditWorkspaceFilters["status"]),
          options: [
            { value: "all", label: t("iamExtract.common.allResults") },
            { value: "success", label: t("iamExtract.workspace.filters.success") },
            { value: "failed", label: t("iamExtract.workspace.filters.failed") },
            { value: "blocked", label: t("iamExtract.workspace.filters.blocked") },
          ],
          style: { width: "100%" },
        }}
      />
      <SelectField
        className="w-44 shrink-0"
        style={{ marginBottom: 0 }}
        selectProps={{
          "aria-label": t("iamExtract.workspace.filters.targetType"),
          value: filters.targetType,
          onChange: (value) => update("targetType", value ?? "all"),
          options: [
            { value: "all", label: t("iamExtract.common.allObjects") },
            { value: "tenant", label: t("iamExtract.workspace.filters.targetEnterprise") },
            { value: "tenant_member", label: t("iamExtract.workspace.metrics.enterpriseMember") },
            { value: "auth_role", label: t("iamExtract.workspace.filters.rolePermission") },
            { value: "support", label: t("iamExtract.workspace.metrics.assistedAccess") },
            { value: "ticket", label: t("iamExtract.detail.ticket") },
            { value: "conversation", label: t("iamExtract.detail.serviceSession") },
            { value: "message", label: t("iamExtract.workspace.filters.chatMessage") },
            { value: "meeting", label: t("iamExtract.workspace.filters.videoCollaboration") },
            { value: "notification", label: t("iamExtract.workspace.filters.notification") },
            { value: "tenant_mail_setting", label: t("iamExtract.workspace.filters.mailChannel") },
          ],
          style: { width: "100%" },
        }}
      />
      <IconButton
        icon={<RotateCcwIcon className="size-4" />}
        tooltip={t("iamExtract.workspace.filters.resetFilter")}
        aria-label={t("iamExtract.workspace.filters.resetAuditFilter")}
        onClick={() => onChange({ period: "90d", source: "auth", status: "all", targetType: "all" })}
      />
    </>
  )
}

function PlatformAuditRetentionDialog({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const t = useI18n()
  const [settings, setSettings] = useState<PlatformAuditRetentionSettings | null>(null)
  const [days, setDays] = useState("90")
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    let cancelled = false
    if (!open) return
    setLoading(true)
    fetchPlatformAuditRetention()
      .then((next) => {
        if (cancelled) return
        setSettings(next)
        setDays(String(next.retentionDays || next.defaultRetentionDays || 90))
      })
      .catch((err) => {
        if (!cancelled) toast.error(err instanceof Error ? err.message : t("iamExtract.retention.loadFailed"))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [open])

  const minDays = settings?.minRetentionDays ?? 1
  const maxDays = settings?.maxRetentionDays ?? 180
  const defaultDays = settings?.defaultRetentionDays ?? 90
  const parsedDays = Number(days)
  const invalid = !Number.isFinite(parsedDays) || parsedDays < minDays || parsedDays > maxDays

  async function save() {
    if (saving || invalid) return
    setSaving(true)
    try {
      const next = await updatePlatformAuditRetention(parsedDays)
      setSettings(next)
      setDays(String(next.retentionDays))
      toast.success(t("iamExtract.retention.saved"))
      onOpenChange(false)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("iamExtract.retention.saveFailed"))
    } finally {
      setSaving(false)
    }
  }

  return (
    <StandardModal
      open={open}
      onCancel={() => onOpenChange(false)}
      title={<span className="flex items-center gap-2"><Settings2Icon className="size-5" />{t("iamExtract.retention.title")}</span>}
      width={448}
      footer={
        <>
          <RailopsButton onClick={() => onOpenChange(false)}>{t("iamExtract.common.cancel")}</RailopsButton>
          <RailopsButton variant="primary" disabled={loading || saving || invalid} onClick={() => void save()}>
            {saving ? t("iamExtract.common.savingEllipsis") : t("iamExtract.common.save")}
          </RailopsButton>
        </>
      }
    >
      <div className="space-y-4">
          <div className="grid grid-cols-3 gap-2">
            <div className="rounded-md border border-border px-3 py-2">
              <div className="text-xs text-muted-foreground">{t("iamExtract.retention.current")}</div>
              <div className="mt-1 text-sm font-semibold text-foreground">{t("iamExtract.retention.unitDays", { count: settings?.retentionDays ?? "-" })}</div>
            </div>
            <div className="rounded-md border border-border px-3 py-2">
              <div className="text-xs text-muted-foreground">{t("iamExtract.retention.default")}</div>
              <div className="mt-1 text-sm font-semibold text-foreground">{t("iamExtract.retention.unitDays", { count: defaultDays })}</div>
            </div>
            <div className="rounded-md border border-border px-3 py-2">
              <div className="text-xs text-muted-foreground">{t("iamExtract.retention.max")}</div>
              <div className="mt-1 text-sm font-semibold text-foreground">{t("iamExtract.retention.unitDays", { count: maxDays })}</div>
            </div>
          </div>
          <label className="block space-y-2">
            <span className="text-sm font-medium text-foreground">{t("iamExtract.retention.retentionDays")}</span>
            <Input
              type="number"
              min={minDays}
              max={maxDays}
              inputMode="numeric"
              value={days}
              onChange={(event) => setDays(event.target.value.replace(/\D/g, ""))}
            />
          </label>
          <div className="flex flex-wrap gap-2">
            {[90, 120, 180].map((value) => (
              <RailopsButton
                key={value}
                variant={String(value) === days ? "primary" : "default"}
                size="small"
                onClick={() => setDays(String(value))}
              >
                {t("iamExtract.retention.unitDays", { count: value })}
              </RailopsButton>
            ))}
          </div>
          {settings?.cutoffAt ? (
            <div className="rounded-md border border-border bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
              {t("iamExtract.retention.currentCutoff", { cutoff: settings.cutoffAt })}
            </div>
          ) : null}
          {invalid ? (
            <div className="text-xs text-destructive">{t("iamExtract.retention.invalidDays", { min: minDays, max: maxDays })}</div>
          ) : null}
      </div>
    </StandardModal>
  )
}

function requiredWorkspaceActionPermission(kind: IAMWorkspaceKind, action: WorkspaceAction) {
  if (action.href?.includes("/permissions")) return "role.view"
  if (action.href?.includes("/audit")) return "session.view"
  if (kind === "platform-audit" && action.label === tr("workspace.actions.retentionSettings")) return "audit.retention.manage"
  if (!action.primary) return undefined
  switch (kind) {
    case "platform-staff":
    case "enterprise-people":
    case "enterprise-partners":
      return "user.create"
    case "enterprise-customer-users":
      return "customer.create"
    case "enterprise-org":
      return "user.update"
    case "platform-permissions":
    case "enterprise-permissions":
      return "role.create"
    case "platform-audit":
    case "enterprise-audit":
      return "session.view"
  }
}

function resolveInviteKind(kind: IAMWorkspaceKind): IAMInviteKind | null {
  if (kind === "platform-staff" || kind === "enterprise-people" || kind === "enterprise-customer-users" || kind === "enterprise-partners") {
    return kind
  }
  return null
}

async function loadIAMWorkspace(kind: IAMWorkspaceKind, tenantId: number, auditFilters: AuditWorkspaceFilters, auditSearch = ""): Promise<IAMWorkspaceModel> {
  switch (kind) {
    case "platform-staff": {
      const staff = await fetchPlatformStaff({ status: "all" })
      return buildPlatformStaff(staff)
    }
    case "platform-permissions": {
      const [roles, audits, permissionCatalog] = await Promise.all([
        fetchIAMRoles({ tenantId: 0, domainType: "platform" }),
        fetchPlatformAudit({ page: 1, limit: 20 }).then((page) => page.results).catch(() => []),
        loadPermissionCatalog(),
      ])
      return buildRoleWorkspace(kind, roles, audits, 0, permissionCatalog)
    }
    case "platform-audit": {
      const page = await fetchPlatformAudit(auditFiltersToQuery(auditFilters, auditSearch, { includeSource: false }))
      return buildAuditWorkspace(kind, page.results, page.page.total)
    }
    case "enterprise-people": {
      const [page, roles] = await Promise.all([
        fetchEnterpriseIAMMembers({ page: 1, limit: 100 }, tenantId),
        fetchEnterpriseIAMRoles(tenantId).catch(() => []),
      ])
      return buildEnterpriseMembers(page.results, page.page.total, tenantId, roles)
    }
    case "enterprise-customer-users": {
      const page = await fetchEnterpriseIAMCustomerUsers({ page: 1, limit: 100 }, tenantId)
      return buildEnterpriseCustomerUsers(page.results, page.page.total, tenantId)
    }
    case "enterprise-partners": {
      const page = await fetchEnterpriseIAMPartners({ page: 1, limit: 100 }, tenantId)
      return buildEnterprisePartners(page.results)
    }
    case "enterprise-org": {
      const page = await fetchEnterpriseIAMDepartments({ page: 1, limit: 100 }, tenantId)
      return buildEnterpriseDepartments(page.results, page.page.total, tenantId)
    }
    case "enterprise-permissions": {
      const [roles, audits, permissionCatalog] = await Promise.all([
        fetchEnterpriseIAMRoles(tenantId),
        fetchEnterpriseIAMAudit({ page: 1, limit: 20 }, tenantId).then((page) => page.results).catch(() => []),
        loadPermissionCatalog(),
      ])
      return buildRoleWorkspace(kind, roles, audits, tenantId, permissionCatalog)
    }
    case "enterprise-audit": {
      const page = await fetchEnterpriseIAMAudit(auditFiltersToQuery(auditFilters, auditSearch), tenantId)
      return buildAuditWorkspace(kind, page.results, page.page.total)
    }
  }
}

function isAuditWorkspace(kind: IAMWorkspaceKind) {
  return kind === "enterprise-audit" || kind === "platform-audit"
}

function shouldHideIAMMetrics(kind: IAMWorkspaceKind) {
  return kind === "enterprise-people" ||
    kind === "enterprise-customer-users" ||
    kind === "enterprise-partners" ||
    kind === "enterprise-permissions" ||
    kind === "enterprise-audit" ||
    kind === "platform-staff" ||
    kind === "platform-permissions" ||
    kind === "platform-audit"
}

function auditFiltersToQuery(filters: AuditWorkspaceFilters, search = "", options?: { includeSource?: boolean }) {
  const trimmedSearch = search.trim()
  return {
    period: filters.period,
    source: options?.includeSource === false ? undefined : filters.source,
    search: trimmedSearch || undefined,
    status: filters.status === "all" ? undefined : filters.status,
    targetType: filters.targetType === "all" ? undefined : filters.targetType,
    page: 1,
    limit: 100,
  }
}

async function loadPermissionCatalog() {
  try {
    return await fetchPermissionCatalog({ status: 0 })
  } catch {
    return []
  }
}

function resolveIAMTenantId(rawTenantId: string | null, sessionTenantId?: number) {
  const queryTenantId = Number(rawTenantId)
  if (Number.isFinite(queryTenantId) && queryTenantId > 0) {
    return queryTenantId
  }
  if (sessionTenantId && sessionTenantId > 0) {
    return sessionTenantId
  }
  return 1
}

function buildPlatformStaff(items: PlatformStaff[]): IAMWorkspaceModel {
  const active = items.filter((item) => item.status === 0)
  return mergeBase("platform-staff", {
    columns: platformStaffColumns,
    metrics: [
      metric(tr("workspace.metrics.platformStaff"), items.length, "", "blue"),
      metric(tr("workspace.metrics.activeAccounts"), active.length, "", "green"),
      metric(tr("workspace.metrics.team"), uniqueCount(items.map((item) => item.teamCode)), "", "slate"),
      metric(tr("workspace.metrics.supportLevel"), uniqueCount(items.map((item) => item.supportLevel)), "", "amber"),
    ],
    panels: [],
    rows: items.map((item) => ({
      id: String(item.id),
      platformStaff: item,
      searchText: compactSearch([
        item.displayName,
        item.username,
        item.email,
        item.teamCode,
        item.jobTitle,
        item.supportLevel,
        item.employmentType,
      ]),
      tags: [
        ...(matchesStaffCategory(item, "operations") ? [tr("workspace.tabs.platformOperations")] : []),
        ...(matchesStaffCategory(item, "audit") ? [tr("workspace.tabs.platformAudit")] : []),
        ...statusTags(item.status),
      ],
      cells: {
        subject: (
          <div className="flex min-w-0 items-center gap-3">
            <span className="inline-flex size-8 shrink-0 items-center justify-center rounded-md border border-primary/15 bg-primary/5 text-primary">
              <UserRoundIcon className="size-4" />
            </span>
            <span className="min-w-0">
              <span className="block truncate font-medium text-foreground">{item.displayName || item.username || `#${item.id}`}</span>
              <span className="mt-0.5 block truncate text-xs text-muted-foreground">{item.username || tr("common.userWithId", { id: item.userId })}</span>
            </span>
          </div>
        ),
        email: item.email || "-",
        roles: detail(item.supportLevel || tr("common.unset"), item.jobTitle || tr("common.unsetJobTitle")),
        scope: detail(item.teamCode || tr("common.unsetTeam"), item.employmentType || "employee"),
        status: statusPill(item.status),
        updated: item.updatedAt || "-",
      },
    })),
  })
}

function buildEnterpriseMembers(items: EnterpriseIAMMember[], total: number, tenantId: number, roleCatalog: IAMRole[]): IAMWorkspaceModel {
  const roleNameByCode = new Map(roleCatalog.map((role) => [role.code, role.name || role.code]))
  const members = items.map((member) => ({
    ...member,
    role_names: member.roles.map((code) => roleNameByCode.get(code) || code),
  }))
  return mergeBase("enterprise-people", {
    columns: identityColumns,
    details: members.map((member) => ({ id: String(member.id), kind: "enterprise-member", member })),
    metrics: [
      metric(tr("workspace.metrics.enterpriseMember"), total, tr("common.tenant", { id: tenantId }), "blue"),
      metric(tr("workspace.tabs.dispatchable"), items.filter((item) => item.dispatch_enabled).length, tr("workspace.metrics.engineers"), "green"),
      metric(tr("workspace.metrics.productGroupMembers"), items.filter((item) => item.product_groups?.length > 0).length, tr("workspace.metrics.repairScope"), "blue"),
      metric(tr("workspace.tabs.assignedRole"), items.filter((item) => item.roles.length > 0).length, tr("workspace.metrics.assignedRoles"), "blue"),
    ],
    panels: [],
    roleCatalog,
    rows: members.map((item) => ({
      id: String(item.id),
      enterpriseMember: item,
      subjectStatus: item.status,
      subjectUserId: item.user_id,
      accessibleLabel: tr("workspace.detail.enterpriseMemberAccessible", { name: item.display_name || item.username || tr("common.memberWithId", { id: item.id }) }),
      searchText: compactSearch([
        item.display_name,
        item.username,
        item.email,
        item.member_no,
        item.department_name,
        item.job_title,
        ...(item.product_groups || []).flatMap((group) => [group.product_name, group.team_name]),
        ...item.roles,
        ...(item.role_names || []),
      ]),
      tags: [
        ...(item.dispatch_enabled ? [tr("workspace.tabs.dispatchable")] : []),
        ...(item.product_groups?.length ? [tr("workspace.detail.productGroup")] : []),
        ...(item.roles.length > 0 ? [tr("workspace.tabs.assignedRole")] : []),
        ...statusTags(item.status),
      ],
      cells: {
        subject: strong(item.display_name || item.username || `member:${item.id}`),
        identity: detail(getAuditSubjectLabel("enterprise_member"), item.email || item.username || item.member_no || tr("common.userWithId", { id: item.user_id })),
        roles: roleChips(item.role_names || item.roles),
        scope: detail(item.department_name || tr("common.unassignedDepartment"), productGroupSummary(item.product_groups) || jsonSummary(item.skill_tags_json) || item.job_title || "-"),
        status: statusPill(item.status),
        updated: item.updated_at || item.joined_at || "-",
      },
    })),
  })
}

function productGroupSummary(groups?: EnterpriseIAMMember["product_groups"]) {
  if (!groups?.length) {
    return ""
  }
  const names = groups.map((group) => group.product_name || group.team_name || tr("workspace.detail.productGroupWithId", { id: group.team_id })).filter(Boolean)
  if (names.length <= 2) {
    return tr("workspace.detail.productGroupLabel", { names: names.join("、") })
  }
  return tr("workspace.detail.productGroupMore", { names: names.slice(0, 2).join("、"), count: names.length })
}

function buildEnterpriseCustomerUsers(items: EnterpriseIAMCustomerUser[], total: number, tenantId: number): IAMWorkspaceModel {
  return mergeBase("enterprise-customer-users", {
    columns: customerUserColumns,
    details: items.map((customerUser) => ({
      id: String(customerUser.id),
      kind: "enterprise-customer-user",
      customerUser,
    })),
    metrics: [
      metric(tr("workspace.metrics.customerUsers"), total, tr("common.tenant", { id: tenantId }), "blue"),
      metric(tr("workspace.metrics.customerOrg"), uniqueCount(items.map((item) => item.customer_org_id)), tr("workspace.metrics.customerOrg"), "slate"),
      metric(tr("workspace.metrics.disabled"), items.filter((item) => item.status !== 0).length, tr("workspace.metrics.accountStatus"), "amber"),
    ],
    panels: [],
    rows: items.map((item) => ({
      id: String(item.id),
      customerUser: item,
      accessibleLabel: tr("workspace.detail.customerUserAccessible", { name: item.display_name || item.email || tr("workspace.detail.customerUserWithId", { id: item.id }) }),
      searchText: compactSearch([
        item.display_name,
        item.email,
        item.phone,
        item.customer_org_name,
        item.locale,
      ]),
      tags: [
        ...(item.customer_org_id > 0 ? [tr("workspace.tabs.boundOrganization")] : []),
        ...statusTags(item.status),
      ],
      cells: {
        subject: strong(item.display_name || item.email || `customer:${item.id}`),
        identity: detail(getAuditSubjectLabel("customer_user"), item.email || item.phone || tr("common.userWithId", { id: item.user_id })),
        scope: detail(item.customer_org_name || tr("workspace.detail.customerOrgWithId", { id: item.customer_org_id || "-" }), `${item.locale || "-"} / ${item.timezone || "-"}`),
        status: statusPill(item.status),
        updated: item.last_seen_at || item.updated_at || "-",
      },
    })),
  })
}

function buildEnterprisePartners(items: EnterpriseIAMPartner[]): IAMWorkspaceModel {
  return mergeBase("enterprise-partners", {
    columns: identityColumns,
    details: items.map((partner) => ({
      id: String(partner.id),
      kind: "enterprise-partner",
      partner,
    })),
    metrics: [],
    panels: [],
    rows: items.map((item) => ({
      id: String(item.id),
      partner: item,
      accessibleLabel: tr("workspace.detail.supplierAccessible", { name: item.name || item.partner_no || tr("workspace.detail.supplierWithId", { id: item.id }) }),
      searchText: compactSearch([
        item.name,
        item.partner_no,
        item.partner_type,
        item.country_region,
        item.contact_name,
      ]),
      tags: [],
      cells: {
        subject: strong(item.name || item.partner_no || `partner:${item.id}`),
        identity: detail(getAuditTargetLabel("partner_company"), item.partner_no || `#${item.id}`),
        roles: detail(tr("workspace.detail.accountCount", { count: item.account_count }), tr("workspace.detail.contractCount", { count: item.contract_count })),
        scope: detail(item.country_region || "-", item.partner_type || item.contact_name || "-"),
        status: statusPill(item.status),
        updated: item.updated_at || "-",
      },
    })),
  })
}

function buildEnterpriseDepartments(items: EnterpriseIAMDepartment[], total: number, tenantId: number): IAMWorkspaceModel {
  return mergeBase("enterprise-org", {
    columns: identityColumns,
    metrics: [
      metric(tr("workspace.metrics.organizationNode"), total, tr("common.tenant", { id: tenantId }), "blue"),
      metric(tr("workspace.metrics.memberCoverage"), items.reduce((sum, item) => sum + item.member_count, 0), "members", "green"),
      metric(tr("workspace.metrics.serviceRegion"), uniqueCount(items.map((item) => item.region_code)), "region", "slate"),
      metric(tr("workspace.metrics.disabled"), items.filter((item) => item.status !== 0).length, tr("workspace.metrics.accountStatus"), "amber"),
    ],
    panels: [],
    rows: items.map((item) => ({
      id: String(item.id),
      searchText: compactSearch([
        item.name,
        item.department_code,
        item.path,
        item.region_code,
        item.manager_name,
      ]),
      tags: [
        ...(item.member_count > 0 ? [tr("workspace.tabs.hasMembers")] : []),
        ...(item.manager_member_id > 0 ? [tr("workspace.tabs.hasLeader")] : []),
        ...statusTags(item.status),
      ],
      cells: {
        subject: strong(item.name || item.department_code || `dept:${item.id}`),
        identity: detail(tr("workspace.detail.department"), item.department_code || `#${item.id}`),
        roles: detail(item.manager_name || tr("workspace.detail.unsetManager"), tr("workspace.detail.peopleCount", { count: item.member_count })),
        scope: detail(item.region_code || "-", item.path || `parent:${item.parent_id}`),
        status: statusPill(item.status),
        updated: item.updated_at || "-",
      },
    })),
  })
}

function buildRoleWorkspace(
  kind: "enterprise-permissions" | "platform-permissions",
  roles: IAMRole[],
  audits: IAMAuditLog[],
  tenantId = 0,
  permissionCatalog: AdminPermission[] = []
): IAMWorkspaceModel {
  const domain = kind === "platform-permissions" ? tr("workspace.detail.platform") : tr("workspace.detail.enterprise")
  return mergeBase(kind, {
    columns: roleColumns,
    details: roles.map((role) => ({ id: String(role.id), kind: "role", role })),
    metrics: [
      metric(tr("workspace.metrics.role"), roles.length, domain, "blue"),
      metric(tr("workspace.metrics.builtinRole"), roles.filter((role) => role.isBuiltin).length, "builtin", "green"),
      metric(tr("workspace.metrics.permissionCodes"), roles.reduce((sum, role) => sum + (role.permissions?.length ?? 0), 0), tr("workspace.metrics.permissionPolicy"), "blue"),
      metric(tr("workspace.metrics.recentOperations"), audits.length, tr("workspace.metrics.auditRecord"), "slate"),
    ],
    panels: [],
    permissionCatalog,
    rows: roles.map((role) => ({
      id: String(role.id),
      role,
      accessibleLabel: tr("workspace.detail.roleAccessible", { name: role.name || role.code }),
      searchText: compactSearch([role.name, role.code, role.description, ...(role.permissions ?? [])]),
      tags: [
        role.isBuiltin ? tr("workspace.detail.systemRole") : tr("workspace.detail.customRole"),
        ...statusTags(role.status),
      ],
      cells: {
        role: detail(role.name || role.code, role.code),
        domain: detail(role.domainType === "platform" ? tr("workspace.detail.platformPermissionDomain") : tr("workspace.detail.enterprisePermissionDomain"), tr("common.tenant", { id: role.tenantId })),
        permissions: `${role.permissions?.length ?? 0}`,
        builtin: role.isBuiltin ? <StatusPill tone="blue">{tr("workspace.detail.systemRole")}</StatusPill> : <StatusPill>{tr("workspace.detail.custom")}</StatusPill>,
        status: statusPill(role.status),
        updated: role.updatedAt || "-",
      },
    })),
  })
}

function buildAuditWorkspace(kind: "enterprise-audit" | "platform-audit", audits: IAMAuditLog[], total: number): IAMWorkspaceModel {
  const platformAudit = kind === "platform-audit"
  const failedOrBlocked = audits.filter((item) => isFailedAudit(item.status))
  const assistedAccess = audits.filter(isAssistedAccessAudit)
  const chatRecords = audits.filter(isChatRecordAudit)
  return mergeBase(kind, {
    columns: auditColumns,
    details: audits.map((audit) => ({
      id: String(audit.id),
      kind: "audit-log",
      audit,
      scope: platformAudit ? "platform" : "enterprise",
    })),
    metrics: [
      metric(tr("workspace.metrics.auditRecord"), total, platformAudit ? tr("workspace.metrics.platformRange") : tr("workspace.metrics.currentEnterprise"), "blue"),
      metric(tr("workspace.metrics.operationSuccess"), audits.filter((item) => item.status === "success").length, tr("workspace.metrics.currentList"), "green"),
      metric(tr("workspace.metrics.operationNotSuccessful"), failedOrBlocked.length, tr("workspace.metrics.currentList"), "slate"),
      metric(tr("workspace.metrics.assistedAccess"), assistedAccess.length, tr("workspace.metrics.authorization"), "slate"),
      ...(platformAudit ? [] : [metric(tr("workspace.metrics.chatRecord"), chatRecords.length, tr("workspace.metrics.currentList"), "blue")]),
    ],
    panels: [],
    rows: audits.map((item) => ({
      id: String(item.id),
      accessibleLabel: auditAccessibleLabel(item, platformAudit),
      searchText: compactSearch([
        getAuditActionLabel(item.action),
        item.action,
        getAuditSubjectLabel(item.actorSubjectType || ""),
        item.actorSubjectType,
        item.actorSubjectId,
        item.actorUserId,
        getAuditTargetLabel(item.targetType || ""),
        item.targetType,
        item.targetId,
        item.summary,
        ...Object.values(item.metadata ?? {}),
        item.requestId,
      ]),
      cells: {
        action: detail(
          auditActionLabel(item.action, item.id),
          auditRecordSubLabel(item),
        ),
        actor: auditActorCell(item),
        target: auditTargetCell(item, platformAudit),
        status: auditStatusPill(item.status),
        occurred: item.occurredAt || "-",
      },
    })),
  })
}

function mergeBase(kind: IAMWorkspaceKind, data: Partial<IAMWorkspaceModel>): IAMWorkspaceModel {
  const copy = baseCopy[kind]
  return {
    ...defaultState,
    ...copy,
    ...data,
    actions: data.actions ?? copy.actions ?? [],
    emptyMessage: data.emptyMessage ?? copy.emptyMessage,
    searchPlaceholder: data.searchPlaceholder ?? copy.searchPlaceholder,
    tabs: data.tabs ?? copy.tabs ?? [],
    title: data.title ?? copy.title,
  }
}

function metric(label: string, value: number | string, meta: string, tone: WorkspaceMetric["tone"] = "slate"): WorkspaceMetric {
  return { label, value: String(value), meta, tone }
}

function strong(value: ReactNode) {
  return <span className="rhd-railops-iam-primary-value">{value}</span>
}

function auditActionLabel(action: string, id: number) {
  const raw = action.trim()
  return (
    <span className="font-semibold text-foreground" title={raw || `audit:${id}`}>
      {getAuditActionLabel(raw || `audit:${id}`)}
    </span>
  )
}

function auditRecordLabel(id: number) {
  return id < 0 ? tr("common.businessAudit", { id: Math.abs(id) }) : tr("common.authAudit", { id })
}

function auditRecordSubLabel(item: IAMAuditLog) {
  if (isChatRecordAudit(item)) {
    const conversationId = item.metadata?.conversationId
    const messageId = item.metadata?.messageId || item.targetId
    if (conversationId && messageId) return tr("workspace.auditConversationMessage", { conversationId, messageId })
    if (conversationId) return tr("workspace.auditConversation", { id: conversationId })
    if (messageId) return tr("workspace.auditMessage", { id: messageId })
  }
  return item.supportGrantId > 0 ? tr("workspace.auditSupportGrant", { id: item.supportGrantId }) : auditRecordLabel(item.id)
}

function detail(primary: ReactNode, secondary?: ReactNode) {
  return (
    <span className="rhd-railops-iam-inline-detail">
      <span className="rhd-railops-iam-inline-primary">{primary}</span>
      {secondary ? (
        <>
          <span className="rhd-railops-iam-inline-separator">·</span>
          <span className="rhd-railops-iam-inline-secondary">{secondary}</span>
        </>
      ) : null}
    </span>
  )
}

function roleChips(roles: string[]) {
  if (!roles.length) {
    return <span className="rhd-railops-iam-muted-value">{tr("common.unbound")}</span>
  }
  return (
    <div className="rhd-railops-iam-role-chips">
      {roles.slice(0, 2).map((role) => (
        <StatusPill key={role} tone="blue">{role}</StatusPill>
      ))}
      {roles.length > 2 ? <StatusPill>+{roles.length - 2}</StatusPill> : null}
    </div>
  )
}

function statusPill(status: number) {
  return status === 0 ? <StatusPill tone="green">{tr("common.enabled")}</StatusPill> : <StatusPill tone="amber">{tr("common.disabled")}</StatusPill>
}

function auditStatusPill(status: string) {
  if (status === "success") {
    return <StatusPill tone="green">{getAuditStatusLabel(status)}</StatusPill>
  }
  if (status === "blocked" || status === "failure") {
    return <StatusPill tone="red">{getAuditStatusLabel(status)}</StatusPill>
  }
  return <StatusPill>{getAuditStatusLabel(status)}</StatusPill>
}

function auditActorCell(item: IAMAuditLog) {
  const subject = getAuditSubjectLabel(item.actorSubjectType || "")
  const subjectId = item.actorSubjectId || item.actorUserId
  return detail(
    subjectId ? `${subject} #${subjectId}` : subject,
    item.actorUserId ? tr("common.accountId", { id: item.actorUserId }) : tr("common.systemAutomatic"),
  )
}

function auditTargetCell(item: IAMAuditLog, platformAudit: boolean) {
  const target = getAuditTargetLabel(item.targetType || "")
  if (isChatRecordAudit(item)) {
    const conversationId = item.metadata?.conversationId
    const messageId = item.metadata?.messageId || item.targetId
    return detail(
      messageId ? `${target} #${messageId}` : target,
      conversationId ? tr("workspace.serviceSessionWithId", { id: conversationId }) : item.metadata?.contentSummary || item.summary,
    )
  }
  if (!platformAudit && item.targetType === "tenant" && String(item.tenantId) === item.targetId) {
    return detail(tr("common.currentEnterprise"), tr("common.enterpriseId", { id: item.targetId }))
  }
  return detail(item.targetId ? `${target} #${item.targetId}` : target)
}

function auditAccessibleLabel(item: IAMAuditLog, platformAudit: boolean) {
  const actor = getAuditSubjectLabel(item.actorSubjectType || "")
  const target = !platformAudit && item.targetType === "tenant" && String(item.tenantId) === item.targetId
    ? tr("common.currentEnterprise")
    : getAuditTargetLabel(item.targetType || "")
  return tr("workspace.auditAccessibleLabel", {
    action: getAuditActionLabel(item.action),
    actor,
    target,
    status: getAuditStatusLabel(item.status),
  })
}

function isAssistedAccessAudit(item: IAMAuditLog) {
  return item.supportGrantId > 0 || item.action.endsWith(".support_session_started")
}

function isChatRecordAudit(item: IAMAuditLog) {
  return item.targetType === "message" ||
    item.targetType === "conversation_message" ||
    item.action.startsWith("conversation.message_") ||
    item.action.startsWith("message.")
}

function isFailedAudit(status: string) {
  return ["blocked", "error", "failed", "failure"].includes(status.toLowerCase())
}

function jsonSummary(raw: string) {
  if (!raw) {
    return ""
  }
  try {
    const parsed = JSON.parse(raw) as unknown
    if (Array.isArray(parsed)) {
      return parsed.slice(0, 3).join(", ")
    }
  } catch {
    return raw
  }
  return raw
}

function uniqueCount(values: Array<string | number | null | undefined>) {
  return new Set(values.filter((value) => value !== undefined && value !== null && value !== "")).size
}

function compactSearch(values: Array<string | number | null | undefined>) {
  return values
    .filter((value) => value !== undefined && value !== null && String(value).trim() !== "")
    .map(String)
    .join(" ")
}

function statusTags(status: number) {
  return status === 0 ? [] : [tr("common.disabled")]
}

function matchesStaffCategory(item: PlatformStaff, category: "audit" | "operations") {
  const haystack = compactSearch([
    item.teamCode,
    item.jobTitle,
    item.supportLevel,
    item.employmentType,
  ]).toLowerCase()
  if (category === "audit") {
    return haystack.includes("audit") || haystack.includes("审计")
  }
  return haystack.includes("operation") || haystack.includes("ops") || haystack.includes("运维")
}
