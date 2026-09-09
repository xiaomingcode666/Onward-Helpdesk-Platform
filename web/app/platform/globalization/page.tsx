"use client"

import { type FormEvent, useCallback, useEffect, useMemo, useState } from "react"
import {
  Building2Icon,
  ChevronDownIcon,
  DatabaseIcon,
  Globe2Icon,
  LanguagesIcon,
  PencilIcon,
  PlusIcon,
  RefreshCwIcon,
} from "lucide-react"
import { toast } from "sonner"
import { Checkbox as AntCheckbox, Input } from "antd"
import type { TableColumnsType } from "antd"

import {
  ContentModule,
  DataTable,
  IconButton,
  PageShell,
  RailopsButton,
  StandardModal,
  StatCard,
  StatusTag,
  type StatCardTone,
  type StatusTagTone,
} from "@railops/ui"

import { useAuth } from "@/components/auth-provider"
import { CanUseButton } from "@/components/layout/permission-guard"
import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { OptionCombobox, type ComboboxOption } from "@/components/option-combobox"
import {
  PlatformEmpty,
  PlatformError,
  formatNumber,
} from "@/components/platform/platform-live-ui"
import { ModuleLoading } from "@/components/shared/loading-states"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { useAppLocale, useI18n } from "@/i18n/provider"
import {
  fetchPlatformGlobalization,
  fetchPlatformTenants,
  updatePlatformTenant,
  type PlatformGlobalizationResponse,
  type PlatformTenantItem,
} from "@/lib/api/platform"

const DEFAULT_CONFIG_FORM = {
  tenantId: "",
  countryRegion: "",
  dataRegion: "",
  languagePreferences: ["en-US"],
  timezonePreferences: ["UTC"],
}

const COMMON_TIMEZONES = [
  "UTC",
  "Asia/Shanghai",
  "Asia/Singapore",
  "Asia/Tokyo",
  "Asia/Dubai",
  "Europe/Berlin",
  "Europe/London",
  "America/New_York",
  "America/Chicago",
  "America/Denver",
  "America/Los_Angeles",
  "America/Sao_Paulo",
  "Australia/Sydney",
]

function tenantConfigComplete(tenant: PlatformTenantItem) {
  return Boolean(
    tenant.countryRegion &&
    tenant.dataRegion &&
    tenant.defaultLocale &&
    tenant.timezone &&
    (tenant.supportedLocales?.length ?? 0) > 0 &&
    (tenant.supportedTimezones?.length ?? 0) > 0
  )
}

function formForTenant(tenant?: PlatformTenantItem | null) {
  if (!tenant) return DEFAULT_CONFIG_FORM
  return {
    tenantId: String(tenant.id),
    countryRegion: tenant.countryRegion || "",
    dataRegion: tenant.dataRegion || "",
    languagePreferences: tenant.supportedLocales?.length ? tenant.supportedLocales : [tenant.defaultLocale || "en-US"],
    timezonePreferences: tenant.supportedTimezones?.length ? tenant.supportedTimezones : [tenant.timezone || "UTC"],
  }
}

type PlatformMetricTone = "blue" | "success" | "amber" | "red" | "slate"

const metricToneMap: Record<PlatformMetricTone, StatCardTone> = {
  blue: "blue",
  success: "success",
  amber: "warning",
  red: "error",
  slate: "neutral",
}

const statusTagToneMap: Record<"neutral" | "info" | "success" | "warning" | "danger", StatusTagTone> = {
  neutral: "neutral",
  info: "blue",
  success: "success",
  warning: "warning",
  danger: "error",
}

function preferenceSummary(
  moreSummary: (count: number) => string,
  options: ComboboxOption[],
  values: string[],
  placeholder: string,
) {
  if (!values.length) return placeholder
  const labels = values.map((value) => options.find((option) => option.value === value)?.label ?? value)
  return labels.length <= 2 ? labels.join("、") : `${labels[0]} ${moreSummary(labels.length)}`
}

