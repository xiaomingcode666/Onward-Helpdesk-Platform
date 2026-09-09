"use client"

import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react"
import {
  Building2Icon,
  CheckIcon,
  ImageIcon,
  LockKeyholeIcon,
  LockKeyholeOpenIcon,
  KeyRoundIcon,
  LogInIcon,
  Maximize2Icon,
  MoreHorizontalIcon,
  PencilIcon,
  PlusIcon,
  RefreshCwIcon,
  Trash2Icon,
  UploadIcon,
  XIcon,
} from "lucide-react"

import {
  DataTable,
  DEFAULT_PAGE_SIZE,
  IconButton,
  PageShell,
  RailopsButton,
  SearchField,
  SelectField,
  StandardModal,
  StatusTag,
  UnderlineTabs,
  type StatusTagTone,
} from "@railops/ui"
import { Input } from "antd"
import type { TableColumnsType } from "antd"

import { useAuth } from "@/components/auth-provider"
import { useConfirm } from "@/components/confirm-provider"
import { CanUseButton } from "@/components/layout/permission-guard"
import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Switch } from "@/components/ui/switch"
import {
  PlatformError,
  formatCompactNumber,
  formatDateTime,
  formatNumber,
} from "@/components/platform/platform-live-ui"
import { ModuleLoading } from "@/components/shared/loading-states"
import { useAppLocale, useI18n } from "@/i18n/provider"
import {
  createPlatformTenant,
  deletePlatformTenant,
  enterPlatformTenant,
  fetchPlatformTenants,
  freezePlatformTenant,
  resetPlatformTenantAdminPassword,
  unfreezePlatformTenant,
  updatePlatformTenant,
  uploadPlatformTenantLogo,
  type PlatformTenantItem,
  type PlatformTenantListResponse,
  type PlatformTenantAdminPasswordResult,
} from "@/lib/api/platform"
import { stashPlatformReturnSession, writeSession } from "@/lib/auth"
import {
  normalizeCustomerTheme,
  type CustomerTheme,
} from "@/lib/customer-theme"
import { cn } from "@/lib/utils"
import { toast } from "sonner"

type SelectOption = { label: string; value: string; detail?: string; labelKey?: string }
type TenantServiceScene = "equipment_after_sales" | "knowledge_support"
type TenantServerConsoleMode = "external" | "iframe"
type TenantEditTab = "profile" | "service" | "branding" | "localization"

const tenantServiceSceneOptions: Array<{ value: TenantServiceScene; labelKey: string }> = [
  { value: "equipment_after_sales", labelKey: "platformExtract.tenants.equipmentAfterSalesScene" },
  { value: "knowledge_support", labelKey: "platformExtract.tenants.knowledgeSupportScene" },
]

const tenantIndustryOptions: SelectOption[] = [
  { label: "工业设备", value: "工业设备", labelKey: "platformExtract.tenantIndustries.industrial" },
  { label: "工程机械", value: "工程机械", labelKey: "platformExtract.tenantIndustries.construction" },
  { label: "能源设备", value: "能源设备", labelKey: "platformExtract.tenantIndustries.energy" },
  { label: "医疗设备", value: "医疗设备", labelKey: "platformExtract.tenantIndustries.medical" },
  { label: "轨道交通", value: "轨道交通", labelKey: "platformExtract.tenantIndustries.rail" },
  { label: "智能制造", value: "智能制造", labelKey: "platformExtract.tenantIndustries.manufacturing" },
  { label: "物流仓储", value: "物流仓储", labelKey: "platformExtract.tenantIndustries.logistics" },
  { label: "农业装备", value: "农业装备", labelKey: "platformExtract.tenantIndustries.agricultural" },
]

const tenantCountryOptions: SelectOption[] = [
  { label: "Germany", value: "Germany" },
  { label: "United States", value: "United States" },
  { label: "United Kingdom", value: "United Kingdom" },
  { label: "France", value: "France" },
  { label: "Japan", value: "Japan" },
  { label: "Singapore", value: "Singapore" },
  { label: "China Mainland", value: "China Mainland" },
  { label: "Hong Kong SAR", value: "Hong Kong SAR" },
  { label: "Australia", value: "Australia" },
  { label: "Brazil", value: "Brazil" },
  { label: "United Arab Emirates", value: "United Arab Emirates" },
]

const tenantDataRegionOptions: SelectOption[] = [
  { label: "欧洲中部", value: "eu-central", detail: "法兰克福" },
  { label: "美国东部", value: "us-east", detail: "弗吉尼亚" },
  { label: "美国西部", value: "us-west", detail: "俄勒冈" },
  { label: "亚太东南", value: "ap-southeast", detail: "新加坡" },
  { label: "亚太东北", value: "ap-northeast", detail: "东京" },
  { label: "中国华东", value: "cn-hangzhou", detail: "杭州" },
  { label: "中国香港", value: "cn-hongkong", detail: "香港" },
  { label: "全球区域", value: "global", detail: "默认区域" },
]

const tenantLanguageOptions: SelectOption[] = [
  { label: "English (US)", value: "en-US", labelKey: "platformExtract.tenantLanguages.enUS" },
  { label: "简体中文", value: "zh-CN", labelKey: "platformExtract.tenantLanguages.zhCN" },
  { label: "Deutsch", value: "de-DE", labelKey: "platformExtract.tenantLanguages.deDE" },
  { label: "日本語", value: "ja-JP", labelKey: "platformExtract.tenantLanguages.jaJP" },
  { label: "Français", value: "fr-FR", labelKey: "platformExtract.tenantLanguages.frFR" },
  { label: "Español", value: "es-ES", labelKey: "platformExtract.tenantLanguages.esES" },
  { label: "Português (BR)", value: "pt-BR", labelKey: "platformExtract.tenantLanguages.ptBR" },
  { label: "العربية", value: "ar-AE", labelKey: "platformExtract.tenantLanguages.arAE" },
]

const tenantCustomerH5DefaultLocales = new Set(["en-US", "zh-CN", "es-ES"])

const tenantTimezoneOptions: SelectOption[] = [
  { label: "UTC", value: "UTC" },
  { label: "Asia/Shanghai", value: "Asia/Shanghai" },
  { label: "Europe/Berlin", value: "Europe/Berlin" },
  { label: "Europe/London", value: "Europe/London" },
  { label: "Europe/Paris", value: "Europe/Paris" },
  { label: "America/New_York", value: "America/New_York" },
  { label: "America/Los_Angeles", value: "America/Los_Angeles" },
  { label: "Asia/Tokyo", value: "Asia/Tokyo" },
  { label: "Asia/Singapore", value: "Asia/Singapore" },
  { label: "Asia/Dubai", value: "Asia/Dubai" },
  { label: "Australia/Sydney", value: "Australia/Sydney" },
  { label: "America/Sao_Paulo", value: "America/Sao_Paulo" },
]

function withLanguageDefaults(tenantDefaultLocale: string, customerDefaultLocale: string, values: string[]) {
  return Array.from(new Set([tenantDefaultLocale, customerDefaultLocale, ...values].filter(Boolean)))
}

const initialForm = {
  name: "",
  serviceScene: "equipment_after_sales" as TenantServiceScene,
  brandName: "",
  logoAssetId: 0,
  logoUrl: "",
  logoPreviewUrl: "",
  customDomain: "",
  customerTheme: "default" as CustomerTheme,
  serverConsoleName: "1Panel",
  serverConsoleUrl: "",
  serverConsoleMode: "external" as TenantServerConsoleMode,
  serverConsoleEnabled: false,
  industry: "",
  countryRegion: "",
  dataRegion: "",
  defaultLocale: "en-US",
  customerDefaultLocale: "en-US",
  languagePreferences: ["en-US"],
  timezonePreferences: ["UTC"],
  aiEnabled: false,
  trialEndsAt: "",
  adminUsername: "",
  adminNickname: "",
  adminEmail: "",
  adminMobile: "",
  adminPassword: "",
}

const initialEditForm = {
  name: "",
  serviceScene: "equipment_after_sales" as TenantServiceScene,
  brandName: "",
  logoAssetId: 0,
  logoUrl: "",
  logoPreviewUrl: "",
  customDomain: "",
  customerTheme: "default" as CustomerTheme,
  serverConsoleName: "1Panel",
  serverConsoleUrl: "",
  serverConsoleMode: "external" as TenantServerConsoleMode,
  serverConsoleEnabled: false,
  industry: "",
  countryRegion: "",
  dataRegion: "",
  defaultLocale: "en-US",
  customerDefaultLocale: "en-US",
  languagePreferences: ["en-US"],
  timezone: "UTC",
  timezonePreferences: ["UTC"],
  aiEnabled: true,
  trialEndsAt: "",
  formalTenant: true,
}

type I18nT = ReturnType<typeof useI18n>

