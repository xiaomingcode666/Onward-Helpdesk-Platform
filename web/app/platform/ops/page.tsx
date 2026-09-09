"use client"

import { useCallback, useEffect, useRef, useState, type ReactNode } from "react"
import {
  ActivityIcon,
  BellIcon,
  CableIcon,
  CircleCheckIcon,
  Clock3Icon,
  CpuIcon,
  DatabaseIcon,
  HardDriveIcon,
  Layers3Icon,
  RefreshCwIcon,
  RouterIcon,
  ServerCogIcon,
  ShieldCheckIcon,
  TriangleAlertIcon,
} from "lucide-react"
import { ContentModule, DataTable, FilterTabs, RailopsButton, StatusTag, type StatusTagTone } from "@railops/ui"
import type { TableColumnsType } from "antd"

import {
  PlatformError,
  formatBytes,
  formatDateTime,
  formatDuration,
  formatNumber,
} from "@/components/platform/platform-live-ui"
import { ModuleLoading } from "@/components/shared/loading-states"
import { useAppLocale, useI18n } from "@/i18n/provider"
import {
  fetchPlatformOpsAccess,
  fetchPlatformOpsDatabase,
  fetchPlatformOpsInfrastructure,
  fetchPlatformOpsKnowledgeQueue,
  fetchPlatformOpsNotificationQueue,
  fetchPlatformOpsPipeline,
  fetchPlatformOpsRuntime,
  fetchPlatformOpsSub2API,
  type PlatformInfrastructureDependency,
  type PlatformOpsAccess,
  type PlatformOpsDatabase,
  type PlatformOpsInfrastructure,
  type PlatformOpsKnowledgeQueue,
  type PlatformOpsNotificationQueue,
  type PlatformOpsPipeline,
  type PlatformOpsRuntime,
  type PlatformOpsSub2API,
} from "@/lib/api/platform"
import { cn } from "@/lib/utils"

type PlatformOpsData = {
  runtime?: PlatformOpsRuntime
  database?: PlatformOpsDatabase
  infrastructure?: PlatformOpsInfrastructure
  pipeline?: PlatformOpsPipeline
  knowledgeQueue?: PlatformOpsKnowledgeQueue
  notificationQueue?: PlatformOpsNotificationQueue
  sub2api?: PlatformOpsSub2API
  access?: PlatformOpsAccess
}
type PlatformOpsSection = "runtime" | "dependency" | "storage" | "alerts"
type PlatformOpsDataKey = keyof PlatformOpsData
type PlatformOpsLoadState = Record<PlatformOpsDataKey, boolean>

type AlertEvent = {
  id: string
  level: "P0" | "P1" | "P2"
  source: string
  title: string
  detail: string
  status: "alerting" | "attention"
}

const storageColors: Record<PlatformInfrastructureDependency["key"], string> = {
  database: "bg-primary",
  redis: "bg-sky-500",
  vector: "bg-violet-500",
  object_storage: "bg-amber-500",
}

const platformOpsDataKeys: PlatformOpsDataKey[] = [
  "runtime",
  "database",
  "infrastructure",
  "pipeline",
  "knowledgeQueue",
  "notificationQueue",
  "sub2api",
  "access",
]

function createOpsLoadState(value: boolean): PlatformOpsLoadState {
  return {
    runtime: value,
    database: value,
    infrastructure: value,
    pipeline: value,
    knowledgeQueue: value,
    notificationQueue: value,
    sub2api: value,
    access: value,
  }
}

function latestGeneratedAt(data: PlatformOpsData | null) {
  if (!data) return undefined
  return [
    data.runtime,
    data.database,
    data.infrastructure,
    data.pipeline,
    data.knowledgeQueue,
    data.notificationQueue,
    data.sub2api,
    data.access,
  ].map((item) => item?.generatedAt || "").sort().at(-1) || undefined
}

function healthStatusTagTone(tone: "success" | "warning" | "danger"): StatusTagTone {
  if (tone === "danger") return "error"
  return tone
}

