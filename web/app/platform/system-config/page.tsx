"use client"

import { FormEvent, useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react"
import {
  BookOpenIcon,
  CableIcon,
  ChevronRightIcon,
  GaugeIcon,
  FileUpIcon,
  KeyRoundIcon,
  PencilIcon,
  RefreshCwIcon,
  SaveIcon,
  Settings2Icon,
  Trash2Icon,
  UploadCloudIcon,
} from "lucide-react"

import {
  ContentModule,
  DataTable,
  DEFAULT_PAGE_SIZE,
  IconButton,
  PageShell,
  RailopsButton,
  SearchField,
  StandardModal,
  StatusTag,
  type StatusTagTone,
} from "@railops/ui"
import { Input } from "antd"
import type { TableColumnsType } from "antd"

import { useAuth } from "@/components/auth-provider"
import { CanUseButton } from "@/components/layout/permission-guard"
import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { useConfirm } from "@/components/confirm-provider"
import { ModuleLoading } from "@/components/shared/loading-states"
import { PlatformError, formatDateTime } from "@/components/platform/platform-live-ui"
import { useAppLocale, useI18n } from "@/i18n/provider"
import { formatFileSize } from "@/lib/im-message"
import { toast } from "sonner"

import {
  deletePlatformSystemIntroDoc,
  fetchPlatformSystemIntroDocs,
  updatePlatformSystemIntroDoc,
  uploadPlatformSystemIntroDoc,
  type PlatformSystemIntroDoc,
} from "@/lib/api/platform-system-intro"
import {
  fetchPlatformIntegrationSettings,
  fetchPlatformSystemPolicy,
  updatePlatformSystemPolicy,
  type PlatformSystemPolicySettings,
  type PlatformIntegrationSettings,
} from "@/lib/api/platform"
import { PlatformIntegrationConfigDialog } from "@/components/platform/platform-integration-config-dialog"

const ACCEPTED_FILE_TYPES = ".pdf,.png,.jpg,.jpeg,.gif,.webp,.mp4,.webm,.mov,.txt,.doc,.docx"

/**
 * System configuration is the platform-level configuration shell, shown in sub-sections.
 *
 * Sections:
 *  - system-intro: system intro documents (upload / edit / publish / unpublish / delete)
 *  - integrations: shared platform service connections
 *  - tenant-quota: free / paid tenant knowledge-document defaults
 *  - upload-limit: default knowledge-document single-file limit
 */
type SystemConfigSectionKey = "system-intro" | "integrations" | "tenant-quota" | "upload-limit"

type SystemPolicyForm = {
  freeTenantDocumentLimit: string
  paidTenantDocumentLimit: string
  knowledgeDocumentMaxMB: string
}

export default function PlatformSystemConfigPage() {
  const { locale } = useAppLocale()
  const t = useI18n()
  const { session } = useAuth()
  const confirm = useConfirm()
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const introNamespace = locale === "zh-CN" ? "platformExtract.systemIntroPage" : "systemIntroPage"
  const commonNamespace = locale === "zh-CN" ? "platformExtract.common" : "common"
  const introT = useCallback((key: string, values?: Record<string, string | number>) => t(`${introNamespace}.${key}`, values), [introNamespace, t])
  const commonT = useCallback((key: string, values?: Record<string, string | number>) => t(`${commonNamespace}.${key}`, values), [commonNamespace, t])

  const [sectionKey, setSectionKey] = useState<SystemConfigSectionKey>("system-intro")

  const systemConfigSections = useMemo(
    () => [
      {
        key: "system-intro" as const,
        label: introT("tabLabel"),
        description: introT("systemIntroNavDescription"),
        icon: <BookOpenIcon className="size-4" />,
      },
      {
        key: "integrations" as const,
        label: t("platformExtract.access.externalServicesTitle"),
        description: introT("integrationsNavDescription"),
        icon: <CableIcon className="size-4" />,
      },
      {
        key: "tenant-quota" as const,
        label: introT("tenantQuotaLabel"),
        description: introT("tenantQuotaNavDescription"),
        icon: <GaugeIcon className="size-4" />,
      },
      {
        key: "upload-limit" as const,
        label: introT("uploadLimitLabel"),
        description: introT("uploadLimitNavDescription"),
        icon: <FileUpIcon className="size-4" />,
      },
    ],
    [introT, t],
  )

  const activeSection = systemConfigSections.find((item) => item.key === sectionKey) ?? systemConfigSections[0]!
  const isPolicySection = sectionKey === "tenant-quota" || sectionKey === "upload-limit"

  const [items, setItems] = useState<PlatformSystemIntroDoc[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [searchInput, setSearchInput] = useState("")
  const [search, setSearch] = useState("")
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")
  const [uploading, setUploading] = useState(false)

  const [editing, setEditing] = useState<PlatformSystemIntroDoc | null>(null)
  const [editTitle, setEditTitle] = useState("")
  const [editSortNo, setEditSortNo] = useState("0")
  const [editSaving, setEditSaving] = useState(false)
  const [actingId, setActingId] = useState<number | null>(null)
  const [integrationSettings, setIntegrationSettings] = useState<PlatformIntegrationSettings | null>(null)
  const [integrationLoading, setIntegrationLoading] = useState(false)
  const [integrationLoaded, setIntegrationLoaded] = useState(false)
  const [integrationError, setIntegrationError] = useState("")
  const [sub2APIConfigOpen, setSub2APIConfigOpen] = useState(false)
  const [systemPolicy, setSystemPolicy] = useState<PlatformSystemPolicySettings | null>(null)
  const [systemPolicyLoading, setSystemPolicyLoading] = useState(false)
  const [systemPolicyLoaded, setSystemPolicyLoaded] = useState(false)
  const [systemPolicyError, setSystemPolicyError] = useState("")
  const [systemPolicySaving, setSystemPolicySaving] = useState(false)
  const [systemPolicyForm, setSystemPolicyForm] = useState<SystemPolicyForm>({
    freeTenantDocumentLimit: "0",
    paidTenantDocumentLimit: "0",
    knowledgeDocumentMaxMB: "20",
  })

  const canCreate = CanUseButton("systemIntro.create", session?.permissions)
  const canUpdate = CanUseButton("systemIntro.update", session?.permissions)
  const canDelete = CanUseButton("systemIntro.delete", session?.permissions)
  const canUpdateSystemPolicy = Boolean(systemPolicy?.canUpdate) && CanUseButton("tenant.update", session?.permissions)

  const load = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const result = await fetchPlatformSystemIntroDocs({ page, limit: DEFAULT_PAGE_SIZE, keyword: search || undefined })
      setItems(result.items ?? [])
      setTotal(result.total ?? 0)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }, [page, search])

  useEffect(() => {
    void load()
  }, [load])

  const loadIntegrationSettings = useCallback(async () => {
    setIntegrationLoading(true)
    setIntegrationError("")
    try {
      const result = await fetchPlatformIntegrationSettings()
      setIntegrationSettings(result)
    } catch (err) {
      setIntegrationError(err instanceof Error ? err.message : String(err))
    } finally {
      setIntegrationLoaded(true)
      setIntegrationLoading(false)
    }
  }, [])

  useEffect(() => {
    if (sectionKey !== "integrations" || integrationLoaded) return
    void loadIntegrationSettings()
  }, [integrationLoaded, loadIntegrationSettings, sectionKey])

  const loadSystemPolicy = useCallback(async () => {
    setSystemPolicyLoading(true)
    setSystemPolicyError("")
    try {
      const result = await fetchPlatformSystemPolicy()
      setSystemPolicy(result)
      setSystemPolicyForm({
        freeTenantDocumentLimit: String(result.tenantQuota.freeTenantDocumentLimit ?? 0),
        paidTenantDocumentLimit: String(result.tenantQuota.paidTenantDocumentLimit ?? 0),
        knowledgeDocumentMaxMB: String(result.uploadLimit.knowledgeDocumentMaxMB ?? 0),
      })
    } catch (err) {
      setSystemPolicyError(err instanceof Error ? err.message : String(err))
    } finally {
      setSystemPolicyLoaded(true)
      setSystemPolicyLoading(false)
    }
  }, [])

  useEffect(() => {
    if ((sectionKey !== "tenant-quota" && sectionKey !== "upload-limit") || systemPolicyLoaded) return
    void loadSystemPolicy()
  }, [loadSystemPolicy, sectionKey, systemPolicyLoaded])

  useEffect(() => {
    const timer = setTimeout(() => {
      setPage(1)
      setSearch(searchInput.trim())
    }, 300)
    return () => clearTimeout(timer)
  }, [searchInput])

  const pageCount = useMemo(() => Math.max(1, Math.ceil(total / DEFAULT_PAGE_SIZE)), [total])

  const saveSystemPolicy = async () => {
    const freeTenantDocumentLimit = Number(systemPolicyForm.freeTenantDocumentLimit)
    const paidTenantDocumentLimit = Number(systemPolicyForm.paidTenantDocumentLimit)
    const knowledgeDocumentMaxMB = Number(systemPolicyForm.knowledgeDocumentMaxMB)
    if (![freeTenantDocumentLimit, paidTenantDocumentLimit, knowledgeDocumentMaxMB].every((value) => Number.isInteger(value) && value >= 0)) {
      toast.error(introT("policyInvalidValue"))
      return
    }
    setSystemPolicySaving(true)
    try {
      const result = await updatePlatformSystemPolicy({ freeTenantDocumentLimit, paidTenantDocumentLimit, knowledgeDocumentMaxMB })
      setSystemPolicy(result)
      setSystemPolicyLoaded(true)
      toast.success(introT("policySaveSuccess"))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    } finally {
      setSystemPolicySaving(false)
    }
  }

  const openEdit = (item: PlatformSystemIntroDoc) => {
    setEditing(item)
    setEditTitle(item.title || "")
    setEditSortNo(String(item.sort_no ?? 0))
  }

  const saveEdit = async (event: FormEvent) => {
    event.preventDefault()
    if (!editing) return
    setEditSaving(true)
    try {
      await updatePlatformSystemIntroDoc({
        id: editing.id,
        title: editTitle.trim() || undefined,
        sort_no: Number(editSortNo) || 0,
      })
      toast.success(introT("editSuccess"))
      setEditing(null)
      void load()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    } finally {
      setEditSaving(false)
    }
  }

  const togglePublish = useCallback(async (item: PlatformSystemIntroDoc) => {
    const nextStatus = item.status === "published" ? "draft" : "published"
    const confirmed = await confirm({
      title: nextStatus === "published" ? introT("publish") : introT("unpublish"),
      description: nextStatus === "published"
        ? introT("publishConfirm")
        : introT("unpublishConfirm"),
      confirmText: nextStatus === "published" ? introT("publish") : introT("unpublish"),
    })
    if (!confirmed) return
    setActingId(item.id)
    try {
      await updatePlatformSystemIntroDoc({ id: item.id, status: nextStatus })
      toast.success(nextStatus === "published"
        ? introT("publishSuccess")
        : introT("unpublishSuccess"))
      void load()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    } finally {
      setActingId(null)
    }
  }, [confirm, introT, load])

  const removeItem = useCallback(async (item: PlatformSystemIntroDoc) => {
    const confirmed = await confirm({
      title: introT("deleteTitle"),
      description: introT("deleteConfirm"),
      confirmText: introT("delete"),
      variant: "destructive",
    })
    if (!confirmed) return
    setActingId(item.id)
    try {
      await deletePlatformSystemIntroDoc(item.id)
      toast.success(introT("deleteSuccess"))
      void load()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    } finally {
      setActingId(null)
    }
  }, [confirm, introT, load])

  const handleUpload = async (files: FileList | null) => {
    if (!files || files.length === 0) return
    setUploading(true)
    let succeeded = 0
    try {
      for (const file of Array.from(files)) {
        try {
          await uploadPlatformSystemIntroDoc(file)
          succeeded += 1
        } catch (err) {
          toast.error(`${file.name}：${err instanceof Error ? err.message : String(err)}`)
        }
      }
      if (succeeded > 0) toast.success(introT("uploadSuccess", { count: String(succeeded) }))
      void load()
    } finally {
      setUploading(false)
      if (fileInputRef.current) fileInputRef.current.value = ""
    }
  }

  const columns: TableColumnsType<PlatformSystemIntroDoc> = useMemo(() => [
    {
      title: introT("tableTitle"),
      key: "title",
      render: (_value, item) => (
        <>
          <div className="font-medium text-foreground">{item.title || item.filename}</div>
          <div className="mt-1 max-w-xl truncate text-rhd-2xs text-muted-foreground" title={item.filename}>
            {item.filename} · {formatFileSize(item.file_size)}
          </div>
        </>
      ),
    },
    {
      title: introT("sortNoLabel"),
      key: "sort_no",
      width: 90,
      render: (_value, item) => <span className="tabular-nums text-muted-foreground">{item.sort_no}</span>,
    },
    {
      title: introT("statusLabel"),
      key: "status",
      width: 110,
      render: (_value, item) => (
        <StatusTag tone={(item.status === "published" ? "success" : "neutral") as StatusTagTone}>
          {item.status === "published" ? introT("statusPublished") : introT("statusDraft")}
        </StatusTag>
      ),
    },
    {
      title: introT("publishTimeLabel"),
      key: "published_at",
      width: 150,
      render: (_value, item) => (
        <span className="whitespace-nowrap text-rhd-2xs text-muted-foreground">
          {item.published_at ? formatDateTime(item.published_at) : "-"}
        </span>
      ),
    },
    {
      title: introT("updatedAtLabel"),
      key: "updated_at",
      width: 150,
      render: (_value, item) => (
        <span className="whitespace-nowrap text-rhd-2xs text-muted-foreground">{formatDateTime(item.updated_at)}</span>
      ),
    },
    {
      title: introT("creatorLabel"),
      key: "create_user_name",
      width: 120,
      render: (_value, item) => (
        <span className="whitespace-nowrap text-rhd-2xs text-muted-foreground">{item.create_user_name || "-"}</span>
      ),
    },
    {
      title: introT("actionsLabel"),
      key: "actions",
      width: 210,
      fixed: "right",
      render: (_value, item) => (
        <div className="flex items-center gap-1">
          {canUpdate ? (
            <>
              <RailopsButton size="small" variant="text" onClick={() => openEdit(item)} disabled={actingId === item.id}>
                <PencilIcon data-icon="inline-start" />
                {introT("edit")}
              </RailopsButton>
              <RailopsButton size="small" variant="text" onClick={() => void togglePublish(item)} disabled={actingId === item.id}>
                {item.status === "published" ? introT("unpublish") : introT("publish")}
              </RailopsButton>
            </>
          ) : null}
          {canDelete ? (
            <RailopsButton size="small" variant="text" className="text-destructive" onClick={() => void removeItem(item)} disabled={actingId === item.id}>
              <Trash2Icon data-icon="inline-start" />
              {introT("delete")}
            </RailopsButton>
          ) : null}
        </div>
      ),
    },
  ], [introT, canUpdate, canDelete, actingId, removeItem, togglePublish])

  return (
    <PageShell
      title={introT("pageTitle")}
      description={introT("pageDescription")}
      breadcrumb={useRouteBreadcrumbItems()}
      className="rhd-railops-platform-system-config-page"
      actions={(
        <IconButton
          icon={<RefreshCwIcon className={(sectionKey === "integrations" ? integrationLoading : isPolicySection ? systemPolicyLoading : loading) ? "animate-spin" : ""} />}
          tooltip={commonT("refresh")}
          aria-label={commonT("refresh")}
          onClick={() => void (sectionKey === "integrations" ? loadIntegrationSettings() : isPolicySection ? loadSystemPolicy() : load())}
          disabled={sectionKey === "integrations" ? integrationLoading : isPolicySection ? systemPolicyLoading : loading}
        />
      )}
    >
      <div className="grid min-w-0 gap-4 lg:grid-cols-[248px_minmax(0,1fr)]">
        <aside className="flex min-w-0 flex-col self-start rounded-lg border border-border bg-card p-3 shadow-sm">
          <div className="flex items-start gap-3 border-b border-border px-2 pb-3">
            <span className="inline-flex size-8 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
              <Settings2Icon className="size-4" />
            </span>
            <div className="min-w-0">
              <div className="text-sm font-semibold text-foreground">{introT("navigationTitle")}</div>
              <div className="mt-0.5 text-xs text-muted-foreground">{introT("navigationHint")}</div>
            </div>
          </div>
          <nav className="mt-3 space-y-1" aria-label={introT("navigationTitle")}>
            {systemConfigSections.map((item) => {
              const isActive = item.key === sectionKey
              return (
                <button
                  key={item.key}
                  type="button"
                  aria-current={isActive ? "page" : undefined}
                  className={`group flex w-full items-center gap-3 rounded-md px-3 py-3 text-left transition-colors ${isActive ? "bg-primary/10 text-primary" : "text-muted-foreground hover:bg-muted/60 hover:text-foreground"}`}
                  onClick={() => setSectionKey(item.key)}
                >
                  <span className={`inline-flex size-8 shrink-0 items-center justify-center rounded-md ${isActive ? "bg-primary text-primary-foreground" : "bg-muted text-muted-foreground group-hover:bg-background"}`}>
                    {item.icon}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-medium">{item.label}</span>
                    <span className="mt-0.5 block truncate text-xs opacity-75">{item.description}</span>
                  </span>
                  <ChevronRightIcon className={`size-4 shrink-0 transition-transform ${isActive ? "translate-x-0 text-primary" : "-translate-x-1 opacity-0 group-hover:translate-x-0 group-hover:opacity-60"}`} />
                </button>
              )
            })}
          </nav>
          <div className="mt-4 border-t border-dashed border-border px-2 pt-3">
            <div className="text-xs font-medium text-muted-foreground">{introT("futureTitle")}</div>
            <p className="mt-1 text-xs leading-5 text-muted-foreground">{introT("futureHint")}</p>
          </div>
        </aside>

        <section className="min-w-0 space-y-4">
          <div className="flex items-start gap-3 border-b border-border pb-4">
            <span className="inline-flex size-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
              {activeSection.icon}
            </span>
            <div className="min-w-0">
              <div className="text-xs font-semibold text-primary">{introT("workspaceEyebrow")}</div>
              <h3 className="mt-1 text-xl font-semibold text-foreground">{activeSection.label}</h3>
              <p className="mt-1 max-w-2xl text-sm leading-6 text-muted-foreground">{introT(
                sectionKey === "system-intro"
                  ? "systemIntroDescription"
                  : sectionKey === "integrations"
                    ? "integrationsDescription"
                    : sectionKey === "tenant-quota"
                      ? "tenantQuotaDescription"
                      : "uploadLimitDescription",
              )}</p>
            </div>
          </div>

        {sectionKey === "system-intro" ? (
          <div className="space-y-4">
            {!loading && error ? <PlatformError message={error} onRetry={() => void load()} /> : null}

            <ContentModule
              title={introT("pageSubtitle")}
              note={introT("systemIntroHint")}
              extra={canCreate ? (
                <>
                  <input
                    ref={fileInputRef}
                    type="file"
                    multiple
                    accept={ACCEPTED_FILE_TYPES}
                    className="hidden"
                    onChange={(event) => void handleUpload(event.target.files)}
                  />
                  <RailopsButton variant="primary" size="small" onClick={() => fileInputRef.current?.click()} loading={uploading}>
                    <UploadCloudIcon data-icon="inline-start" />
                    {introT("uploadTitle")}
                  </RailopsButton>
                </>
              ) : null}
            >
              <div className="mb-3 flex flex-wrap items-center justify-between gap-3 border-b border-border pb-3">
                <span className="text-xs text-muted-foreground">{introT("systemIntroTableHint")}</span>
                <SearchField
                  allowClear
                  aria-label={introT("searchPlaceholder")}
                  className="w-full sm:w-64"
                  placeholder={introT("searchPlaceholder")}
                  value={searchInput}
                  onChange={(event) => setSearchInput(event.target.value)}
                />
              </div>

              {loading && items.length === 0 ? (
                <ModuleLoading count={4} label={introT("loading")} variant="table" />
              ) : (
                <DataTable<PlatformSystemIntroDoc>
                  columns={columns}
                  current={page}
                  dataSource={items}
                  emptyDescription={introT("emptyTitle")}
                  footerNote={introT("paginationSummary", { page: String(page), totalPages: String(pageCount), total: String(total) })}
                  rowKey="id"
                  scroll={{ x: 1120 }}
                  size="small"
                  total={total}
                  onPageChange={(nextPage) => setPage(nextPage)}
                />
              )}
            </ContentModule>
          </div>
        ) : null}

        {sectionKey === "integrations" ? (
          <IntegrationSettingsPanel
            settings={integrationSettings}
            loading={integrationLoading}
            error={integrationError}
            updatedAtLabel={introT("updatedAtLabel")}
            sectionHint={introT("integrationsHint")}
            onRetry={() => void loadIntegrationSettings()}
            onConfigure={() => setSub2APIConfigOpen(true)}
          />
        ) : null}
        {isPolicySection ? (
          <PlatformSystemPolicyPanel
            activeSection={sectionKey}
            settings={systemPolicy}
            loading={systemPolicyLoading}
            error={systemPolicyError}
            form={systemPolicyForm}
            canUpdate={canUpdateSystemPolicy}
            saving={systemPolicySaving}
            updatedAtLabel={introT("updatedAtLabel")}
            onChange={(field, value) => setSystemPolicyForm((current) => ({ ...current, [field]: value }))}
            onRetry={() => void loadSystemPolicy()}
            onSave={() => void saveSystemPolicy()}
          />
        ) : null}
        </section>
      </div>

      {integrationSettings && sub2APIConfigOpen ? (
        <PlatformIntegrationConfigDialog
          open
          kind="sub2api"
          settings={integrationSettings}
          onOpenChange={(open) => {
            if (!open) setSub2APIConfigOpen(false)
          }}
          onSaved={(next) => {
            setIntegrationSettings(next)
            setIntegrationLoaded(true)
          }}
        />
      ) : null}

      <StandardModal
        open={Boolean(editing)}
        onCancel={() => setEditing(null)}
        title={introT("editTitle")}
        width={480}
        footer={(
          <>
            <RailopsButton variant="text" onClick={() => setEditing(null)} disabled={editSaving}>{introT("cancel")}</RailopsButton>
            <RailopsButton variant="primary" form="platform-system-intro-edit-form" htmlType="submit" loading={editSaving}>{introT("save")}</RailopsButton>
          </>
        )}
      >
        <form id="platform-system-intro-edit-form" className="space-y-4" onSubmit={(event) => void saveEdit(event)}>
          <div className="space-y-1.5">
            <label className="text-sm font-medium text-foreground" htmlFor="system-intro-edit-title">{introT("titleLabel")}</label>
            <Input
              id="system-intro-edit-title"
              value={editTitle}
              onChange={(event) => setEditTitle(event.target.value)}
              placeholder={introT("titlePlaceholder")}
            />
          </div>
          <div className="space-y-1.5">
            <label className="text-sm font-medium text-foreground" htmlFor="system-intro-edit-sort">{introT("sortNoLabel")}</label>
            <Input
              id="system-intro-edit-sort"
              type="number"
              min={0}
              value={editSortNo}
              onChange={(event) => setEditSortNo(event.target.value)}
            />
          </div>
        </form>
      </StandardModal>
    </PageShell>
  )
}

