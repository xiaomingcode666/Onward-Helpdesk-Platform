"use client"

import { useCallback, useEffect, useMemo, useRef, useState } from "react"
import { Skeleton } from "antd"
import type { TableColumnsType } from "antd"
import {
  CheckIcon,
  DatabaseIcon,
  DownloadIcon,
  EyeIcon,
  FilePenLineIcon,
  Link2Icon,
  Loader2Icon,
  RotateCcwIcon,
  Trash2Icon,
  UploadIcon,
  XIcon,
} from "lucide-react"
import {
  DataTable,
  IconButton,
  PageShell,
  RailopsButton,
  SelectField,
  StandardModal,
  StatusTag,
  UnderlineTabs,
  type RailopsTabItem,
  type StatusTagTone,
} from "@railops/ui"

import { useAuth } from "@/components/auth-provider"
import { useConfirm } from "@/components/confirm-provider"
import { EnterpriseAIFeatureGuard } from "@/components/enterprise/ai-feature-guard"
import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { ProductTree, buildProductTreeGroups, getProductTreeDisplayName } from "@/components/product/product-tree"
import { EmptyState, ErrorState } from "@/components/shared/error-states"
import { useI18n } from "@/i18n/provider"
import { createKnowledgeBase, fetchKnowledgeBasesAll, type KnowledgeBase } from "@/lib/api/admin"
import { ensureTenantDefaultAIAgent } from "@/lib/api/enterprise-ai"
import {
  createProductKnowledgeBase,
  deleteProductKnowledgeDocument,
  getProductKnowledgeCoverage,
  getProductKnowledgeDocuments,
  getProductKnowledgeLinks,
  getProductProfile,
  fetchProductCount,
  listAllProducts,
  reprocessProductKnowledgeDocument,
  type ProductKnowledgeDocumentFile,
  type ProductManualLink,
  type ProductKnowledgeCoverageDetail,
  type ProductServiceProfile,
  uploadProductKnowledgeDocument,
} from "@/lib/api/enterprise-products"
import type { ProductListItem } from "@/lib/api/types"
import {
  approveKnowledgeCandidate,
  deleteTenantKnowledgeDocument,
  detachDuplicateKnowledgeCandidate,
  enrichKnowledgeCandidate,
  fetchKnowledgeCandidatePage,
  fetchKnowledgeEntry,
  fetchKnowledgeEntries,
  fetchKnowledgeIndexTasks,
  fetchKnowledgeUploadQuota,
  fetchTenantKnowledgeDocuments,
  mergeKnowledgeCandidate,
  publishEntry,
  reprocessTenantKnowledgeDocument,
  rejectKnowledgeCandidate,
  submitForReview,
  uploadTenantKnowledgeDocument,
  type KnowledgeCandidate,
  type EnrichKnowledgeCandidateInput,
  type KnowledgeEntry,
  type KnowledgeEntryListItem,
  type KnowledgeIndexTask,
  type KnowledgeUploadQuota,
} from "@/lib/api/knowledge"

type ActiveTab = "docs" | "entries"
type CandidateQueueView = "review" | "blocked" | "history"

const tenantDefaultKnowledgeBaseRemark = "builtin-tenant-default-service"

type KnowledgeContentRow =
  | { type: "candidate"; candidate: KnowledgeCandidate }
  | { type: "entry"; entry: KnowledgeEntryListItem }

type KnowledgePreviewEntry = Pick<KnowledgeEntryListItem, "id" | "knowledge_base_id" | "title">

type KnowledgePreview =
  | { type: "candidate"; candidate: KnowledgeCandidate }
  | { type: "entry"; entry: KnowledgePreviewEntry }

type ProductKnowledgeState = {
  knowledgeBaseId: number | null
  totalKnowledgeEntryCount: number
  linkedEntries: number
  pendingCandidates: number
  coverageScore: number | null
}

type KnowledgeBaseFormState = {
  name: string
  description: string
  supportLocales: string
  supportRegions: string
}

type TenantKnowledgeBaseFormState = {
  name: string
  description: string
}

type TenantKnowledgeState = {
  knowledgeBase: KnowledgeBase | null
  productCount: number
}

const PER_PAGE = 10
type I18nT = ReturnType<typeof useI18n>
const knowledgePageI18nPrefix = "enterpriseExtract.knowledgePage."
const ke = (t: I18nT, key: string, values?: Record<string, string | number>) => t(`${knowledgePageI18nPrefix}${key}`, values)

function readErrorMessage(response: { error?: { message?: string } | null }, fallback: string) {
  return response.error?.message || fallback
}

function getKnowledgeDocumentDeleteTitle(t: I18nT, item: ProductKnowledgeDocumentFile) {
  return item.deletable ? ke(t, "document.deleteTitleDeletable") : ke(t, "document.deleteTitleReadonly")
}

function getKnowledgeDocumentDeleteDescription(t: I18nT, item: ProductKnowledgeDocumentFile) {
  const title = item.title || item.filename || ke(t, "document.fallbackName", { id: item.id })
  return ke(t, "document.deleteConfirmDescription", { title })
}