export default function PlatformOpsPage() {
  const t = useI18n()
  const { locale } = useAppLocale()
  const [data, setData] = useState<PlatformOpsData | null>(null)
  const [activeSection, setActiveSection] = useState<PlatformOpsSection>("runtime")
  const [loading, setLoading] = useState<PlatformOpsLoadState>(() => createOpsLoadState(true))
  const [loaded, setLoaded] = useState<PlatformOpsLoadState>(() => createOpsLoadState(false))
  const [error, setError] = useState("")
  const requestInFlight = useRef(false)

  const load = useCallback(async (silent = false) => {
    if (requestInFlight.current) return
    requestInFlight.current = true
    if (!silent) setLoading(createOpsLoadState(true))
    setError("")
    const failures: unknown[] = []

    const loadOne = async <K extends PlatformOpsDataKey>(
      key: K,
      request: () => Promise<NonNullable<PlatformOpsData[K]>>
    ) => {
      try {
        const value = await request()
        setData((current) => ({ ...(current ?? {}), [key]: value }) as PlatformOpsData)
        setLoaded((current) => ({ ...current, [key]: true }))
      } catch (err) {
        failures.push(err)
      } finally {
        if (!silent) setLoading((current) => ({ ...current, [key]: false }))
      }
    }

    try {
      await Promise.all([
        loadOne("runtime", fetchPlatformOpsRuntime),
        loadOne("database", fetchPlatformOpsDatabase),
        loadOne("infrastructure", fetchPlatformOpsInfrastructure),
        loadOne("pipeline", fetchPlatformOpsPipeline),
        loadOne("knowledgeQueue", fetchPlatformOpsKnowledgeQueue),
        loadOne("notificationQueue", fetchPlatformOpsNotificationQueue),
        loadOne("sub2api", fetchPlatformOpsSub2API),
        loadOne("access", fetchPlatformOpsAccess),
      ])

      if (failures.length === platformOpsDataKeys.length) {
        const firstFailure = failures[0]
        throw firstFailure
      }
      if (failures.length > 0) setError(t("platformExtract.ops.partialLoadFailed", { count: failures.length }))
    } catch (err) {
      setError(err instanceof Error ? err.message : t("platformExtract.ops.loadFailed"))
    } finally {
      requestInFlight.current = false
      if (!silent) setLoading(createOpsLoadState(false))
    }
  }, [t])

  useEffect(() => {
    void load()
    const timer = window.setInterval(() => void load(true), 30_000)
    return () => window.clearInterval(timer)
  }, [load])

  const runtime = data?.runtime
  const database = data?.database?.database
  const infrastructure = data?.infrastructure
  const dependencies = infrastructure?.dependencies ?? []
  const configuredDependencies = dependencies.filter((item) => item.status !== "unconfigured")
  const unhealthyDependencies = dependencies.filter((item) => item.status === "unhealthy").length
  const degradedDependencies = dependencies.filter((item) => item.status === "degraded").length
  const healthyDependencies = dependencies.filter((item) => item.status === "healthy").length
  const pipeline = data?.pipeline?.pipeline
  const knowledgeIndex = data?.knowledgeQueue?.knowledgeIndex
  const notifications = data?.notificationQueue?.notifications
  const sub2api = data?.sub2api?.sub2api
  const access = data?.access?.access
  const queueFailures = (pipeline?.failedSyncs24h ?? 0) + (knowledgeIndex?.failedTasks24h ?? 0)
  const notificationFailures = notifications?.failedDeliveries24h ?? 0
  const runningJobs = (pipeline?.runningSyncs ?? 0) + (knowledgeIndex?.runningTasks ?? 0)
  const queueDepth = runningJobs + (knowledgeIndex?.pendingTasks ?? 0) + (notifications?.pendingNotifications ?? 0)
  const accessProblems = (access?.degradedConnectors ?? 0) + (access?.unhealthyConnectors ?? 0)
  const attentionAccounts = sub2api?.attentionAccounts ?? 0
  const staleCredentials = sub2api?.staleCredentials24h ?? 0
  const incidentCount = unhealthyDependencies + degradedDependencies + queueFailures + notificationFailures + accessProblems + attentionAccounts + staleCredentials
  const heapUsagePercent = runtime?.runtime.heapSysBytes
    ? Math.min(100, Math.round((runtime.runtime.heapAllocBytes / runtime.runtime.heapSysBytes) * 100))
    : 0
  const databaseUsagePercent = database?.maxOpen
    ? Math.min(100, Math.round((database.openConnections / database.maxOpen) * 100))
    : 0
  const dependencyHealthPercent = configuredDependencies.length
    ? Math.round((healthyDependencies / configuredDependencies.length) * 10000) / 100
    : 0
  const maximumLatency = Math.max(0, ...configuredDependencies.map((item) => item.latencyMs))
  const averageLatency = configuredDependencies.length
    ? Math.round(configuredDependencies.reduce((sum, item) => sum + item.latencyMs, 0) / configuredDependencies.length)
    : 0
  const riskPoints = unhealthyDependencies * 20
    + degradedDependencies * 8
    + Math.min(20, queueFailures * 4 + notificationFailures * 4)
    + Math.min(12, accessProblems * 4)
    + Math.min(12, attentionAccounts * 3 + staleCredentials * 2)
    + (heapUsagePercent >= 85 ? 12 : heapUsagePercent >= 70 ? 5 : 0)
    + (databaseUsagePercent >= 85 ? 12 : databaseUsagePercent >= 70 ? 5 : 0)
  const healthScore = Math.max(0, 100 - riskPoints)
  const healthTone = healthScore >= 90 ? "success" : healthScore >= 70 ? "warning" : "danger"
  const maximumTableBytes = Math.max(1, ...(infrastructure?.databaseTables ?? []).map((table) => table.sizeBytes))
  const generatedAt = latestGeneratedAt(data)
  const isRefreshing = platformOpsDataKeys.some((key) => loading[key])
  const hasAnyLoadedOps = platformOpsDataKeys.some((key) => loaded[key])
  const runtimeInitialLoading = loading.runtime && !loaded.runtime
  const databaseInitialLoading = loading.database && !loaded.database
  const infrastructureInitialLoading = loading.infrastructure && !loaded.infrastructure
  const pipelineInitialLoading = (loading.pipeline && !loaded.pipeline)
    || (loading.knowledgeQueue && !loaded.knowledgeQueue)
    || (loading.notificationQueue && !loaded.notificationQueue)
  const modelAccessInitialLoading = (loading.sub2api && !loaded.sub2api) || (loading.access && !loaded.access)
  const alertsInitialLoading = platformOpsDataKeys.some((key) => loading[key] && !loaded[key])
  const heroInitialLoading = isRefreshing && !hasAnyLoadedOps
  const hasData = data !== null

  const alertEvents = (() => {
    const events: AlertEvent[] = []
    for (const dependency of dependencies) {
      if (dependency.status === "healthy" || dependency.status === "unconfigured") continue
      events.push({
        id: `dependency-${dependency.key}`,
        level: dependency.status === "unhealthy" ? "P0" : "P1",
        source: dependency.provider,
        title: dependency.status === "unhealthy"
          ? t("platformExtract.ops.event.dependencyDown")
          : t("platformExtract.ops.event.dependencyDegraded"),
        detail: dependency.error || t("platformExtract.ops.detail.probeLatency", { value: formatNumber(dependency.latencyMs, locale) }),
        status: dependency.status === "unhealthy" ? "alerting" : "attention",
      })
    }
    if (queueFailures > 0) events.push({ id: "queue", level: "P1", source: t("platformExtract.ops.source.asyncJobs"), title: t("platformExtract.ops.event.queueFailed"), detail: t("platformExtract.ops.detail.failedIn24h", { count: formatNumber(queueFailures, locale) }), status: "attention" })
    if (notificationFailures > 0) events.push({ id: "notification", level: "P1", source: t("platformExtract.ops.source.notifications"), title: t("platformExtract.ops.event.notificationFailed"), detail: t("platformExtract.ops.detail.failedIn24h", { count: formatNumber(notificationFailures, locale) }), status: "attention" })
    if (attentionAccounts > 0 || staleCredentials > 0) events.push({ id: "sub2api", level: "P1", source: t("platformExtract.ops.source.sub2api"), title: t("platformExtract.ops.event.modelAccessAttention"), detail: t("platformExtract.ops.detail.modelAccessAttention", { accounts: formatNumber(attentionAccounts, locale), credentials: formatNumber(staleCredentials, locale) }), status: "attention" })
    if (accessProblems > 0) events.push({ id: "access", level: "P1", source: t("platformExtract.ops.source.connectors"), title: t("platformExtract.ops.event.connectorAbnormal"), detail: t("platformExtract.ops.detail.connectorAbnormal", { count: formatNumber(accessProblems, locale) }), status: "attention" })
    if (heapUsagePercent >= 85) events.push({ id: "heap", level: "P0", source: t("platformExtract.ops.source.goRuntime"), title: t("platformExtract.ops.event.heapHigh"), detail: t("platformExtract.ops.detail.currentUsage", { value: formatNumber(heapUsagePercent, locale) }), status: "alerting" })
    if (databaseUsagePercent >= 85) events.push({ id: "database-pool", level: "P0", source: t("platformExtract.ops.source.postgres"), title: t("platformExtract.ops.event.databasePoolHigh"), detail: t("platformExtract.ops.detail.currentUsage", { value: formatNumber(databaseUsagePercent, locale) }), status: "alerting" })
    return events
  })()
  const opsTabs = [
    { label: t("platformExtract.ops.section.concurrency"), value: "runtime", count: queueDepth },
    { label: t("platformExtract.ops.section.dependency"), value: "dependency", count: configuredDependencies.length },
    { label: t("platformExtract.ops.section.storage"), value: "storage", count: infrastructure?.databaseTables?.length ?? undefined },
    { label: t("platformExtract.ops.section.alerts"), value: "alerts", count: alertEvents.length },
  ]

  return (
    <div className="rhd-railops-platform-ops-page min-h-[calc(100vh-97px)] space-y-4">
      {!isRefreshing && error && !hasData ? <PlatformError message={error} onRetry={() => void load()} /> : null}
      {error && hasData ? (
        <div className="rhd-railops-platform-ops-alert">
          <StatusTag tone="warning">{t("platformExtract.ops.warning")}</StatusTag>
          <span>{error}</span>
        </div>
      ) : null}

      <ContentModule className="rhd-railops-platform-ops-hero">
          <div aria-busy={isRefreshing || undefined} aria-label={t("platformExtract.ops.overviewLabel")}>
        <div className="flex min-h-16 flex-wrap items-center justify-between gap-3 border-b border-border px-5 py-3">
          <div className="flex min-w-0 items-center gap-3">
            <span className="grid size-9 shrink-0 place-items-center rounded-md bg-primary/10 text-primary"><ActivityIcon className="size-5" /></span>
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <h1 className="text-base font-semibold text-foreground">{t("platformExtract.ops.pageTitle")}</h1>
                <StatusTag tone={heroInitialLoading ? "neutral" : healthStatusTagTone(healthTone)}>
                  {heroInitialLoading ? t("platformExtract.ops.syncing") : healthTone === "success" ? t("platformExtract.ops.ready") : healthTone === "warning" ? t("platformExtract.ops.warning") : t("platformExtract.ops.actionRequired")}
                </StatusTag>
              </div>
              <p className="mt-0.5 truncate text-rhd-xs text-muted-foreground">{t("platformExtract.ops.autoRefresh")}{generatedAt ? ` · ${formatDateTime(generatedAt, locale)}` : ""}</p>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <span className="hidden sm:inline-flex"><StatusTag tone="blue">{t("platformExtract.ops.liveSnapshot")}</StatusTag></span>
            <RailopsButton size="small" onClick={() => void load()} disabled={isRefreshing} aria-label={t("platformExtract.ops.refresh")}>
              <RefreshCwIcon className={isRefreshing ? "animate-spin" : ""} />
              {t("platformExtract.ops.refresh")}
            </RailopsButton>
          </div>
        </div>

        <div className="grid xl:grid-cols-[minmax(420px,1.15fr)_minmax(0,2fr)]">
              <div className="flex min-h-52 items-center gap-5 border-b border-border px-5 py-4 xl:border-r xl:border-b-0">
                <HealthGauge score={healthScore} tone={healthTone} loading={heroInitialLoading} t={t} />
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2 text-xs text-muted-foreground"><span className={cn("size-2 rounded-full", heroInitialLoading ? "bg-muted-foreground/40" : healthTone === "success" ? "bg-[var(--railops-success)]" : healthTone === "warning" ? "bg-amber-500" : "bg-destructive")} />{t("platformExtract.ops.liveInfo")}</div>
                  <div className="mt-2 flex flex-wrap items-baseline gap-x-6 gap-y-2">
                    <LiveValue value={runtimeInitialLoading ? "-" : runtime ? formatNumber(runtime.runtime.goroutines, locale) : "-"} unit={t("platformExtract.ops.goroutines")} />
                    <LiveValue value={runtimeInitialLoading ? "-" : runtime ? formatBytes(runtime.runtime.heapAllocBytes) : "-"} unit={t("platformExtract.ops.heap")} />
                  </div>
                  <div className="mt-4 grid grid-cols-2 gap-x-5 gap-y-3 border-t border-border pt-3">
                    <SmallStat label={t("platformExtract.ops.label.runtime")} value={runtimeInitialLoading ? t("platformExtract.common.loading") : runtime ? formatDuration(runtime.uptimeSeconds) : "-"} />
                    <SmallStat label={t("platformExtract.ops.label.goVersion")} value={runtimeInitialLoading ? t("platformExtract.common.loading") : runtime?.runtime.goVersion || "-"} />
                    <SmallStat label={t("platformExtract.ops.label.gcCount")} value={runtimeInitialLoading ? t("platformExtract.common.loading") : runtime ? formatNumber(runtime.runtime.gcCount, locale) : "-"} />
                    <SmallStat label={t("platformExtract.ops.label.signals")} value={alertsInitialLoading ? t("platformExtract.common.loading") : formatNumber(incidentCount, locale)} danger={incidentCount > 0} />
                  </div>
                </div>
              </div>

              <div className="grid sm:grid-cols-2 lg:grid-cols-3">
                <PrimaryMetric label={t("platformExtract.ops.label.dependencyHealth")} value={infrastructureInitialLoading ? "-" : `${dependencyHealthPercent.toFixed(2)}%`} tone={dependencyHealthPercent >= 100 ? "success" : dependencyHealthPercent >= 75 ? "warning" : "danger"} icon={<ShieldCheckIcon className="size-4" />} detail={infrastructureInitialLoading ? t("platformExtract.common.loading") : t("platformExtract.ops.detail.healthyAbnormal", { healthy: formatNumber(healthyDependencies, locale), abnormal: formatNumber(degradedDependencies + unhealthyDependencies, locale) })} />
                <PrimaryMetric label={t("platformExtract.ops.label.dependencyResponse")} value={infrastructureInitialLoading ? "-" : `${formatNumber(maximumLatency, locale)} ms`} unit="MAX" tone={maximumLatency > 100 ? "danger" : maximumLatency > 50 ? "warning" : "neutral"} icon={<Clock3Icon className="size-4" />} detail={infrastructureInitialLoading ? t("platformExtract.common.loading") : t("platformExtract.ops.detail.averageLatency", { value: formatNumber(averageLatency, locale) })} />
                <PrimaryMetric label={t("platformExtract.ops.label.database")} value={databaseInitialLoading ? "-" : database?.status === "healthy" ? t("platformExtract.ops.status.healthy") : t("platformExtract.ops.status.abnormal")} tone={database?.status === "healthy" ? "success" : "danger"} icon={<DatabaseIcon className="size-4" />} detail={databaseInitialLoading ? t("platformExtract.common.loading") : database ? t("platformExtract.ops.detail.databaseInUse", { latency: formatNumber(database.latencyMs, locale), inUse: formatNumber(database.inUse, locale) }) : t("platformExtract.common.statusUnavailable")} />
                <PrimaryMetric label={t("platformExtract.ops.label.queueDepth")} value={pipelineInitialLoading ? "-" : formatNumber(queueDepth, locale)} tone={queueFailures > 0 ? "danger" : queueDepth > 0 ? "warning" : "success"} icon={<Layers3Icon className="size-4" />} detail={pipelineInitialLoading ? t("platformExtract.common.loading") : t("platformExtract.ops.detail.runningFailed", { running: formatNumber(runningJobs, locale), failed: formatNumber(queueFailures, locale) })} />
                <PrimaryMetric label={t("platformExtract.ops.label.modelAccess")} value={loading.sub2api && !loaded.sub2api ? "-" : sub2api ? `${formatNumber(sub2api.healthyAccounts, locale)} / ${formatNumber(sub2api.tenantAccounts, locale)}` : "-"} tone={attentionAccounts > 0 ? "warning" : "success"} icon={<CpuIcon className="size-4" />} detail={loading.sub2api && !loaded.sub2api ? t("platformExtract.common.loading") : t("platformExtract.ops.detail.attentionCredentials", { attention: formatNumber(attentionAccounts, locale), credentials: formatNumber(sub2api?.productCredentials ?? 0, locale) })} />
                <PrimaryMetric label={t("platformExtract.ops.label.accessConnectors")} value={loading.access && !loaded.access ? "-" : access ? `${formatNumber(access.healthyConnectors, locale)} / ${formatNumber(access.totalConnectors, locale)}` : "-"} tone={accessProblems > 0 ? "warning" : "success"} icon={<CableIcon className="size-4" />} detail={loading.access && !loaded.access ? t("platformExtract.common.loading") : t("platformExtract.ops.detail.failedCalls", { count: formatNumber(access?.failedCalls24h ?? 0, locale) })} />
              </div>
            </div>

            <div className="grid border-t border-border sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-6">
              <ResourceCell icon={<CpuIcon className="size-4" />} label={t("platformExtract.ops.resource.heapMemory")} value={runtimeInitialLoading ? "-" : `${formatNumber(heapUsagePercent, locale)}%`} detail={runtimeInitialLoading ? t("platformExtract.common.loading") : runtime ? `${formatBytes(runtime.runtime.heapAllocBytes)} / ${formatBytes(runtime.runtime.heapSysBytes)}` : t("platformExtract.common.statusUnavailable")} abnormalValue={t("platformExtract.ops.status.abnormal")} percent={runtimeInitialLoading ? undefined : heapUsagePercent} />
              <ResourceCell icon={<DatabaseIcon className="size-4" />} label={t("platformExtract.ops.resource.databaseConnection")} value={databaseInitialLoading ? "-" : `${formatNumber(databaseUsagePercent, locale)}%`} detail={databaseInitialLoading ? t("platformExtract.common.loading") : database ? `${formatNumber(database.openConnections, locale)} / ${formatNumber(database.maxOpen, locale)}` : t("platformExtract.common.statusUnavailable")} abnormalValue={t("platformExtract.ops.status.abnormal")} percent={databaseInitialLoading ? undefined : databaseUsagePercent} />
              <ResourceCell icon={<HardDriveIcon className="size-4" />} label={t("platformExtract.ops.resource.databaseCapacity")} value={infrastructureInitialLoading ? "-" : dependencyStatusLabel(t, dependencies.find((item) => item.key === "database"))} detail={infrastructureInitialLoading ? t("platformExtract.common.loading") : formatBytes(dependencies.find((item) => item.key === "database")?.storedBytes ?? 0)} abnormalValue={t("platformExtract.ops.status.abnormal")} />
              <ResourceCell icon={<RouterIcon className="size-4" />} label={t("platformExtract.ops.resource.redis")} value={infrastructureInitialLoading ? "-" : dependencyStatusLabel(t, dependencies.find((item) => item.key === "redis"))} detail={infrastructureInitialLoading ? t("platformExtract.common.loading") : formatBytes(dependencies.find((item) => item.key === "redis")?.storedBytes ?? 0)} abnormalValue={t("platformExtract.ops.status.abnormal")} />
              <ResourceCell icon={<ServerCogIcon className="size-4" />} label={t("platformExtract.ops.resource.backgroundJobs")} value={pipelineInitialLoading ? "-" : queueFailures > 0 ? t("platformExtract.ops.status.abnormal") : t("platformExtract.ops.resource.normal")} detail={pipelineInitialLoading ? t("platformExtract.common.loading") : t("platformExtract.ops.detail.queuedRunning", { queued: formatNumber(queueDepth, locale), running: formatNumber(runningJobs, locale) })} abnormalValue={t("platformExtract.ops.status.abnormal")} />
              <ResourceCell icon={<BellIcon className="size-4" />} label={t("platformExtract.ops.resource.notificationDelivery")} value={loading.notificationQueue && !loaded.notificationQueue ? "-" : notificationFailures > 0 ? t("platformExtract.ops.status.abnormal") : t("platformExtract.ops.resource.normal")} detail={loading.notificationQueue && !loaded.notificationQueue ? t("platformExtract.common.loading") : t("platformExtract.ops.detail.sentFailed", { sent: formatNumber(notifications?.sentDeliveries24h ?? 0, locale), failed: formatNumber(notificationFailures, locale) })} abnormalValue={t("platformExtract.ops.status.abnormal")} />
            </div>
        </div>
      </ContentModule>

      <div className="rhd-railops-dashboard-tabs">
        <FilterTabs
          ariaLabel={t("platformExtract.ops.metricsAria")}
          items={opsTabs}
          value={activeSection}
          onChange={(value) => setActiveSection(value as PlatformOpsSection)}
        />
      </div>

      <section className="rhd-railops-platform-ops-section" role="tabpanel" aria-label={opsTabs.find((item) => item.value === activeSection)?.label}>
        {activeSection === "runtime" ? (
          <div className="grid gap-4 xl:grid-cols-[minmax(300px,0.85fr)_minmax(0,1fr)]">
            <DashboardPanel title={t("platformExtract.ops.panel.concurrency")} icon={<Layers3Icon className="size-4 text-primary" />}>
              {pipelineInitialLoading ? (
                <div className="p-4">
                  <ModuleLoading count={3} label={t("platformExtract.ops.panel.concurrencyLoading")} variant="list" />
                </div>
              ) : (
                <div className="divide-y divide-border px-4">
                  <WorkloadRow t={t} label={t("platformExtract.ops.panel.dataSync")} running={pipeline?.runningSyncs ?? 0} pending={0} failed={pipeline?.failedSyncs24h ?? 0} />
                  <WorkloadRow t={t} label={t("platformExtract.ops.panel.knowledgeIndex")} running={knowledgeIndex?.runningTasks ?? 0} pending={knowledgeIndex?.pendingTasks ?? 0} failed={knowledgeIndex?.failedTasks24h ?? 0} />
                  <WorkloadRow t={t} label={t("platformExtract.ops.resource.notificationDelivery")} running={0} pending={notifications?.pendingNotifications ?? 0} failed={notificationFailures} />
                </div>
              )}
            </DashboardPanel>

            <DashboardPanel title={t("platformExtract.ops.panel.modelAccessStatus")} icon={<CpuIcon className="size-4 text-primary" />}>
              {modelAccessInitialLoading ? (
                <div className="p-4">
                  <ModuleLoading count={4} label={t("platformExtract.ops.panel.modelAccessStatusLoading")} variant="metrics" />
                </div>
              ) : (
                <div className="grid grid-cols-2 divide-x divide-y divide-border">
                  <CompactMetric label={t("platformExtract.ops.label.modelAccount")} value={formatNumber(sub2api?.tenantAccounts ?? 0, locale)} detail={t("platformExtract.ops.detail.healthyCount", { count: formatNumber(sub2api?.healthyAccounts ?? 0, locale) })} />
                  <CompactMetric label={t("platformExtract.ops.label.productCredentials")} value={formatNumber(sub2api?.productCredentials ?? 0, locale)} detail={t("platformExtract.ops.detail.staleCount", { count: formatNumber(staleCredentials, locale) })} danger={staleCredentials > 0} />
                  <CompactMetric label={t("platformExtract.ops.label.connectors")} value={formatNumber(access?.totalConnectors ?? 0, locale)} detail={t("platformExtract.ops.detail.healthyCount", { count: formatNumber(access?.healthyConnectors ?? 0, locale) })} />
                  <CompactMetric label={t("platformExtract.ops.label.failedCalls24h")} value={formatNumber(access?.failedCalls24h ?? 0, locale)} detail={t("platformExtract.ops.detail.abnormalConnectors", { count: formatNumber(accessProblems, locale) })} danger={(access?.failedCalls24h ?? 0) > 0 || accessProblems > 0} />
                </div>
              )}
            </DashboardPanel>
          </div>
        ) : null}

        {activeSection === "dependency" ? (
          <div className="grid gap-4 xl:grid-cols-[minmax(360px,0.95fr)_minmax(0,1fr)]">
            <DashboardPanel title={t("platformExtract.ops.panel.dependencyResponse")} icon={<ActivityIcon className="size-4 text-primary" />} action={<span className="text-rhd-xs text-muted-foreground">{t("platformExtract.ops.label.probe")}</span>}>
              {infrastructureInitialLoading ? (
                <div className="p-4">
                  <ModuleLoading count={4} label={t("platformExtract.ops.panel.dependencyResponseLoading")} variant="list" />
                </div>
              ) : (
                <div className="space-y-3.5 px-5 py-4">
                  {configuredDependencies.length ? configuredDependencies.map((dependency) => (
                    <LatencyRow key={dependency.key} dependency={dependency} maximum={Math.max(1, maximumLatency)} />
                  )) : <EmptyState title={t("platformExtract.ops.panel.noDependencyData")} />}
                </div>
              )}
            </DashboardPanel>

            <DashboardPanel title={t("platformExtract.ops.panel.dependencyLatency")} icon={<Clock3Icon className="size-4 text-violet-600" />} action={<span className="text-rhd-xs text-muted-foreground">{t("platformExtract.ops.panel.dependencyProbeCount", { count: formatNumber(configuredDependencies.length, locale) })}</span>}>
              {infrastructureInitialLoading ? (
                <div className="p-4">
                  <ModuleLoading count={2} label={t("platformExtract.ops.panel.dependencyLatencyLoading")} variant="detail" />
                </div>
              ) : (
                <LatencyDistribution dependencies={configuredDependencies} />
              )}
            </DashboardPanel>
          </div>
        ) : null}

        {activeSection === "storage" ? (
          <div className="grid gap-4 xl:grid-cols-[minmax(360px,0.95fr)_minmax(0,1fr)]">
            <DashboardPanel title={t("platformExtract.ops.panel.storageDistribution")} icon={<HardDriveIcon className="size-4 text-violet-600" />} action={<span className="text-rhd-xs text-muted-foreground">{t("platformExtract.ops.panel.totalStorage", { value: formatBytes(infrastructure?.totalMeasuredBytes ?? 0) })}</span>}>
              {infrastructureInitialLoading ? (
                <div className="p-4">
                  <ModuleLoading count={3} label={t("platformExtract.ops.panel.storageDistributionLoading")} variant="detail" />
                </div>
              ) : (
                <div className="p-5">
                  <StorageStack dependencies={dependencies} />
                  <div className="mt-5 grid gap-x-6 gap-y-4 sm:grid-cols-2">
                    {dependencies.map((dependency) => <StorageLegend key={dependency.key} dependency={dependency} />)}
                  </div>
                </div>
              )}
            </DashboardPanel>

            <DashboardPanel title={t("platformExtract.ops.panel.databaseHotspot")} icon={<DatabaseIcon className="size-4 text-primary" />} action={<span className="text-rhd-xs text-muted-foreground">{t("platformExtract.ops.panel.top", { count: 6 })}</span>}>
              {infrastructureInitialLoading ? (
                <div className="p-4">
                  <ModuleLoading count={5} label={t("platformExtract.ops.panel.databaseHotspotLoading")} variant="list" />
                </div>
              ) : (
                <div className="divide-y divide-border px-4">
                  {infrastructure?.databaseTables?.length ? infrastructure.databaseTables.slice(0, 6).map((table, index) => (
                    <DatabaseHotspotRow key={table.name} t={t} table={table} rank={index + 1} maximum={maximumTableBytes} />
                  )) : <EmptyState title={t("platformExtract.ops.panel.tableLevelDataEmpty")} />}
                </div>
              )}
            </DashboardPanel>
          </div>
        ) : null}

        {activeSection === "alerts" ? (
          <DashboardPanel title={t("platformExtract.ops.panel.alertEvents")} icon={<TriangleAlertIcon className="size-4 text-destructive" />} action={<span className="text-rhd-xs text-muted-foreground">{t("platformExtract.ops.panel.currentSnapshotCount", { count: formatNumber(alertEvents.length, locale) })}</span>}>
            {alertsInitialLoading ? (
              <div className="p-4">
                <ModuleLoading count={4} label={t("platformExtract.ops.panel.alertEventsLoading")} variant="table" />
              </div>
            ) : (
              <AlertTable t={t} events={alertEvents} generatedAt={generatedAt} />
            )}
          </DashboardPanel>
        ) : null}
      </section>
    </div>
  )
}