function IntegrationSettingsPanel({
  settings,
  loading,
  error,
  updatedAtLabel,
  sectionHint,
  onRetry,
  onConfigure,
}: {
  settings: PlatformIntegrationSettings | null
  loading: boolean
  error: string
  updatedAtLabel: string
  sectionHint: string
  onRetry: () => void
  onConfigure: () => void
}) {
  const t = useI18n()

  if (loading && !settings) {
    return <ContentModule title={t("platformExtract.integration.dialog.sub2apiTitle")} note={sectionHint}><ModuleLoading count={3} label={t("platformExtract.common.loading")} variant="detail" /></ContentModule>
  }

  if (error && !settings) {
    return <PlatformError message={error} onRetry={onRetry} />
  }

  if (!settings) return null

  const translationConfigured = settings.sub2api.translationApiKeyConfigured
  const fingerprint = settings.sub2api.translationApiKeyFingerprint
  const fingerprintLabel = fingerprint
    ? `${fingerprint.slice(0, 12)}...${fingerprint.slice(-8)}`
    : t("platformExtract.integration.notGenerated")

  return (
    <div className="space-y-4">
      {error ? <PlatformError message={error} onRetry={onRetry} /> : null}
      <ContentModule
        title={
          <span className="inline-flex items-center gap-2">
            <span className="inline-flex size-7 items-center justify-center rounded bg-primary/10 text-primary">
              <KeyRoundIcon className="size-4" />
            </span>
            <span>{t("platformExtract.integration.dialog.sub2apiTitle")}</span>
          </span>
        }
        note={sectionHint}
        extra={
          <div className="flex flex-wrap items-center justify-end gap-2">
            <StatusTag tone={translationConfigured ? "success" : "neutral"}>
              {translationConfigured ? t("platformExtract.access.status.configured") : t("platformExtract.access.status.notConfigured")}
            </StatusTag>
            <RailopsButton size="small" onClick={onConfigure}>
              <Settings2Icon data-icon="inline-start" />
              {t("platformExtract.access.configure")}
            </RailopsButton>
          </div>
        }
      >
        <dl className="grid gap-x-8 md:grid-cols-2">
          <SummaryItem
            label={t("platformExtract.integration.field.gatewayUrl")}
            value={settings.sub2api.host || t("platformExtract.access.serviceUrlNotConfigured")}
          />
          <SummaryItem
            label={t("platformExtract.integration.field.currentTranslationApiKey")}
            value={settings.sub2api.translationApiKeyMasked || t("platformExtract.access.status.notConfigured")}
            code
          />
          <SummaryItem
            label={t("platformExtract.usage.sharedTranslation.key")}
            value={t("platformExtract.integration.fingerprint", { fingerprint: fingerprintLabel })}
            title={fingerprint || fingerprintLabel}
          />
          <SummaryItem
            label={updatedAtLabel}
            value={formatDateTime(settings.sub2api.updatedAt)}
          />
        </dl>
        <div className="mt-4 flex items-start gap-3 border-t border-border pt-3">
          <KeyRoundIcon className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
          <span className="text-rhd-xs leading-5 text-muted-foreground">
            {translationConfigured ? t("platformExtract.integration.credentialEncrypted") : t("platformExtract.usage.sharedTranslation.notConfiguredNote")}
          </span>
        </div>
      </ContentModule>
    </div>
  )
}