function tenantAICapabilityItems(t: I18nT, enabled: boolean, serviceScene: TenantServiceScene) {
  const knowledgeSupport = serviceScene === "knowledge_support"
  const keys = enabled
    ? knowledgeSupport
      ? ["aiKnowledge", "aiDiagnosis", "aiAgent", "aiUsage"]
      : ["aiKnowledge", "aiDiagnosis", "aiAgent", "aiUsage", "aiProductResources"]
    : knowledgeSupport
      ? ["ticketOrders", "ticketDispatch"]
      : ["ticketProducts", "ticketServiceCodes", "ticketOrders", "ticketDispatch", "ticketWorkflow"]
  return keys.map((key) => t(`platformExtract.tenants.aiCapabilityItems.${key}`))
}

function TenantAICapabilityField({
  enabled,
  serviceScene,
  onChange,
  t,
}: {
  enabled: boolean
  serviceScene: TenantServiceScene
  onChange: (enabled: boolean) => void
  t: I18nT
}) {
  return (
    <div className="flex min-h-10 items-start justify-between gap-4 border-t border-border pt-4 sm:col-span-2">
      <div className="min-w-0">
        <div className="text-sm font-medium text-foreground">{t("platformExtract.tenants.aiCapability")}</div>
        <div className="mt-1 text-xs font-medium text-muted-foreground">
          {enabled ? t("platformExtract.tenants.aiEnabled") : t("platformExtract.tenants.aiDisabled")}
        </div>
        <p className="mt-1 text-xs leading-5 text-muted-foreground">
          {enabled
            ? t(serviceScene === "knowledge_support"
              ? "platformExtract.tenants.knowledgeSupportAIEnabledHint"
              : "platformExtract.tenants.aiEnabledHint")
            : t("platformExtract.tenants.aiDisabledHint")}
        </p>
        <div className="mt-2 flex flex-wrap gap-1.5">
          {tenantAICapabilityItems(t, enabled, serviceScene).map((item) => (
            <StatusTag key={item} tone={enabled ? "blue" : "neutral"}>{item}</StatusTag>
          ))}
        </div>
      </div>
      <Switch
        checked={enabled}
        onCheckedChange={onChange}
        aria-label={t("platformExtract.tenants.aiCapability")}
      />
    </div>
  )
}

type TenantPortalFormValue = {
  brandName: string
  logoAssetId: number
  logoUrl: string
  logoPreviewUrl: string
  customDomain: string
  customerTheme: CustomerTheme
  serverConsoleName: string
  serverConsoleUrl: string
  serverConsoleMode: TenantServerConsoleMode
  serverConsoleEnabled: boolean
}

const tenantCustomerThemeOptions: Array<{
  value: CustomerTheme
  labelKey: string
  descriptionKey: string
  previewClassName: string
}> = [
  {
    value: "default",
    labelKey: "platformExtract.tenants.customerThemeDefault",
    descriptionKey: "platformExtract.tenants.customerThemeDefaultDescription",
    previewClassName: "rhd-tenant-theme-preview-default",
  },
  {
    value: "odt-intelligence",
    labelKey: "platformExtract.tenants.customerThemeODT",
    descriptionKey: "platformExtract.tenants.customerThemeODTDescription",
    previewClassName: "rhd-tenant-theme-preview-odt",
  },
  {
    value: "clinical-calm",
    labelKey: "platformExtract.tenants.customerThemeClinical",
    descriptionKey: "platformExtract.tenants.customerThemeClinicalDescription",
    previewClassName: "rhd-tenant-theme-preview-clinical",
  },
  {
    value: "signal-coral",
    labelKey: "platformExtract.tenants.customerThemeCoral",
    descriptionKey: "platformExtract.tenants.customerThemeCoralDescription",
    previewClassName: "rhd-tenant-theme-preview-coral",
  },
]

type TenantCustomerThemeOption = (typeof tenantCustomerThemeOptions)[number]

function TenantThemePreview({
  option,
  displayName,
  logoUrl,
  expanded = false,
}: {
  option: TenantCustomerThemeOption
  displayName: string
  logoUrl: string
  expanded?: boolean
}) {
  return (
    <div
      className={cn(
        "rhd-tenant-theme-preview",
        expanded && "rhd-tenant-theme-preview-expanded",
        option.previewClassName,
      )}
      aria-hidden="true"
    >
      <div className="rhd-tenant-theme-preview-header">
        <span className="rhd-tenant-theme-preview-brand">
          {logoUrl ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={logoUrl} alt="" />
          ) : (
            <span>{displayName.slice(0, 1).toUpperCase()}</span>
          )}
        </span>
        <i />
        <i />
      </div>
      <div className="rhd-tenant-theme-preview-workspace">
        <div className="rhd-tenant-theme-preview-nav">
          <span className="is-active" />
          <span />
          <span />
        </div>
        <div className="rhd-tenant-theme-preview-content">
          <div className="rhd-tenant-theme-preview-title">
            <strong>{displayName}</strong>
            <em />
          </div>
          <div className="rhd-tenant-theme-preview-panels">
            <span><i /><i /></span>
            <span><i /><i /></span>
          </div>
        </div>
      </div>
    </div>
  )
}

function TenantCustomerThemePicker({
  value,
  brandName,
  logoUrl,
  onChange,
  disabled,
  t,
}: {
  value: CustomerTheme
  brandName: string
  logoUrl: string
  onChange: (value: CustomerTheme) => void
  disabled?: boolean
  t: I18nT
}) {
  const displayName = brandName.trim() || t("remoteTopbar.brandTitle")
  const [previewTheme, setPreviewTheme] = useState<CustomerTheme | null>(null)
  const previewOption = tenantCustomerThemeOptions.find((option) => option.value === previewTheme) ?? null

  return (
    <>
      <fieldset className="grid min-w-0 gap-2 sm:col-span-2" disabled={disabled}>
        <legend className="text-sm font-medium text-foreground">{t("platformExtract.tenants.customerTheme")}</legend>
        <div
          className="grid min-w-0 grid-cols-2 gap-2 md:grid-cols-4"
          role="radiogroup"
          aria-label={t("platformExtract.tenants.customerTheme")}
        >
          {tenantCustomerThemeOptions.map((option) => {
            const selected = value === option.value
            return (
              <div
                key={option.value}
                className={cn(
                  "flex h-14 min-w-0 items-center gap-1 rounded-md border bg-background px-2 transition",
                  selected
                    ? "border-[var(--railops-primary)] bg-muted/60 shadow-[0_0_0_1px_var(--railops-primary)]"
                    : "border-border hover:border-[var(--railops-border-strong)] hover:bg-muted/40",
                )}
              >
                <button
                  type="button"
                  role="radio"
                  aria-checked={selected}
                  aria-label={t(option.labelKey)}
                  disabled={disabled}
                  className="flex h-full min-w-0 flex-1 items-center gap-2 text-left focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-60"
                  onClick={() => onChange(option.value)}
                >
                  <span
                    className={cn("rhd-tenant-theme-preview", "rhd-tenant-theme-preview-compact", option.previewClassName)}
                    aria-hidden="true"
                  />
                  <span className="min-w-0 flex-1 truncate text-sm font-semibold text-foreground">{t(option.labelKey)}</span>
                  {selected ? (
                    <span className="flex size-5 shrink-0 items-center justify-center rounded-full bg-[var(--railops-primary)] text-white">
                      <CheckIcon className="size-3.5" aria-hidden="true" />
                    </span>
                  ) : null}
                </button>
                <IconButton
                  htmlType="button"
                  variant="text"
                  icon={<Maximize2Icon className="size-3.5" />}
                  tooltip={t("platformExtract.tenants.customerThemePreviewAction")}
                  aria-label={`${t("platformExtract.tenants.customerThemePreviewAction")} · ${t(option.labelKey)}`}
                  disabled={disabled}
                  className="!size-8 !min-w-8 shrink-0 !p-0"
                  onClick={() => setPreviewTheme(option.value)}
                />
              </div>
            )
          })}
        </div>
      </fieldset>

      <StandardModal
        open={Boolean(previewOption)}
        onCancel={() => setPreviewTheme(null)}
        title={previewOption ? t(previewOption.labelKey) : t("platformExtract.tenants.customerTheme")}
        width={760}
        rootClassName="rhd-platform-theme-preview-modal"
        footer={null}
        destroyOnHidden
      >
        {previewOption ? (
          <div className="grid gap-3">
            <TenantThemePreview option={previewOption} displayName={displayName} logoUrl={logoUrl} expanded />
            <p className="text-sm leading-6 text-muted-foreground">{t(previewOption.descriptionKey)}</p>
          </div>
        ) : null}
      </StandardModal>
    </>
  )
}

