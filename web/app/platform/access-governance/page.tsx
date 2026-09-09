"use client"

import { useCallback, useEffect, useRef, useState, type ReactNode } from "react"
import {
  ActivityIcon,
  CableIcon,
  KeyRoundIcon,
  LanguagesIcon,
  LoaderCircleIcon,
  MailIcon,
  PlugZapIcon,
  RefreshCwIcon,
  Settings2Icon,
  VideoIcon,
} from "lucide-react"
import { ContentModule, PageShell, RailopsButton, StatCard, StatusTag, type StatCardTone, type StatusTagTone } from "@railops/ui"
import { toast } from "sonner"

import {
  buildPlatformIntegrationMutation,
  PlatformIntegrationConfigDialog,
} from "@/components/platform/platform-integration-config-dialog"
import {
  PlatformError,
  formatDateTime,
  formatNumber,
} from "@/components/platform/platform-live-ui"
import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { ModuleLoading } from "@/components/shared/loading-states"
import { useAppLocale, useI18n } from "@/i18n/provider"
import {
  fetchPlatformIntegrationSettings,
  fetchPlatformOpsAccess,
  fetchPlatformOpsJitsi,
  fetchPlatformOpsSpeech,
  fetchPlatformOpsSub2API,
  testPlatformIntegration,
  type PlatformIntegrationKind,
  type PlatformIntegrationSettings,
  type PlatformIntegrationTestResult,
  type PlatformOpsAccess,
  type PlatformOpsJitsi,
  type PlatformOpsSpeech,
  type PlatformOpsSub2API,
} from "@/lib/api/platform"

type ServiceStatus = {
  label: string
  tone: "neutral" | "info" | "success" | "warning" | "danger"
}

type PlatformAccessGovernanceData = {
  access?: PlatformOpsAccess
  sub2api?: PlatformOpsSub2API
  jitsi?: PlatformOpsJitsi
  speech?: PlatformOpsSpeech
  settings?: PlatformIntegrationSettings
}
type PlatformAccessGovernanceDataKey = keyof PlatformAccessGovernanceData
type PlatformAccessGovernanceLoadState = Record<PlatformAccessGovernanceDataKey, boolean>

const accessGovernanceDataKeys: PlatformAccessGovernanceDataKey[] = [
  "access",
  "sub2api",
  "jitsi",
  "speech",
  "settings",
]

function createAccessGovernanceLoadState(value: boolean): PlatformAccessGovernanceLoadState {
  return {
    access: value,
    sub2api: value,
    jitsi: value,
    speech: value,
    settings: value,
  }
}

function providerStatus(t: ReturnType<typeof useI18n>, configured?: boolean, enabled = true): ServiceStatus {
  if (configured === undefined) return { label: t("platformExtract.access.statusUnavailable"), tone: "neutral" }
  if (configured && enabled) return { label: t("platformExtract.access.status.configured"), tone: "success" }
  if (configured) return { label: t("platformExtract.access.status.pendingEnable"), tone: "warning" }
  return { label: t("platformExtract.access.status.notEnabled"), tone: "neutral" }
}

function jitsiStatus(t: ReturnType<typeof useI18n>, status?: string): ServiceStatus {
  if (!status) return { label: t("platformExtract.common.statusUnavailable"), tone: "neutral" }
  if (status === "healthy") return { label: t("platformExtract.access.status.healthy"), tone: "success" }
  if (status === "unhealthy") return { label: t("platformExtract.access.status.unhealthy"), tone: "danger" }
  return { label: t("platformExtract.access.status.notConfigured"), tone: "neutral" }
}

function sub2apiStatus(t: ReturnType<typeof useI18n>, data?: PlatformOpsSub2API["sub2api"], settings?: PlatformIntegrationSettings): ServiceStatus {
  if (!data && !settings) return { label: t("platformExtract.common.statusUnavailable"), tone: "neutral" }
  if (!settings?.sub2api?.host || !settings.sub2api.adminApiKeyConfigured) return { label: t("platformExtract.access.status.notConfigured"), tone: "neutral" }
  if ((data?.attentionAccounts ?? 0) > 0 || (data?.staleCredentials24h ?? 0) > 0) return { label: t("platformExtract.access.status.needsAttention"), tone: "warning" }
  return { label: t("platformExtract.access.status.healthy"), tone: "success" }
}