type SystemPolicyField = keyof SystemPolicyForm

function PlatformSystemPolicyPanel({
  activeSection,
  settings,
  loading,
  error,
  form,
  canUpdate,
  saving,
  updatedAtLabel,
  onChange,
  onRetry,
  onSave,
}: {
  activeSection: "tenant-quota" | "upload-limit"
  settings: PlatformSystemPolicySettings | null
  loading: boolean
  error: string
  form: SystemPolicyForm
  canUpdate: boolean
  saving: boolean
  updatedAtLabel: string
  onChange: (field: SystemPolicyField, value: string) => void
  onRetry: () => void
  onSave: () => void
}) {
  const t = useI18n()
  const isTenantQuota = activeSection === "tenant-quota"
  const title = isTenantQuota ? t("systemIntroPage.tenantQuotaTitle") : t("systemIntroPage.uploadLimitTitle")
  const note = isTenantQuota ? t("systemIntroPage.tenantQuotaHint") : t("systemIntroPage.uploadLimitHint")

  if (loading && !settings) {
    return <ContentModule title={title} note={note}><ModuleLoading count={isTenantQuota ? 2 : 1} label={t("platformExtract.common.loading")} variant="detail" /></ContentModule>
  }

  if (error && !settings) return <PlatformError message={error} onRetry={onRetry} />
  if (!settings) return null

  const field = (name: SystemPolicyField, label: string, description: string, suffix: string) => (
    <div className="grid items-start gap-4 py-4 first:pt-0 last:pb-0 sm:grid-cols-[minmax(240px,360px)_minmax(0,1fr)]">
      <div className="min-w-0">
        <label className="block text-sm font-medium text-foreground" htmlFor={`platform-policy-${name}`}>{label}</label>
        <p className="mt-1 text-xs leading-5 text-muted-foreground">{description}</p>
      </div>
      <Input
        id={`platform-policy-${name}`}
        className="w-full"
        type="number"
        min={0}
        step={1}
        suffix={suffix}
        value={form[name]}
        disabled={!canUpdate || loading || saving}
        onChange={(event) => onChange(name, event.target.value)}
      />
    </div>
  )

  return (
    <>
      {error ? <PlatformError message={error} onRetry={onRetry} /> : null}
      <ContentModule
        title={title}
        note={note}
        extra={(
          <div className="flex flex-wrap items-center justify-end gap-2">
            <StatusTag tone={settings.configured ? "success" : "neutral"}>
              {settings.configured ? t("platformExtract.access.status.configured") : t("platformExtract.access.status.notConfigured")}
            </StatusTag>
            {canUpdate ? (
              <RailopsButton variant="primary" size="small" loading={saving} disabled={loading} onClick={onSave}>
                <SaveIcon data-icon="inline-start" />
                {t("systemIntroPage.savePolicy")}
              </RailopsButton>
            ) : null}
          </div>
        )}
      >
        <div className="divide-y divide-border">
          {isTenantQuota ? (
            <>
              {field("freeTenantDocumentLimit", t("systemIntroPage.freeTenantDocumentLimit"), t("systemIntroPage.freeTenantDocumentLimitHint"), t("systemIntroPage.documentUnit"))}
              {field("paidTenantDocumentLimit", t("systemIntroPage.paidTenantDocumentLimit"), t("systemIntroPage.paidTenantDocumentLimitHint"), t("systemIntroPage.documentUnit"))}
            </>
          ) : (
            field("knowledgeDocumentMaxMB", t("systemIntroPage.knowledgeDocumentMaxMB"), t("systemIntroPage.knowledgeDocumentMaxMBHint"), "MB")
          )}
        </div>
        <div className="mt-4 flex items-start gap-3 border-t border-border pt-3">
          <Settings2Icon className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
          <div className="min-w-0 text-xs leading-5 text-muted-foreground">
            <div>{isTenantQuota ? t("systemIntroPage.quotaZeroHint") : t("systemIntroPage.uploadLimitZeroHint")}</div>
            {settings.updatedAt ? <div className="mt-1">{updatedAtLabel}: {settings.updatedAt}</div> : null}
          </div>
        </div>
      </ContentModule>
    </>
  )
}

function SummaryItem({
  label,
  value,
  title,
  code = false,
}: {
  label: string
  value: string
  title?: string
  code?: boolean
}) {
  const content: ReactNode = code ? <code>{value}</code> : value
  return (
    <div className="min-w-0 border-b border-border py-3.5 last:border-b-0">
      <dt className="text-rhd-xs text-muted-foreground">{label}</dt>
      <dd className="mt-1 truncate text-xs font-semibold text-foreground" title={title || value}>{content}</dd>
    </div>
  )
}