function TenantLogoUploadField({
  value,
  onChange,
  disabled,
  t,
}: {
  value: TenantPortalFormValue
  onChange: (patch: Partial<TenantPortalFormValue>) => void
  disabled?: boolean
  t: I18nT
}) {
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const [uploading, setUploading] = useState(false)
  const previewUrl = value.logoPreviewUrl || value.logoUrl

  async function uploadLogo(file: File) {
    if (!file.type.startsWith("image/")) {
      toast.error(t("upload.chooseImage"))
      return
    }
    if (file.size > 5 * 1024 * 1024) {
      toast.error(t("upload.imageTooLarge", { maxSize: 5 }))
      return
    }
    setUploading(true)
    try {
      const result = await uploadPlatformTenantLogo(file)
      onChange({ logoAssetId: result.assetId, logoUrl: result.url, logoPreviewUrl: result.previewUrl })
      toast.success(t("upload.imageUploaded"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("upload.imageUploadFailed"))
    } finally {
      setUploading(false)
      if (fileInputRef.current) fileInputRef.current.value = ""
    }
  }

  return (
    <div className="grid gap-2 sm:col-span-2">
      <span className="text-sm font-medium text-foreground">{t("platformExtract.tenants.logoUrl")}</span>
      <input
        ref={fileInputRef}
        type="file"
        accept="image/png,image/jpeg,image/webp"
        className="hidden"
        disabled={disabled || uploading}
        onChange={(event) => {
          const file = event.target.files?.[0]
          if (file) void uploadLogo(file)
        }}
      />
      <div className="flex flex-wrap items-center gap-3">
        <div className="grid h-20 w-40 shrink-0 place-items-center overflow-hidden rounded-md border border-border bg-white p-2">
          {previewUrl ? (
            // eslint-disable-next-line @next/next/no-img-element
            <img src={previewUrl} alt={value.brandName || "Tenant logo"} className="max-h-full max-w-full object-contain" />
          ) : (
            <ImageIcon className="size-6 text-muted-foreground" aria-hidden="true" />
          )}
        </div>
        <div className="flex items-center gap-2">
          <RailopsButton
            htmlType="button"
            icon={uploading ? <RefreshCwIcon className="animate-spin" /> : <UploadIcon />}
            disabled={disabled || uploading}
            onClick={() => fileInputRef.current?.click()}
          >
            {previewUrl ? t("upload.replaceImage") : t("platformExtract.tenants.logoUpload")}
          </RailopsButton>
          {previewUrl ? (
            <IconButton
              htmlType="button"
              icon={<XIcon />}
              tooltip={t("platformExtract.tenants.logoRemove")}
              aria-label={t("platformExtract.tenants.logoRemove")}
              disabled={disabled || uploading}
              onClick={() => onChange({ logoAssetId: 0, logoUrl: "", logoPreviewUrl: "" })}
            />
          ) : null}
        </div>
      </div>
      <span className="text-xs text-muted-foreground">{t("platformExtract.tenants.logoUploadHint")}</span>
    </div>
  )
}

function TenantPortalSettingsFields({
  value,
  onChange,
  disabled,
  showSectionHeading = true,
  t,
}: {
  value: TenantPortalFormValue
  onChange: (patch: Partial<TenantPortalFormValue>) => void
  disabled?: boolean
  showSectionHeading?: boolean
  t: I18nT
}) {
  const serverConsoleModeOptions = [
    { value: "external", label: t("platformExtract.tenants.serverConsoleExternal") },
    { value: "iframe", label: t("platformExtract.tenants.serverConsoleIframe") },
  ]
  return (
    <>
      {showSectionHeading ? (
        <div className="border-t border-border pt-4 text-sm font-semibold text-foreground sm:col-span-2">
          {t("platformExtract.tenants.brandSettings")}
        </div>
      ) : null}
      <label className="grid gap-1.5 text-sm font-medium text-foreground">
        {t("platformExtract.tenants.brandName")}
        <Input value={value.brandName} onChange={(event) => onChange({ brandName: event.target.value })} />
      </label>
      <label className="grid gap-1.5 text-sm font-medium text-foreground">
        {t("platformExtract.tenants.customDomain")}
        <Input value={value.customDomain} onChange={(event) => onChange({ customDomain: event.target.value })} placeholder="www.example.com" />
      </label>
      <TenantLogoUploadField value={value} onChange={onChange} disabled={disabled} t={t} />
      <TenantCustomerThemePicker
        value={value.customerTheme}
        brandName={value.brandName}
        logoUrl={value.logoPreviewUrl || value.logoUrl}
        disabled={disabled}
        onChange={(customerTheme) => onChange({ customerTheme })}
        t={t}
      />
      <div className="flex items-center justify-between gap-4 border-t border-border pt-4 sm:col-span-2">
        <div>
          <div className="text-sm font-semibold text-foreground">{t("platformExtract.tenants.serverConsoleSettings")}</div>
          <div className="mt-1 text-xs text-muted-foreground">{t("platformExtract.tenants.serverConsoleEnabled")}</div>
        </div>
        <Switch checked={value.serverConsoleEnabled} onCheckedChange={(checked) => onChange({ serverConsoleEnabled: checked })} />
      </div>
      {value.serverConsoleEnabled ? (
        <>
          <label className="grid gap-1.5 text-sm font-medium text-foreground">
            {t("platformExtract.tenants.serverConsoleName")}
            <Input value={value.serverConsoleName} onChange={(event) => onChange({ serverConsoleName: event.target.value })} placeholder="1Panel" />
          </label>
          <SelectField
            label={t("platformExtract.tenants.serverConsoleMode")}
            style={{ marginBottom: 0 }}
            selectProps={{
              "aria-label": t("platformExtract.tenants.serverConsoleMode"),
              value: value.serverConsoleMode,
              onChange: (mode) => onChange({ serverConsoleMode: mode as TenantServerConsoleMode }),
              options: serverConsoleModeOptions,
              style: { width: "100%" },
            }}
          />
          <label className="grid gap-1.5 text-sm font-medium text-foreground sm:col-span-2">
            {t("platformExtract.tenants.serverConsoleUrl")}
            <Input
              value={value.serverConsoleUrl}
              onChange={(event) => {
                const serverConsoleUrl = event.target.value
                onChange({
                  serverConsoleUrl,
                  ...(serverConsoleUrl.trim().toLowerCase().startsWith("http://") ? { serverConsoleMode: "external" as const } : {}),
                })
              }}
              placeholder="https://panel.example.com/security-entry"
            />
            {value.serverConsoleUrl.trim().toLowerCase().startsWith("http://") ? (
              <span className="text-xs font-normal text-amber-700">{t("platformExtract.tenants.serverConsoleHttpHint")}</span>
            ) : null}
          </label>
        </>
      ) : null}
    </>
  )
}

function tenantStatus(t: I18nT, tenant: PlatformTenantItem) {
  if (tenant.decommissionedAt) return { label: t("platformExtract.tenants.decommissioned"), tone: "danger" as const }
  if (tenant.status === 1) return { label: t("platformExtract.tenants.frozen"), tone: "danger" as const }
  if (tenant.trialEndsAt) {
    const expired = new Date(tenant.trialEndsAt).getTime() < Date.now()
    return expired
      ? { label: t("platformExtract.tenants.trialExpired"), tone: "warning" as const }
      : { label: t("platformExtract.tenants.trialing"), tone: "info" as const }
  }
  return { label: t("platformExtract.tenants.active"), tone: "success" as const }
}

const statusTagToneMap: Record<ReturnType<typeof tenantStatus>["tone"], StatusTagTone> = {
  danger: "error",
  info: "blue",
  success: "success",
  warning: "warning",
}

function toDateInputValue(value: string) {
  return value ? value.slice(0, 10) : ""
}

export default function PlatformTenantsPage() {
  const t = useI18n()
  const { locale } = useAppLocale()
  const { session } = useAuth()
  const confirm = useConfirm()
  const breadcrumbItems = useRouteBreadcrumbItems()
  const [data, setData] = useState<PlatformTenantListResponse | null>(null)
  const [searchInput, setSearchInput] = useState("")
  const [search, setSearch] = useState("")
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [hasLoadedTenants, setHasLoadedTenants] = useState(false)
  const [error, setError] = useState("")
  const [createOpen, setCreateOpen] = useState(false)
  const [form, setForm] = useState(initialForm)
  const [saving, setSaving] = useState(false)
  const [formError, setFormError] = useState("")
  const [editingTenant, setEditingTenant] = useState<PlatformTenantItem | null>(null)
  const [editForm, setEditForm] = useState(initialEditForm)
  const [editActiveTab, setEditActiveTab] = useState<TenantEditTab>("profile")
  const [editError, setEditError] = useState("")
  const [editSaving, setEditSaving] = useState(false)
  const [actingTenantId, setActingTenantId] = useState<number | null>(null)
  const [statusTenant, setStatusTenant] = useState<PlatformTenantItem | null>(null)
  const [statusError, setStatusError] = useState("")
  const [freezeReason, setFreezeReason] = useState("")
  const [passwordTenant, setPasswordTenant] = useState<PlatformTenantItem | null>(null)
  const [adminPassword, setAdminPassword] = useState("")
  const [passwordResult, setPasswordResult] = useState<PlatformTenantAdminPasswordResult | null>(null)
  const [passwordError, setPasswordError] = useState("")
  const [resettingPassword, setResettingPassword] = useState(false)
  const canCreateTenant = CanUseButton("tenant.create", session?.permissions)
  const canUpdateTenant = CanUseButton("tenant.update", session?.permissions)
  const canDeleteTenant = CanUseButton("tenant.delete", session?.permissions)
  const localizedIndustryOptions = useMemo(
    () => tenantIndustryOptions.map((option) => ({
      ...option,
      label: option.labelKey ? t(option.labelKey) : option.label,
    })),
    [t]
  )
  const localizedLanguageOptions = useMemo(
    () => tenantLanguageOptions.map((option) => ({
      ...option,
      label: option.labelKey ? t(option.labelKey) : option.label,
    })),
    [t]
  )
  const localizedPortalLanguageOptions = useMemo(
    () => localizedLanguageOptions.filter((option) => tenantCustomerH5DefaultLocales.has(option.value)),
    [localizedLanguageOptions]
  )
  const editTabItems = useMemo(() => [
    { value: "profile", label: t("platformExtract.tenants.editTabProfile") },
    { value: "service", label: t("platformExtract.tenants.editTabService") },
    { value: "branding", label: t("platformExtract.tenants.editTabBranding") },
    { value: "localization", label: t("platformExtract.tenants.editTabLocalization") },
  ], [t])

  const load = useCallback(async () => {
    setLoading(true)
    setError("")
    try {
      const tenantData = await fetchPlatformTenants({ search, page, limit: DEFAULT_PAGE_SIZE })
      setData(tenantData)
      setHasLoadedTenants(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : t("platformExtract.tenants.tenantDataUnavailable"))
    } finally {
      setLoading(false)
    }
  }, [page, search, t])

  const refreshAll = useCallback(async () => {
    await load()
  }, [load])

  useEffect(() => {
    void load()
  }, [load])

  const pageCount = useMemo(() => Math.max(1, Math.ceil((data?.totalCount ?? 0) / DEFAULT_PAGE_SIZE)), [data])

  useEffect(() => {
    const timer = setTimeout(() => {
      setPage(1)
      setSearch(searchInput.trim())
    }, 300)
    return () => clearTimeout(timer)
  }, [searchInput])

  async function submitCreate(event: FormEvent) {
    event.preventDefault()
    if (!form.name.trim() || !form.adminUsername.trim() || !form.adminNickname.trim()) {
      setFormError(t("platformExtract.tenants.nameAdminRequired"))
      return
    }
    if (form.adminPassword.length < 8) {
      setFormError(t("platformExtract.tenants.passwordTooShort"))
      return
    }
    if (!form.defaultLocale || !form.customerDefaultLocale || !form.languagePreferences.length || !form.timezonePreferences.length) {
      setFormError(t("platformExtract.tenants.submitLocaleTimezoneRequired"))
      return
    }
    setSaving(true)
    setFormError("")
    try {
      await createPlatformTenant({
        name: form.name.trim(),
        serviceScene: form.serviceScene,
        brandName: form.brandName.trim(),
        logoAssetId: form.logoAssetId,
        logoUrl: form.logoUrl.trim(),
        customDomain: form.customDomain.trim(),
        customerTheme: form.customerTheme,
        serverConsoleName: form.serverConsoleName.trim(),
        serverConsoleUrl: form.serverConsoleUrl.trim(),
        serverConsoleMode: form.serverConsoleMode,
        serverConsoleEnabled: form.serverConsoleEnabled,
        industry: form.industry.trim(),
        countryRegion: form.countryRegion.trim(),
        dataRegion: form.dataRegion.trim(),
        aiEnabled: form.aiEnabled,
        trialEndsAt: form.trialEndsAt,
        defaultLocale: form.defaultLocale,
        customerDefaultLocale: form.customerDefaultLocale,
        supportedLocales: withLanguageDefaults(form.defaultLocale, form.customerDefaultLocale, form.languagePreferences),
        timezone: form.timezonePreferences[0],
        supportedTimezones: form.timezonePreferences,
        adminUsername: form.adminUsername.trim(),
        adminNickname: form.adminNickname.trim(),
        adminEmail: form.adminEmail.trim(),
        adminMobile: form.adminMobile.trim(),
        adminPassword: form.adminPassword,
      })
      setCreateOpen(false)
      setForm(initialForm)
      setPage(1)
      await load()
    } catch (err) {
      setFormError(err instanceof Error ? err.message : t("platformExtract.tenants.createFailed"))
    } finally {
      setSaving(false)
    }
  }

  function openEditDialog(tenant: PlatformTenantItem) {
    const trialEndsAt = toDateInputValue(tenant.trialEndsAt)
    setEditingTenant(tenant)
    setEditForm({
      name: tenant.name,
      serviceScene: tenant.serviceScene || "equipment_after_sales",
      brandName: tenant.brandName || "",
      logoAssetId: tenant.logoAssetId || 0,
      logoUrl: tenant.logoUrl || "",
      logoPreviewUrl: "",
      customDomain: tenant.customDomain || "",
      customerTheme: normalizeCustomerTheme(tenant.customerTheme),
      serverConsoleName: tenant.serverConsoleName || "1Panel",
      serverConsoleUrl: tenant.serverConsoleUrl || "",
      serverConsoleMode: tenant.serverConsoleMode === "iframe" ? "iframe" : "external",
      serverConsoleEnabled: tenant.serverConsoleEnabled || false,
      industry: tenant.industry || "",
      countryRegion: tenant.countryRegion || "",
      dataRegion: tenant.dataRegion || "",
      defaultLocale: tenant.defaultLocale || "en-US",
      customerDefaultLocale: tenant.customerDefaultLocale || tenant.defaultLocale || "en-US",
      languagePreferences: withLanguageDefaults(
        tenant.defaultLocale || "en-US",
        tenant.customerDefaultLocale || tenant.defaultLocale || "en-US",
        tenant.supportedLocales?.length ? tenant.supportedLocales : [],
      ),
      timezone: tenant.timezone || "UTC",
      timezonePreferences: tenant.supportedTimezones?.length ? tenant.supportedTimezones : [tenant.timezone || "UTC"],
      aiEnabled: tenant.aiEnabled ?? true,
      trialEndsAt,
      formalTenant: !trialEndsAt,
    })
    setEditActiveTab("profile")
    setEditError("")
  }

  async function submitEdit(event: FormEvent) {
    event.preventDefault()
    if (!editingTenant) return
    if (!editForm.name.trim()) {
      setEditActiveTab("profile")
      setEditError(t("platformExtract.tenants.nameRequired"))
      return
    }
    if (!editForm.defaultLocale || !editForm.customerDefaultLocale || !editForm.languagePreferences.length || !editForm.timezonePreferences.length) {
      setEditActiveTab("localization")
      setEditError(t("platformExtract.tenants.submitLocaleTimezoneRequired"))
      return
    }
    if (!editForm.formalTenant && !editForm.trialEndsAt) {
      setEditActiveTab("profile")
      setEditError(t("platformExtract.tenants.submitInvalidTrial"))
      return
    }
    setEditSaving(true)
    setEditError("")
    try {
      await updatePlatformTenant({
        id: editingTenant.id,
        name: editForm.name.trim(),
        serviceScene: editForm.serviceScene,
        brandName: editForm.brandName.trim(),
        logoAssetId: editForm.logoAssetId,
        logoUrl: editForm.logoUrl.trim(),
        customDomain: editForm.customDomain.trim(),
        customerTheme: editForm.customerTheme,
        serverConsoleName: editForm.serverConsoleName.trim(),
        serverConsoleUrl: editForm.serverConsoleUrl.trim(),
        serverConsoleMode: editForm.serverConsoleMode,
        serverConsoleEnabled: editForm.serverConsoleEnabled,
        industry: editForm.industry.trim(),
        countryRegion: editForm.countryRegion.trim(),
        dataRegion: editForm.dataRegion.trim(),
        aiEnabled: editForm.aiEnabled,
        defaultLocale: editForm.defaultLocale,
        customerDefaultLocale: editForm.customerDefaultLocale,
        supportedLocales: withLanguageDefaults(editForm.defaultLocale, editForm.customerDefaultLocale, editForm.languagePreferences),
        timezone: editForm.timezonePreferences[0],
        supportedTimezones: editForm.timezonePreferences,
        trialEndsAt: editForm.formalTenant ? "" : editForm.trialEndsAt,
      })
      setEditingTenant(null)
      await load()
    } catch (err) {
      setEditError(err instanceof Error ? err.message : t("platformExtract.tenants.updateFailed"))
    } finally {
      setEditSaving(false)
    }
  }

  function openStatusDialog(tenant: PlatformTenantItem) {
    setStatusTenant(tenant)
    setStatusError("")
    setFreezeReason("")
  }

  async function confirmTenantStatusChange() {
    if (!statusTenant) return
    const frozen = statusTenant.status === 1
    setActingTenantId(statusTenant.id)
    setError("")
    setStatusError("")
    try {
      if (frozen) await unfreezePlatformTenant(statusTenant.id)
      else await freezePlatformTenant(statusTenant.id, freezeReason.trim())
      setStatusTenant(null)
      await load()
    } catch (err) {
      setStatusError(err instanceof Error ? err.message : t("platformExtract.tenants.statusUpdateFailed"))
    } finally {
      setActingTenantId(null)
    }
  }

  async function enterTenant(tenant: PlatformTenantItem) {
    if (!session) return
    setActingTenantId(tenant.id)
    setError("")
    try {
      const tenantSession = await enterPlatformTenant(tenant.id)
      stashPlatformReturnSession(session)
      writeSession(tenantSession)
      window.location.assign("/enterprise")
    } catch (err) {
      setError(err instanceof Error ? err.message : t("platformExtract.tenants.enterFailed"))
      setActingTenantId(null)
    }
  }

  function openPasswordDialog(tenant: PlatformTenantItem) {
    setPasswordTenant(tenant)
    setAdminPassword("")
    setPasswordResult(null)
    setPasswordError("")
  }

  function closePasswordDialog() {
    if (resettingPassword) return
    setPasswordTenant(null)
    setAdminPassword("")
    setPasswordResult(null)
    setPasswordError("")
  }

  async function submitAdminPassword(event: FormEvent) {
    event.preventDefault()
    if (!passwordTenant || adminPassword.length < 8) return
    setResettingPassword(true)
    setPasswordError("")
    try {
      const result = await resetPlatformTenantAdminPassword(passwordTenant.id, adminPassword)
      setPasswordResult(result)
      setAdminPassword("")
      toast.success(t("platformExtract.tenants.passwordResetDone"))
    } catch (err) {
      setPasswordError(err instanceof Error ? err.message : t("platformExtract.tenants.adminPasswordReset"))
    } finally {
      setResettingPassword(false)
    }
  }

  async function removeTenant(tenant: PlatformTenantItem) {
    if (actingTenantId !== null || !tenant.decommissionedAt) return
    const confirmed = await confirm({
      title: t("platformExtract.tenants.deleteTitle"),
      description: t("platformExtract.tenants.deleteConfirm", { name: tenant.name }),
      confirmText: t("platformExtract.tenants.delete"),
      variant: "destructive",
    })
    if (!confirmed) return
    setActingTenantId(tenant.id)
    setError("")
    try {
      await deletePlatformTenant(tenant.id)
      toast.success(t("platformExtract.tenants.deleteSuccess"))
      await load()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("platformExtract.tenants.deleteFailed"))
    } finally {
      setActingTenantId(null)
    }
  }

  const isInitialLoading = loading && !hasLoadedTenants
  const hasData = data !== null

  const columns: TableColumnsType<PlatformTenantItem> = [
    {
      title: t("platformExtract.tenants.enterprise"),
      dataIndex: "name",
      width: 220,
      render: (_value, tenant) => (
        <div className="flex min-w-0 items-center gap-2.5">
          <span className="inline-flex size-8 shrink-0 items-center justify-center overflow-hidden rounded-md border border-primary/15 bg-primary/5 text-primary">
            {tenant.logoUrl ? (
              <>
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img src={tenant.logoUrl} alt="" className="max-h-6 max-w-6 object-contain" />
              </>
            ) : (
              <Building2Icon className="size-4" />
            )}
          </span>
          <span className="min-w-0">
            <span className="block truncate font-medium" title={tenant.name}>
              {tenant.name}
            </span>
          </span>
        </div>
      ),
    },
    {
      title: t("platformExtract.tenants.adminInfo"),
      dataIndex: "adminDisplayName",
      width: 190,
      render: (_value, tenant) => (
        <span className="rhd-railops-platform-tenant-inline">
          <span>{tenant.adminDisplayName || tenant.adminUsername || t("platformExtract.tenants.notSet")}</span>
          <span>{tenant.adminEmail || (tenant.adminDisplayName ? tenant.adminUsername : "") || "-"}</span>
        </span>
      ),
    },
    {
      title: t("platformExtract.tenants.regionIndustry"),
      dataIndex: "countryRegion",
      width: 160,
      render: (_value, tenant) => (
        <span className="rhd-railops-platform-tenant-inline">
          <span>{tenant.countryRegion || t("platformExtract.tenants.notSet")}</span>
          <span>{tenant.industry || t("platformExtract.tenants.noIndustry")}</span>
        </span>
      ),
    },
    {
      title: t("platformExtract.tenants.aiCapability"),
      dataIndex: "aiEnabled",
      width: 120,
      render: (_value, tenant) => (
        <StatusTag tone={tenant.aiEnabled !== false ? "blue" : "neutral"} className="rhd-railops-platform-status">
          {tenant.aiEnabled !== false ? t("platformExtract.tenants.aiEnabled") : t("platformExtract.tenants.aiDisabled")}
        </StatusTag>
      ),
    },
    {
      title: t("platformExtract.tenants.serviceScene"),
      dataIndex: "serviceScene",
      width: 150,
      render: (_value, tenant) => (
        <StatusTag tone={tenant.serviceScene === "knowledge_support" ? "blue" : "neutral"} className="rhd-railops-platform-status">
          {tenant.serviceScene === "knowledge_support"
            ? t("platformExtract.tenants.knowledgeSupportScene")
            : t("platformExtract.tenants.equipmentAfterSalesScene")}
        </StatusTag>
      ),
    },
    {
      title: t("platformExtract.tenants.membersDevices"),
      dataIndex: "memberCount",
      width: 110,
      align: "right",
      render: (_value, tenant) => (
        <span className="tabular-nums">
          {formatNumber(tenant.memberCount)} / {formatNumber(tenant.deviceCount)}
        </span>
      ),
    },
    {
      title: t("platformExtract.tenants.monthAiCalls"),
      dataIndex: "monthlyAiRequests",
      width: 150,
      align: "right",
      render: (_value, tenant) => (
        <span className="rhd-railops-platform-tenant-inline justify-end">
          <span>{formatCompactNumber(tenant.monthlyAiRequests)}</span>
          <span>{formatCompactNumber(tenant.monthlyAiTokens)} Token</span>
        </span>
      ),
    },
    {
      title: t("platformExtract.tenants.status"),
      dataIndex: "status",
      width: 100,
      render: (_value, tenant) => {
        const status = tenantStatus(t, tenant)
        return <StatusTag tone={statusTagToneMap[status.tone]} className="rhd-railops-platform-status">{status.label}</StatusTag>
      },
    },
    {
      title: t("platformExtract.tenants.lastMeteredAt"),
      dataIndex: "lastMeteredAt",
      width: 140,
      render: (_value, tenant) => (
        <span className="text-xs text-[var(--railops-text-secondary)]">{formatDateTime(tenant.lastMeteredAt, locale)}</span>
      ),
    },
    {
      title: t("platformExtract.tenants.actions"),
      key: "actions",
      width: 190,
      align: "right",
      render: (_value, tenant) =>
        canUpdateTenant || (canDeleteTenant && Boolean(tenant.decommissionedAt)) ? (
          <div className="flex items-center justify-end gap-1.5 whitespace-nowrap">
            {canUpdateTenant ? (
              <RailopsButton
                size="small"
                disabled={actingTenantId === tenant.id}
                onClick={() => void enterTenant(tenant)}
              >
                <LogInIcon />
                {t("platformExtract.tenants.enterTenant")}
              </RailopsButton>
            ) : null}
            <DropdownMenu>
              <DropdownMenuTrigger
                render={<RailopsButton variant="text" size="small" className="px-1.5" />}
                aria-label={t("platformExtract.tenants.manageTenant", { name: tenant.name })}
                title={t("platformExtract.tenants.moreActions")}
                disabled={actingTenantId === tenant.id}
              >
                <MoreHorizontalIcon />
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-48 min-w-48">
                {canUpdateTenant ? (
                  <>
                    <DropdownMenuItem onClick={() => openEditDialog(tenant)}>
                      <PencilIcon />
                      {t("platformExtract.tenants.editTenant")}
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => openPasswordDialog(tenant)}>
                      <KeyRoundIcon />
                      {t("platformExtract.tenants.resetPassword")}
                    </DropdownMenuItem>
                    {!tenant.decommissionedAt ? (
                      <>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem
                          variant={tenant.status === 1 ? "default" : "destructive"}
                          onClick={() => openStatusDialog(tenant)}
                        >
                          {tenant.status === 1 ? <LockKeyholeOpenIcon /> : <LockKeyholeIcon />}
                          {tenant.status === 1 ? t("platformExtract.tenants.unfreeze") : t("platformExtract.tenants.freeze")}
                        </DropdownMenuItem>
                      </>
                    ) : null}
                  </>
                ) : null}
                {canDeleteTenant && tenant.decommissionedAt ? (
                  <>
                    {canUpdateTenant ? <DropdownMenuSeparator /> : null}
                    <DropdownMenuItem variant="destructive" onClick={() => void removeTenant(tenant)}>
                      <Trash2Icon />
                      {t("platformExtract.tenants.delete")}
                    </DropdownMenuItem>
                  </>
                ) : null}
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        ) : <span className="text-xs text-muted-foreground">{t("platformExtract.tenants.readonly")}</span>,
    },
  ]

  return (
    <PageShell
      title={t("platformExtract.tenants.title")}
      breadcrumb={breadcrumbItems}
      className="rhd-railops-platform-tenants-page min-h-[calc(100vh-97px)]"
      actions={
        <>
          <IconButton
            icon={<RefreshCwIcon className={loading ? "animate-spin" : ""} />}
            tooltip={t("platformExtract.tenants.refresh")}
            aria-label={t("platformExtract.tenants.refresh")}
            onClick={() => void refreshAll()}
            disabled={loading}
          />
          {canCreateTenant ? (
            <RailopsButton variant="primary" onClick={() => setCreateOpen(true)}>
              <PlusIcon data-icon="inline-start" />
              {t("platformExtract.tenants.createTitle")}
            </RailopsButton>
          ) : null}
        </>
      }
    >

      <div className="self-start min-w-0 w-full space-y-3">
        {!loading && error && !hasData ? <PlatformError message={error} onRetry={() => void load()} /> : null}
        {error && hasData ? <div className="rhd-railops-admin-inline-alert border border-border bg-muted px-3 py-2 text-sm text-muted-foreground">{error}</div> : null}
        <div className="rhd-railops-platform-panel rhd-railops-platform-tenants-table-module overflow-hidden border border-border bg-card">
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border bg-muted/50 px-4 py-3">
            <span className="text-sm font-semibold text-foreground">{t("platformExtract.tenants.tableTitle")}</span>
            <div className="flex flex-wrap items-center gap-3">
              <SearchField
                allowClear
                aria-label={t("platformExtract.tenants.searchAria")}
                className="rhd-railops-platform-tenants-search w-64"
                placeholder={t("platformExtract.tenants.searchPlaceholder")}
                value={searchInput}
                onChange={(event) => setSearchInput(event.target.value)}
              />
            </div>
          </div>

          {isInitialLoading ? (
            <div className="p-4">
              <ModuleLoading count={6} label={t("platformExtract.tenants.loadingTenants")} variant="table" />
            </div>
          ) : (
            <DataTable<PlatformTenantItem>
              className="rhd-railops-platform-tenants-table"
              columns={columns}
              current={page}
              dataSource={data?.tenants ?? []}
              emptyDescription={t("platformExtract.tenants.empty")}
              footerNote={`${t("pagination.total", { total: data?.totalCount ?? 0 })}，${t("pagination.pageSummary", { page, totalPages: pageCount })}`}
              rowKey="id"
              scroll={{ x: 1520 }}
              size="middle"
              total={data?.totalCount ?? 0}
              onPageChange={(nextPage) => setPage(nextPage)}
            />
          )}
        </div>
      </div>

      <StandardModal
        open={canCreateTenant && createOpen}
        onCancel={() => setCreateOpen(false)}
        title={t("platformExtract.tenants.createTitle")}
        width={672}
        styles={{ body: { maxHeight: "90vh", overflowY: "auto" } }}
        footer={
          <>
            <RailopsButton onClick={() => setCreateOpen(false)} disabled={saving}>{t("platformExtract.tenants.cancel")}</RailopsButton>
            <RailopsButton variant="primary" htmlType="submit" form="create-tenant-form" disabled={saving}>{saving ? t("platformExtract.tenants.creating") : t("platformExtract.tenants.createTenant")}</RailopsButton>
          </>
        }
      >
        <form id="create-tenant-form" className="grid gap-4 sm:grid-cols-2" onSubmit={submitCreate}>
            <label className="grid gap-1.5 text-sm font-medium text-foreground sm:col-span-2">
              {t("platformExtract.tenants.enterpriseName")}
              <Input value={form.name} onChange={(event) => setForm((value) => ({ ...value, name: event.target.value }))} autoFocus />
            </label>
            <SelectField
              label={t("platformExtract.tenants.serviceScene")}
              style={{ marginBottom: 0 }}
              selectProps={{
                "aria-label": t("platformExtract.tenants.serviceScene"),
                value: form.serviceScene,
                onChange: (value) => setForm((current) => {
                  const serviceScene = value as TenantServiceScene
                  return { ...current, serviceScene, aiEnabled: serviceScene === "knowledge_support" ? true : current.aiEnabled }
                }),
                options: tenantServiceSceneOptions.map((option) => ({ value: option.value, label: t(option.labelKey) })),
                style: { width: "100%" },
              }}
            />
            <div className="self-end pb-1 text-xs leading-5 text-muted-foreground">
              {form.serviceScene === "knowledge_support"
                ? t("platformExtract.tenants.knowledgeSupportSceneHint")
                : t("platformExtract.tenants.equipmentAfterSalesSceneHint")}
            </div>
            <TenantAICapabilityField
              enabled={form.aiEnabled}
              serviceScene={form.serviceScene}
              onChange={(checked) => setForm((value) => ({ ...value, aiEnabled: checked }))}
              t={t}
            />
            <TenantPortalSettingsFields
              value={form}
              onChange={(patch) => setForm((current) => ({ ...current, ...patch }))}
              disabled={saving}
              t={t}
            />
            <div className="border-t border-border pt-4 text-sm font-semibold text-foreground sm:col-span-2">
              {t("platformExtract.tenants.adminInfo")}
            </div>
            <label className="grid gap-1.5 text-sm font-medium text-foreground">
              {t("platformExtract.tenants.loginAccount")}
              <Input value={form.adminUsername} onChange={(event) => setForm((value) => ({ ...value, adminUsername: event.target.value }))} autoComplete="off" placeholder="atlas.admin" />
            </label>
            <label className="grid gap-1.5 text-sm font-medium text-foreground">
              {t("platformExtract.tenants.adminName")}
              <Input value={form.adminNickname} onChange={(event) => setForm((value) => ({ ...value, adminNickname: event.target.value }))} placeholder="Anna Schmidt" />
            </label>
            <label className="grid gap-1.5 text-sm font-medium text-foreground">
              {t("platformExtract.tenants.email")}
              <Input type="email" value={form.adminEmail} onChange={(event) => setForm((value) => ({ ...value, adminEmail: event.target.value }))} placeholder="admin@example.com" />
            </label>
            <label className="grid gap-1.5 text-sm font-medium text-foreground">
              {t("platformExtract.tenants.mobile")}
              <Input value={form.adminMobile} onChange={(event) => setForm((value) => ({ ...value, adminMobile: event.target.value }))} placeholder="+49 30 000000" />
            </label>
            <label className="grid gap-1.5 text-sm font-medium text-foreground sm:col-span-2">
              {t("platformExtract.tenants.initialPassword")}
              <Input type="password" value={form.adminPassword} onChange={(event) => setForm((value) => ({ ...value, adminPassword: event.target.value }))} autoComplete="new-password" placeholder={t("platformExtract.tenants.passwordHint")} />
            </label>
            <SelectField
              label={t("platformExtract.tenants.industry")}
              style={{ marginBottom: 0 }}
              selectProps={{
                "aria-label": t("platformExtract.tenants.industry"),
                placeholder: t("platformExtract.tenants.selectIndustry"),
                value: form.industry || undefined,
                onChange: (value) => setForm((current) => ({ ...current, industry: (value as string) ?? "" })),
                options: localizedIndustryOptions,
                style: { width: "100%" },
              }}
            />
            <SelectField
              label={t("platformExtract.tenants.countryRegion")}
              style={{ marginBottom: 0 }}
              selectProps={{
                "aria-label": t("platformExtract.tenants.countryRegion"),
                placeholder: t("platformExtract.tenants.selectCountry"),
                value: form.countryRegion || undefined,
                onChange: (value) => setForm((current) => ({ ...current, countryRegion: (value as string) ?? "" })),
                options: tenantCountryOptions,
                style: { width: "100%" },
              }}
            />
            <SelectField
              label={t("platformExtract.tenants.dataRegion")}
              style={{ marginBottom: 0 }}
              selectProps={{
                "aria-label": t("platformExtract.tenants.dataRegion"),
                placeholder: t("platformExtract.tenants.selectDataRegion"),
                value: form.dataRegion || undefined,
                onChange: (value) => setForm((current) => ({ ...current, dataRegion: (value as string) ?? "" })),
                options: tenantDataRegionOptions.map((option) => ({
                  value: option.value,
                  label: option.detail ? `${option.label} · ${option.detail}` : option.label,
                })),
                style: { width: "100%" },
              }}
            />
            <SelectField
              label={t("platformExtract.tenants.defaultLanguage")}
              style={{ marginBottom: 0 }}
              selectProps={{
                "aria-label": t("platformExtract.tenants.defaultLanguage"),
                value: form.defaultLocale,
                onChange: (value) => setForm((current) => ({
                  ...current,
                  defaultLocale: value as string,
                  languagePreferences: withLanguageDefaults(value as string, current.customerDefaultLocale, current.languagePreferences),
                })),
                options: localizedPortalLanguageOptions,
                style: { width: "100%" },
              }}
            />
            <SelectField
              label={t("platformExtract.tenants.customerDefaultLanguage")}
              style={{ marginBottom: 0 }}
              selectProps={{
                "aria-label": t("platformExtract.tenants.customerDefaultLanguage"),
                value: form.customerDefaultLocale,
                onChange: (value) => setForm((current) => ({
                  ...current,
                  customerDefaultLocale: value as string,
                  languagePreferences: withLanguageDefaults(current.defaultLocale, value as string, current.languagePreferences),
                })),
                options: localizedPortalLanguageOptions,
                style: { width: "100%" },
              }}
            />
            <SelectField
              label={t("platformExtract.tenants.supportedLanguages")}
              style={{ marginBottom: 0 }}
              selectProps={{
                "aria-label": t("platformExtract.tenants.supportedLanguages"),
                mode: "multiple",
                placeholder: t("platformExtract.tenants.selectLanguage"),
                value: form.languagePreferences,
                onChange: (value) => setForm((current) => ({
                  ...current,
                  languagePreferences: withLanguageDefaults(current.defaultLocale, current.customerDefaultLocale, (value as string[]) ?? []),
                })),
                options: localizedLanguageOptions,
                style: { width: "100%" },
              }}
            />
            <SelectField
              label={t("platformExtract.tenants.timezone")}
              style={{ marginBottom: 0 }}
              selectProps={{
                "aria-label": t("platformExtract.tenants.timezone"),
                mode: "multiple",
                placeholder: t("platformExtract.tenants.selectTimezone"),
                value: form.timezonePreferences,
                onChange: (value) => setForm((current) => ({ ...current, timezonePreferences: (value as string[]) ?? [] })),
                options: tenantTimezoneOptions,
                style: { width: "100%" },
              }}
            />
            <label className="grid gap-1.5 text-sm font-medium text-foreground sm:col-span-2">
              {t("platformExtract.tenants.trialEndsAt")}
              <Input type="date" value={form.trialEndsAt} onChange={(event) => setForm((value) => ({ ...value, trialEndsAt: event.target.value }))} />
            </label>
            {formError ? <div className="border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive sm:col-span-2">{formError}</div> : null}
          </form>
      </StandardModal>

      <StandardModal
        open={Boolean(editingTenant)}
        onCancel={() => { if (!editSaving) setEditingTenant(null) }}
        title={t("platformExtract.tenants.editTitle")}
        width={672}
        rootClassName="rhd-platform-tenant-edit-modal"
        footer={
          <>
            <RailopsButton onClick={() => setEditingTenant(null)} disabled={editSaving}>{t("platformExtract.tenants.cancel")}</RailopsButton>
            <RailopsButton variant="primary" htmlType="submit" form="edit-tenant-form" disabled={editSaving}>{editSaving ? t("platformExtract.tenants.saving") : t("platformExtract.tenants.saveChanges")}</RailopsButton>
          </>
        }
      >
        <form id="edit-tenant-form" className="space-y-4" onSubmit={submitEdit}>
          <UnderlineTabs
            ariaLabel={t("platformExtract.tenants.editTabsAria")}
            items={editTabItems}
            value={editActiveTab}
            onChange={(value) => setEditActiveTab(value as TenantEditTab)}
          />
          {editError ? (
            <div className="border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">
              {editError}
            </div>
          ) : null}

          {editActiveTab === "profile" ? (
            <div className="grid gap-4 sm:grid-cols-2" role="tabpanel" aria-label={t("platformExtract.tenants.editTabProfile")}>
              <label className="grid gap-1.5 text-sm font-medium text-foreground sm:col-span-2">
                {t("platformExtract.tenants.enterpriseName")}
                <Input value={editForm.name} onChange={(event) => setEditForm((value) => ({ ...value, name: event.target.value }))} autoFocus />
              </label>
              <SelectField
                label={t("platformExtract.tenants.industry")}
                style={{ marginBottom: 0 }}
                selectProps={{
                  "aria-label": t("platformExtract.tenants.industry"),
                  placeholder: t("platformExtract.tenants.selectIndustry"),
                  value: editForm.industry || undefined,
                  onChange: (value) => setEditForm((current) => ({ ...current, industry: (value as string) ?? "" })),
                  options: localizedIndustryOptions,
                  style: { width: "100%" },
                }}
              />
              <SelectField
                label={t("platformExtract.tenants.countryRegion")}
                style={{ marginBottom: 0 }}
                selectProps={{
                  "aria-label": t("platformExtract.tenants.countryRegion"),
                  placeholder: t("platformExtract.tenants.selectCountry"),
                  value: editForm.countryRegion || undefined,
                  onChange: (value) => setEditForm((current) => ({ ...current, countryRegion: (value as string) ?? "" })),
                  options: tenantCountryOptions,
                  style: { width: "100%" },
                }}
              />
              <SelectField
                label={t("platformExtract.tenants.dataRegion")}
                style={{ marginBottom: 0, gridColumn: "1 / -1" }}
                selectProps={{
                  "aria-label": t("platformExtract.tenants.dataRegion"),
                  placeholder: t("platformExtract.tenants.selectDataRegion"),
                  value: editForm.dataRegion || undefined,
                  onChange: (value) => setEditForm((current) => ({ ...current, dataRegion: (value as string) ?? "" })),
                  options: tenantDataRegionOptions.map((option) => ({
                    value: option.value,
                    label: option.detail ? `${option.label} · ${option.detail}` : option.label,
                  })),
                  style: { width: "100%" },
                }}
              />
              <div className="flex min-h-10 items-center justify-between gap-4 border-t border-border pt-4 sm:col-span-2">
                <div>
                  <div className="text-sm font-medium text-foreground">{t("platformExtract.tenants.formalTenant")}</div>
                  <div className="mt-1 text-xs text-muted-foreground">{t("platformExtract.tenants.formalHint")}</div>
                </div>
                <Switch
                  checked={editForm.formalTenant}
                  onCheckedChange={(checked) => setEditForm((value) => ({ ...value, formalTenant: checked, trialEndsAt: checked ? "" : value.trialEndsAt }))}
                  aria-label={t("platformExtract.tenants.formalTenant")}
                />
              </div>
              {!editForm.formalTenant ? (
                <label className="grid gap-1.5 text-sm font-medium text-foreground sm:col-span-2">
                  {t("platformExtract.tenants.trialEndsAt")}
                  <Input type="date" value={editForm.trialEndsAt} onChange={(event) => setEditForm((value) => ({ ...value, trialEndsAt: event.target.value }))} />
                </label>
              ) : null}
            </div>
          ) : null}

          {editActiveTab === "service" ? (
            <div className="grid gap-4 sm:grid-cols-2" role="tabpanel" aria-label={t("platformExtract.tenants.editTabService")}>
              <SelectField
                label={t("platformExtract.tenants.serviceScene")}
                style={{ marginBottom: 0 }}
                selectProps={{
                  "aria-label": t("platformExtract.tenants.serviceScene"),
                  value: editForm.serviceScene,
                  onChange: (value) => setEditForm((current) => {
                    const serviceScene = value as TenantServiceScene
                    return { ...current, serviceScene, aiEnabled: serviceScene === "knowledge_support" ? true : current.aiEnabled }
                  }),
                  options: tenantServiceSceneOptions.map((option) => ({ value: option.value, label: t(option.labelKey) })),
                  style: { width: "100%" },
                }}
              />
              <div className="self-end pb-1 text-xs leading-5 text-muted-foreground">
                {editForm.serviceScene === "knowledge_support"
                  ? t("platformExtract.tenants.knowledgeSupportSceneHint")
                  : t("platformExtract.tenants.equipmentAfterSalesSceneHint")}
              </div>
              <TenantAICapabilityField
                enabled={editForm.aiEnabled}
                serviceScene={editForm.serviceScene}
                onChange={(checked) => setEditForm((value) => ({ ...value, aiEnabled: checked }))}
                t={t}
              />
            </div>
          ) : null}

          {editActiveTab === "branding" ? (
            <div className="grid gap-4 sm:grid-cols-2" role="tabpanel" aria-label={t("platformExtract.tenants.editTabBranding")}>
              <TenantPortalSettingsFields
                value={editForm}
                onChange={(patch) => setEditForm((current) => ({ ...current, ...patch }))}
                disabled={editSaving}
                showSectionHeading={false}
                t={t}
              />
            </div>
          ) : null}

          {editActiveTab === "localization" ? (
            <div className="grid gap-4 sm:grid-cols-2" role="tabpanel" aria-label={t("platformExtract.tenants.editTabLocalization")}>
              <SelectField
                label={t("platformExtract.tenants.defaultLanguage")}
                style={{ marginBottom: 0 }}
                selectProps={{
                  "aria-label": t("platformExtract.tenants.defaultLanguage"),
                  value: editForm.defaultLocale,
                  onChange: (value) => setEditForm((current) => ({
                    ...current,
                    defaultLocale: value as string,
                    languagePreferences: withLanguageDefaults(value as string, current.customerDefaultLocale, current.languagePreferences),
                  })),
                  options: localizedPortalLanguageOptions,
                  style: { width: "100%" },
                }}
              />
              <SelectField
                label={t("platformExtract.tenants.customerDefaultLanguage")}
                style={{ marginBottom: 0 }}
                selectProps={{
                  "aria-label": t("platformExtract.tenants.customerDefaultLanguage"),
                  value: editForm.customerDefaultLocale,
                  onChange: (value) => setEditForm((current) => ({
                    ...current,
                    customerDefaultLocale: value as string,
                    languagePreferences: withLanguageDefaults(current.defaultLocale, value as string, current.languagePreferences),
                  })),
                  options: localizedPortalLanguageOptions,
                  style: { width: "100%" },
                }}
              />
              <SelectField
                label={t("platformExtract.tenants.supportedLanguages")}
                style={{ marginBottom: 0 }}
                selectProps={{
                  "aria-label": t("platformExtract.tenants.supportedLanguages"),
                  mode: "multiple",
                  placeholder: t("platformExtract.tenants.selectLanguage"),
                  value: editForm.languagePreferences,
                  onChange: (value) => setEditForm((current) => ({
                    ...current,
                    languagePreferences: withLanguageDefaults(current.defaultLocale, current.customerDefaultLocale, (value as string[]) ?? []),
                  })),
                  options: localizedLanguageOptions,
                  style: { width: "100%" },
                }}
              />
              <SelectField
                label={t("platformExtract.tenants.timezone")}
                style={{ marginBottom: 0, gridColumn: "1 / -1" }}
                selectProps={{
                  "aria-label": t("platformExtract.tenants.timezone"),
                  mode: "multiple",
                  placeholder: t("platformExtract.tenants.selectTimezone"),
                  value: editForm.timezonePreferences,
                  onChange: (value) => setEditForm((current) => ({ ...current, timezonePreferences: (value as string[]) ?? [] })),
                  options: tenantTimezoneOptions,
                  style: { width: "100%" },
                }}
              />
            </div>
          ) : null}
        </form>
      </StandardModal>

      <StandardModal
        open={Boolean(passwordTenant)}
        onCancel={closePasswordDialog}
        title={t("platformExtract.tenants.resetPassword")}
        width={448}
        footer={
          passwordResult ? (
            <RailopsButton variant="primary" onClick={closePasswordDialog}>{t("platformExtract.tenants.close")}</RailopsButton>
          ) : (
            <>
              <RailopsButton onClick={closePasswordDialog} disabled={resettingPassword}>{t("platformExtract.tenants.close")}</RailopsButton>
              <RailopsButton
                variant="primary"
                htmlType="submit"
                form="tenant-admin-password-form"
                disabled={resettingPassword || adminPassword.length < 8}
              >
                {resettingPassword ? t("platformExtract.tenants.resetting") : t("platformExtract.tenants.confirmReset")}
              </RailopsButton>
            </>
          )
        }
      >
        <form id="tenant-admin-password-form" className="space-y-4" onSubmit={submitAdminPassword}>
            {passwordTenant ? (
              <div className="border border-border bg-muted px-3 py-2 text-sm text-muted-foreground">
                {passwordTenant.name} · {t("platformExtract.tenants.submitSessionsInvalidated")}
              </div>
            ) : null}
            {!passwordResult ? (
              <label className="grid gap-1.5 text-sm font-medium text-foreground">
                {t("platformExtract.tenants.newPassword")}
                <Input
                  type="password"
                  value={adminPassword}
                  onChange={(event) => {
                    setAdminPassword(event.target.value)
                    setPasswordError("")
                  }}
                  autoComplete="new-password"
                  placeholder={t("platformExtract.tenants.passwordHint")}
                />
                <span className="text-xs font-normal text-muted-foreground">{t("platformExtract.tenants.passwordHint")}</span>
              </label>
            ) : null}
            {passwordError ? (
              <div className="border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive" role="alert">
                {passwordError}
              </div>
            ) : null}
            {passwordResult ? (
              <div className="border-l-2 border-primary/20 bg-muted px-3 py-2 text-sm text-foreground">
                <div className="font-medium">{t("platformExtract.tenants.passwordResetDone")}</div>
                <div className="mt-1 text-xs">{t("platformExtract.tenants.passwordResultAccount")}：{passwordResult.username} · {t("platformExtract.tenants.passwordResultName")}：{passwordResult.displayName || t("platformExtract.tenants.notSet")}</div>
              </div>
            ) : null}
          </form>
      </StandardModal>

      <StandardModal
        open={Boolean(statusTenant)}
        onCancel={() => { if (actingTenantId === null) setStatusTenant(null) }}
        title={statusTenant?.status === 1 ? t("platformExtract.tenants.unfreeze") : t("platformExtract.tenants.freeze")}
        width={448}
        footer={
          <>
            <RailopsButton onClick={() => setStatusTenant(null)} disabled={actingTenantId !== null}>{t("platformExtract.tenants.cancel")}</RailopsButton>
            <RailopsButton
              danger={statusTenant?.status !== 1}
              onClick={() => void confirmTenantStatusChange()}
              disabled={!statusTenant || actingTenantId !== null || (statusTenant.status !== 1 && !freezeReason.trim())}
            >
              {actingTenantId !== null
                ? t("platformExtract.tenants.processing")
                : statusTenant?.status === 1
                  ? t("platformExtract.tenants.confirmUnfreeze")
                  : t("platformExtract.tenants.confirmFreeze")}
            </RailopsButton>
          </>
        }
      >
          <div className="rounded-lg border border-border bg-muted px-4 py-3 text-sm">
            <div className="flex items-center justify-between gap-4">
              <span className="text-muted-foreground">{t("platformExtract.tenants.enterprise")}</span>
              <span className="font-medium text-foreground">{statusTenant?.name}</span>
            </div>
            <div className="mt-2 flex items-center justify-between gap-4">
              <span className="text-muted-foreground">{t("platformExtract.tenants.statusChange")}</span>
              <span className={statusTenant?.status === 1 ? "font-medium text-primary" : "font-medium text-destructive"}>
                {statusTenant?.status === 1 ? t("platformExtract.tenants.statusFrozenToNormal") : t("platformExtract.tenants.statusNormalToFrozen")}
              </span>
            </div>
            {statusTenant?.status === 1 && statusTenant.frozenReason ? (
              <div className="mt-2 flex items-start justify-between gap-4 border-t border-border pt-2">
                <span className="shrink-0 text-muted-foreground">{t("platformExtract.tenants.freezeReason")}</span>
                <span className="text-right text-foreground">{statusTenant.frozenReason}</span>
              </div>
            ) : null}
          </div>
          {statusTenant?.status !== 1 ? (
            <label className="grid gap-1.5 text-sm font-medium text-foreground">
              {t("platformExtract.tenants.freezeReason")}
              <Input.TextArea
                value={freezeReason}
                onChange={(event) => setFreezeReason(event.target.value)}
                maxLength={200}
                placeholder={t("platformExtract.tenants.freezeReasonPlaceholder")}
                aria-label={t("platformExtract.tenants.freezeReason")}
              />
              <span className="text-xs font-normal text-muted-foreground">{t("platformExtract.tenants.freezeReasonAudit")}</span>
            </label>
          ) : null}
          {statusError ? <div className="border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive">{statusError}</div> : null}
      </StandardModal>
    </PageShell>
  )
}