function HealthGauge({ score, tone, loading = false, t }: { score: number; tone: "success" | "warning" | "danger"; loading?: boolean; t: ReturnType<typeof useI18n> }) {
  const color = loading ? "var(--railops-border)" : tone === "success" ? "var(--railops-success)" : tone === "warning" ? "var(--railops-warning)" : "var(--railops-error)"
  const label = loading ? t("platformExtract.ops.gauge.synced") : tone === "success" ? t("platformExtract.ops.gauge.healthy") : tone === "warning" ? t("platformExtract.ops.gauge.risk") : t("platformExtract.ops.gauge.abnormal")
  return (
    <div className="shrink-0 text-center">
      <div className="relative grid size-24 place-items-center rounded-full" style={{ background: `conic-gradient(${color} ${loading ? 360 : score * 3.6}deg, var(--railops-border) 0deg)` }}>
        <div className="absolute inset-2.5 rounded-full bg-card" />
        <div className="relative"><div className="text-2xl font-semibold tabular-nums text-foreground">{loading ? "-" : formatNumber(score)}</div><div className="mt-0.5 text-rhd-2xs text-muted-foreground">{t("platformExtract.ops.gauge.healthScore")}</div></div>
      </div>
      <div className={cn("mt-2 text-xs font-medium", loading ? "text-muted-foreground" : tone === "success" ? "text-[#047857]" : tone === "warning" ? "text-amber-600" : "text-destructive")}>{label}</div>
    </div>
  )
}