function PreferenceMultiSelect({
  options,
  values,
  placeholder,
  moreSummary,
  disabled,
  onChange,
}: {
  options: ComboboxOption[]
  values: string[]
  placeholder: string
  moreSummary: (count: number) => string
  disabled: boolean
  onChange: (values: string[]) => void
}) {
  return (
    <Popover>
      <PopoverTrigger
        render={
          <RailopsButton className="w-full justify-between px-2.5 font-normal" disabled={disabled} />
        }
      >
        <span className="min-w-0 truncate text-left">{preferenceSummary(moreSummary, options, values, placeholder)}</span>
        <ChevronDownIcon className="size-4 shrink-0 text-muted-foreground" />
      </PopoverTrigger>
      <PopoverContent align="start" className="w-(--anchor-width) min-w-72 p-1">
        <div className="max-h-64 overflow-y-auto">
          {options.map((option) => {
            const checked = values.includes(option.value)
            return (
              <button
                key={option.value}
                type="button"
                className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm hover:bg-muted"
                onClick={() => onChange(checked ? values.filter((value) => value !== option.value) : [...values, option.value])}
              >
                <AntCheckbox checked={checked} className="pointer-events-none" />
                <span className="min-w-0 flex-1 truncate">{option.label}</span>
                <span className="shrink-0 font-mono text-xs text-muted-foreground">{option.value}</span>
              </button>
            )
          })}
        </div>
      </PopoverContent>
    </Popover>
  )
}