type PlatformMetricTone = "blue" | "success" | "amber" | "red" | "slate"

const metricToneMap: Record<PlatformMetricTone, StatCardTone> = {
  blue: "blue",
  success: "success",
  amber: "warning",
  red: "error",
  slate: "neutral",
}

const statusTagToneMap: Record<ServiceStatus["tone"], StatusTagTone> = {
  neutral: "neutral",
  info: "blue",
  success: "success",
  warning: "warning",
  danger: "error",
}

export default function PlatformAccessGovernancePage() {
  const t = useI18n()
  const { locale } = useAppLocale()
  const [data, setData] = useState<PlatformAccessGovernanceData | null>(null)
  const [loading, setLoading] = useState<PlatformAccessGovernanceLoadState>(() => createAccessGovernanceLoadState(true))
  const [loaded, setLoaded] = useState<PlatformAccessGovernanceLoadState>(() => createAccessGovernanceLoadState(false))
  const [error, setError] = useState("")
  const [configuring, setConfiguring] = useState<PlatformIntegrationKind | null>(null)
  const [testing, setTesting] = useState<PlatformIntegrationKind | null>(null)
  const [testResults, setTestResults] = useState<Partial<Record<PlatformIntegrationKind, PlatformIntegrationTestResult>>>({})
  const requestInFlight = useRef(false)

  const load = useCallback(async (silent = false) => {
    if (requestInFlight.current) return
    requestInFlight.current = true
    if (!silent) setLoading(createAccessGovernanceLoadState(true))
    setError("")
    const failures: unknown[] = []

    const loadOne = async <K extends PlatformAccessGovernanceDataKey>(
      key: K,
      request: () => Promise<NonNullable<PlatformAccessGovernanceData[K]>>
    ) => {
      try {
        const value = await request()
        setData((current) => ({ ...(current ?? {}), [key]: value }) as PlatformAccessGovernanceData)
        setLoaded((current) => ({ ...current, [key]: true }))
      } catch (err) {
        failures.push(err)
      } finally {
        if (!silent) setLoading((current) => ({ ...current, [key]: false }))
      }
    }

    try {
      await Promise.all([
        loadOne("access", fetchPlatformOpsAccess),
        loadOne("sub2api", fetchPlatformOpsSub2API),
        loadOne("jitsi", fetchPlatformOpsJitsi),
        loadOne("speech", fetchPlatformOpsSpeech),
        loadOne("settings", fetchPlatformIntegrationSettings),
      ])
      if (failures.length === accessGovernanceDataKeys.length) {
        const firstFailure = failures[0]
        throw firstFailure
      }
      if (failures.length > 0) setError(t("platformExtract.access.statusApiUnavailableCount", { count: failures.length }))
    } catch (err) {
      setError(err instanceof Error ? err.message : t("platformExtract.access.dataUnavailable"))
    } finally {
      requestInFlight.current = false
      if (!silent) setLoading(createAccessGovernanceLoadState(false))
    }
  }, [t])

  useEffect(() => {
    void load()
    const timer = window.setInterval(() => void load(true), 30_000)
    return () => window.clearInterval(timer)
  }, [load])

  const access = data?.access?.access
  const sub2api = data?.sub2api?.sub2api
  const jitsi = data?.jitsi?.jitsi
  const speech = data?.speech?.speech
  const settings = data?.settings
  const connectorAlerts = (access?.degradedConnectors ?? 0) + (access?.unhealthyConnectors ?? 0)
  const jitsiState = jitsiStatus(t, jitsi?.serviceStatus)
  const speechState = providerStatus(t, speech?.configured, speech?.jigasiEnabled ?? false)
  const sub2State = sub2apiStatus(t, sub2api, settings)
  const smtpState = providerStatus(t, settings?.smtp?.configured)
  const integrationStates = [jitsiState, speechState, sub2State, smtpState]
  const configuredCount = integrationStates.filter((status) => status.tone === "success").length
  const integrationAlerts = integrationStates.filter((status) => status.tone === "warning" || status.tone === "danger").length
  const integrationTotal = integrationStates.length
  const generatedAt = [data?.access, data?.sub2api, data?.jitsi, data?.speech]
    .map((item) => item?.generatedAt || "")
    .sort()
    .at(-1) || undefined
  const hasData = data !== null
  const isRefreshing = accessGovernanceDataKeys.some((key) => loading[key])
  const accessInitialLoading = loading.access && !loaded.access
  const sub2apiInitialLoading = loading.sub2api && !loaded.sub2api
  const jitsiInitialLoading = loading.jitsi && !loaded.jitsi
  const speechInitialLoading = loading.speech && !loaded.speech
  const settingsInitialLoading = loading.settings && !loaded.settings
  const jitsiCardLoading = jitsiInitialLoading || settingsInitialLoading
  const speechCardLoading = speechInitialLoading || settingsInitialLoading
  const sub2apiCardLoading = sub2apiInitialLoading || settingsInitialLoading
  const smtpCardLoading = settingsInitialLoading
  const integrationSummaryLoading = jitsiCardLoading || speechCardLoading || sub2apiCardLoading || smtpCardLoading
  const alertSummaryLoading = accessInitialLoading || integrationSummaryLoading

  async function handleTest(kind: PlatformIntegrationKind) {
    if (!settings) return
    setTesting(kind)
    try {
      const result = await testPlatformIntegration(buildPlatformIntegrationMutation(kind, settings))
      setTestResults((current) => ({ ...current, [kind]: result }))
      if (result.success) toast.success(t("platformExtract.access.testPassedToast"))
      else toast.error(result.message || t("platformExtract.access.testFailedToast"))
      void load(true)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("platformExtract.access.testFailedToast"))
    } finally {
      setTesting(null)
    }
  }

  return (
    <PageShell
      title={t("platformExtract.access.pageTitle")}
      breadcrumb={useRouteBreadcrumbItems()}
      className="rhd-railops-platform-access-page min-h-[calc(100vh-97px)]"
      actions={
        <RailopsButton onClick={() => void load()} disabled={isRefreshing}>
          <RefreshCwIcon className={isRefreshing ? "animate-spin" : ""} data-icon="inline-start" />
          {t("platformExtract.access.refreshStatus")}
        </RailopsButton>
      }
    >

      <div className="space-y-4">
        {!isRefreshing && error && !hasData ? <PlatformError message={error} onRetry={() => void load()} /> : null}
        {error && hasData ? (
          <div className="rhd-railops-platform-ops-alert">
            <StatusTag tone="warning" className="rhd-railops-platform-status">{t("platformExtract.access.partiallyAbnormal")}</StatusTag>
            <span>{error}</span>
          </div>
        ) : null}

        <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4" aria-label={t("platformExtract.access.metricsAria")}>
            <StatCard
              label={t("platformExtract.access.metric.externalIntegrations")}
              value={integrationSummaryLoading ? "-" : `${configuredCount} / ${integrationTotal}`}
              note={<StatusTag tone={integrationSummaryLoading ? "neutral" : integrationAlerts > 0 ? "warning" : "blue"} className="rhd-railops-platform-metric-detail">{integrationSummaryLoading ? t("platformExtract.common.loading") : t("platformExtract.access.itemsNeedAttention", { count: formatNumber(integrationAlerts, locale) })}</StatusTag>}
              icon={<PlugZapIcon className="size-4" />}
              tone={integrationSummaryLoading ? "neutral" : integrationAlerts > 0 ? "warning" : "blue"}
              className="rhd-railops-platform-metric"
            />
            <StatCard
              label={t("platformExtract.access.metric.connectorTotal")}
              value={accessInitialLoading ? "-" : access ? formatNumber(access.totalConnectors) : "-"}
              note={<StatusTag tone={accessInitialLoading ? "neutral" : "blue"} className="rhd-railops-platform-metric-detail">{accessInitialLoading ? t("platformExtract.common.loading") : access ? t("platformExtract.access.enabledCount", { count: formatNumber(access.activeConnectors, locale) }) : t("platformExtract.common.statusUnavailable")}</StatusTag>}
              icon={<CableIcon className="size-4" />}
              tone={accessInitialLoading ? "neutral" : "blue"}
              className="rhd-railops-platform-metric"
            />
            <StatCard
              label={t("platformExtract.access.metric.productApiCredentials")}
              value={sub2apiInitialLoading ? "-" : sub2api ? formatNumber(sub2api.productCredentials) : "-"}
              note={<StatusTag tone={sub2apiInitialLoading ? "neutral" : !sub2api ? "neutral" : sub2api.staleCredentials24h > 0 ? "warning" : "blue"} className="rhd-railops-platform-metric-detail">{sub2apiInitialLoading ? t("platformExtract.common.loading") : sub2api ? t("platformExtract.access.stale24hCount", { count: formatNumber(sub2api.staleCredentials24h, locale) }) : t("platformExtract.common.statusUnavailable")}</StatusTag>}
              icon={<KeyRoundIcon className="size-4" />}
              tone={sub2apiInitialLoading ? "neutral" : !sub2api ? "neutral" : sub2api.staleCredentials24h > 0 ? "warning" : "blue"}
              className="rhd-railops-platform-metric"
            />
            <StatCard
              label={t("platformExtract.access.metric.accessExceptions")}
              value={alertSummaryLoading ? "-" : formatNumber(connectorAlerts + integrationAlerts)}
              note={<StatusTag tone={alertSummaryLoading ? "neutral" : connectorAlerts + integrationAlerts > 0 ? "error" : "blue"} className="rhd-railops-platform-metric-detail">{alertSummaryLoading ? t("platformExtract.common.loading") : t("platformExtract.access.exceptionTotal")}</StatusTag>}
              icon={<ActivityIcon className="size-4" />}
              tone={alertSummaryLoading ? "neutral" : connectorAlerts + integrationAlerts > 0 ? "error" : "blue"}
              className="rhd-railops-platform-metric"
            />
          </section>

        <section className="space-y-3" aria-labelledby="integration-services-title">
          <div className="px-0.5">
            <h2 id="integration-services-title" className="text-sm font-semibold text-foreground">{t("platformExtract.access.externalServicesTitle")}</h2>
          </div>
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
              <IntegrationCard icon={<VideoIcon className="size-4" />} name={t("platformExtract.access.integration.jitsi")} identity={jitsi?.serviceUrl || settings?.jitsi?.url || t("platformExtract.access.serviceUrlNotConfigured")} status={jitsiState} kind="jitsi" settings={settings} testing={testing} testResult={testResults.jitsi} loading={jitsiCardLoading} onConfigure={setConfiguring} onTest={handleTest}>
                <CardStat label="HTTP" value={jitsi?.probeHttpStatus ? String(jitsi.probeHttpStatus) : "-"} />
                <CardStat label={t("platformExtract.access.card.latency")} value={jitsi ? `${formatNumber(jitsi.probeLatencyMs, locale)} ms` : "-"} />
                <CardStat label={t("platformExtract.access.card.lastProbe")} value={formatDateTime(jitsi?.probeCheckedAt, locale)} wide />
              </IntegrationCard>

              <IntegrationCard icon={<LanguagesIcon className="size-4" />} name={t("platformExtract.access.integration.speech")} identity={speech?.provider || "disabled"} status={speechState} kind="speech" settings={settings} testing={testing} testResult={testResults.speech} loading={speechCardLoading} onConfigure={setConfiguring} onTest={handleTest}>
                <CardStat label="Provider" value={speech?.provider || "-"} />
                <CardStat label="Jigasi" value={!speech ? "-" : speech.jigasiEnabled ? t("platformExtract.common.enabled") : t("platformExtract.common.notEnabled")} />
                <CardStat label={t("platformExtract.access.credentialEncrypted")} value={settings?.speech && (settings.speech.xfyunApiKeyConfigured || settings.speech.aliyunApiKeyConfigured) ? t("platformExtract.integration.credentialEncrypted") : t("platformExtract.access.status.notConfigured")} wide />
              </IntegrationCard>

              <IntegrationCard icon={<CableIcon className="size-4" />} name="Sub2API" identity={settings?.sub2api?.host || t("platformExtract.access.serviceUrlNotConfigured")} status={sub2State} kind="sub2api" settings={settings} testing={testing} testResult={testResults.sub2api} loading={sub2apiCardLoading} onConfigure={setConfiguring} onTest={handleTest}>
                <CardStat label={t("platformExtract.access.healthyAccounts")} value={sub2api ? formatNumber(sub2api.healthyAccounts, locale) : "-"} />
                <CardStat label={t("platformExtract.access.card.productCredentials")} value={sub2api ? formatNumber(sub2api.productCredentials, locale) : "-"} />
                <CardStat label={t("platformExtract.access.card.adminApiKey")} value={settings?.sub2api?.adminApiKeyMasked || t("platformExtract.access.status.notConfigured")} />
                <CardStat label={t("platformExtract.access.lastCredentialSync")} value={formatDateTime(sub2api?.lastCredentialSyncAt, locale)} wide />
              </IntegrationCard>

              <IntegrationCard icon={<MailIcon className="size-4" />} name="SMTP Server" identity={settings?.smtp?.smtpHost || t("platformExtract.access.serviceUrlNotConfigured")} status={smtpState} kind="smtp" settings={settings} testing={testing} testResult={testResults.smtp} loading={smtpCardLoading} onConfigure={setConfiguring} onTest={handleTest}>
                <CardStat label={t("platformExtract.access.card.port")} value={settings?.smtp?.smtpPort ? String(settings.smtp.smtpPort) : "-"} />
                <CardStat label={t("platformExtract.access.card.connectionMode")} value={!settings ? "-" : settings?.smtp?.useTls ? "SSL/TLS" : t("platformExtract.access.smtp.starttlsOrPlain")} />
                <CardStat label={t("platformExtract.common.account")} value={settings?.smtp?.username || "-"} />
                <CardStat label={t("platformExtract.access.smtp.fromAddress")} value={settings?.smtp?.fromAddress || "-"} wide />
              </IntegrationCard>
          </div>
        </section>

        <div className="grid gap-4 xl:grid-cols-2">
          <ContentModule title={t("platformExtract.access.connectorTitle")} className="rhd-railops-platform-panel">
            {accessInitialLoading ? (
              <div className="p-4">
                <ModuleLoading count={4} label={t("platformExtract.access.connectorLoading")} variant="list" />
              </div>
            ) : (
              <dl className="divide-y divide-border px-4 text-sm">
                <StatusRow label={t("platformExtract.common.status.normal")} value={access?.healthyConnectors} tone="success" />
                <StatusRow label={t("platformExtract.access.status.degraded")} value={access?.degradedConnectors} tone={(access?.degradedConnectors ?? 0) > 0 ? "warning" : "neutral"} />
                <StatusRow label={t("platformExtract.access.status.abnormal")} value={access?.unhealthyConnectors} tone={(access?.unhealthyConnectors ?? 0) > 0 ? "danger" : "neutral"} />
                <StatusRow label={t("platformExtract.access.failedCalls24h")} value={access?.failedCalls24h} tone={(access?.failedCalls24h ?? 0) > 0 ? "danger" : "neutral"} />
              </dl>
            )}
          </ContentModule>

          <ContentModule title={t("platformExtract.access.sub2apiCredentialTitle")} className="rhd-railops-platform-panel">
            {sub2apiInitialLoading ? (
              <div className="p-4">
                <ModuleLoading count={4} label={t("platformExtract.access.sub2apiCredentialLoading")} variant="list" />
              </div>
            ) : (
              <dl className="divide-y divide-border px-4 text-sm">
                <StatusRow label={t("platformExtract.access.healthyAccounts")} value={sub2api?.healthyAccounts} tone="success" />
                <StatusRow label={t("platformExtract.access.itemsNeedAttention", { count: formatNumber(sub2api?.attentionAccounts ?? 0, locale) })} value={sub2api?.attentionAccounts} tone={(sub2api?.attentionAccounts ?? 0) > 0 ? "warning" : "neutral"} />
                <StatusRow label={t("platformExtract.access.card.productCredentials")} value={sub2api?.productCredentials} tone="info" />
                <div className="flex items-center justify-between gap-3 py-3"><dt className="text-muted-foreground">{t("platformExtract.access.lastCredentialSync")}</dt><dd className="font-medium text-foreground">{formatDateTime(sub2api?.lastCredentialSyncAt, locale)}</dd></div>
              </dl>
            )}
          </ContentModule>
        </div>
      </div>

      {settings && configuring ? (
        <PlatformIntegrationConfigDialog
          open
          kind={configuring}
          settings={settings}
          onOpenChange={(open) => { if (!open) setConfiguring(null) }}
          onSaved={(next) => { setData((current) => current ? { ...current, settings: next } : current); void load(true) }}
        />
      ) : null}
    </PageShell>
  )
}