function LiveValue({ value, unit }: { value: string; unit: string }) {
  return <div className="min-w-0"><span className="text-2xl font-semibold tabular-nums text-foreground">{value}</span><span className="ml-1.5 text-rhd-xs font-medium text-muted-foreground">{unit}</span></div>
}

function SmallStat({ label, value, danger = false }: { label: string; value: string; danger?: boolean }) {
  return <div className="min-w-0"><div className="text-rhd-2xs text-muted-foreground">{label}</div><div className={cn("mt-1 truncate text-xs font-semibold tabular-nums", danger ? "text-destructive" : "text-foreground")} title={value}>{value}</div></div>
}

const metricToneClasses = {
  success: "text-[#047857]",
  warning: "text-amber-600",
  danger: "text-destructive",
  neutral: "text-foreground",
} as const

function PrimaryMetric({ label, value, unit, detail, tone, icon }: { label: string; value: string; unit?: string; detail: string; tone: keyof typeof metricToneClasses; icon: ReactNode }) {
  return (
    <div className="min-h-32 border-b border-r border-border px-4 py-4">
      <div className="flex items-center justify-between gap-3 text-rhd-2xs font-medium text-muted-foreground"><span>{label}</span><span className="text-muted-foreground/60">{icon}</span></div>
      <div className={cn("mt-3 truncate text-2xl font-semibold tabular-nums", metricToneClasses[tone])} title={value}>{value}{unit ? <span className="ml-1 text-rhd-2xs font-medium text-muted-foreground">{unit}</span> : null}</div>
      <div className="mt-2 truncate text-rhd-xs text-muted-foreground" title={detail}>{detail}</div>
    </div>
  )
}

