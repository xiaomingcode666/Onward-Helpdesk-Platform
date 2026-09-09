"use client"

import { useCallback, useEffect, useMemo, useState } from "react"
import Link from "next/link"
import { useRouter, useSearchParams } from "next/navigation"
import {
  EyeIcon,
  FolderOpenIcon,
  PencilIcon,
  PlusIcon,
  RefreshCwIcon,
  XIcon,
} from "lucide-react"
import { toast } from "sonner"
import type { TableColumnsType } from "antd"
import {
  DataTable,
  PageShell,
  RailopsButton,
  SearchField,
  SelectField,
  StandardModal,
  StatusTag,
  TablePagination,
  type StatusTagTone,
} from "@railops/ui"

import { useAuth } from "@/components/auth-provider"
import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { CanUseButton } from "@/components/layout/permission-guard"
import { EmptyState, ErrorState } from "@/components/shared/error-states"
import { useI18n } from "@/i18n/provider"
import {
  createEnterpriseProduct,
  fetchProductCount,
  getProductResources,
  listAllProducts,
  resetProductAIQuota,
  updateEnterpriseProduct,
  updateProductAIQuota,
  type ProductResources,
} from "@/lib/api/enterprise-products"
import { getEnterpriseAIModelsWorkspace } from "@/lib/api/enterprise-models"
import { getRequestTenantId } from "@/lib/api/client"
import {
  fetchEnterpriseIAMMembers,
  type EnterpriseIAMMember,
} from "@/lib/api/platform-iam"
import type { CreateProductPayload, ProductListItem, UpdateProductPayload } from "@/lib/api/types"
import { cn } from "@/lib/utils"
import { ProductDetailWorkspace } from "./[productId]/_components/product-detail-workspace"

const WORKSPACE_TABS = [
  { value: "faultStats", labelKey: "productCenter.tabs.faultStats" },
  { value: "repairHistory", labelKey: "productCenter.tabs.repairHistory" },
  { value: "models", labelKey: "productCenter.tabs.models" },
  { value: "modules", labelKey: "productCenter.tabs.modules" },
  { value: "manuals", labelKey: "productCenter.tabs.manuals" },
] as const

const DEFAULT_PRODUCT_LOCALE = "zh-CN"
const PRODUCT_OWNER_OPTION_PAGE_SIZE = 50

type WorkspaceTab = (typeof WORKSPACE_TABS)[number]["value"]
type I18nT = ReturnType<typeof useI18n>
type ProductResourcesById = Partial<Record<number, ProductResources>>

function enterpriseMemberName(member: EnterpriseIAMMember, t?: I18nT) {
  return member.display_name || member.username || t?.("productCenter.page.memberFallback", { id: member.id }) || `Member #${member.id}`
}

function enterpriseMemberOptionLabel(t: I18nT, member: EnterpriseIAMMember) {
  const roleText = member.dispatch_enabled
    ? t("productCenter.page.memberDispatch")
    : (member.role_names || member.roles || []).join("、") || member.job_title || t("productCenter.page.memberDefault")
  return `${enterpriseMemberName(member, t)} · ${roleText}`
}

function activeProductOwnerOptions(members: EnterpriseIAMMember[]) {
  return (members || [])
    .filter((item) => item.status === 0)
    .sort((left, right) => {
      const dispatchPriority = Number(right.dispatch_enabled) - Number(left.dispatch_enabled)
      if (dispatchPriority !== 0) return dispatchPriority
      return enterpriseMemberName(left).localeCompare(enterpriseMemberName(right), "zh-CN")
    })
}

function ownerMemberOptions(
  t: I18nT,
  members: EnterpriseIAMMember[],
  currentOwnerId?: number,
  currentOwnerName?: string
): { value: string; label: string }[] {
  const hasCurrentOwner =
    !currentOwnerId || members.some((member) => member.id === currentOwnerId)
  const fallbackOwnerName =
    currentOwnerName?.trim() || (currentOwnerId ? t("productCenter.page.memberFallback", { id: currentOwnerId }) : "")

  return [
    ...(currentOwnerId && !hasCurrentOwner
      ? [{ value: String(currentOwnerId), label: fallbackOwnerName }]
      : []),
    ...members.map((member) => ({
      value: String(member.id),
      label: enterpriseMemberOptionLabel(t, member),
    })),
  ]
}

function canEditProductForSession(
  product: ProductListItem | null,
  session: { subjectId?: number; user?: { id: number } } | null | undefined,
  hasProductUpdatePermission: boolean
) {
  if (!product) return false
  if (hasProductUpdatePermission) return true
  if (product.owner_member_id && session?.subjectId === product.owner_member_id) return true
  return Boolean(product.owner_user_id && session?.user?.id === product.owner_user_id)
}

function parseWorkspaceTab(value: string | null): WorkspaceTab {
  return WORKSPACE_TABS.some((tab) => tab.value === value)
    ? (value as WorkspaceTab)
    : "faultStats"
}

function buildProductArchivePath(productId: number, tab: WorkspaceTab = "faultStats") {
  const params = new URLSearchParams()
  params.set("product_id", String(productId))
  params.set("tab", tab)
  return `/enterprise/products?${params.toString()}`
}