function formatDate(value: string | null | undefined) {
  if (!value) return "-"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleDateString("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  })
}

function formatFileSize(bytes: number | null | undefined) {
  if (bytes == null || bytes < 0) return "-"
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`
  return `${(bytes / 1024 / 1024 / 1024).toFixed(1)} GB`
}

function formatQuotaCount(t: I18nT, value: number | null | undefined) {
  if (typeof value !== "number" || value < 0) return ke(t, "quota.unlimited")
  return value.toLocaleString("zh-CN")
}

function KnowledgeUploadQuotaStrip({
  error,
  loading,
  productScoped = false,
  quota,
}: {
  error: string
  loading: boolean
  productScoped?: boolean
  quota: KnowledgeUploadQuota | null
}) {
  const t = useI18n()
  if (loading && !quota) {
    return (
      <div className="mx-3 mt-3 grid gap-2 rounded-md border border-dashed border-border bg-muted/30 p-3 sm:grid-cols-4" aria-busy="true">
        {Array.from({ length: 4 }).map((_, index) => <Skeleton.Node key={index} active className="w-full" style={{ width: "100%", height: 40 }} />)}
      </div>
    )
  }
  if (!quota && !error) return null
  if (error && !quota) {
    return <div className="mx-3 mt-3 kb-error-strip">{error}</div>
  }
  const accountLimit = (quota?.document_limit ?? 0) > 0 ? formatQuotaCount(t, quota?.document_limit) : ke(t, "quota.unlimited")
  const remaining = (quota?.document_remaining ?? -1) >= 0 ? formatQuotaCount(t, quota?.document_remaining) : ke(t, "quota.unlimited")
  const tenantLimit = (quota?.tenant_document_limit ?? 0) > 0 ? formatQuotaCount(t, quota?.tenant_document_limit) : ke(t, "quota.unlimited")
  const productLimit = (quota?.product_document_limit ?? 0) > 0 ? formatQuotaCount(t, quota?.product_document_limit) : ke(t, "quota.unlimited")
  const scopeQuotaText = productScoped
    ? ke(t, "quota.scopeProduct", { used: formatQuotaCount(t, quota?.product_document_used), limit: productLimit })
    : ke(t, "quota.scopeTenant", { used: formatQuotaCount(t, quota?.tenant_document_used), limit: tenantLimit })
  const items = [
    { label: ke(t, "quota.uploadedLabel"), value: ke(t, "quota.countValue", { count: formatQuotaCount(t, quota?.document_used) }) },
    { label: ke(t, "quota.remainingLabel"), value: ke(t, "quota.countValue", { count: remaining }) },
    { label: ke(t, "quota.totalSizeLabel"), value: formatFileSize(quota?.total_file_size) },
    { label: ke(t, "quota.singleFileLimitLabel"), value: quota?.single_file_size_limit ? formatFileSize(quota.single_file_size_limit) : ke(t, "quota.unlimited") },
  ]
  return (
    <div className="mx-3 mt-3 rounded-md border border-[#dbeafe] bg-[#f8fbff] p-3">
      <div className="grid gap-2 sm:grid-cols-4">
        {items.map((item) => (
          <div key={item.label} className="min-w-0">
            <div className="text-rhd-xs font-medium text-muted-foreground">{item.label}</div>
            <div className="mt-1 truncate text-sm font-semibold text-foreground" title={item.value}>{item.value}</div>
          </div>
        ))}
      </div>
      <div className="mt-2 text-rhd-xs text-muted-foreground">
        {ke(t, "quota.accountSummary", { used: formatQuotaCount(t, quota?.document_used), limit: accountLimit, scopeText: scopeQuotaText })}
      </div>
      {error ? <div className="mt-2 text-rhd-xs font-medium text-destructive">{error}</div> : null}
    </div>
  )
}

function getKnowledgeDocumentIndexLabel(t: I18nT, item: ProductKnowledgeDocumentFile) {
  switch (item.index_status) {
    case "indexed":
      return ke(t, "index.indexed")
    case "failed":
      return ke(t, "index.indexFailed")
    case "pending":
      return ke(t, "index.waitingIndex")
    default:
      return item.index_status || ke(t, "index.waitingIndex")
  }
}

function isKnowledgeIndexTaskActive(task: KnowledgeIndexTask) {
  return task.status === "pending" || task.status === "running" || task.status === "waiting_retry"
}

function KnowledgeParsingProgress({
  item,
  task,
}: {
  item: ProductKnowledgeDocumentFile
  task?: KnowledgeIndexTask
}) {
  const t = useI18n()
  const taskStatus = task?.status || ""
  const errorSummary = task?.error_summary || item.index_error
  let label = getKnowledgeDocumentIndexLabel(t, item)
  let detail = ""
  let tone: "complete" | "active" | "retry" | "failed" | "cancelled" = "active"

  if (taskStatus === "running") {
    label = ke(t, "index.parsing")
    detail = ke(t, "index.generatingVectorIndex")
  } else if (taskStatus === "waiting_retry") {
    label = ke(t, "index.waitingRetry")
    detail = task?.max_retries
      ? ke(t, "index.retryCount", { count: `${task.retry_count}/${task.max_retries}` })
      : ke(t, "index.retryCount", { count: `${task?.retry_count || 0}` })
    tone = "retry"
  } else if (taskStatus === "pending") {
    label = ke(t, "index.waitingParsing")
    detail = ke(t, "index.inParsingQueue")
  } else if (taskStatus === "failed" || item.index_status === "failed") {
    label = ke(t, "index.parsingFailed")
    detail = errorSummary ? truncateIndexError(errorSummary) : ke(t, "index.parsingTaskFailed")
    tone = "failed"
  } else if (taskStatus === "cancelled") {
    label = ke(t, "index.cancelled")
    detail = ke(t, "index.parsingTaskCancelled")
    tone = "cancelled"
  } else if (taskStatus === "succeeded" || item.index_status === "indexed") {
    label = ke(t, "index.parsingCompleted")
    detail = item.indexed_at ? ke(t, "index.completedAt", { time: item.indexed_at }) : ke(t, "index.vectorIndexReady")
    tone = "complete"
  }

  const fillClass = {
    complete: "w-full bg-primary",
    active: "w-2/5 animate-pulse bg-primary",
    retry: "w-2/5 animate-pulse bg-amber-500",
    failed: "w-full bg-destructive",
    cancelled: "w-full bg-zinc-400",
  }[tone]

  return (
    <div className="w-32 min-w-32 space-y-1.5" title={errorSummary || detail}>
      <div className="flex items-center justify-between gap-2 text-xs">
        <span className="flex min-w-0 items-center gap-1.5 font-medium">
          {tone === "complete" ? <CheckIcon className="size-3.5 text-primary" /> : null}
          {tone === "active" ? <Loader2Icon className="size-3.5 animate-spin text-primary" /> : null}
          {tone === "retry" ? <RotateCcwIcon className="size-3.5 text-amber-600" /> : null}
          {tone === "failed" || tone === "cancelled" ? <XIcon className="size-3.5 text-destructive" /> : null}
          <span className="truncate">{label}</span>
        </span>
        {tone === "complete" ? <span className="text-muted-foreground">100%</span> : null}
      </div>
      <div
        className="h-1.5 overflow-hidden rounded-full bg-muted"
        role="progressbar"
        aria-label={ke(t, "index.progressAriaLabel", { name: item.title || item.filename || ke(t, "document.fallbackName", { id: item.id }) })}
        aria-valuenow={tone === "complete" ? 100 : undefined}
        aria-valuemin={tone === "complete" ? 0 : undefined}
        aria-valuemax={tone === "complete" ? 100 : undefined}
        aria-valuetext={label}
      >
        <span className={`block h-full rounded-full transition-all ${fillClass}`} />
      </div>
      <div className="truncate text-rhd-xs text-muted-foreground">{detail || ke(t, "index.waitingForStatusUpdate")}</div>
    </div>
  )
}

function getKnowledgeDocumentSourceLabel(t: I18nT, sourceType: string) {
  switch (sourceType) {
    case "product_manual":
      return ke(t, "source.productManual")
    case "knowledge_entry":
      return ke(t, "source.knowledgeEntry")
    default:
      return ke(t, "source.directUpload")
  }
}

function getKnowledgeDocumentSourceTone(sourceType: string): StatusTagTone {
  if (sourceType === "product_manual") return "blue"
  if (sourceType === "knowledge_entry") return "success"
  return "neutral"
}

function getKnowledgeEntryStatusLabel(t: I18nT, status: string) {
  switch (status) {
    case "published":
      return ke(t, "entryStatus.published")
    case "review":
      return ke(t, "entryStatus.pendingReview")
    case "deprecated":
      return ke(t, "entryStatus.deprecated")
    default:
      return ke(t, "entryStatus.draft")
  }
}

function getKnowledgeEntryStatusTone(status: string): StatusTagTone {
  if (status === "published") return "success"
  if (status === "review") return "warning"
  if (status === "deprecated") return "disabled"
  return "neutral"
}

function truncateIndexError(value: string) {
  const message = value.trim()
  if (!message) return ""
  if (message.length <= 72) return message
  return `${message.slice(0, 72)}...`
}

function getKnowledgeCandidateStatusLabel(t: I18nT, status: string) {
  switch (status) {
    case "pending": return ke(t, "candidateStatus.pending")
    case "needs_enrichment": return ke(t, "candidateStatus.needsEnrichment")
    case "low_quality": return ke(t, "candidateStatus.lowQuality")
    case "low_value": return ke(t, "candidateStatus.lowValue")
    case "duplicate": return ke(t, "candidateStatus.duplicate")
    case "approved": return ke(t, "candidateStatus.approved")
    case "merged": return ke(t, "candidateStatus.merged")
    case "rejected": return ke(t, "candidateStatus.rejected")
    default: return status || ke(t, "candidateStatus.unscored")
  }
}

function getKnowledgeCandidateStatusTone(status: string, requiresReassessment: boolean): StatusTagTone {
  if (requiresReassessment) return "error"
  if (status === "approved" || status === "merged") return "success"
  if (status === "pending" || status === "needs_enrichment") return "warning"
  if (status === "rejected" || status === "low_quality" || status === "low_value") return "error"
  return "neutral"
}

function getKnowledgeCandidateFlagLabel(t: I18nT, flag: string) {
  const labels: Record<string, string> = {
    missing_root_cause: ke(t, "qualityFlags.missingRootCause"),
    missing_solution: ke(t, "qualityFlags.missingSolution"),
    missing_resolution_summary: ke(t, "qualityFlags.missingResolutionSummary"),
    missing_verification: ke(t, "qualityFlags.missingVerification"),
    partial_verification: ke(t, "qualityFlags.partialVerification"),
    verification_failed: ke(t, "qualityFlags.verificationFailed"),
    negative_customer_feedback: ke(t, "qualityFlags.negativeCustomerFeedback"),
    missing_product_context: ke(t, "qualityFlags.missingProductContext"),
    missing_fault_code: ke(t, "qualityFlags.missingFaultCode"),
    generic_title: ke(t, "qualityFlags.genericTitle"),
    title_too_short: ke(t, "qualityFlags.titleTooShort"),
    content_too_short: ke(t, "qualityFlags.contentTooShort"),
    missing_structure: ke(t, "qualityFlags.missingStructure"),
    missing_actionable_steps: ke(t, "qualityFlags.missingActionableSteps"),
    missing_context_tags: ke(t, "qualityFlags.missingContextTags"),
    missing_product_scope: ke(t, "qualityFlags.missingProductScope"),
    duplicate_published_entry: ke(t, "qualityFlags.duplicatePublishedEntry"),
    candidate_source_content_mismatch: ke(t, "qualityFlags.candidateSourceContentMismatch"),
  }
  return labels[flag] || flag
}

function buildProductKnowledgeState(
  profile: ProductServiceProfile | null,
  links: ProductManualLink[] = [],
  coverage: ProductKnowledgeCoverageDetail | null = null
): ProductKnowledgeState {
  const linkWithKnowledgeBase = links.find((link) => Number(link.knowledge_base_id) > 0)
  const knowledgeBaseId =
    profile?.serviceProfile?.knowledge_base_id ??
    coverage?.knowledge_base_id ??
    linkWithKnowledgeBase?.knowledge_base_id ??
    null
  return {
    knowledgeBaseId,
    totalKnowledgeEntryCount: profile?.totalKnowledgeEntryCount ?? links.length,
    linkedEntries: coverage?.linked_entries ?? links.length,
    pendingCandidates: coverage?.pending_candidates ?? 0,
    coverageScore: coverage?.coverage_score ?? null,
  }
}

function hasKnowledgeMounted(state: ProductKnowledgeState | undefined) {
  if (!state) return false
  return Boolean(
    Number(state.knowledgeBaseId) > 0 ||
    state.linkedEntries > 0 ||
    state.totalKnowledgeEntryCount > 0
  )
}

function datasetDisplay(t: I18nT, state: ProductKnowledgeState | undefined) {
  if (!state) return ke(t, "dataset.loading")
  if (Number(state.knowledgeBaseId) > 0) return `KB-${state.knowledgeBaseId}`
  return ke(t, "dataset.notCreated")
}

function splitMultiValueInput(value: string) {
  return Array.from(new Set(
    value
      .split(/[\n,，/]+/)
      .map((item) => item.trim())
      .filter(Boolean)
  ))
}

function buildKnowledgeBaseFormState(t: I18nT, product: ProductListItem | null, profile: ProductServiceProfile | null): KnowledgeBaseFormState {
  const locales = profile?.serviceProfile?.support_locales?.filter(Boolean) || []
  const regions = profile?.serviceProfile?.support_regions?.filter(Boolean) || []
  return {
    name: profile?.serviceProfile?.knowledge_base_name?.trim() || (product ? ke(t, "dataset.productKnowledgeBaseName", { name: product.name }) : ""),
    description: profile?.serviceProfile?.knowledge_base_description?.trim() || "",
    supportLocales: locales.length ? locales.join(", ") : (product?.default_locale || ""),
    supportRegions: regions.length ? regions.join(", ") : "",
  }
}

function buildTenantKnowledgeBaseFormState(
  t: I18nT,
  knowledgeBase: KnowledgeBase | null = null
): TenantKnowledgeBaseFormState {
  return {
    name: knowledgeBase?.name?.trim() || ke(t, "dataset.tenantDefaultName"),
    description: knowledgeBase?.description?.trim() || "",
  }
}

function buildTenantKnowledgeState(
  knowledgeBases: KnowledgeBase[],
  productCount: number
): TenantKnowledgeState {
  const sharedKnowledgeBases = knowledgeBases.filter((knowledgeBase) => knowledgeBase.accessScope === "tenant")
  const systemDefault = sharedKnowledgeBases.find((knowledgeBase) => knowledgeBase.remark === tenantDefaultKnowledgeBaseRemark) ?? null
  const exactMatch = sharedKnowledgeBases.find((knowledgeBase) => knowledgeBase.name.trim() === "企业通用知识库") ?? null
  return {
    knowledgeBase: systemDefault ?? exactMatch ?? sharedKnowledgeBases[0] ?? null,
    productCount,
  }
}

function buildKnowledgeBasePayload(name: string, description: string) {
  return {
    name,
    description,
    knowledgeType: "document",
    accessScope: "tenant" as const,
    defaultTopK: 10,
    defaultScoreThreshold: 0.2,
    defaultRerankLimit: 5,
    chunkProvider: "structured",
    chunkTargetTokens: 300,
    chunkMaxTokens: 400,
    chunkOverlapTokens: 40,
    answerMode: 1,
    remark: tenantDefaultKnowledgeBaseRemark,
  }
}

function pageSlice<T>(rows: T[], page: number) {
  return rows.slice((page - 1) * PER_PAGE, page * PER_PAGE)
}

function totalPages(rows: unknown[]) {
  return Math.max(1, Math.ceil(rows.length / PER_PAGE))
}

function TenantKnowledgePanel({
  state,
  hasProductConcept,
  form,
  submitting,
  error,
  onChange,
  onReset,
  onSubmit,
}: {
  state: TenantKnowledgeState
  hasProductConcept: boolean
  form: TenantKnowledgeBaseFormState
  submitting: boolean
  error: string
  onChange: (patch: Partial<TenantKnowledgeBaseFormState>) => void
  onReset: () => void
  onSubmit: () => void
}) {
  const t = useI18n()
  const knowledgeBase = state.knowledgeBase
  const productCount = state.productCount

  return (
    <div className="ent-panel kb-info-card kb-create-panel">
      <div className="ent-panel-head">
        <div>
          <strong>{t("enterpriseKnowledge.tenantPanel.title")}</strong>
          <span>{t(hasProductConcept ? "enterpriseKnowledge.tenantPanel.subtitle" : "enterpriseKnowledge.tenantPanel.knowledgeOnlySubtitle")}</span>
        </div>
        <StatusTag tone={knowledgeBase ? "blue" : "neutral"}>
          {knowledgeBase ? t("enterpriseKnowledge.tenantPanel.ready") : t("enterpriseKnowledge.tenantPanel.notReady")}
        </StatusTag>
      </div>
      <div className="kb-info-body">
        <div className={`kb-dataset-callout${knowledgeBase ? "" : " muted"}`}>
          <DatabaseIcon className="size-5" />
          <div>
            <span>{t("enterpriseKnowledge.tenantPanel.datasetTitle")}</span>
            <strong>{knowledgeBase?.name || t("enterpriseKnowledge.tenantPanel.datasetEmptyTitle")}</strong>
          </div>
        </div>

        {knowledgeBase ? (
          <div className="kb-info-stats">
            <div className="kb-info-stat"><span>{t("enterpriseKnowledge.tenantPanel.platformLabel")}</span><strong>KB-{knowledgeBase.id}</strong></div>
            {hasProductConcept ? <div className="kb-info-stat"><span>{t("enterpriseKnowledge.tenantPanel.productCountLabel")}</span><strong>{productCount}</strong></div> : null}
            <div className="kb-info-stat"><span>{t("enterpriseKnowledge.tenantPanel.documentCountLabel")}</span><strong>{knowledgeBase.documentCount || 0}</strong></div>
            <div className="kb-info-stat"><span>{t("enterpriseKnowledge.tenantPanel.faqCountLabel")}</span><strong>{knowledgeBase.faqCount || 0}</strong></div>
            <div className="kb-info-stat"><span>{t("enterpriseKnowledge.tenantPanel.knowledgeTypeLabel")}</span><strong>{knowledgeBase.knowledgeTypeName || knowledgeBase.knowledgeType || "-"}</strong></div>
          </div>
        ) : null}

        {error ? <div className="kb-error-strip">{error}</div> : null}

        {!knowledgeBase ? (
          <form
            className="kb-create-form"
            onSubmit={(event) => {
              event.preventDefault()
              onSubmit()
            }}
          >
            <div className="kb-form-row">
              <label className="kb-form-field">
                <span>{t("enterpriseKnowledge.tenantPanel.formNameLabel")}</span>
                <input
                  value={form.name}
                  placeholder={t("enterpriseKnowledge.tenantPanel.formNamePlaceholder")}
                  disabled={submitting}
                  onChange={(event) => onChange({ name: event.target.value })}
                />
              </label>
            </div>

            <div className="kb-form-row">
              <label className="kb-form-field">
                <span>{t("enterpriseKnowledge.tenantPanel.formDescriptionLabel")}</span>
                <textarea
                  value={form.description}
                  placeholder={t("enterpriseKnowledge.tenantPanel.formDescriptionPlaceholder")}
                  disabled={submitting}
                  onChange={(event) => onChange({ description: event.target.value })}
                />
              </label>
            </div>

            <div className="panel-actions">
              <RailopsButton htmlType="submit" variant="primary" disabled={submitting || !form.name.trim()}>
                {submitting ? t("enterpriseKnowledge.tenantPanel.creating") : t("enterpriseKnowledge.tenantPanel.createButton")}
              </RailopsButton>
              <RailopsButton disabled={submitting} onClick={onReset}>
                {t("enterpriseKnowledge.tenantPanel.resetButton")}
              </RailopsButton>
            </div>
          </form>
        ) : null}
      </div>
    </div>
  )
}

function KnowledgeCreatePanel({
  product,
  form,
  submitting,
  onChange,
  onReset,
  onSubmit,
}: {
  product: ProductListItem | null
  form: KnowledgeBaseFormState
  submitting: boolean
  onChange: (patch: Partial<KnowledgeBaseFormState>) => void
  onReset: () => void
  onSubmit: () => void
}) {
  const t = useI18n()
  if (!product) {
    return (
      <div className="ent-panel">
        <EmptyState title={t("enterpriseKnowledge.productPanel.emptyTitle")} />
      </div>
    )
  }

  return (
    <div className="ent-panel kb-info-card kb-create-panel">
      <div className="ent-panel-head">
        <div>
          <strong>{t("enterpriseKnowledge.productPanel.title", { name: product.name })}</strong>
          <span>{product.code || `PROD-${product.id}`} · {product.category || t("enterpriseKnowledge.productPanel.categoryFallback")}</span>
        </div>
        <StatusTag tone="neutral">{t("enterpriseKnowledge.productPanel.badge")}</StatusTag>
      </div>
      <div className="kb-info-body">
        <div className="kb-dataset-callout muted">
          <DatabaseIcon className="size-5" />
          <div>
            <span>{t("enterpriseKnowledge.productPanel.calloutTitle")}</span>
          </div>
        </div>

        <form
          className="kb-create-form"
          onSubmit={(event) => {
            event.preventDefault()
            onSubmit()
          }}
        >
          <div className="kb-form-row">
            <label className="kb-form-field">
              <span>{t("enterpriseKnowledge.productPanel.formNameLabel")}</span>
              <input
                value={form.name}
                placeholder={t("enterpriseKnowledge.productPanel.formNamePlaceholder", { name: product.name })}
                disabled={submitting}
                onChange={(event) => onChange({ name: event.target.value })}
              />
            </label>
          </div>

          <div className="kb-form-row">
            <label className="kb-form-field">
              <span>{t("enterpriseKnowledge.productPanel.formDescriptionLabel")}</span>
              <textarea
                value={form.description}
                placeholder={t("enterpriseKnowledge.productPanel.formDescriptionPlaceholder")}
                disabled={submitting}
                onChange={(event) => onChange({ description: event.target.value })}
              />
            </label>
          </div>

          <div className="kb-form-row two-col">
            <label className="kb-form-field">
              <span>{t("enterpriseKnowledge.productPanel.formLocalesLabel")}</span>
              <input
                value={form.supportLocales}
                placeholder={t("enterpriseKnowledge.productPanel.formLocalesPlaceholder")}
                disabled={submitting}
                onChange={(event) => onChange({ supportLocales: event.target.value })}
              />
            </label>
            <label className="kb-form-field">
              <span>{t("enterpriseKnowledge.productPanel.formRegionsLabel")}</span>
              <input
                value={form.supportRegions}
                placeholder={t("enterpriseKnowledge.productPanel.formRegionsPlaceholder")}
                disabled={submitting}
                onChange={(event) => onChange({ supportRegions: event.target.value })}
              />
            </label>
          </div>

          <div className="panel-actions">
            <RailopsButton htmlType="submit" variant="primary" disabled={submitting || !form.name.trim()}>
              {submitting ? t("enterpriseKnowledge.productPanel.creating") : t("enterpriseKnowledge.productPanel.createButton")}
            </RailopsButton>
            <RailopsButton disabled={submitting} onClick={onReset}>
              {t("enterpriseKnowledge.productPanel.resetButton")}
            </RailopsButton>
          </div>
        </form>
      </div>
    </div>
  )
}

function Pagination({
  page,
  rows,
  total,
  onPageChange,
}: {
  page: number
  rows: unknown[]
  total?: number
  onPageChange: (page: number) => void
}) {
  const t = useI18n()
  const pages = typeof total === "number"
    ? Math.max(1, Math.ceil(total / PER_PAGE))
    : totalPages(rows)
  return (
    <div className="kb-pagination">
      <RailopsButton size="small" disabled={page <= 1} onClick={() => onPageChange(page - 1)}>{t("enterpriseKnowledge.pagination.previous")}</RailopsButton>
      <span>{page} / {pages}</span>
      <RailopsButton size="small" disabled={page >= pages} onClick={() => onPageChange(page + 1)}>{t("enterpriseKnowledge.pagination.next")}</RailopsButton>
    </div>
  )
}

function KnowledgeEntryTable({
  rows,
  actionCandidateId,
  actionEntryId,
  onViewCandidate,
  onViewEntry,
  onApprove,
  onSubmitEntry,
  onPublishEntry,
  onReject,
}: {
  rows: KnowledgeContentRow[]
  actionCandidateId: number | null
  actionEntryId: number | null
  onViewCandidate: (candidate: KnowledgeCandidate) => void
  onViewEntry: (entry: KnowledgeEntryListItem) => void
  onApprove: (candidate: KnowledgeCandidate, publish: boolean) => void
  onSubmitEntry: (entry: KnowledgeEntryListItem) => void
  onPublishEntry: (entry: KnowledgeEntryListItem) => void
  onReject: (candidate: KnowledgeCandidate) => void
}) {
  const t = useI18n()
  const columns: TableColumnsType<KnowledgeContentRow> = [
    {
      title: t("enterpriseKnowledge.entryTable.title"),
      key: "title",
      width: 260,
      fixed: "left",
      render: (_, row) => {
        if (row.type === "candidate") {
          const candidate = row.candidate
          return (
            <div className="rhd-railops-table-cell">
              <strong>{candidate.title || t("enterpriseKnowledge.entryTable.candidateFallback", { id: candidate.id })}</strong>
              <span>{candidate.source_id ? t("enterpriseKnowledge.entryTable.ticketSource", { id: candidate.source_id }) : t("enterpriseKnowledge.entryTable.ticketFallback", { id: candidate.ticket_id })}</span>
            </div>
          )
        }
        return (
          <div className="rhd-railops-table-cell">
            <strong>{row.entry.title}</strong>
            <span>{row.entry.type === "faq" ? t("enterpriseKnowledge.entryTable.faqType") : t("enterpriseKnowledge.entryTable.entryType")}</span>
          </div>
        )
      },
    },
    {
      title: t("enterpriseKnowledge.entryTable.source"),
      key: "source",
      width: 150,
      render: (_, row) => row.type === "candidate"
        ? <StatusTag tone="blue">{ke(t, "source.ticketDerived")}</StatusTag>
        : <StatusTag tone="neutral">{row.entry.type === "faq" ? t("enterpriseKnowledge.entryTable.faqType") : t("enterpriseKnowledge.entryTable.entryType")}</StatusTag>,
    },
    {
      title: t("enterpriseKnowledge.entryTable.score"),
      key: "score",
      width: 160,
      render: (_, row) => {
        const score = row.type === "candidate" ? row.candidate.candidate_score : row.entry.entry_score
        const quality = row.type === "candidate" ? row.candidate.quality_score : row.entry.quality_score
        const value = row.type === "candidate" ? row.candidate.value_score : row.entry.value_score
        return (
          <div className="rhd-railops-table-cell">
            <strong>{score}</strong>
            <span>{t("enterpriseKnowledge.entryTable.qualityValue", { quality, value })}</span>
          </div>
        )
      },
    },
    {
      title: t("enterpriseKnowledge.entryTable.status"),
      key: "status",
      width: 130,
      render: (_, row) => {
        if (row.type === "candidate") {
          const candidate = row.candidate
          return (
            <StatusTag tone={getKnowledgeCandidateStatusTone(candidate.review_status, candidate.requires_reassessment)}>
              {candidate.requires_reassessment ? ke(t, "candidateStatus.requiresReassessment") : getKnowledgeCandidateStatusLabel(t, candidate.review_status)}
            </StatusTag>
          )
        }
        return (
          <StatusTag tone={getKnowledgeEntryStatusTone(row.entry.status)}>
            {getKnowledgeEntryStatusLabel(t, row.entry.status)}
          </StatusTag>
        )
      },
    },
    {
      title: t("enterpriseKnowledge.entryTable.language"),
      key: "language",
      width: 140,
      render: (_, row) => row.type === "entry"
        ? row.entry.languages?.join(" / ") || t("enterpriseKnowledge.entryTable.defaultLanguage")
        : "-",
    },
    {
      title: t("enterpriseKnowledge.entryTable.updatedAt"),
      key: "updatedAt",
      width: 130,
      render: (_, row) => row.type === "candidate" ? formatDate(row.candidate.created_at) : formatDate(row.entry.updated_at),
    },
    {
      title: t("enterpriseKnowledge.entryTable.actions"),
      key: "actions",
      width: 132,
      fixed: "right",
      align: "right",
      render: (_, row) => {
        if (row.type === "candidate") {
          const candidate = row.candidate
          const finalized = ["approved", "rejected", "merged"].includes(candidate.review_status)
          return (
            <div className="rhd-railops-row-actions">
              <IconButton
                className="rhd-railops-knowledge-action-button"
                icon={<EyeIcon className="size-4" />}
                tooltip={t("enterpriseKnowledge.entryTable.viewCandidate")}
                aria-label={t("enterpriseKnowledge.entryTable.viewCandidate")}
                disabled={actionCandidateId === candidate.id}
                onClick={() => onViewCandidate(candidate)}
              />
              {!finalized ? (
                <IconButton
                  className="rhd-railops-knowledge-action-button"
                  icon={actionCandidateId === candidate.id ? <Loader2Icon className="size-4 animate-spin" /> : <CheckIcon className="size-4" />}
                  tooltip={candidate.review_eligible ? t("enterpriseKnowledge.entryTable.approveAndPublish") : (candidate.review_remark || t("enterpriseKnowledge.entryTable.reviewThreshold"))}
                  aria-label={t("enterpriseKnowledge.entryTable.approveAndPublish")}
                  disabled={actionCandidateId === candidate.id || !candidate.review_eligible}
                  onClick={() => onApprove(candidate, true)}
                />
              ) : null}
              {!finalized ? (
                <IconButton
                  className="rhd-railops-knowledge-action-button"
                  icon={<XIcon className="size-4" />}
                  tooltip={t("enterpriseKnowledge.entryTable.reject")}
                  aria-label={t("enterpriseKnowledge.entryTable.reject")}
                  danger
                  disabled={actionCandidateId === candidate.id}
                  onClick={() => onReject(candidate)}
                />
              ) : null}
            </div>
          )
        }

        const entry = row.entry
        return (
          <div className="rhd-railops-row-actions">
            <IconButton
              className="rhd-railops-knowledge-action-button"
              icon={<EyeIcon className="size-4" />}
              tooltip={t("enterpriseKnowledge.entryTable.viewEntry")}
              aria-label={t("enterpriseKnowledge.entryTable.viewEntry")}
              disabled={actionEntryId === entry.id}
              onClick={() => onViewEntry(entry)}
            />
            <IconButton
              className="rhd-railops-knowledge-action-button"
              icon={actionEntryId === entry.id ? <Loader2Icon className="size-4 animate-spin" /> : entry.status === "draft" ? <FilePenLineIcon className="size-4" /> : <CheckIcon className="size-4" />}
              tooltip={entry.status === "draft" ? (entry.review_eligible ? t("enterpriseKnowledge.entryTable.submitForReview") : t("enterpriseKnowledge.entryTable.reviewThreshold")) : t("enterpriseKnowledge.entryTable.publish")}
              aria-label={entry.status === "draft" ? t("enterpriseKnowledge.entryTable.submitForReview") : t("enterpriseKnowledge.entryTable.publish")}
              disabled={actionEntryId === entry.id || (entry.status === "draft" && !entry.review_eligible)}
              onClick={() => entry.status === "draft" ? onSubmitEntry(entry) : onPublishEntry(entry)}
            />
          </div>
        )
      },
    },
  ]

  return (
    <div className="kb-doc-table-wrap rhd-railops-knowledge-table-wrap">
      <DataTable<KnowledgeContentRow>
        className="rhd-railops-knowledge-table"
        columns={columns}
        dataSource={rows}
        rowKey={(row) => row.type === "candidate" ? `candidate-${row.candidate.id}` : `entry-${row.entry.id}`}
        size="small"
        scroll={{ x: 1120 }}
        emptyDescription={t("enterpriseKnowledge.entryTable.empty")}
      />
    </div>
  )
}

function KnowledgePreviewDialog({
  preview,
  entryDetail,
  loading,
  error,
  actionCandidateId,
  availableEntries,
  onOpenChange,
  onApprove,
  onEnrich,
  onMerge,
  onDetachDuplicate,
}: {
  preview: KnowledgePreview | null
  entryDetail: KnowledgeEntry | null
  loading: boolean
  error: string
  actionCandidateId: number | null
  availableEntries: KnowledgeEntryListItem[]
  onOpenChange: (open: boolean) => void
  onApprove: (candidate: KnowledgeCandidate, publish: boolean) => void
  onEnrich: (candidate: KnowledgeCandidate, input: EnrichKnowledgeCandidateInput) => void
  onMerge: (candidate: KnowledgeCandidate, entryId: number) => void
  onDetachDuplicate: (candidate: KnowledgeCandidate) => void
}) {
  const t = useI18n()
  const candidate = preview?.type === "candidate" ? preview.candidate : null
  const entry = preview?.type === "entry" ? preview.entry : null
  const title = candidate?.title || entryDetail?.title || entry?.title || t("enterpriseKnowledge.preview.title")
  const sourceLabel = candidate
    ? (candidate.source_id ? t("enterpriseKnowledge.preview.ticketSource", { id: candidate.source_id }) : t("enterpriseKnowledge.preview.ticketFallback", { id: candidate.ticket_id }))
    : t("enterpriseKnowledge.preview.knowledgeBase", { id: entryDetail?.knowledge_base_id || entry?.knowledge_base_id || "-" })
  const actionInProgress = candidate ? actionCandidateId === candidate.id : false
  const [candidateDraft, setCandidateDraft] = useState<EnrichKnowledgeCandidateInput>(candidate ? {
    title: candidate.title,
    suggestion: candidate.suggestion,
    root_cause_summary: candidate.root_cause_summary,
    solution_summary: candidate.solution_summary,
  } : {})
  const [mergeEntryId, setMergeEntryId] = useState("")
  const canEnrich = Boolean(candidate && !candidate.review_eligible && !["approved", "rejected", "merged"].includes(candidate.review_status))

  return (
    <StandardModal
      open={preview !== null}
      onCancel={() => onOpenChange(false)}
      rootClassName="rhd-railops-knowledge-preview-dialog"
      width={1024}
      title={
        <div className="min-w-0">
          <span>{title}</span>
          <div className="mt-2 flex flex-wrap gap-1.5">
            <StatusTag tone="neutral">{sourceLabel}</StatusTag>
          </div>
        </div>
      }
      footer={candidate ? (
        <div className="rhd-railops-knowledge-preview-footer">
          {candidate.review_status === "duplicate" ? (
              <RailopsButton
                disabled={actionInProgress}
                onClick={() => onDetachDuplicate(candidate)}
              >
                <RotateCcwIcon className="size-4" />
                {t("enterpriseKnowledge.preview.detachDuplicate")}
              </RailopsButton>
            ) : canEnrich ? (
              <RailopsButton
                variant="primary"
                disabled={actionInProgress}
                onClick={() => onEnrich(candidate, candidateDraft)}
              >
                {actionInProgress ? <Loader2Icon className="size-4 animate-spin" /> : <RotateCcwIcon className="size-4" />}
                {t("enterpriseKnowledge.preview.saveAndRegrade")}
              </RailopsButton>
            ) : candidate.review_eligible ? (
              <>
                <SelectField
                  style={{ marginBottom: 0 }}
                  selectProps={{
                    "aria-label": t("enterpriseKnowledge.preview.mergeAriaLabel"),
                    placeholder: t("enterpriseKnowledge.preview.mergePlaceholder"),
                    value: mergeEntryId || undefined,
                    onChange: (value) => setMergeEntryId((value as string) ?? ""),
                    options: availableEntries
                      .filter((item) => item.status === "published")
                      .map((item) => ({ value: String(item.id), label: item.title })),
                    style: { width: 200 },
                  }}
                />
                <RailopsButton
                  disabled={actionInProgress || !mergeEntryId}
                  onClick={() => onMerge(candidate, Number(mergeEntryId))}
                >
                  <Link2Icon className="size-4" />
                  {t("enterpriseKnowledge.preview.merge")}
                </RailopsButton>
                <RailopsButton
                  disabled={actionInProgress}
                  onClick={() => onApprove(candidate, false)}
                >
                  <FilePenLineIcon className="size-4" />
                  {t("enterpriseKnowledge.preview.saveDraft")}
                </RailopsButton>
                <RailopsButton
                  variant="primary"
                  disabled={actionInProgress}
                  onClick={() => onApprove(candidate, true)}
                >
                  {actionInProgress ? <Loader2Icon className="size-4 animate-spin" /> : <CheckIcon className="size-4" />}
                  {t("enterpriseKnowledge.preview.approveAndPublish")}
                </RailopsButton>
              </>
            ) : null}
        </div>
      ) : null}
    >
        {candidate ? (
          <div className="rhd-railops-knowledge-preview-body divide-y rounded-md border">
            <div className="grid gap-3 p-4 sm:grid-cols-3">
              <div>
                <span className="block text-xs text-muted-foreground">{t("enterpriseKnowledge.preview.candidateScore")}</span>
                <strong className="text-lg">{candidate.candidate_score}</strong>
              </div>
              <div>
                <span className="block text-xs text-muted-foreground">{t("enterpriseKnowledge.preview.qualityScore")}</span>
                <strong className="text-lg">{candidate.quality_score}</strong>
              </div>
              <div>
                <span className="block text-xs text-muted-foreground">{t("enterpriseKnowledge.preview.valueScore")}</span>
                <strong className="text-lg">{candidate.value_score}</strong>
              </div>
            </div>
            <div className="grid gap-1 p-4 sm:grid-cols-[7rem_1fr] sm:gap-4">
              <span className="text-muted-foreground">{t("enterpriseKnowledge.preview.poolStatus")}</span>
              <div className="flex flex-wrap items-center gap-2">
                <StatusTag tone="neutral">{getKnowledgeCandidateStatusLabel(t, candidate.review_status)}</StatusTag>
                <span className="text-sm text-muted-foreground">
                  {t("enterpriseKnowledge.preview.recurrence", { count: candidate.recurrence_count, devices: candidate.affected_device_count })}
                </span>
              </div>
            </div>
            {candidate.quality_flags.length > 0 || candidate.review_remark ? (
              <div className="grid gap-1 p-4 sm:grid-cols-[7rem_1fr] sm:gap-4">
                <span className="text-muted-foreground">{t("enterpriseKnowledge.preview.reviewBasis")}</span>
                <p className="whitespace-pre-wrap leading-6">
                  {candidate.review_remark || candidate.quality_flags.map((flag) => getKnowledgeCandidateFlagLabel(t, flag)).join("、")}
                </p>
              </div>
            ) : null}
            {canEnrich ? (
              <div className="grid gap-1 p-4 sm:grid-cols-[7rem_1fr] sm:gap-4">
                <label className="text-muted-foreground" htmlFor={`candidate-title-${candidate.id}`}>{t("enterpriseKnowledge.preview.candidateTitle")}</label>
                <input
                  id={`candidate-title-${candidate.id}`}
                  className="w-full rounded-md border bg-background px-3 py-2 text-sm"
                  value={candidateDraft.title || ""}
                  onChange={(event) => setCandidateDraft((current) => ({ ...current, title: event.target.value }))}
                />
              </div>
            ) : null}
            <div className="grid gap-1 p-4 sm:grid-cols-[7rem_1fr] sm:gap-4">
              <span className="text-muted-foreground">{t("enterpriseKnowledge.preview.customerSuggestion")}</span>
              {canEnrich ? (
                <textarea
                  className="min-h-20 w-full rounded-md border bg-background px-3 py-2 text-sm leading-6"
                  value={candidateDraft.suggestion || ""}
                  onChange={(event) => setCandidateDraft((current) => ({ ...current, suggestion: event.target.value }))}
                />
              ) : <p className="whitespace-pre-wrap leading-6">{candidate.suggestion || "-"}</p>}
            </div>
            <div className="grid gap-1 p-4 sm:grid-cols-[7rem_1fr] sm:gap-4">
              <span className="text-muted-foreground">{t("enterpriseKnowledge.preview.rootCause")}</span>
              {canEnrich ? (
                <textarea
                  className="min-h-20 w-full rounded-md border bg-background px-3 py-2 text-sm leading-6"
                  value={candidateDraft.root_cause_summary || ""}
                  onChange={(event) => setCandidateDraft((current) => ({ ...current, root_cause_summary: event.target.value }))}
                />
              ) : <p className="whitespace-pre-wrap leading-6">{candidate.root_cause_summary || "-"}</p>}
            </div>
            <div className="grid gap-1 p-4 sm:grid-cols-[7rem_1fr] sm:gap-4">
              <span className="text-muted-foreground">{t("enterpriseKnowledge.preview.solution")}</span>
              {canEnrich ? (
                <textarea
                  className="min-h-24 w-full rounded-md border bg-background px-3 py-2 text-sm leading-6"
                  value={candidateDraft.solution_summary || ""}
                  onChange={(event) => setCandidateDraft((current) => ({ ...current, solution_summary: event.target.value }))}
                />
              ) : <p className="whitespace-pre-wrap leading-6">{candidate.solution_summary || "-"}</p>}
            </div>
            <div className="grid gap-1 p-4 sm:grid-cols-[7rem_1fr] sm:gap-4">
              <span className="text-muted-foreground">{t("enterpriseKnowledge.preview.createdAt")}</span>
              <p>{candidate.created_at || "-"}</p>
            </div>
          </div>
        ) : loading ? (
          <div className="space-y-3 py-2">
            <Skeleton.Node active style={{ width: "40%", height: 20 }} />
            <Skeleton.Node active style={{ width: "100%", height: 96 }} />
            <Skeleton.Node active style={{ width: "60%", height: 20 }} />
          </div>
        ) : error ? (
          <div className="kb-error-strip">{error}</div>
        ) : entryDetail ? (
          <div className="rhd-railops-knowledge-preview-body space-y-4">
            <div className="grid grid-cols-2 gap-x-6 gap-y-3 border-b pb-4 text-sm sm:grid-cols-3">
              <div><span className="block text-muted-foreground">{ke(t, "preview.status")}</span><strong>{getKnowledgeEntryStatusLabel(t, entryDetail.status)}</strong></div>
              <div><span className="block text-muted-foreground">{t("enterpriseKnowledge.preview.category")}</span><strong>{entryDetail.category || "-"}</strong></div>
              <div><span className="block text-muted-foreground">{t("enterpriseKnowledge.preview.language")}</span><strong>{entryDetail.languages?.join(" / ") || t("enterpriseKnowledge.preview.defaultLanguage")}</strong></div>
              <div><span className="block text-muted-foreground">{t("enterpriseKnowledge.preview.author")}</span><strong>{entryDetail.author_name || "-"}</strong></div>
              <div><span className="block text-muted-foreground">{t("enterpriseKnowledge.preview.updatedAt")}</span><strong>{entryDetail.updated_at || "-"}</strong></div>
              <div><span className="block text-muted-foreground">{t("enterpriseKnowledge.preview.relatedProducts")}</span><strong>{entryDetail.related_products?.map((product) => product.name).join(" / ") || "-"}</strong></div>
              <div><span className="block text-muted-foreground">{t("enterpriseKnowledge.preview.entryScore")}</span><strong>{entryDetail.entry_score}</strong></div>
              <div><span className="block text-muted-foreground">{t("enterpriseKnowledge.preview.qualityScore")}</span><strong>{entryDetail.quality_score}</strong></div>
              <div><span className="block text-muted-foreground">{t("enterpriseKnowledge.preview.valueScore")}</span><strong>{entryDetail.value_score}</strong></div>
            </div>
            {entryDetail.quality_flags?.length ? (
              <div className="rounded-md border px-3 py-2 text-sm text-muted-foreground">
                {entryDetail.quality_flags.map((flag) => getKnowledgeCandidateFlagLabel(t, flag)).join("、")}
              </div>
            ) : null}
            <div>
              <h3 className="mb-2 text-sm font-medium">{t("enterpriseKnowledge.preview.contentTitle")}</h3>
              <div className="whitespace-pre-wrap break-words text-sm leading-7">{entryDetail.content || "-"}</div>
            </div>
          </div>
        ) : null}
    </StandardModal>
  )
}

function ProductKnowledgeDocumentsTable({
  rows,
  indexTasks,
  deletingId,
  reprocessingId,
  emptyText,
  onView,
  onDelete,
  onReprocess,
}: {
  rows: ProductKnowledgeDocumentFile[]
  indexTasks: KnowledgeIndexTask[]
  deletingId: number | null
  reprocessingId: number | null
  emptyText: string
  onView: (item: ProductKnowledgeDocumentFile) => void
  onDelete: (item: ProductKnowledgeDocumentFile) => void
  onReprocess: (item: ProductKnowledgeDocumentFile) => void
}) {
  const t = useI18n()
  const latestTaskByDocumentId = new Map<number, KnowledgeIndexTask>()
  for (const task of indexTasks) {
    if (task.subject_type !== "knowledge_document") continue
    const current = latestTaskByDocumentId.get(task.subject_id)
    if (!current || task.id > current.id) {
      latestTaskByDocumentId.set(task.subject_id, task)
    }
  }
  const columns: TableColumnsType<ProductKnowledgeDocumentFile> = [
    {
      title: t("enterpriseKnowledge.documentTable.content"),
      key: "content",
      width: 260,
      fixed: "left",
      render: (_, item) => {
        const contentTitle = item.title || item.filename || t("enterpriseKnowledge.documentFallback", { id: item.id })
        const showFilename = Boolean(item.filename && item.filename !== contentTitle)
        return (
          <div className="rhd-railops-table-cell">
            <strong title={contentTitle}>{contentTitle}</strong>
            {showFilename ? <span title={item.filename}>{item.filename}</span> : null}
            {item.knowledge_base_id > 0 ? <span>{`KB-${item.knowledge_base_id}`}</span> : null}
          </div>
        )
      },
    },
    {
      title: t("enterpriseKnowledge.documentTable.source"),
      key: "source",
      width: 132,
      render: (_, item) => (
        <StatusTag tone={getKnowledgeDocumentSourceTone(item.source_type)}>
          {getKnowledgeDocumentSourceLabel(t, item.source_type)}
        </StatusTag>
      ),
    },
    {
      title: t("enterpriseKnowledge.documentTable.progress"),
      key: "progress",
      width: 180,
      render: (_, item) => <KnowledgeParsingProgress item={item} task={latestTaskByDocumentId.get(item.id)} />,
    },
    {
      title: t("enterpriseKnowledge.documentTable.size"),
      key: "size",
      width: 100,
      render: (_, item) => formatFileSize(item.file_size),
    },
    {
      title: t("enterpriseKnowledge.documentTable.type"),
      key: "type",
      width: 160,
      render: (_, item) => (
        <div className="rhd-railops-table-cell">
          <strong title={item.mime_type || item.provider || "-"}>{item.mime_type || item.provider || "-"}</strong>
          <span>{item.provider || ke(t, "document.providerFallback")}</span>
        </div>
      ),
    },
    {
      title: t("enterpriseKnowledge.documentTable.uploadedAt"),
      key: "uploadedAt",
      width: 150,
      render: (_, item) => item.uploaded_at || item.created_at || "-",
    },
    {
      title: t("enterpriseKnowledge.documentTable.uploadedBy"),
      key: "uploadedBy",
      width: 120,
      render: (_, item) => item.uploaded_by || "-",
    },
    {
      title: t("enterpriseKnowledge.documentTable.actions"),
      key: "actions",
      width: 144,
      fixed: "right",
      align: "right",
      render: (_, item) => (
        <div className="rhd-railops-row-actions">
          <IconButton
            className="rhd-railops-knowledge-action-button"
            icon={<EyeIcon className="size-4" />}
            tooltip={t("enterpriseKnowledge.documentTable.view")}
            aria-label={t("enterpriseKnowledge.documentTable.view")}
            onClick={() => onView(item)}
          />
          <IconButton
            className="rhd-railops-knowledge-action-button"
            icon={reprocessingId === item.id ? <Loader2Icon className="size-4 animate-spin" /> : <RotateCcwIcon className="size-4" />}
            tooltip={item.index_status === "failed" ? t("enterpriseKnowledge.documentTable.reprocess") : t("enterpriseKnowledge.documentTable.reindex")}
            aria-label={item.index_status === "failed" ? t("enterpriseKnowledge.documentTable.reprocess") : t("enterpriseKnowledge.documentTable.reindex")}
            disabled={reprocessingId === item.id}
            onClick={() => onReprocess(item)}
          />
          {item.url ? (
            <IconButton
              className="rhd-railops-knowledge-action-button"
              icon={<DownloadIcon className="size-4" />}
              tooltip={ke(t, "actions.download")}
              aria-label={ke(t, "actions.download")}
              onClick={() => window.open(item.url, "_blank", "noopener,noreferrer")}
            />
          ) : (
            <IconButton
              className="rhd-railops-knowledge-action-button"
              icon={<DownloadIcon className="size-4" />}
              tooltip={ke(t, "actions.download")}
              aria-label={ke(t, "actions.download")}
              disabled
            />
          )}
          <IconButton
            className="rhd-railops-knowledge-action-button"
            icon={deletingId === item.id ? <Loader2Icon className="size-4 animate-spin" /> : <Trash2Icon className="size-4" />}
            tooltip={getKnowledgeDocumentDeleteTitle(t, item)}
            aria-label={getKnowledgeDocumentDeleteTitle(t, item)}
            danger
            disabled={deletingId === item.id || !item.deletable}
            onClick={() => onDelete(item)}
          />
        </div>
      ),
    },
  ]

  return (
    <div className="kb-doc-table-wrap rhd-railops-knowledge-table-wrap">
      <DataTable<ProductKnowledgeDocumentFile>
        className="rhd-railops-knowledge-table"
        columns={columns}
        dataSource={rows}
        rowKey="id"
        size="small"
        scroll={{ x: 1300 }}
        emptyDescription={emptyText}
      />
    </div>
  )
}

function KnowledgeRightPanel({
  uploadEnabled,
  uploadQuota,
  uploadQuotaError,
  uploadQuotaLoading,
  productScoped = false,
  showEntries = true,
  emptyText,
  entries,
  candidates,
  knowledgeDocuments,
  knowledgeDocumentsTotal,
  knowledgeIndexTasks,
  state,
  selectedProductId,
  knowledgeEntriesLoadedProductId,
  activeTab,
  docsPage,
  entriesPage,
  loading,
  knowledgeDocumentsLoading,
  knowledgeDocumentsError,
  knowledgeEntriesLoading,
  knowledgeEntriesError,
  entriesTotal,
  candidatesTotal,
  knowledgeDocumentUploading,
  deletingKnowledgeDocumentId,
  reprocessingKnowledgeDocumentId,
  actionCandidateId,
  actionEntryId,
  onTabChange,
  onDocsPageChange,
  onEntriesPageChange,
  onUploadFiles,
  onDeleteKnowledgeDocument,
  onReprocessKnowledgeDocument,
  onViewAdmittedKnowledge,
  onViewCandidate,
  onViewEntry,
  onApproveCandidate,
  onSubmitEntry,
  onPublishEntry,
  onRejectCandidate,
}: {
  uploadEnabled: boolean
  uploadQuota: KnowledgeUploadQuota | null
  uploadQuotaError: string
  uploadQuotaLoading: boolean
  productScoped?: boolean
  showEntries?: boolean
  emptyText: string
  entries: KnowledgeEntryListItem[]
  candidates: KnowledgeCandidate[]
  knowledgeDocuments: ProductKnowledgeDocumentFile[]
  knowledgeDocumentsTotal: number
  knowledgeIndexTasks: KnowledgeIndexTask[]
  state: ProductKnowledgeState | undefined
  selectedProductId?: number | null
  knowledgeEntriesLoadedProductId?: number | null
  activeTab: ActiveTab
  docsPage: number
  entriesPage: number
  loading: boolean
  knowledgeDocumentsLoading: boolean
  knowledgeDocumentsError: string
  knowledgeEntriesLoading: boolean
  knowledgeEntriesError: string
  entriesTotal: number
  candidatesTotal: number
  knowledgeDocumentUploading: boolean
  deletingKnowledgeDocumentId: number | null
  reprocessingKnowledgeDocumentId: number | null
  actionCandidateId: number | null
  actionEntryId: number | null
  onTabChange: (tab: ActiveTab) => void
  onDocsPageChange: (page: number) => void
  onEntriesPageChange: (page: number) => void
  onUploadFiles: (files: FileList | null) => Promise<void>
  onDeleteKnowledgeDocument: (item: ProductKnowledgeDocumentFile) => void
  onReprocessKnowledgeDocument: (item: ProductKnowledgeDocumentFile) => void
  onViewAdmittedKnowledge: (item: ProductKnowledgeDocumentFile) => void
  onViewCandidate: (candidate: KnowledgeCandidate) => void
  onViewEntry: (entry: KnowledgeEntryListItem) => void
  onApproveCandidate: (candidate: KnowledgeCandidate, publish: boolean) => void
  onSubmitEntry: (entry: KnowledgeEntryListItem) => void
  onPublishEntry: (entry: KnowledgeEntryListItem) => void
  onRejectCandidate: (candidate: KnowledgeCandidate) => void
}) {
  const t = useI18n()
  const uploadInputRef = useRef<HTMLInputElement | null>(null)
  const [candidateQueueView, setCandidateQueueView] = useState<CandidateQueueView>("review")
  const reviewEntries = entries.filter((entry) => ["draft", "review"].includes(entry.status) && entry.review_eligible)
  const blockedEntries = entries.filter((entry) => ["draft", "review"].includes(entry.status) && !entry.review_eligible)
  const reviewCandidates = candidates.filter((candidate) => candidate.review_eligible)
  const terminalCandidateStatuses = new Set(["approved", "rejected", "merged"])
  const blockedCandidates = candidates.filter((candidate) => candidate.requires_reassessment || (!candidate.review_eligible && !terminalCandidateStatuses.has(candidate.review_status)))
  const historyCandidates = candidates.filter((candidate) => terminalCandidateStatuses.has(candidate.review_status) && !candidate.requires_reassessment)
  const entryRows: KnowledgeContentRow[] = candidateQueueView === "review"
    ? [
        ...reviewCandidates.map((candidate) => ({ type: "candidate" as const, candidate })),
        ...reviewEntries.map((entry) => ({ type: "entry" as const, entry })),
      ]
    : candidateQueueView === "blocked" ? [
        ...blockedCandidates.map((candidate) => ({ type: "candidate" as const, candidate })),
        ...blockedEntries.map((entry) => ({ type: "entry" as const, entry })),
      ]
    : historyCandidates.map((candidate) => ({ type: "candidate" as const, candidate }))
  const pendingKnowledgeCount = knowledgeEntriesLoadedProductId === selectedProductId
    ? entriesTotal + candidatesTotal
    : (state?.totalKnowledgeEntryCount ?? 0) + (state?.pendingCandidates ?? 0)
  const kbHeadTabs: RailopsTabItem[] = [
    { value: "docs", label: t("enterpriseKnowledge.rightPanel.docsTabLabel"), count: knowledgeDocumentsTotal },
    ...(showEntries
      ? [{ value: "entries" as const, label: t("enterpriseKnowledge.rightPanel.entriesTabLabel"), count: pendingKnowledgeCount }]
      : []),
  ]
  const candidateQueueTabs: RailopsTabItem[] = [
    { value: "review", label: t("enterpriseKnowledge.rightPanel.reviewQueueLabel"), count: reviewCandidates.length + reviewEntries.length },
    { value: "blocked", label: t("enterpriseKnowledge.rightPanel.blockedQueueLabel"), count: blockedCandidates.length + blockedEntries.length },
    { value: "history", label: t("enterpriseKnowledge.rightPanel.historyQueueLabel"), count: historyCandidates.length },
  ]
  const pagedEntryRows = pageSlice(entryRows, entriesPage)

  return (
    <section className="kb-docs-side kb-content-panel rhd-railops-knowledge-content-panel">
      <div className="ent-panel kb-right-panel rhd-railops-knowledge-panel">
        <div className="ent-panel-head kb-tab-panel-head rhd-railops-knowledge-panel-head">
          <div className="min-w-0 flex-1">
            <UnderlineTabs
              ariaLabel={t("enterpriseKnowledge.rightPanel.tabsAriaLabel")}
              items={kbHeadTabs}
              value={activeTab}
              onChange={(value) => onTabChange(value as ActiveTab)}
            />
          </div>
          <div className="flex items-center gap-2">
            <span className="kb-dataset-status">{datasetDisplay(t, state)}</span>
            {uploadEnabled && hasKnowledgeMounted(state) && activeTab === "docs" ? (
              <>
                <input
                  ref={uploadInputRef}
                  type="file"
                  className="hidden"
                  multiple
                  accept=".docx,.md,.markdown,.txt,.html,.htm"
                  onChange={(event) => {
                    void onUploadFiles(event.target.files).finally(() => {
                      event.target.value = ""
                    })
                  }}
                />
                <RailopsButton
                  variant="primary"
                  disabled={knowledgeDocumentUploading}
                  onClick={() => uploadInputRef.current?.click()}
                >
                  {knowledgeDocumentUploading ? (
                    <Loader2Icon className="size-4 animate-spin" />
                  ) : (
                    <UploadIcon className="size-4" />
                  )}
                  {knowledgeDocumentUploading ? t("enterpriseKnowledge.rightPanel.uploading") : t("enterpriseKnowledge.rightPanel.upload")}
                </RailopsButton>
              </>
            ) : null}
          </div>
        </div>

        {uploadEnabled && hasKnowledgeMounted(state) && activeTab === "docs" ? (
          <KnowledgeUploadQuotaStrip
            error={uploadQuotaError}
            loading={uploadQuotaLoading}
            productScoped={productScoped}
            quota={uploadQuota}
          />
        ) : null}

        {knowledgeDocumentsError || knowledgeEntriesError ? <div className="mx-3 mt-3 kb-error-strip">{knowledgeDocumentsError || knowledgeEntriesError}</div> : null}

        {loading || (activeTab === "docs" && knowledgeDocumentsLoading) || (activeTab === "entries" && knowledgeEntriesLoading) ? (
          <div className="kb-tab-panel active">
            <div className="kb-doc-table-wrap">
              {Array.from({ length: 6 }).map((_, index) => <Skeleton.Node key={index} active className="mb-2" style={{ width: "100%", height: 40 }} />)}
            </div>
          </div>
        ) : activeTab === "docs" ? (
          <div className="kb-tab-panel active">
            <ProductKnowledgeDocumentsTable
              rows={knowledgeDocuments}
              indexTasks={knowledgeIndexTasks}
              deletingId={deletingKnowledgeDocumentId}
              reprocessingId={reprocessingKnowledgeDocumentId}
              emptyText={emptyText}
              onView={onViewAdmittedKnowledge}
              onDelete={onDeleteKnowledgeDocument}
              onReprocess={onReprocessKnowledgeDocument}
            />
            <Pagination page={docsPage} rows={knowledgeDocuments} total={knowledgeDocumentsTotal} onPageChange={onDocsPageChange} />
          </div>
        ) : (
          <div className="kb-tab-panel active">
            <div className="mb-3">
              <UnderlineTabs
                ariaLabel={t("enterpriseKnowledge.rightPanel.candidateQueueAriaLabel")}
                items={candidateQueueTabs}
                value={candidateQueueView}
                onChange={(value) => {
                  setCandidateQueueView(value as CandidateQueueView)
                  onEntriesPageChange(1)
                }}
              />
            </div>
            <KnowledgeEntryTable
              rows={pagedEntryRows}
              actionCandidateId={actionCandidateId}
              actionEntryId={actionEntryId}
              onViewCandidate={onViewCandidate}
              onViewEntry={onViewEntry}
              onApprove={onApproveCandidate}
              onSubmitEntry={onSubmitEntry}
              onPublishEntry={onPublishEntry}
              onReject={onRejectCandidate}
            />
            <Pagination page={entriesPage} rows={entryRows} onPageChange={onEntriesPageChange} />
          </div>
        )}
      </div>
    </section>
  )
}

function EnterpriseKnowledgePageContent() {
  const t = useI18n()
  const confirm = useConfirm()
  const { ready: authReady, session } = useAuth()
  const hasProductConcept = session?.featureFlags?.product !== false
  const showProductTree = authReady && hasProductConcept
  const [urlProductId] = useState<number | null>(() => {
    if (typeof window === "undefined") return null
    const params = new URLSearchParams(window.location.search)
    const value = params.get("product_id") || params.get("productId")
    const parsed = value ? Number(value) : Number.NaN
    return Number.isNaN(parsed) ? null : parsed
  })
  const [products, setProducts] = useState<ProductListItem[]>([])
  const [productTotal, setProductTotal] = useState(0)
  const [profilesByProductId, setProfilesByProductId] = useState<Record<number, ProductServiceProfile>>({})
  const [knowledgeStates, setKnowledgeStates] = useState<Record<number, ProductKnowledgeState>>({})
  const [selectedProductId, setSelectedProductId] = useState<number | null>(null)
  const [tenantKnowledgeSelected, setTenantKnowledgeSelected] = useState(() => urlProductId == null)
  const [treeSearch, setTreeSearch] = useState("")
  const [selectedProfile, setSelectedProfile] = useState<ProductServiceProfile | null>(null)
  const [links, setLinks] = useState<ProductManualLink[]>([])
  const [knowledgeDocuments, setKnowledgeDocuments] = useState<ProductKnowledgeDocumentFile[]>([])
  const [knowledgeDocumentsTotal, setKnowledgeDocumentsTotal] = useState(0)
  const [knowledgeIndexTasks, setKnowledgeIndexTasks] = useState<KnowledgeIndexTask[]>([])
  const [knowledgeEntries, setKnowledgeEntries] = useState<KnowledgeEntryListItem[]>([])
  const [knowledgeCandidates, setKnowledgeCandidates] = useState<KnowledgeCandidate[]>([])
  const [knowledgeEntriesTotal, setKnowledgeEntriesTotal] = useState(0)
  const [knowledgeCandidatesTotal, setKnowledgeCandidatesTotal] = useState(0)
  const [knowledgeEntriesLoadedProductId, setKnowledgeEntriesLoadedProductId] = useState<number | null>(null)
  const [activeTab, setActiveTab] = useState<ActiveTab>("docs")
  const [docsPage, setDocsPage] = useState(1)
  const [entriesPage, setEntriesPage] = useState(1)
  const [productsLoading, setProductsLoading] = useState(true)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailLoadedProductId, setDetailLoadedProductId] = useState<number | null>(null)
  const [productsError, setProductsError] = useState("")
  const [detailError, setDetailError] = useState("")
  const [createError, setCreateError] = useState("")
  const [createSubmitting, setCreateSubmitting] = useState(false)
  const [form, setForm] = useState<KnowledgeBaseFormState>(() => buildKnowledgeBaseFormState(t, null, null))
  const [tenantKnowledgeBases, setTenantKnowledgeBases] = useState<KnowledgeBase[]>([])
  const [tenantKnowledgeError, setTenantKnowledgeError] = useState("")
  const [tenantKnowledgeSubmitting, setTenantKnowledgeSubmitting] = useState(false)
  const [tenantKnowledgeForm, setTenantKnowledgeForm] = useState<TenantKnowledgeBaseFormState>(() => buildTenantKnowledgeBaseFormState(t))
  const [knowledgeDocumentsLoading, setKnowledgeDocumentsLoading] = useState(false)
  const [knowledgeDocumentsError, setKnowledgeDocumentsError] = useState("")
  const [knowledgeEntriesLoading, setKnowledgeEntriesLoading] = useState(false)
  const [knowledgeEntriesError, setKnowledgeEntriesError] = useState("")
  const [actionCandidateId, setActionCandidateId] = useState<number | null>(null)
  const [actionEntryId, setActionEntryId] = useState<number | null>(null)
  const [knowledgePreview, setKnowledgePreview] = useState<KnowledgePreview | null>(null)
  const [previewEntryDetail, setPreviewEntryDetail] = useState<KnowledgeEntry | null>(null)
  const [previewEntryLoading, setPreviewEntryLoading] = useState(false)
  const [previewEntryError, setPreviewEntryError] = useState("")
  const [knowledgeDocumentUploading, setKnowledgeDocumentUploading] = useState(false)
  const [deletingKnowledgeDocumentId, setDeletingKnowledgeDocumentId] = useState<number | null>(null)
  const [reprocessingKnowledgeDocumentId, setReprocessingKnowledgeDocumentId] = useState<number | null>(null)
  const [uploadQuota, setUploadQuota] = useState<KnowledgeUploadQuota | null>(null)
  const [uploadQuotaLoading, setUploadQuotaLoading] = useState(false)
  const [uploadQuotaError, setUploadQuotaError] = useState("")
  const selectedProduct = useMemo(
    () => products.find((product) => product.id === selectedProductId) ?? null,
    [products, selectedProductId]
  )

  const filteredProducts = useMemo(() => {
    const keyword = treeSearch.trim().toLowerCase()
    return products.filter((product) => {
      if (!keyword) return true
      const label = getProductTreeDisplayName(product)
      return `${label} ${product.code} ${product.category} ${product.product_line}`.toLowerCase().includes(keyword)
    })
  }, [products, treeSearch])
  const filteredTreeGroups = useMemo(() => buildProductTreeGroups(filteredProducts), [filteredProducts])

  const tenantKnowledgeState = useMemo(
    () => buildTenantKnowledgeState(tenantKnowledgeBases, productTotal),
    [productTotal, tenantKnowledgeBases]
  )
  const tenantKnowledgeBaseID = tenantKnowledgeState.knowledgeBase?.id ?? null
  const tenantDocumentState = useMemo<ProductKnowledgeState | undefined>(() => {
    const knowledgeBase = tenantKnowledgeState.knowledgeBase
    if (!knowledgeBase) return undefined
    return {
      knowledgeBaseId: knowledgeBase.id,
      totalKnowledgeEntryCount: (knowledgeBase.documentCount || 0) + (knowledgeBase.faqCount || 0),
      linkedEntries: knowledgeBase.documentCount || 0,
      pendingCandidates: 0,
      coverageScore: null,
    }
  }, [tenantKnowledgeState.knowledgeBase])

  const selectedState = selectedProductId ? knowledgeStates[selectedProductId] : undefined
  const selectedKnowledgeBaseID = selectedState?.knowledgeBaseId ?? null
  const effectiveProfile = selectedProfile ?? (selectedProductId ? profilesByProductId[selectedProductId] ?? null : null)
  const detailInitialLoading = detailLoading && detailLoadedProductId !== selectedProductId
  const showKnowledgeInfo = hasKnowledgeMounted(selectedState) || detailInitialLoading

  const applyKnowledgeDetail = useCallback((
    productId: number,
    nextProfile: ProductServiceProfile | null,
    nextLinks: ProductManualLink[],
    nextState: ProductKnowledgeState,
    nextError: string
  ) => {
    setSelectedProfile(nextProfile)
    setLinks(nextLinks)
    setKnowledgeStates((current) => ({ ...current, [productId]: nextState }))
    if (nextProfile) {
      setProfilesByProductId((current) => ({ ...current, [productId]: nextProfile }))
    }
    setDetailError(nextError)
  }, [])

  const fetchKnowledgeDetail = useCallback(async (productId: number) => {
    const [profileResponse, linksResponse, coverageResponse] = await Promise.all([
      getProductProfile(productId),
      getProductKnowledgeLinks(productId),
      getProductKnowledgeCoverage(productId),
    ])

    const nextProfile = profileResponse.success && profileResponse.data ? profileResponse.data : null
    const nextLinks = linksResponse.success && linksResponse.data ? linksResponse.data : []
    const nextCoverage = coverageResponse.success && coverageResponse.data ? coverageResponse.data : null
    const nextState = buildProductKnowledgeState(nextProfile, nextLinks, nextCoverage)
    const nextError = [
      !profileResponse.success ? readErrorMessage(profileResponse, ke(t, "errors.profileLoadFailed")) : "",
      !linksResponse.success ? readErrorMessage(linksResponse, ke(t, "errors.linksLoadFailed")) : "",
      !coverageResponse.success ? readErrorMessage(coverageResponse, ke(t, "errors.coverageLoadFailed")) : "",
    ].filter(Boolean).join("；")

    return { nextProfile, nextLinks, nextState, nextError }
  }, [t])

  const refreshSelectedKnowledge = useCallback(async (productId: number) => {
    const detail = await fetchKnowledgeDetail(productId)
    applyKnowledgeDetail(
      productId,
      detail.nextProfile,
      detail.nextLinks,
      detail.nextState,
      detail.nextError
    )
  }, [applyKnowledgeDetail, fetchKnowledgeDetail])

  const loadTenantKnowledgeBases = useCallback(async () => {
    const knowledgeBaseRows = await fetchKnowledgeBasesAll()
    setTenantKnowledgeBases(knowledgeBaseRows || [])
  }, [])

  const loadKnowledgeDocuments = useCallback(async (productId: number | null, page = 1, silent = false) => {
    if (!productId) {
      setKnowledgeDocuments([])
      setKnowledgeDocumentsTotal(0)
      setKnowledgeIndexTasks([])
      setKnowledgeDocumentsError("")
      setKnowledgeDocumentsLoading(false)
      return [] as ProductKnowledgeDocumentFile[]
    }
    if (!silent) {
      setKnowledgeDocumentsLoading(true)
      setKnowledgeDocumentsError("")
    }
    try {
      const [response, taskResponse] = await Promise.all([
        getProductKnowledgeDocuments(productId, { page, page_size: PER_PAGE }),
        fetchKnowledgeIndexTasks(productId).catch(() => null),
      ])
      if (!response.success || !response.data) {
        throw new Error(readErrorMessage(response, ke(t, "errors.documentsLoadFailed")))
      }
      setKnowledgeDocuments(response.data.items || [])
      setKnowledgeDocumentsTotal(response.data.total || 0)
      setKnowledgeIndexTasks(taskResponse?.success && taskResponse.data ? taskResponse.data : [])
      return response.data.items || []
    } catch (error) {
      if (!silent) {
        setKnowledgeDocuments([])
        setKnowledgeDocumentsTotal(0)
        setKnowledgeDocumentsError(error instanceof Error ? error.message : ke(t, "errors.documentsLoadFailed"))
      }
      return [] as ProductKnowledgeDocumentFile[]
    } finally {
      if (!silent) {
        setKnowledgeDocumentsLoading(false)
      }
    }
  }, [t])

  const loadTenantKnowledgeDocuments = useCallback(async (knowledgeBaseId: number | null, page = 1, silent = false) => {
    if (!knowledgeBaseId) {
      setKnowledgeDocuments([])
      setKnowledgeDocumentsTotal(0)
      setKnowledgeIndexTasks([])
      setKnowledgeDocumentsError("")
      setKnowledgeDocumentsLoading(false)
      return [] as ProductKnowledgeDocumentFile[]
    }
    if (!silent) {
      setKnowledgeDocumentsLoading(true)
      setKnowledgeDocumentsError("")
    }
    try {
      const [response, taskResponse] = await Promise.all([
        fetchTenantKnowledgeDocuments(knowledgeBaseId, { page, page_size: PER_PAGE }),
        fetchKnowledgeIndexTasks().catch(() => null),
      ])
      if (!response.success || !response.data) {
        throw new Error(readErrorMessage(response, ke(t, "errors.tenantDocumentsLoadFailed")))
      }
      setKnowledgeDocuments(response.data.items || [])
      setKnowledgeDocumentsTotal(response.data.total || 0)
      setKnowledgeIndexTasks(
        taskResponse?.success && taskResponse.data
          ? taskResponse.data.filter((task) => task.knowledge_base_id === knowledgeBaseId)
          : []
      )
      return response.data.items || []
    } catch (error) {
      if (!silent) {
        setKnowledgeDocuments([])
        setKnowledgeDocumentsTotal(0)
        setKnowledgeDocumentsError(error instanceof Error ? error.message : ke(t, "errors.tenantDocumentsLoadFailed"))
      }
      return [] as ProductKnowledgeDocumentFile[]
    } finally {
      if (!silent) {
        setKnowledgeDocumentsLoading(false)
      }
    }
  }, [t])

  const loadUploadQuota = useCallback(async () => {
    setUploadQuotaLoading(true)
    setUploadQuotaError("")
    try {
      const response = await fetchKnowledgeUploadQuota(tenantKnowledgeSelected ? undefined : selectedProductId ?? undefined)
      if (!response.success || !response.data) {
        throw new Error(readErrorMessage(response, ke(t, "errors.uploadQuotaLoadFailed")))
      }
      setUploadQuota(response.data)
    } catch (error) {
      setUploadQuotaError(error instanceof Error ? error.message : ke(t, "errors.uploadQuotaLoadFailed"))
    } finally {
      setUploadQuotaLoading(false)
    }
  }, [selectedProductId, tenantKnowledgeSelected, t])

  const loadKnowledgeEntries = useCallback(async (productId: number | null) => {
    if (!productId) {
      setKnowledgeEntries([])
      setKnowledgeCandidates([])
      setKnowledgeEntriesTotal(0)
      setKnowledgeCandidatesTotal(0)
      setKnowledgeEntriesLoadedProductId(null)
      setKnowledgeEntriesError("")
      setKnowledgeEntriesLoading(false)
      return
    }
    setKnowledgeEntriesLoading(true)
    setKnowledgeEntriesError("")
    try {
      const [entryResponse, candidateResponse] = await Promise.all([
        fetchKnowledgeEntries({ product_id: productId, page: 1, page_size: 100 }),
        fetchKnowledgeCandidatePage({
          product_id: productId,
          review_status: "pending,needs_enrichment,low_value,low_quality,duplicate,approved,rejected,merged",
          page: 1,
          page_size: 100,
        }),
      ])
      if (!entryResponse.success || !entryResponse.data) {
        throw new Error(readErrorMessage(entryResponse, ke(t, "errors.entriesLoadFailed")))
      }
      if (!candidateResponse.success || !candidateResponse.data) {
        throw new Error(readErrorMessage(candidateResponse, ke(t, "errors.candidatesLoadFailed")))
      }
      setKnowledgeEntries(entryResponse.data.items || [])
      setKnowledgeCandidates(candidateResponse.data.items || [])
      setKnowledgeEntriesTotal(entryResponse.data.total || 0)
      setKnowledgeCandidatesTotal(candidateResponse.data.total || 0)
      setKnowledgeEntriesLoadedProductId(productId)
      setEntriesPage(1)
    } catch (error) {
      setKnowledgeEntries([])
      setKnowledgeCandidates([])
      setKnowledgeEntriesTotal(0)
      setKnowledgeCandidatesTotal(0)
      setKnowledgeEntriesLoadedProductId(null)
      setKnowledgeEntriesError(error instanceof Error ? error.message : ke(t, "errors.entriesLoadFailed"))
    } finally {
      setKnowledgeEntriesLoading(false)
    }
  }, [t])

  const refreshKnowledgeDocuments = useCallback(async (silent = false) => {
    if (tenantKnowledgeSelected) {
      return loadTenantKnowledgeDocuments(tenantKnowledgeBaseID, docsPage, silent)
    }
    return loadKnowledgeDocuments(selectedProductId, docsPage, silent)
  }, [docsPage, loadKnowledgeDocuments, loadTenantKnowledgeDocuments, selectedProductId, tenantKnowledgeBaseID, tenantKnowledgeSelected])

  const loadProducts = useCallback(async () => {
    if (!authReady) return
    setProductsLoading(true)
    setProductsError("")
    if (!hasProductConcept) {
      try {
        await loadTenantKnowledgeBases()
        setProducts([])
        setProductTotal(0)
        setSelectedProductId(null)
        setTenantKnowledgeSelected(true)
        setTenantKnowledgeError("")
      } catch (error) {
        setTenantKnowledgeError(error instanceof Error ? error.message : ke(t, "errors.tenantKnowledgeBaseLoadFailed"))
      } finally {
        setProductsLoading(false)
      }
      return
    }
    try {
      const [productRowsResult, countResult, tenantKnowledgeResult] = await Promise.allSettled([
        listAllProducts(),
        fetchProductCount(),
        loadTenantKnowledgeBases(),
      ])
      if (productRowsResult.status !== "fulfilled") {
        throw productRowsResult.reason
      }
      const productRows = productRowsResult.value
      setProducts(productRows)
      setProductTotal(
        countResult.status === "fulfilled" && countResult.value.success && countResult.value.data
          ? countResult.value.data.total
          : productRows.length
      )
      setSelectedProductId((current) => {
        if (current && productRows.some((product) => product.id === current)) return current
        if (urlProductId && productRows.some((product) => product.id === urlProductId)) return urlProductId
        return null
      })

      if (tenantKnowledgeResult.status === "rejected") {
        setTenantKnowledgeError(tenantKnowledgeResult.reason instanceof Error ? tenantKnowledgeResult.reason.message : ke(t, "errors.tenantKnowledgeBaseLoadFailed"))
      } else {
        setTenantKnowledgeError("")
      }
    } catch (error) {
      setProductsError(error instanceof Error ? error.message : ke(t, "errors.productsLoadFailed"))
    } finally {
      setProductsLoading(false)
    }
  }, [authReady, hasProductConcept, loadTenantKnowledgeBases, t, urlProductId])

  useEffect(() => {
    void loadProducts()
  }, [loadProducts])

  useEffect(() => {
    void loadUploadQuota()
  }, [loadUploadQuota])

  useEffect(() => {
    if (tenantKnowledgeSelected) return
    if (selectedProductId && filteredProducts.some((product) => product.id === selectedProductId)) {
      return
    }
    setSelectedProductId(filteredProducts[0]?.id ?? null)
  }, [filteredProducts, selectedProductId, tenantKnowledgeSelected])

  useEffect(() => {
    if (!selectedProductId) {
      setSelectedProfile(null)
      setLinks([])
      setKnowledgeDocuments([])
      setDetailLoading(false)
      setDetailLoadedProductId(null)
      setDetailError("")
      setKnowledgeDocumentsError("")
      setCreateError("")
      setForm(buildKnowledgeBaseFormState(t, null, null))
      return
    }

    const productId = selectedProductId
    let cancelled = false
    setDetailLoading(true)
    setDetailError("")

    async function loadSelectedKnowledge() {
      const detail = await fetchKnowledgeDetail(productId)

      if (cancelled) return
      applyKnowledgeDetail(
        productId,
        detail.nextProfile,
        detail.nextLinks,
        detail.nextState,
        detail.nextError
      )
      setDetailLoadedProductId(productId)
      setDetailLoading(false)
    }

    void loadSelectedKnowledge().catch((error) => {
      if (cancelled) return
      setDetailError(error instanceof Error ? error.message : ke(t, "errors.knowledgeDataLoadFailed"))
      setSelectedProfile(null)
      setLinks([])
      setDetailLoadedProductId(null)
      setDetailLoading(false)
    })

    return () => {
      cancelled = true
    }
  }, [applyKnowledgeDetail, fetchKnowledgeDetail, selectedProductId, t])

  useEffect(() => {
    if (!selectedProduct || hasKnowledgeMounted(selectedState)) return
    setForm(buildKnowledgeBaseFormState(t, selectedProduct, effectiveProfile))
  }, [effectiveProfile, selectedProduct, selectedState, t])

  useEffect(() => {
    setTenantKnowledgeForm(buildTenantKnowledgeBaseFormState(t, tenantKnowledgeState.knowledgeBase))
  }, [t, tenantKnowledgeState])

  useEffect(() => {
    if (tenantKnowledgeSelected) {
      if (!tenantKnowledgeBaseID) {
        void loadTenantKnowledgeDocuments(null, docsPage)
        return
      }
      let cancelled = false
      void (async () => {
        const rows = await loadTenantKnowledgeDocuments(tenantKnowledgeBaseID, docsPage)
        if (cancelled) return
        if (rows.length > 0) {
          setKnowledgeDocumentsError("")
        }
      })()
      return () => {
        cancelled = true
      }
    }
    if (!selectedProductId || !selectedKnowledgeBaseID) {
      void loadKnowledgeDocuments(null, docsPage)
      return
    }
    let cancelled = false
    void (async () => {
      const rows = await loadKnowledgeDocuments(selectedProductId, docsPage)
      if (cancelled) return
      if (rows.length > 0) {
        await refreshSelectedKnowledge(selectedProductId)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [docsPage, loadKnowledgeDocuments, loadTenantKnowledgeDocuments, refreshSelectedKnowledge, selectedKnowledgeBaseID, selectedProductId, tenantKnowledgeBaseID, tenantKnowledgeSelected])

  useEffect(() => {
    if (!tenantKnowledgeSelected) return
    void refreshKnowledgeDocuments()
  }, [refreshKnowledgeDocuments, tenantKnowledgeSelected])

  useEffect(() => {
    if (
      (!knowledgeDocuments.some((item) => item.index_status === "pending") && !knowledgeIndexTasks.some(isKnowledgeIndexTaskActive))
    ) return
    const timer = window.setTimeout(() => {
      void refreshKnowledgeDocuments(true)
    }, 3000)
    return () => window.clearTimeout(timer)
  }, [knowledgeDocuments, knowledgeIndexTasks, refreshKnowledgeDocuments])

  useEffect(() => {
    if (activeTab !== "entries") return
    void loadKnowledgeEntries(selectedProductId)
  }, [activeTab, loadKnowledgeEntries, selectedProductId])

  const handleUploadKnowledgeDocuments = useCallback(async (files: FileList | null) => {
    if (!files || files.length === 0) return
    if (tenantKnowledgeSelected && !tenantKnowledgeBaseID) return
    if (!tenantKnowledgeSelected && !selectedProductId) return
    const selectedFiles = Array.from(files)
    const singleFileLimit = uploadQuota?.single_file_size_limit ?? 0
    const oversizedFile = singleFileLimit > 0
      ? selectedFiles.find((file) => file.size > singleFileLimit)
      : null
    if (oversizedFile) {
      setKnowledgeDocumentsError(ke(t, "errors.fileTooLarge", { filename: oversizedFile.name, limit: formatFileSize(singleFileLimit) }))
      return
    }
    if (
      uploadQuota &&
      !uploadQuota.unlimited_documents &&
      uploadQuota.document_remaining >= 0 &&
      selectedFiles.length > uploadQuota.document_remaining
    ) {
      setKnowledgeDocumentsError(ke(t, "errors.remainingDocuments", { count: uploadQuota.document_remaining }))
      return
    }
    setKnowledgeDocumentUploading(true)
    setKnowledgeDocumentsError("")
    try {
      for (const file of selectedFiles) {
        const response = tenantKnowledgeSelected
          ? await uploadTenantKnowledgeDocument(tenantKnowledgeBaseID!, file)
          : await uploadProductKnowledgeDocument(selectedProductId!, file)
        if (!response.success) {
          throw new Error(readErrorMessage(response, ke(t, "errors.uploadFileFailed", { filename: file.name })))
        }
      }
      if (tenantKnowledgeSelected) {
        await Promise.all([
          refreshKnowledgeDocuments(),
          loadTenantKnowledgeBases(),
          loadUploadQuota(),
        ])
      } else {
        await Promise.all([
          refreshKnowledgeDocuments(),
          refreshSelectedKnowledge(selectedProductId!),
          loadUploadQuota(),
        ])
      }
    } catch (error) {
      setKnowledgeDocumentsError(error instanceof Error ? error.message : ke(t, "errors.uploadDocumentsFailed"))
    } finally {
      setKnowledgeDocumentUploading(false)
    }
  }, [loadTenantKnowledgeBases, loadUploadQuota, refreshKnowledgeDocuments, refreshSelectedKnowledge, selectedProductId, t, tenantKnowledgeSelected, tenantKnowledgeBaseID, uploadQuota])

  const handleDeleteKnowledgeDocument = useCallback(async (item: ProductKnowledgeDocumentFile) => {
    if (!item.deletable) return
    if (tenantKnowledgeSelected && !tenantKnowledgeBaseID) return
    if (!tenantKnowledgeSelected && !selectedProductId) return
    const accepted = await confirm({
      title: t("enterpriseKnowledge.documentDeleteDialogTitle"),
      description: getKnowledgeDocumentDeleteDescription(t, item),
      confirmText: ke(t, "actions.delete"),
      variant: "destructive",
    })
    if (!accepted) return
    setDeletingKnowledgeDocumentId(item.id)
    setKnowledgeDocumentsError("")
    try {
      const response = tenantKnowledgeSelected
        ? await deleteTenantKnowledgeDocument(tenantKnowledgeBaseID!, item.id)
        : await deleteProductKnowledgeDocument(selectedProductId!, item.id)
      if (!response.success) {
        throw new Error(readErrorMessage(response, ke(t, "errors.deleteDocumentFailed")))
      }
      if (tenantKnowledgeSelected) {
        await Promise.all([
          refreshKnowledgeDocuments(),
          loadTenantKnowledgeBases(),
          loadUploadQuota(),
        ])
      } else {
        await Promise.all([
          refreshKnowledgeDocuments(),
          refreshSelectedKnowledge(selectedProductId!),
          loadUploadQuota(),
        ])
      }
    } catch (error) {
      setKnowledgeDocumentsError(error instanceof Error ? error.message : ke(t, "errors.deleteDocumentFailed"))
    } finally {
      setDeletingKnowledgeDocumentId(null)
    }
  }, [confirm, loadTenantKnowledgeBases, loadUploadQuota, refreshKnowledgeDocuments, refreshSelectedKnowledge, selectedProductId, t, tenantKnowledgeBaseID, tenantKnowledgeSelected])

  const handleReprocessKnowledgeDocument = useCallback(async (item: ProductKnowledgeDocumentFile) => {
    if (tenantKnowledgeSelected && !tenantKnowledgeBaseID) return
    if (!tenantKnowledgeSelected && !selectedProductId) return
    setReprocessingKnowledgeDocumentId(item.id)
    setKnowledgeDocumentsError("")
    try {
      const response = tenantKnowledgeSelected
        ? await reprocessTenantKnowledgeDocument(tenantKnowledgeBaseID!, item.id)
        : await reprocessProductKnowledgeDocument(selectedProductId!, item.id)
      if (!response.success) {
        throw new Error(readErrorMessage(response, ke(t, "errors.reprocessFailed")))
      }
      await refreshKnowledgeDocuments()
    } catch (error) {
      setKnowledgeDocumentsError(error instanceof Error ? error.message : ke(t, "errors.reprocessFailed"))
    } finally {
      setReprocessingKnowledgeDocumentId(null)
    }
  }, [refreshKnowledgeDocuments, selectedProductId, t, tenantKnowledgeBaseID, tenantKnowledgeSelected])

  const handleViewKnowledgeCandidate = useCallback((candidate: KnowledgeCandidate) => {
    setKnowledgePreview({ type: "candidate", candidate })
    setPreviewEntryDetail(null)
    setPreviewEntryError("")
    setPreviewEntryLoading(false)
  }, [t])

  const handleViewKnowledgeEntry = useCallback(async (entry: KnowledgePreviewEntry) => {
    setKnowledgePreview({ type: "entry", entry })
    setPreviewEntryDetail(null)
    setPreviewEntryError("")
    setPreviewEntryLoading(true)
    try {
      const response = await fetchKnowledgeEntry(String(entry.id))
      if (!response.success || !response.data) {
        throw new Error(readErrorMessage(response, ke(t, "errors.entryDetailLoadFailed")))
      }
      setPreviewEntryDetail(response.data)
    } catch (error) {
      setPreviewEntryError(error instanceof Error ? error.message : ke(t, "errors.entryDetailLoadFailed"))
    } finally {
      setPreviewEntryLoading(false)
    }
  }, [])

  const handleViewAdmittedKnowledge = useCallback((item: ProductKnowledgeDocumentFile) => {
    void handleViewKnowledgeEntry({
      id: item.knowledge_entry_id,
      knowledge_base_id: item.knowledge_base_id,
      title: item.title || item.filename,
    })
  }, [handleViewKnowledgeEntry])

  const handleKnowledgePreviewOpenChange = useCallback((open: boolean) => {
    if (open) return
    setKnowledgePreview(null)
    setPreviewEntryDetail(null)
    setPreviewEntryError("")
    setPreviewEntryLoading(false)
  }, [])

  const handleApproveCandidate = useCallback(async (candidate: KnowledgeCandidate, publish: boolean) => {
    if (!selectedProductId) return
    setActionCandidateId(candidate.id)
    setKnowledgeEntriesError("")
    try {
      const response = await approveKnowledgeCandidate(candidate.id, {
        publish,
        visibility: publish ? "public" : "internal",
      })
      if (!response.success) {
        throw new Error(readErrorMessage(response, ke(t, "errors.reviewFailed")))
      }
      await Promise.all([
        loadKnowledgeEntries(selectedProductId),
        refreshKnowledgeDocuments(),
        refreshSelectedKnowledge(selectedProductId),
      ])
      setEntriesPage(1)
      setKnowledgePreview(null)
      setPreviewEntryDetail(null)
    } catch (error) {
      setKnowledgeEntriesError(error instanceof Error ? error.message : ke(t, "errors.reviewFailed"))
    } finally {
      setActionCandidateId(null)
    }
  }, [loadKnowledgeEntries, refreshKnowledgeDocuments, refreshSelectedKnowledge, selectedProductId, t])

  const handleEnrichCandidate = useCallback(async (candidate: KnowledgeCandidate, input: EnrichKnowledgeCandidateInput) => {
    if (!selectedProductId) return
    setActionCandidateId(candidate.id)
    setKnowledgeEntriesError("")
    try {
      const response = await enrichKnowledgeCandidate(candidate.id, input)
      if (!response.success || !response.data) {
        throw new Error(readErrorMessage(response, ke(t, "errors.rescoreFailed")))
      }
      await Promise.all([
        loadKnowledgeEntries(selectedProductId),
        refreshSelectedKnowledge(selectedProductId),
      ])
      setKnowledgePreview({ type: "candidate", candidate: response.data })
    } catch (error) {
      setKnowledgeEntriesError(error instanceof Error ? error.message : ke(t, "errors.rescoreFailed"))
    } finally {
      setActionCandidateId(null)
    }
  }, [loadKnowledgeEntries, refreshSelectedKnowledge, selectedProductId, t])

  const handleDetachDuplicateCandidate = useCallback(async (candidate: KnowledgeCandidate) => {
    if (!selectedProductId) return
    setActionCandidateId(candidate.id)
    setKnowledgeEntriesError("")
    try {
      const response = await detachDuplicateKnowledgeCandidate(candidate.id)
      if (!response.success || !response.data) {
        throw new Error(readErrorMessage(response, ke(t, "errors.detachDuplicateFailed")))
      }
      await loadKnowledgeEntries(selectedProductId)
      setKnowledgePreview({ type: "candidate", candidate: response.data })
    } catch (error) {
      setKnowledgeEntriesError(error instanceof Error ? error.message : ke(t, "errors.detachDuplicateFailed"))
    } finally {
      setActionCandidateId(null)
    }
  }, [loadKnowledgeEntries, selectedProductId, t])

  const handleMergeCandidate = useCallback(async (candidate: KnowledgeCandidate, entryId: number) => {
    if (!selectedProductId) return
    setActionCandidateId(candidate.id)
    setKnowledgeEntriesError("")
    try {
      const response = await mergeKnowledgeCandidate(candidate.id, entryId)
      if (!response.success) {
        throw new Error(readErrorMessage(response, ke(t, "errors.mergeFailed")))
      }
      await Promise.all([
        loadKnowledgeEntries(selectedProductId),
        refreshSelectedKnowledge(selectedProductId),
      ])
      setKnowledgePreview(null)
    } catch (error) {
      setKnowledgeEntriesError(error instanceof Error ? error.message : ke(t, "errors.mergeFailed"))
    } finally {
      setActionCandidateId(null)
    }
  }, [loadKnowledgeEntries, refreshSelectedKnowledge, selectedProductId, t])

  const handleRejectCandidate = useCallback(async (candidate: KnowledgeCandidate) => {
    if (!selectedProductId) return
    setActionCandidateId(candidate.id)
    setKnowledgeEntriesError("")
    try {
      const response = await rejectKnowledgeCandidate(candidate.id, ke(t, "review.rejectReason"))
      if (!response.success) {
        throw new Error(readErrorMessage(response, ke(t, "errors.rejectFailed")))
      }
      await loadKnowledgeEntries(selectedProductId)
      setEntriesPage(1)
      setKnowledgePreview(null)
      setPreviewEntryDetail(null)
    } catch (error) {
      setKnowledgeEntriesError(error instanceof Error ? error.message : ke(t, "errors.rejectFailed"))
    } finally {
      setActionCandidateId(null)
    }
  }, [loadKnowledgeEntries, selectedProductId, t])

  const handlePublishKnowledgeEntry = useCallback(async (entry: KnowledgeEntryListItem) => {
    if (!selectedProductId) return
    setActionEntryId(entry.id)
    setKnowledgeEntriesError("")
    try {
      const response = await publishEntry(String(entry.id))
      if (!response.success) {
        throw new Error(readErrorMessage(response, ke(t, "errors.publishFailed")))
      }
      await Promise.all([
        loadKnowledgeEntries(selectedProductId),
        refreshKnowledgeDocuments(),
        refreshSelectedKnowledge(selectedProductId),
      ])
      setEntriesPage(1)
      setKnowledgePreview(null)
      setPreviewEntryDetail(null)
    } catch (error) {
      setKnowledgeEntriesError(error instanceof Error ? error.message : ke(t, "errors.publishFailed"))
    } finally {
      setActionEntryId(null)
    }
  }, [loadKnowledgeEntries, refreshKnowledgeDocuments, refreshSelectedKnowledge, selectedProductId, t])

  const handleSubmitKnowledgeEntry = useCallback(async (entry: KnowledgeEntryListItem) => {
    if (!selectedProductId) return
    setActionEntryId(entry.id)
    setKnowledgeEntriesError("")
    try {
      const response = await submitForReview(String(entry.id))
      if (!response.success) {
        throw new Error(readErrorMessage(response, ke(t, "errors.submitFailed")))
      }
      await loadKnowledgeEntries(selectedProductId)
      setEntriesPage(1)
      setKnowledgePreview(null)
      setPreviewEntryDetail(null)
    } catch (error) {
      setKnowledgeEntriesError(error instanceof Error ? error.message : ke(t, "errors.submitFailed"))
    } finally {
      setActionEntryId(null)
    }
  }, [loadKnowledgeEntries, selectedProductId, t])

  const handleSelectProduct = useCallback((product: ProductListItem) => {
    setTenantKnowledgeSelected(false)
    setSelectedProductId(product.id)
    setActiveTab("docs")
    setDocsPage(1)
    setEntriesPage(1)
    setKnowledgePreview(null)
    setPreviewEntryDetail(null)
    setCreateError("")
  }, [])

  const handleSelectTenantKnowledge = useCallback(() => {
    setTenantKnowledgeSelected(true)
    setSelectedProductId(null)
    setActiveTab("docs")
    setDocsPage(1)
    setEntriesPage(1)
    setKnowledgePreview(null)
    setPreviewEntryDetail(null)
    setCreateError("")
    setDetailError("")
  }, [])

  const handleFormChange = useCallback((patch: Partial<KnowledgeBaseFormState>) => {
    setForm((current) => ({ ...current, ...patch }))
  }, [])

  const handleFormReset = useCallback(() => {
    setForm(buildKnowledgeBaseFormState(t, selectedProduct, effectiveProfile))
    setCreateError("")
  }, [effectiveProfile, selectedProduct, t])

  const handleCreateKnowledgeBase = useCallback(async () => {
    if (!selectedProductId) return

    const payload = {
      name: form.name.trim(),
      description: form.description.trim(),
      support_locales: splitMultiValueInput(form.supportLocales),
      support_regions: splitMultiValueInput(form.supportRegions),
    }

    if (!payload.name) {
      setCreateError(ke(t, "errors.nameRequired"))
      return
    }

    setCreateSubmitting(true)
    setCreateError("")
    try {
      const response = await createProductKnowledgeBase(selectedProductId, payload)
      if (!response.success) {
        throw new Error(readErrorMessage(response, ke(t, "errors.createKnowledgeBaseFailed")))
      }

      await refreshSelectedKnowledge(selectedProductId)
      setForm(buildKnowledgeBaseFormState(t, selectedProduct, response.data ?? null))
    } catch (error) {
      setCreateError(error instanceof Error ? error.message : ke(t, "errors.createKnowledgeBaseFailed"))
    } finally {
      setCreateSubmitting(false)
    }
  }, [form, refreshSelectedKnowledge, selectedProduct, selectedProductId, t])

  const handleTenantKnowledgeFormChange = useCallback((patch: Partial<TenantKnowledgeBaseFormState>) => {
    setTenantKnowledgeForm((current) => ({ ...current, ...patch }))
  }, [])

  const handleTenantKnowledgeFormReset = useCallback(() => {
    setTenantKnowledgeForm(buildTenantKnowledgeBaseFormState(t, tenantKnowledgeState.knowledgeBase))
    setTenantKnowledgeError("")
  }, [t, tenantKnowledgeState])

  const handleCreateTenantKnowledgeBase = useCallback(async () => {
    const name = tenantKnowledgeForm.name.trim()
    const description = tenantKnowledgeForm.description.trim()
    if (!name) {
      setTenantKnowledgeError(ke(t, "errors.nameRequired"))
      return
    }

    setTenantKnowledgeSubmitting(true)
    setTenantKnowledgeError("")
    try {
      const knowledgeBase = await createKnowledgeBase(buildKnowledgeBasePayload(name, description))
      await ensureTenantDefaultAIAgent()
      await loadTenantKnowledgeBases()
      setTenantKnowledgeForm(buildTenantKnowledgeBaseFormState(t, knowledgeBase))
    } catch (error) {
      setTenantKnowledgeError(error instanceof Error ? error.message : ke(t, "errors.createTenantKnowledgeBaseFailed"))
    } finally {
      setTenantKnowledgeSubmitting(false)
    }
  }, [loadTenantKnowledgeBases, t, tenantKnowledgeForm.description, tenantKnowledgeForm.name])

  return (
    <PageShell
      className="rhd-railops-knowledge-page"
      title={t(hasProductConcept ? "enterpriseKnowledge.pageTitle" : "enterpriseKnowledge.tenantPageTitle")}
      breadcrumb={useRouteBreadcrumbItems()}
    >

      {showProductTree && productsError && products.length === 0 ? (
        <section className="ent-panel">
          <ErrorState
            title={t("enterpriseKnowledge.loadProductsFailed")}
            description={productsError}
            action={{ label: t("common.retry"), onClick: loadProducts }}
          />
        </section>
      ) : (
        <section
          className="kb-knowledge-layout rhd-railops-knowledge-workspace"
          style={showProductTree ? undefined : { gridTemplateColumns: "minmax(0, 1fr)" }}
        >
          {showProductTree ? (
            <div className="rhd-railops-product-panel rhd-railops-product-directory-panel rhd-railops-knowledge-product-panel">
              <ProductTree
                title={t("enterpriseKnowledge.productTreeTitle")}
                totalCount={productTotal}
                countLabel={t("enterpriseKnowledge.productTreeCountLabel", { count: productTotal })}
                groups={filteredTreeGroups}
                mode="directory"
                rootLabel={t("enterpriseKnowledge.productTreeRootLabel")}
                loading={productsLoading}
                selectedProductId={selectedProductId}
                rootSelected={false}
                pinnedItems={[{
                  key: "tenant-default-knowledge",
                  label: t("enterpriseKnowledge.tenantPanel.title"),
                  badge: <span className="ent-product-list-badge">{t("enterpriseKnowledge.productTreeDefaultTag")}</span>,
                  selected: tenantKnowledgeSelected,
                  onSelect: handleSelectTenantKnowledge,
                }]}
                search={treeSearch}
                onSearchChange={setTreeSearch}
                onSelectProduct={handleSelectProduct}
                emptyText={t("enterpriseKnowledge.productTreeEmptyText")}
                renderProductMeta={(product) => <small className="kb-product-code">{product.code || `PROD-${product.id}`}</small>}
              />
            </div>
          ) : null}
          <div className="kb-work-area rhd-railops-knowledge-work-area">
            {[createError, detailError, tenantKnowledgeSelected ? tenantKnowledgeError : ""].filter(Boolean).length > 0 ? (
              <div className="kb-error-strip">{[createError, detailError, tenantKnowledgeSelected ? tenantKnowledgeError : ""].filter(Boolean).join("；")}</div>
            ) : null}
            {tenantKnowledgeSelected && !tenantKnowledgeBaseID ? (
              <main className="ent-product-editor rhd-railops-knowledge-editor">
                <TenantKnowledgePanel
                  state={tenantKnowledgeState}
                  hasProductConcept={hasProductConcept}
                  form={tenantKnowledgeForm}
                  submitting={tenantKnowledgeSubmitting}
                  error=""
                  onChange={handleTenantKnowledgeFormChange}
                  onReset={handleTenantKnowledgeFormReset}
                  onSubmit={handleCreateTenantKnowledgeBase}
                />
              </main>
            ) : null}
            {tenantKnowledgeSelected && tenantKnowledgeBaseID ? (
              <KnowledgeRightPanel
                uploadEnabled
                uploadQuota={uploadQuota}
                uploadQuotaError={uploadQuotaError}
                uploadQuotaLoading={uploadQuotaLoading}
                showEntries={false}
                emptyText={t("enterpriseKnowledge.rightPanel.tenantEmptyText")}
                entries={[]}
                candidates={[]}
                knowledgeDocuments={knowledgeDocuments}
                knowledgeDocumentsTotal={knowledgeDocumentsTotal}
                knowledgeIndexTasks={knowledgeIndexTasks}
                state={tenantDocumentState}
                selectedProductId={null}
                knowledgeEntriesLoadedProductId={null}
                activeTab="docs"
                docsPage={docsPage}
                entriesPage={1}
                loading={false}
                knowledgeDocumentsLoading={knowledgeDocumentsLoading}
                knowledgeDocumentsError={knowledgeDocumentsError}
                knowledgeEntriesLoading={false}
                knowledgeEntriesError=""
                entriesTotal={0}
                candidatesTotal={0}
                knowledgeDocumentUploading={knowledgeDocumentUploading}
                deletingKnowledgeDocumentId={deletingKnowledgeDocumentId}
                reprocessingKnowledgeDocumentId={reprocessingKnowledgeDocumentId}
                actionCandidateId={null}
                actionEntryId={null}
                onTabChange={setActiveTab}
                onDocsPageChange={setDocsPage}
                onEntriesPageChange={setEntriesPage}
                onUploadFiles={handleUploadKnowledgeDocuments}
                onDeleteKnowledgeDocument={handleDeleteKnowledgeDocument}
                onReprocessKnowledgeDocument={handleReprocessKnowledgeDocument}
                onViewAdmittedKnowledge={handleViewAdmittedKnowledge}
                onViewCandidate={handleViewKnowledgeCandidate}
                onViewEntry={handleViewKnowledgeEntry}
                onApproveCandidate={handleApproveCandidate}
                onSubmitEntry={handleSubmitKnowledgeEntry}
                onPublishEntry={handlePublishKnowledgeEntry}
                onRejectCandidate={handleRejectCandidate}
              />
            ) : null}
            {!tenantKnowledgeSelected && !showKnowledgeInfo ? (
              <main className="ent-product-editor rhd-railops-knowledge-editor">
                <KnowledgeCreatePanel
                  product={selectedProduct}
                  form={form}
                  submitting={createSubmitting}
                  onChange={handleFormChange}
                  onReset={handleFormReset}
                  onSubmit={handleCreateKnowledgeBase}
                />
              </main>
            ) : null}
            {selectedProductId && showKnowledgeInfo ? (
              <KnowledgeRightPanel
                uploadEnabled={hasKnowledgeMounted(selectedState)}
                uploadQuota={uploadQuota}
                uploadQuotaError={uploadQuotaError}
                uploadQuotaLoading={uploadQuotaLoading}
                productScoped
                emptyText={t("enterpriseKnowledge.rightPanel.productEmptyText")}
                entries={knowledgeEntries}
                candidates={knowledgeCandidates}
                knowledgeDocuments={knowledgeDocuments}
                knowledgeDocumentsTotal={knowledgeDocumentsTotal}
                knowledgeIndexTasks={knowledgeIndexTasks}
                state={selectedState}
                selectedProductId={selectedProductId}
                knowledgeEntriesLoadedProductId={knowledgeEntriesLoadedProductId}
                activeTab={activeTab}
                docsPage={docsPage}
                entriesPage={entriesPage}
                loading={detailInitialLoading}
                knowledgeDocumentsLoading={knowledgeDocumentsLoading}
                knowledgeDocumentsError={knowledgeDocumentsError}
                knowledgeEntriesLoading={knowledgeEntriesLoading}
                knowledgeEntriesError={knowledgeEntriesError}
                entriesTotal={knowledgeEntriesTotal}
                candidatesTotal={knowledgeCandidatesTotal}
                knowledgeDocumentUploading={knowledgeDocumentUploading}
                deletingKnowledgeDocumentId={deletingKnowledgeDocumentId}
                reprocessingKnowledgeDocumentId={reprocessingKnowledgeDocumentId}
                actionCandidateId={actionCandidateId}
                actionEntryId={actionEntryId}
                onTabChange={setActiveTab}
                onDocsPageChange={setDocsPage}
                onEntriesPageChange={setEntriesPage}
                onUploadFiles={handleUploadKnowledgeDocuments}
                onDeleteKnowledgeDocument={handleDeleteKnowledgeDocument}
                onReprocessKnowledgeDocument={handleReprocessKnowledgeDocument}
                onViewAdmittedKnowledge={handleViewAdmittedKnowledge}
                onViewCandidate={handleViewKnowledgeCandidate}
                onViewEntry={handleViewKnowledgeEntry}
                onApproveCandidate={handleApproveCandidate}
                onSubmitEntry={handleSubmitKnowledgeEntry}
                onPublishEntry={handlePublishKnowledgeEntry}
                onRejectCandidate={handleRejectCandidate}
              />
            ) : null}
          </div>
        </section>
      )}
      <KnowledgePreviewDialog
        key={knowledgePreview?.type === "candidate"
          ? `candidate-${knowledgePreview.candidate.id}-${knowledgePreview.candidate.candidate_score}`
          : knowledgePreview?.type === "entry"
            ? `entry-${knowledgePreview.entry.id}`
            : "knowledge-preview-closed"}
        preview={knowledgePreview}
        entryDetail={previewEntryDetail}
        loading={previewEntryLoading}
        error={previewEntryError}
        actionCandidateId={actionCandidateId}
        availableEntries={knowledgeEntries}
        onOpenChange={handleKnowledgePreviewOpenChange}
        onApprove={handleApproveCandidate}
        onEnrich={handleEnrichCandidate}
        onMerge={handleMergeCandidate}
        onDetachDuplicate={handleDetachDuplicateCandidate}
      />
    </PageShell>
  )
}

export default function EnterpriseKnowledgePage() {
  return (
    <EnterpriseAIFeatureGuard title="Knowledge">
      <EnterpriseKnowledgePageContent />
    </EnterpriseAIFeatureGuard>
  )
}