function ResourceCell({ icon, label, value, detail, percent, abnormalValue }: { icon: ReactNode; label: string; value: string; detail: string; percent?: number; abnormalValue: string }) {
  const safePercent = percent === undefined ? undefined : Math.max(0, Math.min(100, percent))
  const warning = safePercent !== undefined && safePercent >= 70
  return (
    <div className="min-w-0 border-r border-b border-border px-4 py-3.5 2xl:border-b-0">
      <div className="flex items-center gap-2 text-rhd-2xs font-medium text-muted-foreground"><span className="text-muted-foreground">{icon}</span>{label}</div>
      <div className={cn("mt-2 text-lg font-semibold tabular-nums", warning ? "text-amber-600" : value === abnormalValue ? "text-destructive" : "text-primary")}>{value}</div>
      {safePercent !== undefined ? <div className="mt-2 h-1 overflow-hidden rounded-full bg-muted"><div className={cn("h-full rounded-full", safePercent >= 85 ? "bg-destructive" : safePercent >= 70 ? "bg-amber-500" : "bg-primary")} style={{ width: `${safePercent}%` }} /></div> : null}
      <div className="mt-1.5 truncate text-rhd-2xs text-muted-foreground" title={detail}>{detail}</div>
    </div>
  )
}

function DashboardPanel({ title, icon, action, children }: { title: string; icon: ReactNode; action?: ReactNode; children: ReactNode }) {
  return (
    <ContentModule
      title={<span className="rhd-railops-platform-dashboard-panel-title">{icon}<span className="truncate">{title}</span></span>}
      extra={action ? <div className="rhd-railops-platform-dashboard-panel-action">{action}</div> : undefined}
      className="rhd-railops-platform-dashboard-panel"
    >
      {children}
    </ContentModule>
  )
}