export default function PlatformGlobalizationPage() {
  const { session } = useAuth()
  const { locale } = useAppLocale()
  const t = useI18n()
  const [data, setData] = useState<PlatformGlobalizationResponse | null>(null)
  const [tenantCatalog, setTenantCatalog] = useState<PlatformTenantItem[]>([])
  const [globalizationLoading, setGlobalizationLoading] = useState(true)
  const [tenantCatalogLoading, setTenantCatalogLoading] = useState(true)
  const [hasLoadedGlobalization, setHasLoadedGlobalization] = useState(false)
  const [hasLoadedTenantCatalog, setHasLoadedTenantCatalog] = useState(false)
  const [error, setError] = useState("")
  const [tenantCatalogError, setTenantCatalogError] = useState("")
  const [page, setPage] = useState(1)
  const [configOpen, setConfigOpen] = useState(false)
  const [editingTenant, setEditingTenant] = useState<PlatformTenantItem | null>(null)
  const [configForm, setConfigForm] = useState(DEFAULT_CONFIG_FORM)
  const [formError, setFormError] = useState("")
  const [saving, setSaving] = useState(false)
  const pageSize = 20
  const canUpdateTenant = CanUseButton("tenant.update", session?.permissions)
  const globalizationNamespace = locale === "zh-CN" ? "platformExtract.globalization" : "platformGlobalization"
  const commonNamespace = locale === "zh-CN" ? "platformExtract.common" : "common"
  const globalizationT = useCallback((key: string, values?: Record<string, string | number>) => t(`${globalizationNamespace}.${key}`, values), [globalizationNamespace, t])
  const commonT = useCallback((key: string, values?: Record<string, string | number>) => t(`${commonNamespace}.${key}`, values), [commonNamespace, t])
  const localeLabels = useMemo<Record<string, string>>(() => ({
    "en-US": "English (United States) · en-US",
    "zh-CN": locale === "zh-CN" ? `${globalizationT("locale.zhCN")} · zh-CN` : "Simplified Chinese · zh-CN",
    "de-DE": "Deutsch · de-DE",
    "fr-FR": "Français · fr-FR",
    "es-ES": "Español · es-ES",
    "pt-BR": "Português (Brasil) · pt-BR",
    "ja-JP": locale === "zh-CN" ? `${globalizationT("locale.jaJP")} · ja-JP` : "Japanese · ja-JP",
    "ko-KR": "한국어 · ko-KR",
    "ar-SA": "العربية · ar-SA",
  }), [globalizationT, locale])

  const loadGlobalization = useCallback(async () => {
    setGlobalizationLoading(true)
    setError("")
    try {
      const globalization = await fetchPlatformGlobalization({ page, pageSize })
      setData(globalization)
      setHasLoadedGlobalization(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : globalizationT("loadFailed"))
    } finally {
      setGlobalizationLoading(false)
    }
  }, [globalizationT, page])

  const loadTenantCatalog = useCallback(async () => {
    setTenantCatalogLoading(true)
    setTenantCatalogError("")
    try {
      const tenants = await fetchPlatformTenants({ page: 1, limit: 200, lifecycle: "all" })
      setTenantCatalog(tenants.tenants)
      setHasLoadedTenantCatalog(true)
    } catch (err) {
      setTenantCatalogError(err instanceof Error ? err.message : globalizationT("loadFailed"))
    } finally {
      setTenantCatalogLoading(false)
    }
  }, [globalizationT])

  const load = useCallback(async () => {
    await Promise.all([loadGlobalization(), loadTenantCatalog()])
  }, [loadGlobalization, loadTenantCatalog])

  useEffect(() => {
    void loadGlobalization()
  }, [loadGlobalization])

  useEffect(() => {
    void loadTenantCatalog()
  }, [loadTenantCatalog])

  const tenantOptions = useMemo<ComboboxOption[]>(
    () => tenantCatalog.map((tenant) => ({ value: String(tenant.id), label: `${tenant.name} · ${tenant.code}` })),
    [tenantCatalog]
  )
  const localeOptions = useMemo(() => {
    const known = new Map(Object.entries(localeLabels).map(([value, label]) => [value, { value, label } as ComboboxOption]))
    data?.regions.flatMap((region) => region.locales).forEach((locale) => {
      if (locale && !known.has(locale)) known.set(locale, { value: locale, label: locale })
    })
    return Array.from(known.values())
  }, [data, localeLabels])
  const timezoneOptions = useMemo(() => {
    const values = new Set(COMMON_TIMEZONES)
    data?.regions.flatMap((region) => region.timezones).forEach((timezone) => timezone && values.add(timezone))
    return Array.from(values).map((timezone) => ({ value: timezone, label: timezone }))
  }, [data])
  const dataRegionSuggestions = useMemo(
    () => Array.from(new Set(data?.regions.map((region) => region.dataRegion).filter(Boolean) ?? [])),
    [data]
  )

  function openCreateConfig() {
    const tenant = tenantCatalog.find((item) => !tenantConfigComplete(item)) ?? tenantCatalog[0]
    setEditingTenant(null)
    setConfigForm(formForTenant(tenant))
    setFormError("")
    setConfigOpen(true)
  }

  function openEditConfig(tenant: PlatformTenantItem) {
    setEditingTenant(tenant)
    setConfigForm(formForTenant(tenant))
    setFormError("")
    setConfigOpen(true)
  }

  function selectTenant(tenantId: string) {
    setConfigForm(formForTenant(tenantCatalog.find((tenant) => String(tenant.id) === tenantId)))
  }

  async function saveConfig(event: FormEvent) {
    event.preventDefault()
    const tenantId = Number(configForm.tenantId)
    if (!tenantId || !configForm.countryRegion.trim() || !configForm.dataRegion.trim() || !configForm.languagePreferences.length || !configForm.timezonePreferences.length) {
      setFormError(globalizationT("formError"))
      return
    }
    setSaving(true)
    setFormError("")
    try {
      await updatePlatformTenant({
        id: tenantId,
        countryRegion: configForm.countryRegion.trim(),
        dataRegion: configForm.dataRegion.trim(),
        defaultLocale: configForm.languagePreferences[0],
        supportedLocales: configForm.languagePreferences,
        timezone: configForm.timezonePreferences[0],
        supportedTimezones: configForm.timezonePreferences,
      })
      toast.success(globalizationT("saveSuccess"))
      setConfigOpen(false)
      await load()
    } catch (err) {
      const message = err instanceof Error ? err.message : globalizationT("saveFailed")
      setFormError(message)
      toast.error(message)
    } finally {
      setSaving(false)
    }
  }

  const metricsInitialLoading = globalizationLoading && !hasLoadedGlobalization
  const regionsInitialLoading = globalizationLoading && !hasLoadedGlobalization
  const tenantDetailsInitialLoading = globalizationLoading && !hasLoadedGlobalization
  const isRefreshing = globalizationLoading || tenantCatalogLoading
  const hasData = data !== null

  const regionColumns: TableColumnsType<PlatformGlobalizationResponse["regions"][number]> = [
    {
      title: globalizationT("regionHeaders.dataRegion"),
      dataIndex: "dataRegion",
      key: "dataRegion",
      width: 140,
      render: (_value, region) => <span className="font-medium text-foreground">{region.dataRegion || globalizationT("regionMissing")}</span>,
    },
    {
      title: globalizationT("regionHeaders.tenantCount"),
      dataIndex: "tenantCount",
      key: "tenantCount",
      width: 90,
      render: (_value, region) => <span className="tabular-nums">{formatNumber(region.tenantCount)}</span>,
    },
    {
      title: globalizationT("regionHeaders.countryRegion"),
      key: "countryRegion",
      render: (_value, region) => <span className="text-muted-foreground">{region.countryRegions.join("、") || "-"}</span>,
    },
    {
      title: globalizationT("regionHeaders.locale"),
      key: "locale",
      render: (_value, region) => <span className="text-muted-foreground">{region.locales.join("、") || "-"}</span>,
    },
    {
      title: globalizationT("regionHeaders.timezone"),
      key: "timezone",
      render: (_value, region) => <span className="text-muted-foreground">{region.timezones.join("、") || "-"}</span>,
    },
    {
      title: globalizationT("regionHeaders.status"),
      key: "status",
      width: 110,
      render: (_value, region) => (
        <StatusTag tone={!region.dataRegion ? "warning" : "success"} className="rhd-railops-platform-status">
          {!region.dataRegion ? globalizationT("pendingSupplement") : globalizationT("configured")}
        </StatusTag>
      ),
    },
  ]

  const tenantColumns: TableColumnsType<PlatformTenantItem> = [
    {
      title: globalizationT("tenantHeaders.tenant"),
      key: "tenant",
      width: 260,
      render: (_value, tenant) => (
        <div>
          <div className="max-w-[260px] truncate font-medium text-foreground">{tenant.name}</div>
          <div className="mt-0.5 text-xs text-muted-foreground">{tenant.code}</div>
        </div>
      ),
    },
    {
      title: globalizationT("tenantHeaders.countryRegion"),
      key: "countryRegion",
      render: (_value, tenant) => <span className="text-muted-foreground">{tenant.countryRegion || globalizationT("unconfigured")}</span>,
    },
    {
      title: globalizationT("tenantHeaders.dataRegion"),
      key: "dataRegion",
      render: (_value, tenant) => <span className="text-muted-foreground">{tenant.dataRegion || globalizationT("unconfigured")}</span>,
    },
    {
      title: globalizationT("tenantHeaders.locale"),
      key: "locale",
      render: (_value, tenant) => <span className="text-muted-foreground">{tenant.supportedLocales?.join("、") || tenant.defaultLocale || globalizationT("unconfigured")}</span>,
    },
    {
      title: globalizationT("tenantHeaders.timezone"),
      key: "timezone",
      render: (_value, tenant) => <span className="text-muted-foreground">{tenant.supportedTimezones?.join("、") || tenant.timezone || globalizationT("unconfigured")}</span>,
    },
    {
      title: globalizationT("tenantHeaders.status"),
      key: "status",
      width: 110,
      render: (_value, tenant) => {
        const complete = tenantConfigComplete(tenant)
        return <StatusTag tone={complete ? "success" : "warning"} className="rhd-railops-platform-status">{complete ? globalizationT("completeConfig") : globalizationT("pendingSupplement")}</StatusTag>
      },
    },
    {
      title: globalizationT("tenantHeaders.actions"),
      key: "actions",
      width: 64,
      align: "right",
      render: (_value, tenant) => (
        <IconButton
          icon={<PencilIcon />}
          tooltip={globalizationT("editConfig")}
          aria-label={globalizationT("editTenantAria", { tenant: tenant.name })}
          disabled={!canUpdateTenant}
          onClick={() => openEditConfig(tenant)}
        />
      ),
    },
  ]

  return (
    <PageShell
      title={globalizationT("pageTitle")}
      breadcrumb={useRouteBreadcrumbItems()}
      className="min-h-[calc(100vh-97px)]"
      actions={
        <div className="flex items-center gap-2">
          <RailopsButton onClick={() => void load()} disabled={isRefreshing}>
            <RefreshCwIcon className={isRefreshing ? "animate-spin" : ""} data-icon="inline-start" />
            {globalizationT("refresh")}
          </RailopsButton>
          {canUpdateTenant ? (
            <RailopsButton variant="primary" onClick={openCreateConfig} disabled={tenantCatalogLoading || tenantCatalog.length === 0}>
              <PlusIcon data-icon="inline-start" />
              {globalizationT("addRegionConfig")}
            </RailopsButton>
          ) : null}
        </div>
      }
    >

      <div className="space-y-4">
        {!globalizationLoading && error && !hasData ? <PlatformError message={error} onRetry={() => void loadGlobalization()} /> : null}
        {error && hasData ? <div className="border border-border bg-muted px-3 py-2 text-sm text-muted-foreground">{error}</div> : null}
        {tenantCatalogError ? (
          <div className="border border-border bg-muted px-3 py-2 text-sm text-muted-foreground">
            {hasLoadedTenantCatalog ? tenantCatalogError : `${globalizationT("selectTenant")}${globalizationT("temporaryUnavailable")}：${tenantCatalogError}`}
          </div>
        ) : null}

        {metricsInitialLoading ? (
          <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4" aria-label={globalizationT("pageTitle")}>
            <StatCard label={globalizationT("tenantCountLabel")} value="-" note={commonT("loadingData")} icon={<Building2Icon className="size-4" />} tone="blue" className="rhd-railops-platform-metric" />
            <StatCard label={globalizationT("regionCountLabel")} value="-" note={commonT("loadingData")} icon={<DatabaseIcon className="size-4" />} tone="blue" className="rhd-railops-platform-metric" />
            <StatCard label={globalizationT("localeCountLabel")} value="-" note={commonT("loadingData")} icon={<LanguagesIcon className="size-4" />} tone="neutral" className="rhd-railops-platform-metric" />
            <StatCard label={globalizationT("timezoneCountLabel")} value="-" note={commonT("loadingData")} icon={<Globe2Icon className="size-4" />} tone="neutral" className="rhd-railops-platform-metric" />
          </section>
        ) : data ? (
          <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4" aria-label={globalizationT("pageTitle")}>
            <StatCard
              label={globalizationT("tenantCountLabel")}
              value={formatNumber(data.totalCount)}
              note={<StatusTag tone="blue" className="rhd-railops-platform-metric-detail">{globalizationT("tenantCountDetail", { count: formatNumber(data.summary.active) })}</StatusTag>}
              icon={<Building2Icon className="size-4" />}
              tone="blue"
              className="rhd-railops-platform-metric"
            />
            <StatCard
              label={globalizationT("regionCountLabel")}
              value={formatNumber(data.configuredRegionCount)}
              note={<StatusTag tone={data.regions.some((region) => !region.dataRegion) ? "warning" : "blue"} className="rhd-railops-platform-metric-detail">{globalizationT("regionCountDetail", { count: formatNumber(data.regions.find((region) => !region.dataRegion)?.tenantCount ?? 0) })}</StatusTag>}
              icon={<DatabaseIcon className="size-4" />}
              tone={data.regions.some((region) => !region.dataRegion) ? "warning" : "blue"}
              className="rhd-railops-platform-metric"
            />
            <StatCard
              label={globalizationT("localeCountLabel")}
              value={formatNumber(data.localeCount)}
              note={<StatusTag tone="neutral" className="rhd-railops-platform-metric-detail">{globalizationT("localeCountDetail")}</StatusTag>}
              icon={<LanguagesIcon className="size-4" />}
              tone="neutral"
              className="rhd-railops-platform-metric"
            />
            <StatCard
              label={globalizationT("timezoneCountLabel")}
              value={formatNumber(data.timezoneCount)}
              note={<StatusTag tone="neutral" className="rhd-railops-platform-metric-detail">{globalizationT("timezoneCountDetail")}</StatusTag>}
              icon={<Globe2Icon className="size-4" />}
              tone="neutral"
              className="rhd-railops-platform-metric"
            />
          </section>
        ) : null}

        <ContentModule title={globalizationT("regionsTitle")} className="rhd-railops-platform-panel">
          {regionsInitialLoading ? (
            <div className="p-4">
              <ModuleLoading count={5} label={commonT("loadingData")} variant="table" />
            </div>
          ) : data?.regions.length ? (
              <DataTable<PlatformGlobalizationResponse["regions"][number]>
                className="rhd-railops-platform-globalization-regions-table"
                columns={regionColumns}
                dataSource={data.regions}
                emptyDescription={globalizationT("emptyRegionTitle")}
                loading={globalizationLoading && hasLoadedGlobalization}
                rowKey={(region) => region.dataRegion || "__unconfigured__"}
                size="small"
                scroll={{ x: 760 }}
              />
          ) : (
            <PlatformEmpty title={globalizationT("emptyRegionTitle")} />
          )}
        </ContentModule>

        <ContentModule title={globalizationT("tenantDetailsTitle")} className="rhd-railops-platform-panel">
          {tenantDetailsInitialLoading ? (
            <div className="p-4">
              <ModuleLoading count={6} label={commonT("loadingData")} variant="table" />
            </div>
          ) : data ? (
              <DataTable<PlatformTenantItem>
                className="rhd-railops-platform-globalization-tenants-table"
                columns={tenantColumns}
                current={data.page || 1}
                dataSource={data.tenants}
                emptyDescription={globalizationT("emptyTenant")}
                footerNote={globalizationT("paginationSummary", { page: data.page || 1, totalPages: Math.max(1, data.totalPages), total: formatNumber(data.totalCount) })}
                loading={globalizationLoading && hasLoadedGlobalization}
                pageSize={pageSize}
                rowKey="id"
                size="small"
                scroll={{ x: 980 }}
                total={data.totalCount}
                onPageChange={(nextPage) => setPage(nextPage)}
              />
          ) : null}
        </ContentModule>
      </div>

      <StandardModal
        open={configOpen}
        onCancel={() => { if (!saving) setConfigOpen(false) }}
        title={editingTenant ? globalizationT("editConfigTitle") : globalizationT("createConfigTitle")}
        width={576}
        footer={
          <>
            <RailopsButton onClick={() => setConfigOpen(false)} disabled={saving}>{globalizationT("cancel")}</RailopsButton>
            <RailopsButton variant="primary" htmlType="submit" form="tenant-globalization-form" disabled={saving}>
              {saving ? globalizationT("saving") : globalizationT("save")}
            </RailopsButton>
          </>
        }
      >
        <form id="tenant-globalization-form" className="grid gap-4 sm:grid-cols-2" onSubmit={saveConfig}>
            <div className="grid gap-2 sm:col-span-2">
              <label className="text-sm font-medium">{globalizationT("tenantLabel")}</label>
              <OptionCombobox
                value={configForm.tenantId}
                options={tenantOptions}
                placeholder={globalizationT("selectTenant")}
                searchPlaceholder={globalizationT("searchTenant")}
                emptyText={globalizationT("emptyTenant")}
                disabled={saving || Boolean(editingTenant)}
                onChange={selectTenant}
              />
            </div>
            <div className="grid gap-2">
              <label htmlFor="globalization-country-region" className="text-sm font-medium">{globalizationT("countryRegionLabel")}</label>
              <Input
                id="globalization-country-region"
                value={configForm.countryRegion}
                placeholder={globalizationT("countryRegionPlaceholder")}
                disabled={saving}
                onChange={(event) => setConfigForm((current) => ({ ...current, countryRegion: event.target.value }))}
              />
            </div>
            <div className="grid gap-2">
              <label htmlFor="globalization-data-region" className="text-sm font-medium">{globalizationT("dataRegionLabel")}</label>
              <Input
                id="globalization-data-region"
                list="globalization-data-region-options"
                value={configForm.dataRegion}
                placeholder={globalizationT("dataRegionPlaceholder")}
                disabled={saving}
                onChange={(event) => setConfigForm((current) => ({ ...current, dataRegion: event.target.value }))}
              />
              <datalist id="globalization-data-region-options">
                {dataRegionSuggestions.map((region) => <option key={region} value={region} />)}
              </datalist>
            </div>
            <div className="grid gap-2">
              <label className="text-sm font-medium">{globalizationT("languageLabel")}</label>
              <PreferenceMultiSelect
                options={localeOptions}
                values={configForm.languagePreferences}
                placeholder={globalizationT("languagePlaceholder")}
                moreSummary={(count) => globalizationT("moreSummary", { count })}
                disabled={saving}
                onChange={(languagePreferences) => setConfigForm((current) => ({ ...current, languagePreferences }))}
              />
            </div>
            <div className="grid gap-2">
              <label className="text-sm font-medium">{globalizationT("timezoneLabel")}</label>
              <PreferenceMultiSelect
                options={timezoneOptions}
                values={configForm.timezonePreferences}
                placeholder={globalizationT("timezonePlaceholder")}
                moreSummary={(count) => globalizationT("moreSummary", { count })}
                disabled={saving}
                onChange={(timezonePreferences) => setConfigForm((current) => ({ ...current, timezonePreferences }))}
              />
            </div>
            {formError ? <div className="border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive sm:col-span-2">{formError}</div> : null}
          </form>
      </StandardModal>
    </PageShell>
  )
}