const cardStyles: Record<ServiceStatus["tone"], { icon: string }> = {
  neutral: { icon: "bg-muted text-muted-foreground" },
  info: { icon: "bg-sky-50 text-sky-700" },
  success: { icon: "bg-primary/10 text-primary" },
  warning: { icon: "bg-amber-50 text-amber-700" },
  danger: { icon: "bg-destructive/10 text-destructive" },
}

function IntegrationCard({ icon, name, identity, status, kind, settings, testing, testResult, loading = false, onConfigure, onTest, children }: {
  icon: ReactNode
  name: string
  identity: string
  status: ServiceStatus
  kind: PlatformIntegrationKind
  settings?: PlatformIntegrationSettings
  testing: PlatformIntegrationKind | null
  testResult?: PlatformIntegrationTestResult
  loading?: boolean
  onConfigure: (kind: PlatformIntegrationKind) => void
  onTest: (kind: PlatformIntegrationKind) => Promise<void>
  children: ReactNode
}) {
  const t = useI18n()
  const styles = cardStyles[status.tone]
  return (
    <ContentModule
      title={
        <span className="rhd-railops-platform-integration-title">
          <span className={`rhd-railops-platform-integration-icon ${styles.icon}`}>{icon}</span>
          <span className="min-w-0">
            <span className="rhd-railops-platform-integration-name">{name}</span>
            <span className="rhd-railops-platform-integration-identity" title={identity}>{identity}</span>
          </span>
        </span>
      }
      extra={<StatusTag tone={statusTagToneMap[status.tone]} className="rhd-railops-platform-status">{status.label}</StatusTag>}
      className={`rhd-railops-platform-integration-card tone-${status.tone}`}
    >
      {loading ? (
        <div className="p-4">
          <ModuleLoading count={3} label={t("platformExtract.access.serviceStatusLoading", { name })} variant="detail" />
        </div>
      ) : (
        <dl className="rhd-railops-platform-integration-stats">{children}</dl>
      )}
      <div className="rhd-railops-platform-integration-actions">
        <span className={testResult?.success ? "text-rhd-xs text-primary" : testResult ? "text-rhd-xs text-destructive" : "text-rhd-xs text-muted-foreground"}>
          {testResult ? `${testResult.success ? t("platformExtract.access.testPassed") : t("platformExtract.access.testFailed")} · ${testResult.latencyMs} ms` : t("platformExtract.access.notManuallyTested")}
        </span>
        <div className="flex items-center gap-1.5">
          <RailopsButton size="small" variant="text" onClick={() => onConfigure(kind)} disabled={!settings}><Settings2Icon />{t("platformExtract.access.configure")}</RailopsButton>
          <RailopsButton size="small" onClick={() => void onTest(kind)} disabled={!settings || testing !== null}>{testing === kind ? <LoaderCircleIcon className="animate-spin" /> : <PlugZapIcon />}{t("platformExtract.access.testConnection")}</RailopsButton>
        </div>
      </div>
    </ContentModule>
  )
}

function CardStat({ label, value, wide = false }: { label: string; value: string; wide?: boolean }) {
  return <div className={wide ? "col-span-2 min-w-0" : "min-w-0"}><dt className="text-rhd-xs text-muted-foreground">{label}</dt><dd className="mt-0.5 truncate text-xs font-semibold text-foreground" title={value}>{value}</dd></div>
}

function StatusRow({ label, value, tone }: { label: string; value?: number; tone: "neutral" | "info" | "success" | "warning" | "danger" }) {
  const t = useI18n()
  return <div className="flex items-center justify-between gap-3 py-3"><dt className="text-muted-foreground">{label}</dt><dd><StatusTag tone={statusTagToneMap[value === undefined ? "neutral" : tone]} className="rhd-railops-platform-status">{value === undefined ? t("platformExtract.common.statusUnavailable") : formatNumber(value)}</StatusTag></dd></div>
}