function WorkloadRow({ t, label, running, pending, failed }: { t: ReturnType<typeof useI18n>; label: string; running: number; pending: number; failed: number }) {
  const total = running + pending + failed
  const health = failed > 0 ? t("platformExtract.ops.status.abnormal") : pending > 0 || running > 0 ? t("platformExtract.ops.status.processing") : t("platformExtract.ops.status.idle")
  return (
    <div className="py-4">
      <div className="flex items-center justify-between gap-3"><span className="text-xs font-semibold text-foreground">{label}</span><span className={cn("text-rhd-2xs font-medium", failed > 0 ? "text-destructive" : running + pending > 0 ? "text-primary" : "text-muted-foreground")}>{health}</span></div>
      <div className="mt-2 flex items-center gap-3"><div className="h-1.5 flex-1 overflow-hidden rounded-full bg-muted"><div className={cn("h-full rounded-full", failed > 0 ? "bg-destructive" : running + pending > 0 ? "bg-primary" : "bg-muted-foreground/40")} style={{ width: `${total > 0 ? Math.min(100, 20 + total * 8) : 8}%` }} /></div><span className="w-10 text-right text-xs font-semibold tabular-nums text-foreground">{formatNumber(total)}</span></div>
      <div className="mt-2 flex gap-4 text-rhd-2xs text-muted-foreground"><span>{t("platformExtract.ops.workload.running")} {formatNumber(running)}</span><span>{t("platformExtract.ops.workload.queued")} {formatNumber(pending)}</span><span className={failed > 0 ? "text-destructive" : undefined}>{t("platformExtract.ops.workload.failed")} {formatNumber(failed)}</span></div>
    </div>
  )
}