function parseProductArchiveId(value: string | null) {
  const productId = Number(value)
  return Number.isSafeInteger(productId) && productId > 0 ? productId : 0
}

function getStatusZh(t: I18nT, status: string) {
  switch (status) {
    case "active":
      return t("productCenter.page.status.active")
    case "inactive":
      return t("productCenter.page.status.inactive")
    case "discontinued":
      return t("productCenter.page.status.discontinued")
    default:
      return status
  }
}

function formatCurrency(value: number, currency = "USD") {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: currency || "USD",
    minimumFractionDigits: 4,
    maximumFractionDigits: 4,
  }).format(Number.isFinite(value) ? value : 0)
}

function formatProductNumber(value: number) {
  return new Intl.NumberFormat("zh-CN").format(Number.isFinite(value) ? value : 0)
}

function getResourceStatusZh(t: I18nT, status?: string) {
  switch (status) {
    case "ready":
      return t("productCenter.page.resourceStatus.ready")
    case "partial_failed":
      return t("productCenter.page.resourceStatus.partialFailed")
    case "failed":
      return t("productCenter.page.resourceStatus.failed")
    case "pending":
      return t("productCenter.page.resourceStatus.pending")
    case "running":
      return t("productCenter.page.resourceStatus.running")
    case "waiting_retry":
      return t("productCenter.page.resourceStatus.waitingRetry")
    case "succeeded":
      return t("productCenter.page.resourceStatus.succeeded")
    case "missing":
      return t("productCenter.page.resourceStatus.missing")
    case "unreviewed":
      return t("productCenter.page.resourceStatus.unreviewed")
    case "approved":
      return t("productCenter.page.resourceStatus.approved")
    case "rejected":
      return t("productCenter.page.resourceStatus.rejected")
    default:
      return status || t("productCenter.page.resourceStatus.unknown")
  }
}

function getProductStatusTone(status: string): StatusTagTone {
  switch (status) {
    case "active":
      return "success"
    case "inactive":
      return "disabled"
    case "discontinued":
      return "warning"
    default:
      return "neutral"
  }
}

function getResourceStatusTone(status?: string): StatusTagTone {
  if (status === "ready" || status === "approved" || status === "succeeded") return "success"
  if (status === "failed" || status === "partial_failed" || status === "rejected") return "error"
  if (status === "pending" || status === "running" || status === "waiting_retry" || status === "unreviewed") return "warning"
  if (status === "missing") return "disabled"
  return "neutral"
}