function LatencyRow({ dependency, maximum }: { dependency: PlatformInfrastructureDependency; maximum: number }) {
  const width = Math.max(3, Math.min(100, dependency.latencyMs / maximum * 100))
  const tone = dependency.status === "healthy" ? "bg-primary" : dependency.status === "degraded" ? "bg-amber-500" : "bg-destructive"
  return (
    <div>
      <div className="flex items-center justify-between gap-3"><div className="flex min-w-0 items-center gap-2"><span className={cn("size-1.5 shrink-0 rounded-full", tone)} /><span className="truncate text-xs font-medium text-foreground">{dependency.provider}</span></div><span className="shrink-0 text-xs font-semibold tabular-nums text-foreground">{formatNumber(dependency.latencyMs)} ms</span></div>
      <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-muted"><div className={cn("h-full rounded-full", tone)} style={{ width: `${width}%` }} /></div>
    </div>
  )
}

function StorageStack({ dependencies }: { dependencies: PlatformInfrastructureDependency[] }) {
  const total = dependencies.reduce((sum, dependency) => sum + dependency.storedBytes, 0)
  if (!dependencies.length || total <= 0) return <div className="h-3 rounded-full bg-muted" />
  return (
    <div className="flex h-3 overflow-hidden rounded-full bg-muted">
      {dependencies.map((dependency) => <div key={dependency.key} className={storageColors[dependency.key]} style={{ width: `${Math.max(0.8, dependency.storedBytes / total * 100)}%` }} title={`${dependency.provider} ${formatBytes(dependency.storedBytes)}`} />)}
    </div>
  )
}

function StorageLegend({ dependency }: { dependency: PlatformInfrastructureDependency }) {
  return (
    <div className="min-w-0">
      <div className="flex items-center gap-2"><span className={cn("size-2 shrink-0", storageColors[dependency.key])} /><span className="truncate text-xs font-medium text-foreground">{dependency.provider}</span><span className="ml-auto text-rhd-2xs text-muted-foreground">{formatPercent(dependency.sharePercent)}</span></div>
      <div className="mt-1 pl-4 text-base font-semibold tabular-nums text-foreground">{formatBytes(dependency.storedBytes)}</div>
      <div className="mt-0.5 truncate pl-4 text-rhd-2xs text-muted-foreground" title={dependency.measurement}>{dependency.measurement}</div>
    </div>
  )
}

function LatencyDistribution({ dependencies }: { dependencies: PlatformInfrastructureDependency[] }) {
  const buckets = [
    { label: "0-5ms", min: 0, max: 5 },
    { label: "5-15ms", min: 5, max: 15 },
    { label: "15-30ms", min: 15, max: 30 },
    { label: "30-60ms", min: 30, max: 60 },
    { label: "60-100ms", min: 60, max: 100 },
    { label: "100ms+", min: 100, max: Number.POSITIVE_INFINITY },
  ].map((bucket) => ({ ...bucket, count: dependencies.filter((item) => item.latencyMs >= bucket.min && item.latencyMs < bucket.max).length }))
  const maximum = Math.max(1, ...buckets.map((bucket) => bucket.count))
  return (
    <div className="flex h-56 items-end gap-2 px-5 pt-5 pb-4">
      {buckets.map((bucket) => (
        <div key={bucket.label} className="flex min-w-0 flex-1 flex-col items-center justify-end gap-2">
          <span className="text-rhd-2xs font-medium tabular-nums text-muted-foreground">{bucket.count}</span>
          <div className="flex h-32 w-full items-end rounded-sm bg-muted"><div className="w-full rounded-sm bg-primary" style={{ height: `${bucket.count > 0 ? Math.max(8, bucket.count / maximum * 100) : 1}%` }} /></div>
          <span className="w-full truncate text-center text-rhd-2xs text-muted-foreground" title={bucket.label}>{bucket.label}</span>
        </div>
      ))}
    </div>
  )
}

const databaseTableLabelKeys = new Set([
  "audit_logs",
  "auth_role_permissions",
  "messages",
  "notifications",
  "delivery_logs",
  "assets",
  "knowledge_documents",
  "knowledge_entries",
  "tickets",
  "conversations",
  "meeting_transcripts",
  "meeting_transcript_segments",
  "product_ai_usage_events",
  "t_ai_workflow_node_run",
  "t_ai_workflow_run",
  "t_conversation",
  "t_knowledge_retrieve_hit",
  "t_login_session",
  "t_message",
  "t_notification",
])

function DatabaseHotspotRow({ t, table, rank, maximum }: { t: ReturnType<typeof useI18n>; table: PlatformOpsInfrastructure["databaseTables"][number]; rank: number; maximum: number }) {
  const width = Math.max(2, Math.min(100, table.sizeBytes / maximum * 100))
  const label = databaseTableLabelKeys.has(table.name)
    ? t(`platformExtract.ops.tableLabels.${table.name}`)
    : table.name.replaceAll("_", " ")
  return (
    <div className="grid grid-cols-[22px_minmax(0,1fr)_auto] items-center gap-3 py-2.5">
      <span className="font-mono text-rhd-2xs text-muted-foreground">{String(rank).padStart(2, "0")}</span>
      <div className="min-w-0"><div className="truncate text-rhd-xs font-medium text-foreground" title={table.name}>{label}</div><div className="mt-1.5 h-1 overflow-hidden rounded-full bg-muted"><div className="h-full rounded-full bg-primary" style={{ width: `${width}%` }} /></div></div>
      <div className="text-right"><div className="text-rhd-xs font-semibold tabular-nums text-foreground">{formatBytes(table.sizeBytes)}</div><div className="mt-0.5 text-rhd-2xs text-muted-foreground">{t("platformExtract.ops.detail.rowCount", { count: formatNumber(table.estimatedRows) })}</div></div>
    </div>
  )
}

function CompactMetric({ label, value, detail, danger = false }: { label: string; value: string; detail: string; danger?: boolean }) {
  return <div className="min-h-28 px-5 py-4"><div className="text-rhd-2xs font-medium text-muted-foreground">{label}</div><div className={cn("mt-2 text-2xl font-semibold tabular-nums", danger ? "text-destructive" : "text-foreground")}>{value}</div><div className="mt-2 text-rhd-2xs text-muted-foreground">{detail}</div></div>
}

function AlertTable({ t, events, generatedAt }: { t: ReturnType<typeof useI18n>; events: AlertEvent[]; generatedAt?: string }) {
  if (!events.length) return <EmptyState title={t("platformExtract.ops.alertTable.empty")} />
  const columns: TableColumnsType<AlertEvent> = [
    {
      title: t("platformExtract.ops.alertTable.time"),
      key: "time",
      width: 140,
      render: (_value, event) => <span className="whitespace-nowrap tabular-nums text-muted-foreground">{formatDateTime(generatedAt)}</span>,
    },
    {
      title: t("platformExtract.ops.alertTable.level"),
      key: "level",
      width: 80,
      render: (_value, event) => <SeverityPill level={event.level} />,
    },
    {
      title: t("platformExtract.ops.alertTable.source"),
      key: "source",
      width: 140,
      render: (_value, event) => <span className="font-medium text-foreground">{event.source}</span>,
    },
    {
      title: t("platformExtract.ops.alertTable.alert"),
      key: "alert",
      render: (_value, event) => (
        <>
          <div className="font-medium text-foreground">{event.title}</div>
          <div className="mt-1 max-w-xl truncate text-rhd-2xs text-muted-foreground" title={event.detail}>{event.detail}</div>
        </>
      ),
    },
    {
      title: t("platformExtract.ops.alertTable.status"),
      key: "status",
      width: 120,
      render: (_value, event) => (
        <span className={cn("inline-flex h-6 items-center rounded-full border px-2 text-rhd-2xs font-medium", event.status === "alerting" ? "border-destructive/20 bg-destructive/10 text-destructive" : "border-amber-200 bg-amber-50 text-amber-600")}>{event.status === "alerting" ? t("platformExtract.ops.status.alerting") : t("platformExtract.ops.status.attention")}</span>
      ),
    },
  ]
  return (
    <DataTable<AlertEvent>
      className="rhd-railops-platform-ops-alert-table"
      columns={columns}
      dataSource={events}
      emptyDescription={t("platformExtract.ops.alertTable.empty")}
      loading={false}
      rowKey="id"
      scroll={{ x: 760 }}
      size="small"
    />
  )
}

function SeverityPill({ level }: { level: AlertEvent["level"] }) {
  return <span className={cn("inline-flex h-6 min-w-8 items-center justify-center rounded-full px-2 text-rhd-2xs font-semibold", level === "P0" ? "bg-destructive/10 text-destructive" : level === "P1" ? "bg-amber-100 text-amber-700" : "bg-primary/10 text-primary")}>{level}</span>
}

function EmptyState({ title }: { title: string }) {
  return <div className="flex min-h-48 flex-col items-center justify-center px-6 text-center"><CircleCheckIcon className="size-8 text-muted-foreground/40" /><div className="mt-3 text-sm font-semibold text-foreground">{title}</div></div>
}

function dependencyStatusLabel(t: ReturnType<typeof useI18n>, dependency?: PlatformInfrastructureDependency) {
  if (!dependency) return t("platformExtract.ops.resource.unavailable")
  if (dependency.status === "healthy") return t("platformExtract.ops.resource.normal")
  if (dependency.status === "degraded") return t("platformExtract.ops.resource.degraded")
  if (dependency.status === "unconfigured") return t("platformExtract.common.notConfigured")
  return t("platformExtract.ops.resource.abnormal")
}

function formatPercent(value: number) {
  if (!Number.isFinite(value) || value <= 0) return "0%"
  return value >= 10 ? `${Math.round(value)}%` : `${value.toFixed(1)}%`
}