function CreateProductModal({
  aiEnabled,
  open,
  onClose,
  onCreated,
}: {
  aiEnabled: boolean
  open: boolean
  onClose: () => void
  onCreated: (id: number) => void
}) {
  const t = useI18n()
  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const [aiQuotaLimit, setAIQuotaLimit] = useState("")
  const [ownerMemberId, setOwnerMemberId] = useState("")
  const [ownerOptions, setOwnerOptions] = useState<EnterpriseIAMMember[]>([])
  const [ownersLoading, setOwnersLoading] = useState(false)
  const [tenantAIQuotaRemaining, setTenantAIQuotaRemaining] = useState<number | null>(null)
  const [tenantAIQuotaLoading, setTenantAIQuotaLoading] = useState(false)
  const [tenantAIQuotaError, setTenantAIQuotaError] = useState("")
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState("")

  const reset = useCallback(() => {
    setName("")
    setDescription("")
    setAIQuotaLimit("")
    setOwnerMemberId("")
    setTenantAIQuotaError("")
    setError("")
  }, [])

  useEffect(() => {
    if (!open) return
    let cancelled = false
    setOwnersLoading(true)
    fetchEnterpriseIAMMembers(
      { page: 1, limit: PRODUCT_OWNER_OPTION_PAGE_SIZE, status: "0" },
      getRequestTenantId()
    )
      .then((page) => {
        if (cancelled) return
        setOwnerOptions(activeProductOwnerOptions(page.results || []))
      })
      .catch((err) => {
        if (!cancelled) {
          toast.error(err instanceof Error ? err.message : t("productCenter.page.errors.memberLoadFailed"))
        }
      })
      .finally(() => {
        if (!cancelled) setOwnersLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [open, t])

  useEffect(() => {
    if (!open || !aiEnabled) return
    let cancelled = false
    setTenantAIQuotaLoading(true)
    setTenantAIQuotaError("")
    getEnterpriseAIModelsWorkspace({ page: 1, pageSize: 1 })
      .then((response) => {
        if (cancelled) return
        if (!response.success || !response.data) {
          throw new Error(response.error?.message || t("productCenter.page.errors.quotaLoadFailed"))
        }
        setTenantAIQuotaRemaining(response.data.user?.balance ?? response.data.account?.balance ?? null)
      })
      .catch((err) => {
        if (!cancelled) {
          setTenantAIQuotaRemaining(null)
          setTenantAIQuotaError(err instanceof Error ? err.message : t("productCenter.page.errors.quotaLoadFailed"))
        }
      })
      .finally(() => {
        if (!cancelled) setTenantAIQuotaLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [aiEnabled, open, t])

  const tenantAIQuotaText = tenantAIQuotaLoading
    ? t("productCenter.page.quotaLoading")
    : tenantAIQuotaError
      ? t("productCenter.page.quotaUnavailable")
      : tenantAIQuotaRemaining !== null
        ? t("productCenter.page.quotaRemaining", { value: formatCurrency(tenantAIQuotaRemaining) })
        : t("productCenter.page.quotaNotConfigured")

  const handleSave = useCallback(async () => {
    if (!name.trim()) {
      setError(t("productCenter.product.nameRequired"))
      return
    }
    const ownerId = Number(ownerMemberId)
    if (!Number.isFinite(ownerId) || ownerId <= 0) {
      setError(t("productCenter.page.errors.ownerRequired"))
      return
    }
    const quotaValue = aiEnabled ? Number(aiQuotaLimit) : 0
    if (aiEnabled && aiQuotaLimit.trim() && (!Number.isFinite(quotaValue) || quotaValue < 0)) {
      setError(t("productCenter.page.errors.quotaInvalid"))
      return
    }
    if (
      aiEnabled &&
      tenantAIQuotaRemaining !== null &&
      Number.isFinite(quotaValue) &&
      quotaValue > tenantAIQuotaRemaining
    ) {
      setError(t("productCenter.page.errors.quotaExceeded", { value: formatCurrency(tenantAIQuotaRemaining) }))
      return
    }

    setSaving(true)
    setError("")
    try {
      const payload: CreateProductPayload = {
        name: name.trim(),
        description: description.trim(),
        owner_member_id: ownerId,
        default_locale: DEFAULT_PRODUCT_LOCALE,
        ai_quota_limit: aiEnabled && quotaValue > 0 ? quotaValue : undefined,
      }
      const response = await createEnterpriseProduct(payload)
      if (!response.success || !response.data) {
        throw new Error(response.error?.message || t("productCenter.page.errors.createFailed"))
      }
      onCreated(response.data.id)
      onClose()
      reset()
      toast.success(t("productCenter.page.createSuccess"))
    } catch (err) {
      const message = err instanceof Error ? err.message : t("productCenter.page.errors.createFailed")
      setError(message)
      toast.error(message)
    } finally {
      setSaving(false)
    }
  }, [aiEnabled, aiQuotaLimit, description, name, onClose, onCreated, ownerMemberId, reset, tenantAIQuotaRemaining])

  if (!open) return null

  return (
    <StandardModal
      open={open}
      title={t("productCenter.product.createTitle")}
      onCancel={onClose}
      width={520}
      footer={(
        <>
          <RailopsButton onClick={onClose} disabled={saving}>
            {t("productCenter.cancel")}
          </RailopsButton>
          <RailopsButton onClick={handleSave} disabled={saving}>
            {saving ? t("productCenter.saving") : t("productCenter.save")}
          </RailopsButton>
        </>
      )}
    >
<div>
          <div className="product-form-grid">
            <label>
              <span>{t("productCenter.product.name")}</span>
              <input
                placeholder={t("productCenter.page.productNamePlaceholder")}
                aria-label={t("productCenter.product.name")}
                value={name}
                onChange={(event) => setName(event.target.value)}
              />
            </label>
            <label>
              <span>{t("productCenter.product.code")}</span>
              <input
                aria-label={t("productCenter.product.code")}
                value={t("productCenter.page.autoGenerated")}
                disabled
                readOnly
              />
            </label>
            <SelectField
              label={t("productCenter.product.ownerMemberId")}
              style={{ marginBottom: 0 }}
              selectProps={{
                "aria-label": t("productCenter.product.ownerMemberId"),
                placeholder: ownersLoading ? t("productCenter.page.loadingMembers") : t("productCenter.page.selectOwner"),
                value: ownerMemberId || undefined,
                onChange: (value) => setOwnerMemberId((value as string) ?? ""),
                options: ownerMemberOptions(t, ownerOptions),
                disabled: ownersLoading,
                style: { width: "100%" },
              }}
            />
            {aiEnabled ? (
              <>
                <label>
                  <span className="product-quota-label">
                    {t("productCenter.page.initialQuota")}
                    <strong className={tenantAIQuotaError ? "product-quota-remaining is-warning" : "product-quota-remaining"}>
                      {tenantAIQuotaText}
                    </strong>
                  </span>
                  <input
                    type="number"
                    min="0"
                    step="0.01"
                    placeholder={t("productCenter.page.quotaPlaceholder")}
                    aria-label={t("productCenter.page.initialQuota")}
                    value={aiQuotaLimit}
                    onChange={(event) => setAIQuotaLimit(event.target.value)}
                  />
                </label>
                <label>
                  <span>{t("productCenter.page.knowledgeBaseLabel")}</span>
                  <input
                    aria-label={t("productCenter.page.knowledgeBaseLabel")}
                    value={t("productCenter.page.createdAfterSave")}
                    disabled
                    readOnly
                  />
                </label>
                <label>
                  <span>{t("productCenter.page.agentLabel")}</span>
                  <input
                    aria-label={t("productCenter.page.agentLabel")}
                    value={t("productCenter.page.createdAfterSave")}
                    disabled
                    readOnly
                  />
                </label>
              </>
            ) : null}
            <label style={{ gridColumn: "1 / -1" }}>
              <span>{t("productCenter.product.description")}</span>
              <textarea
                placeholder={t("productCenter.page.descriptionPlaceholder")}
                aria-label={t("productCenter.product.description")}
                value={description}
                onChange={(event) => setDescription(event.target.value)}
              />
            </label>
          </div>
          {error ? <p className="mt-3 text-sm text-destructive">{error}</p> : null}
        </div>
    </StandardModal>
  )
}

function EditProductModal({
  product,
  resources,
  canManageResources,
  onClose,
  onResourcesChanged,
  onSaved,
}: {
  product: ProductListItem | null
  resources: ProductResources | null
  canManageResources: boolean
  onClose: () => void
  onResourcesChanged: (resources: ProductResources) => void
  onSaved: () => void
}) {
  const t = useI18n()
  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const [ownerMemberId, setOwnerMemberId] = useState("")
  const [ownerOptions, setOwnerOptions] = useState<EnterpriseIAMMember[]>([])
  const [ownersLoading, setOwnersLoading] = useState(false)
  const [quotaLimit, setQuotaLimit] = useState("")
  const [saving, setSaving] = useState(false)
  const [quotaSaving, setQuotaSaving] = useState(false)
  const [resettingQuota, setResettingQuota] = useState(false)
  const [error, setError] = useState("")

  useEffect(() => {
    if (!product) return
    setName(product.name)
    setDescription(product.description || "")
    setOwnerMemberId(product.owner_member_id ? String(product.owner_member_id) : "")
    setError("")
  }, [product])

  useEffect(() => {
    if (!product) return
    let cancelled = false
    setOwnersLoading(true)
    fetchEnterpriseIAMMembers(
      { page: 1, limit: PRODUCT_OWNER_OPTION_PAGE_SIZE, status: "0" },
      getRequestTenantId()
    )
      .then((page) => {
        if (cancelled) return
        setOwnerOptions(activeProductOwnerOptions(page.results || []))
      })
      .catch((err) => {
        if (!cancelled) {
          toast.error(err instanceof Error ? err.message : t("productCenter.page.errors.memberLoadFailed"))
        }
      })
      .finally(() => {
        if (!cancelled) setOwnersLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [product])

  useEffect(() => {
    if (!resources) return
    const nextQuota = resources.ai_key?.quota_limit ?? 0
    setQuotaLimit(nextQuota > 0 ? String(nextQuota) : "")
  }, [resources])

  const handleSave = useCallback(async () => {
    if (!product) return
    if (!name.trim()) {
      setError(t("productCenter.product.nameRequired"))
      return
    }
    const ownerId = Number(ownerMemberId)
    if (!Number.isFinite(ownerId) || ownerId <= 0) {
      setError(t("productCenter.page.errors.ownerRequired"))
      return
    }
    setSaving(true)
    setError("")
    try {
      const payload: UpdateProductPayload = {
        name: name.trim(),
        description: description.trim(),
        owner_member_id: ownerId,
      }
      const response = await updateEnterpriseProduct(product.id, payload)
      if (!response.success) {
        throw new Error(response.error?.message || t("productCenter.product.saveFailed"))
      }
      onSaved()
      toast.success(t("productCenter.page.productSaved"))
      onClose()
    } catch (err) {
      const message = err instanceof Error ? err.message : t("productCenter.product.saveFailed")
      setError(message)
      toast.error(message)
    } finally {
      setSaving(false)
    }
  }, [description, name, onClose, onSaved, ownerMemberId, product])

  const handleSaveQuota = useCallback(async () => {
    if (!product) return
    const nextQuota = Number(quotaLimit)
    if (!Number.isFinite(nextQuota) || nextQuota < 0) {
      setError(t("productCenter.page.errors.quotaInvalid"))
      return
    }
    setQuotaSaving(true)
    setError("")
    try {
      const response = await updateProductAIQuota(product.id, nextQuota)
      if (!response.success || !response.data) {
        throw new Error(response.error?.message || t("productCenter.page.errors.quotaSaveFailed"))
      }
      onResourcesChanged(response.data)
      toast.success(response.data.ai_key?.status === "ready" ? t("productCenter.page.quotaSaved") : t("productCenter.page.quotaSavedQueued"))
    } catch (err) {
      const message = err instanceof Error ? err.message : t("productCenter.page.errors.quotaSaveFailed")
      setError(message)
      toast.error(message)
    } finally {
      setQuotaSaving(false)
    }
  }, [onResourcesChanged, product, quotaLimit])

  const handleResetQuota = useCallback(async () => {
    if (!product) return
    if (!window.confirm(t("productCenter.page.confirmResetQuota"))) {
      return
    }
    setResettingQuota(true)
    setError("")
    try {
      const response = await resetProductAIQuota(product.id)
      if (!response.success || !response.data) {
        throw new Error(response.error?.message || t("productCenter.page.errors.resetQuotaFailed"))
      }
      onResourcesChanged(response.data)
      toast.success(response.data.ai_key?.status === "ready" ? t("productCenter.page.quotaReset") : t("productCenter.page.quotaResetQueued"))
    } catch (err) {
      const message = err instanceof Error ? err.message : t("productCenter.page.errors.resetQuotaFailed")
      setError(message)
      toast.error(message)
    } finally {
      setResettingQuota(false)
    }
  }, [onResourcesChanged, product])

  if (!product) return null

  return (
    <StandardModal
      open={true}
      title={t("productCenter.product.editTitle")}
      onCancel={onClose}
      width={520}
      footer={(
        <>
          <RailopsButton onClick={onClose} disabled={saving}>
            {t("productCenter.cancel")}
          </RailopsButton>
          <RailopsButton onClick={handleSave} disabled={saving}>
            {saving ? t("productCenter.saving") : t("productCenter.save")}
          </RailopsButton>
        </>
      )}
    >
<div>
          <div className="product-form-grid">
            <label>
              <span>{t("productCenter.product.name")}</span>
              <input
                placeholder={t("productCenter.page.productNamePlaceholder")}
                aria-label={t("productCenter.product.name")}
                value={name}
                onChange={(event) => setName(event.target.value)}
              />
            </label>
            <label>
              <span>{t("productCenter.product.code")}</span>
              <input
                aria-label={t("productCenter.product.code")}
                value={product.code || `PROD-${product.id}`}
                readOnly
              />
            </label>
            <SelectField
              label={t("productCenter.product.ownerMemberId")}
              style={{ marginBottom: 0 }}
              selectProps={{
                "aria-label": t("productCenter.product.ownerMemberId"),
                placeholder: ownersLoading ? t("productCenter.page.loadingMembers") : t("productCenter.page.selectOwner"),
                value: ownerMemberId || undefined,
                onChange: (value) => setOwnerMemberId((value as string) ?? ""),
                options: ownerMemberOptions(t, ownerOptions, product.owner_member_id, product.owner_name),
                disabled: ownersLoading,
                style: { width: "100%" },
              }}
            />
            <label style={{ gridColumn: "1 / -1" }}>
              <span>{t("productCenter.product.description")}</span>
              <textarea
                placeholder={t("productCenter.page.descriptionPlaceholder")}
                aria-label={t("productCenter.product.description")}
                value={description}
                onChange={(event) => setDescription(event.target.value)}
              />
            </label>
          </div>

          {canManageResources ? (
            <div className="rhd-railops-product-editor-resource">
              <div className="rhd-railops-product-editor-resource-head">
                <div>
                  <strong>{t("productCenter.page.resourceQuotaTitle")}</strong>
                  <p>
                    {t("productCenter.page.resourceKeyLabel")}：{resources?.ai_key?.key_name || t("productCenter.page.notCreated")} ·{" "}
                    {resources?.ai_key?.key_preview || t("productCenter.page.pendingGeneration")}
                  </p>
                </div>
                <StatusTag tone={getResourceStatusTone(resources?.ai_key?.status)}>
                  {getResourceStatusZh(t, resources?.ai_key?.status)}
                </StatusTag>
              </div>

              <div className="rhd-railops-product-editor-quota-row">
                <label className="product-quota-field">
                  <span>{t("productCenter.page.quotaLimitLabel")}</span>
                  <input
                    type="number"
                    min="0"
                    step="0.01"
                    aria-label={t("productCenter.page.quotaLimitLabel")}
                    value={quotaLimit}
                    onChange={(event) => setQuotaLimit(event.target.value)}
                  />
                </label>
                <div className="flex items-end">
                  <RailopsButton onClick={handleSaveQuota} disabled={quotaSaving}>
                    {quotaSaving ? t("productCenter.saving") : t("productCenter.page.saveQuota")}
                  </RailopsButton>
                </div>
              </div>

              <div className="rhd-railops-product-editor-quota-usage">
                <div className="rhd-railops-product-editor-quota-meta">
                  <span>{t("productCenter.page.quotaUsedLabel")}</span>
                  <strong>
                    {formatCurrency(resources?.ai_key?.quota_used ?? 0, resources?.ai_key?.currency)}
                    {" / "}
                    {formatCurrency(
                      resources?.ai_key?.quota_limit ?? Number(quotaLimit || 0),
                      resources?.ai_key?.currency
                    )}
                  </strong>
                </div>
                <div className="rhd-railops-product-editor-quota-track">
                  <div
                    style={{
                      width: `${Math.min(
                        100,
                        ((resources?.ai_key?.quota_used ?? 0) /
                          Math.max(resources?.ai_key?.quota_limit ?? Number(quotaLimit || 0), 0.01)) *
                          100
                      )}%`,
                    }}
                  />
                </div>
                <div className="rhd-railops-product-editor-quota-actions">
                  <span>
                    {t("productCenter.page.lastSyncedLabel")}：{resources?.ai_key?.last_synced_at || t("productCenter.page.notSynced")}
                  </span>
                  <RailopsButton onClick={handleResetQuota} disabled={resettingQuota}>
                    {resettingQuota ? t("productCenter.page.resettingQuota") : t("productCenter.page.resetQuota")}
                  </RailopsButton>
                </div>
                {resources?.ai_key?.provision_error ? (
                  <p className="mt-2 text-xs text-destructive">{resources.ai_key.provision_error}</p>
                ) : null}
              </div>
            </div>
          ) : null}
          {error ? <p className="mt-3 text-sm text-destructive">{error}</p> : null}
        </div>
    </StandardModal>
  )
}

function ProductTableWorkspace({
  activeTab,
  canEdit,
  onEdit,
  onSelectProduct,
  productTotal,
  products,
  search,
  selectedProductId,
  onSearchChange,
}: {
  activeTab: WorkspaceTab
  canEdit: (product: ProductListItem) => boolean
  onEdit: (product: ProductListItem) => void
  onSelectProduct: (productId: number) => void
  productTotal: number
  products: ProductListItem[]
  search: string
  selectedProductId: number | null
  onSearchChange: (value: string) => void
}) {
  const t = useI18n()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)

  useEffect(() => {
    setPage(1)
  }, [products])

  const pageProducts = useMemo(
    () => products.slice((page - 1) * pageSize, page * pageSize),
    [page, pageSize, products],
  )
  const displayedProductTotal = search.trim() ? products.length : productTotal

  const columns: TableColumnsType<ProductListItem> = [
    {
      title: t("productCenter.product.name"),
      dataIndex: "name",
      width: 260,
      render: (_value, product) => (
        <div className="rhd-railops-product-list-name-cell">
          <strong title={product.name}>{product.name}</strong>
          <span title={product.code || `PROD-${product.id}`}>
            {t("productCenter.page.productCodeLabel")} {product.code || `PROD-${product.id}`}
          </span>
        </div>
      ),
    },
    {
      title: t("productCenter.product.description"),
      dataIndex: "description",
      width: 240,
      render: (value) => {
        const description = typeof value === "string" ? value.trim() : ""
        return (
          <span
            className={cn("rhd-railops-product-table-description", !description && "is-empty")}
            title={description || undefined}
          >
            {description || t("productCenter.page.descriptionUnset")}
          </span>
        )
      },
    },
    {
      title: t("enterpriseExtract.products.columnStatus"),
      dataIndex: "status",
      width: 116,
      align: "center",
      render: (_value, product) => (
        <StatusTag tone={getProductStatusTone(product.status)}>
          {getStatusZh(t, product.status)}
        </StatusTag>
      ),
    },
    {
      title: t("productCenter.page.productCategory"),
      dataIndex: "category",
      width: 160,
      render: (_value, product) => (
        <div className="rhd-railops-product-table-stack">
          <strong>{product.category || "-"}</strong>
          <span>{product.product_line || t("productCenter.page.defaultProductDirectory")}</span>
        </div>
      ),
    },
    {
      title: t("productCenter.page.productOwner"),
      dataIndex: "owner_name",
      width: 180,
      render: (_value, product) => (
        <div className="rhd-railops-product-table-stack">
          <strong>{product.owner_name || t("productCenter.page.ownerEmpty")}</strong>
          <span>{product.owner_name ? t("productCenter.page.ownerSynced") : t("productCenter.page.ownerPending")}</span>
        </div>
      ),
    },
    {
      title: t("productCenter.page.afterSalesTitle"),
      key: "archive",
      width: 156,
      render: (_value, product) => (
        <Link
          className="rhd-railops-product-table-link-cell"
          href={buildProductArchivePath(product.id, "repairHistory")}
          onClick={(event) => event.stopPropagation()}
        >
          <span>
            <FolderOpenIcon className="size-3.5" />
            {t("productCenter.page.afterSalesTitle")}
          </span>
          <strong>{t("productCenter.page.enterDossier")}</strong>
          <small>{t("productCenter.page.afterSalesSummary")}</small>
        </Link>
      ),
    },
    {
      title: t("productCenter.actions"),
      key: "actions",
      fixed: "right",
      width: 146,
      align: "right",
      render: (_value, product) => (
        <div className="rhd-railops-product-table-actions">
          {canEdit(product) ? (
            <RailopsButton
              size="small"
              onClick={(event) => {
                event.stopPropagation()
                onEdit(product)
              }}
            >
              <PencilIcon className="size-3.5" />
              {t("productCenter.page.editInfo")}
            </RailopsButton>
          ) : null}
          <Link
            aria-label={t("common.showDetail")}
            className="rhd-railops-row-action"
            href={buildProductArchivePath(product.id, activeTab)}
            title={t("common.showDetail")}
            onClick={(event) => {
              event.stopPropagation()
            }}
          >
            <EyeIcon className="size-4" />
          </Link>
        </div>
      ),
    },
  ]

  return (
    <main className="rhd-railops-product-table-workspace">
      <div className="rhd-railops-product-table-head">
        <div>
          <strong>{t("productCenter.page.productListTitle")}</strong>
          <span>{t("enterpriseExtract.products.productCount", { count: formatProductNumber(displayedProductTotal) })}</span>
        </div>
        <div className="rhd-railops-product-table-tools">
          <SearchField
            allowClear
            className="rhd-railops-product-table-search"
            placeholder={t("products.searchPlaceholder")}
            value={search}
            onChange={(event) => onSearchChange(event.target.value)}
          />
        </div>
      </div>
      <div className="rhd-railops-product-list-table-wrap">
        <DataTable<ProductListItem>
          className="rhd-railops-product-table rhd-railops-product-list-table"
          size="small"
          columns={columns}
          dataSource={pageProducts}
          rowKey={(record) => record.id}
          scroll={{ x: 1300 }}
          emptyDescription={t("enterpriseExtract.products.empty")}
          rowClassName={(product) => product.id === selectedProductId ? "is-selected" : ""}
          onRow={(product) => ({
            onClick: () => onSelectProduct(product.id),
          })}
        />
      </div>
      <TablePagination
        className="rhd-railops-table-footer"
        current={page}
        pageSize={pageSize}
        total={products.length}
        showSizeChanger
        footerNote=""
        onChange={(nextPage, nextPageSize) => {
          if (nextPageSize !== pageSize) {
            setPageSize(nextPageSize)
            setPage(1)
            return
          }
          setPage(nextPage)
        }}
      />
    </main>
  )
}

export default function EnterpriseProductsPage() {
  const searchParams = useSearchParams()
  const productArchiveId = parseProductArchiveId(searchParams.get("product_id"))
  if (productArchiveId > 0) {
    return <ProductDetailWorkspace productId={productArchiveId} routeMode="query" />
  }
  return <EnterpriseProductsListPage />
}

function EnterpriseProductsListPage() {
  const t = useI18n()
  const { session } = useAuth()
  const router = useRouter()
  const searchParams = useSearchParams()
  const [products, setProducts] = useState<ProductListItem[]>([])
  const [productTotal, setProductTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [search, setSearch] = useState("")
  const [selectedProductId, setSelectedProductId] = useState<number | null>(null)
  const [selectedResources, setSelectedResources] = useState<ProductResources | null>(null)
  const [productResources, setProductResources] = useState<ProductResourcesById>({})
  const [productResourceLoadingIds, setProductResourceLoadingIds] = useState<number[]>([])
  const activeTab = parseWorkspaceTab(searchParams.get("tab"))
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [editingProduct, setEditingProduct] = useState<ProductListItem | null>(null)
  const canCreateProduct = CanUseButton("product.create", session?.permissions)
  const canUpdateProduct = CanUseButton("product.update", session?.permissions)
  const tenantAIEnabled = session?.featureFlags?.ai !== false
  const canViewProductResources = tenantAIEnabled && CanUseButton("productAIUsageCredential.view", session?.permissions)
  const canRetryProductResources = tenantAIEnabled && CanUseButton("productAIUsageCredential.update", session?.permissions)

  const loadProducts = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const [response, countResponse] = await Promise.all([
        listAllProducts(),
        fetchProductCount().catch(() => null),
      ])
      setProducts(response)
      setProductTotal(countResponse?.success && countResponse.data ? countResponse.data.total : response.length)
      setSelectedProductId((current) => {
        if (current && response.some((item) => item.id === current)) {
          return current
        }
        return response[0]?.id ?? null
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : t("productCenter.product.loadFailed"))
    } finally {
      setLoading(false)
    }
  }, [t])

  useEffect(() => {
    void loadProducts()
  }, [loadProducts])

  useEffect(() => {
    const legacyProductId = Number(searchParams.get("product_id"))
    if (Number.isSafeInteger(legacyProductId) && legacyProductId > 0) {
      router.replace(buildProductArchivePath(legacyProductId, parseWorkspaceTab(searchParams.get("tab"))))
      return
    }
    const workspaceParams = new URLSearchParams(window.location.search)
    const requestedProductId = Number(workspaceParams.get("product_id"))
    if (
      Number.isInteger(requestedProductId) &&
      requestedProductId > 0 &&
      products.some((product) => product.id === requestedProductId)
    ) {
      setSelectedProductId(requestedProductId)
    }
  }, [products, router, searchParams])

  const handleSelectProduct = useCallback((productId: number) => {
    setSelectedProductId(productId)
    setSelectedResources((current) => current?.product_id === productId ? current : null)
  }, [])

  const filteredProducts = useMemo(() => {
    const keyword = search.trim().toLowerCase()
    if (!keyword) return products
    return products.filter((product) =>
      `${product.name} ${product.code} ${product.description || ""} ${product.category} ${product.product_line} ${product.owner_name || ""}`
        .toLowerCase()
        .includes(keyword)
    )
  }, [products, search])

  const effectiveSelectedProductId = useMemo(() => {
    if (filteredProducts.length === 0) return null
    if (
      selectedProductId &&
      filteredProducts.some((product) => product.id === selectedProductId)
    ) {
      return selectedProductId
    }
    return filteredProducts[0]?.id ?? null
  }, [filteredProducts, selectedProductId])

  useEffect(() => {
    if (selectedProductId !== effectiveSelectedProductId) {
      setSelectedProductId(effectiveSelectedProductId)
    }
  }, [effectiveSelectedProductId, selectedProductId])

  useEffect(() => {
    if (!effectiveSelectedProductId) {
      setSelectedResources(null)
      return
    }
    setSelectedResources(productResources[effectiveSelectedProductId] ?? null)
  }, [effectiveSelectedProductId, productResources])

  const canEditModalProduct = canEditProductForSession(editingProduct, session, canUpdateProduct)

  const loadProductResources = useCallback(async (productId: number) => {
    const response = await getProductResources(productId)
    if (!response.success || !response.data) {
      throw new Error(response.error?.message || t("productCenter.page.errors.loadResourcesFailed"))
    }
    return response.data
  }, [t])

  useEffect(() => {
    if (!canViewProductResources) {
      setProductResources({})
      setProductResourceLoadingIds([])
      setSelectedResources(null)
    }
  }, [canViewProductResources])

  const handleResourcesChanged = useCallback((resources: ProductResources) => {
    setSelectedResources(resources)
    setProductResources((current) => ({ ...current, [resources.product_id]: resources }))
  }, [])

  const loadProductResourcesOnDemand = useCallback(async (
    productId: number,
    options: { force?: boolean } = {},
  ) => {
    if (!canViewProductResources) return null
    const cachedResources = productResources[productId]
    if (cachedResources && !options.force) {
      return cachedResources
    }
    if (!options.force && productResourceLoadingIds.includes(productId)) {
      return null
    }

    setProductResourceLoadingIds((current) => Array.from(new Set([...current, productId])))

    try {
      const resources = await loadProductResources(productId)
      setProductResources((current) => ({ ...current, [productId]: resources }))
      if (effectiveSelectedProductId === productId) {
        setSelectedResources(resources)
      }
      return resources
    } catch {
      if (effectiveSelectedProductId === productId) {
        setSelectedResources(null)
      }
      return null
    } finally {
      setProductResourceLoadingIds((current) => current.filter((id) => id !== productId))
    }
  }, [canViewProductResources, effectiveSelectedProductId, loadProductResources, productResourceLoadingIds, productResources])

  const handleEditProduct = useCallback((product: ProductListItem) => {
    setEditingProduct(product)
    if (canViewProductResources && canRetryProductResources) {
      void loadProductResourcesOnDemand(product.id)
    }
  }, [canRetryProductResources, canViewProductResources, loadProductResourcesOnDemand])

  const handleCreateProductRequest = useCallback(() => {
    setShowCreateModal(true)
  }, [])

  return (
    <PageShell
      title={t("productCenter.pageTitle")}
      breadcrumb={useRouteBreadcrumbItems()}
      className="enterprise-redesign"
      actions={
        <>
          <RailopsButton onClick={loadProducts} disabled={loading}>
            <RefreshCwIcon className={loading ? "size-4 animate-spin" : "size-4"} />
            {t("productCenter.refresh")}
          </RailopsButton>
          {canCreateProduct ? (
            <RailopsButton onClick={handleCreateProductRequest}>
              <PlusIcon className="size-4" />
              {t("productCenter.product.new")}
            </RailopsButton>
          ) : null}
        </>
      }
    >

      {error && products.length === 0 ? (
        <section className="rhd-railops-product-tab-error">
          <ErrorState
            title={t("productCenter.product.loadFailed")}
            description={error}
            action={{ label: t("productCenter.retry"), onClick: loadProducts }}
          />
        </section>
      ) : products.length === 0 && !loading ? (
        <section className="rhd-railops-product-tab-empty">
          <EmptyState
            title={t("productCenter.page.emptyTitle")}
            action={canCreateProduct ? { label: t("productCenter.product.new"), onClick: handleCreateProductRequest } : undefined}
          />
        </section>
      ) : (
        <section className="ent-product-domain-layout rhd-railops-product-page-table">
          <div className="grid min-w-0 gap-3">
            <ProductTableWorkspace
              activeTab={activeTab}
              products={filteredProducts}
              productTotal={productTotal}
              selectedProductId={effectiveSelectedProductId}
              search={search}
              canEdit={(product) => canEditProductForSession(product, session, canUpdateProduct)}
              onSelectProduct={handleSelectProduct}
              onSearchChange={setSearch}
              onEdit={handleEditProduct}
            />
          </div>
        </section>
      )}

      <CreateProductModal
        aiEnabled={tenantAIEnabled}
        open={canCreateProduct && showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onCreated={(id) => {
          setSelectedProductId(id)
          void loadProducts()
          router.push(buildProductArchivePath(id, "faultStats"))
        }}
      />

      <EditProductModal
        product={canEditModalProduct ? editingProduct : null}
        resources={editingProduct
          ? productResources[editingProduct.id] ?? (selectedResources?.product_id === editingProduct.id ? selectedResources : null)
          : null}
        canManageResources={canRetryProductResources}
        onClose={() => setEditingProduct(null)}
        onResourcesChanged={handleResourcesChanged}
        onSaved={() => {
          setEditingProduct(null)
          void loadProducts()
          if (editingProduct) {
            void loadProductResources(editingProduct.id).then(handleResourcesChanged).catch(() => undefined)
          }
        }}
      />
    </PageShell>
  )
}
