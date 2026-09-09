"use client"

import { translateCurrentMessage } from "@/i18n/messages"
import {
  BellRingIcon,
  CircleAlertIcon,
  CloudCogIcon,
  DatabaseZapIcon,
  KeyRoundIcon,
  MailIcon,
  MessagesSquareIcon,
  PlusIcon,
  RadioTowerIcon,
  RefreshCwIcon,
  RotateCwIcon,
  SendIcon,
  ServerCogIcon,
  Settings2Icon,
  ShieldCheckIcon,
  Trash2Icon,
  WebhookIcon,
} from "lucide-react"
import Link from "next/link"
import { useCallback, useEffect, useMemo, useState, type Dispatch, type ReactNode, type SetStateAction } from "react"
import { toast } from "sonner"
import { Input, Skeleton } from "antd"
import {
  ContentModule,
  FilterTabs,
  IconButton,
  PageShell,
  RailopsButton,
  SelectField,
  StandardModal,
  StatusTag,
  type RailopsTabItem,
  type StatusTagTone,
} from "@railops/ui"

import { useRouteBreadcrumbItems } from "@/components/layout/route-breadcrumbs"
import { ErrorState } from "@/components/shared/error-states"
import { Switch } from "@/components/ui/switch"
import {
  createConnector,
  fetchConnectorStatus,
  fetchConnectors,
  testConnector,
  updateConnector,
  type ConnectorListItem,
} from "@/lib/api/access-connector"
import {
  fetchNotificationMailSettings,
  sendNotificationTestMail,
  updateNotificationMailSettings,
} from "@/lib/api/enterprise-notifications"
import type { NotificationMailSetting } from "@/lib/api/types"
import { readSession } from "@/lib/auth"
import { cn } from "@/lib/utils"
function ee(key: string, values?: Record<string, unknown>) {
  if (!values) {
    return translateCurrentMessage(`enterpriseExtract.${key}`)
  }
  return translateCurrentMessage(
    `enterpriseExtract.${key}`,
    Object.fromEntries(
      Object.entries(values).map(([name, value]) => [
        name,
        typeof value === "string" || typeof value === "number" ? value : String(value ?? ""),
      ]),
    ),
  )
}


type AccessView = "channels" | "systems"

type MailForm = {
  fromAddress: string
  fromName: string
  host: string
  port: string
  username: string
  password: string
  replyTo: string
  retryPolicy: string
  useTls: boolean
}

type FeishuRegion = "china" | "global"

type FeishuForm = {
  region: FeishuRegion
  appId: string
  appSecret: string
}

type MQTTMetricMapping = {
  id: string
  key: string
  path: string
}

type MQTTForm = {
  username: string
  password: string
  clientId: string
  topics: string
  qos: "0" | "1" | "2"
  keepAliveSeconds: string
  cleanSession: boolean
  deviceSerial: string
  faultCode: string
  severity: string
  alarmMessage: string
  recordedAt: string
  metrics: MQTTMetricMapping[]
  unknownDevicePolicy: "dead_letter" | "accept_unbound"
}

const EMPTY_MAIL_FORM: MailForm = {
  fromAddress: "",
  fromName: "",
  host: "",
  port: "465",
  username: "",
  password: "",
  replyTo: "",
  retryPolicy: "retry_3_10m",
  useTls: true,
}

const EMPTY_FEISHU_FORM: FeishuForm = {
  region: "china",
  appId: "",
  appSecret: "",
}

const EMPTY_MQTT_FORM: MQTTForm = {
  username: "",
  password: "",
  clientId: "",
  topics: "",
  qos: "1",
  keepAliveSeconds: "60",
  cleanSession: false,
  deviceSerial: "$.device_id",
  faultCode: "$.fault_code",
  severity: "$.severity",
  alarmMessage: "$.message",
  recordedAt: "$.recorded_at",
  metrics: [{ id: "metric-1", key: "", path: "" }],
  unknownDevicePolicy: "dead_letter",
}

const FEISHU_ENDPOINTS: Record<FeishuRegion, string> = {
  china: "https://open.feishu.cn/open-apis",
  global: "https://open.larksuite.com/open-apis",
}

const RETRY_OPTIONS = [
  { value: "retry_3_10m", label: ee("access.text001") },
  { value: "retry_1_5m", label: ee("access.text002") },
  { value: "no_retry", label: ee("access.text003") },
]

const CONNECTOR_TYPES = [
  { value: "api", label: ee("access.text004") },
  { value: "mqtt", label: "MQTT" },
  { value: "postgres", label: "PostgreSQL" },
  { value: "mysql", label: "MySQL" },
  { value: "elasticsearch", label: "Elasticsearch" },
  { value: "webhook", label: "Webhook" },
]

const PANEL_CLASS = "rhd-railops-access-panel"
const SURFACE_ICON_CLASS = "grid size-9 shrink-0 place-items-center rounded-md bg-muted text-muted-foreground"
const FIELD_LABEL_CLASS = "mb-1 block text-xs font-medium text-muted-foreground"

function resetMQTTForm(): MQTTForm {
  return {
    ...EMPTY_MQTT_FORM,
    metrics: EMPTY_MQTT_FORM.metrics.map((metric) => ({ ...metric })),
  }
}

function mqttTopics(value: string) {
  return Array.from(new Set(value.split(/[\n,]/).map((topic) => topic.trim()).filter(Boolean)))
}

function mailSettingToForm(setting: NotificationMailSetting): MailForm {
  return {
    fromAddress: setting.from_address,
    fromName: setting.from_name,
    host: setting.smtp_host,
    port: String(setting.smtp_port || 465),
    username: setting.username,
    password: "",
    replyTo: setting.reply_to,
    retryPolicy: setting.retry_policy || "retry_3_10m",
    useTls: setting.use_tls,
  }
}

function healthLabel(status: string) {
  if (status === "healthy") return ee("access.text005")
  if (status === "degraded") return ee("access.text006")
  if (status === "unhealthy") return ee("access.text007")
  return ee("access.text008")
}

function healthTone(status: string): StatusTagTone {
  if (status === "healthy") return "success"
  if (status === "degraded") return "warning"
  if (status === "unhealthy") return "error"
  return "disabled"
}

function connectorTypeLabel(type: string) {
  if (type === "feishu") return ee("access.text009")
  return CONNECTOR_TYPES.find((item) => item.value === type)?.label ?? type.toUpperCase()
}

function displayTime(value?: string) {
  if (!value) return ee("access.text010")
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

export default function EnterpriseAccessPage() {
  const session = readSession()
  const [view, setView] = useState<AccessView>("channels")
  const [mailSetting, setMailSetting] = useState<NotificationMailSetting | null>(null)
  const [mailForm, setMailForm] = useState<MailForm>(EMPTY_MAIL_FORM)
  const [connectors, setConnectors] = useState<ConnectorListItem[]>([])
  const [loading, setLoading] = useState(true)
  const [mailLoaded, setMailLoaded] = useState(false)
  const [connectorsLoaded, setConnectorsLoaded] = useState(false)
  const [mailError, setMailError] = useState("")
  const [connectorError, setConnectorError] = useState("")
  const [savingMail, setSavingMail] = useState(false)
  const [testingMail, setTestingMail] = useState(false)
  const [testingConnector, setTestingConnector] = useState<number | null>(null)
  const [createOpen, setCreateOpen] = useState(false)
  const [creating, setCreating] = useState(false)
  const [newConnector, setNewConnector] = useState({ name: "", connectorType: "api", baseUrl: "", authType: "bearer", authConfig: "" })
  const [mqttForm, setMQTTForm] = useState<MQTTForm>(() => resetMQTTForm())
  const [feishuOpen, setFeishuOpen] = useState(false)
  const [savingFeishu, setSavingFeishu] = useState(false)
  const [feishuForm, setFeishuForm] = useState<FeishuForm>(EMPTY_FEISHU_FORM)

  const canManageMail = session?.permissions.includes("notification.channel.manage") ?? false
  const canCreateConnector = session?.permissions.includes("tenantIntegrationConfig.create") ?? false
  const canTestConnector = session?.permissions.includes("tenantIntegrationConfig.update") ?? false

  const load = useCallback(async () => {
    setLoading(true)
    setMailError("")
    setConnectorError("")
    const [mailResult, connectorResult] = await Promise.allSettled([
      canManageMail ? fetchNotificationMailSettings() : Promise.resolve(null),
      fetchConnectors(),
    ])

    if (mailResult.status === "fulfilled" && mailResult.value) {
      if (mailResult.value.success && mailResult.value.data) {
        setMailSetting(mailResult.value.data)
        setMailForm(mailSettingToForm(mailResult.value.data))
      }
      if (!mailResult.value.success) {
        setMailError(mailResult.value.error?.message || ee("access.text011"))
      }
      setMailLoaded(true)
    } else if (mailResult.status === "rejected") {
      setMailError(mailResult.reason instanceof Error ? mailResult.reason.message : ee("access.text011"))
      setMailLoaded(true)
    } else {
      setMailLoaded(true)
    }

    if (connectorResult.status === "fulfilled" && connectorResult.value.success && connectorResult.value.data) {
      const listedConnectors = connectorResult.value.data
      const statuses = await Promise.all(listedConnectors.map(async (connector) => {
        if (connector.connector_type !== "mqtt") return connector
        const statusResult = await fetchConnectorStatus(connector.id).catch(() => null)
        if (!statusResult?.success || !statusResult.data) return connector
        return {
          ...connector,
          health_status: statusResult.data.status || connector.health_status,
          last_health_check_at: statusResult.data.lastChecked || connector.last_health_check_at,
          last_message_at: statusResult.data.lastMessageAt || connector.last_message_at,
        }
      }))
      setConnectors(statuses)
      setConnectorsLoaded(true)
    } else {
      const message = connectorResult.status === "rejected"
        ? connectorResult.reason instanceof Error ? connectorResult.reason.message : ee("access.text012")
        : connectorResult.value.error?.message || ee("access.text012")
      setConnectorError(message)
      setConnectorsLoaded(true)
    }
    setLoading(false)
  }, [canManageMail, setConnectorError, setConnectors, setLoading, setMailError, setMailForm, setMailSetting])

  useEffect(() => {
    const timer = window.setTimeout(() => void load(), 0)
    return () => window.clearTimeout(timer)
  }, [load])

  const feishuConnector = useMemo(
    () => connectors.find((item) => item.connector_type === "feishu") ?? null,
    [connectors]
  )
  const businessConnectors = useMemo(
    () => connectors.filter((item) => item.connector_type !== "feishu"),
    [connectors]
  )
  const connectorSummary = useMemo(() => ({
    total: businessConnectors.length,
    healthy: businessConnectors.filter((item) => item.health_status === "healthy").length,
    attention: businessConnectors.filter((item) => item.health_status === "degraded" || item.health_status === "unhealthy").length,
  }), [businessConnectors])
  const accessTabs = useMemo<RailopsTabItem[]>(() => [
    { value: "channels", label: ee("access.text013"), icon: <BellRingIcon className="size-4" /> },
    { value: "systems", label: ee("access.text014"), icon: <DatabaseZapIcon className="size-4" />, count: businessConnectors.length },
  ], [businessConnectors.length])

  const saveMail = useCallback(async () => {
    const port = Number(mailForm.port)
    if (!mailForm.fromAddress.trim() || !mailForm.host.trim() || !Number.isFinite(port) || port <= 0 || port > 65535) {
      toast.error(ee("access.text015"))
      return
    }
    setSavingMail(true)
    const result = await updateNotificationMailSettings({
      fromAddress: mailForm.fromAddress.trim(),
      fromName: mailForm.fromName.trim(),
      smtpHost: mailForm.host.trim(),
      smtpPort: port,
      username: mailForm.username.trim(),
      password: mailForm.password,
      replyTo: mailForm.replyTo.trim(),
      retryPolicy: mailForm.retryPolicy,
      useTls: mailForm.useTls,
    })
    setSavingMail(false)
    if (!result.success || !result.data) {
      toast.error(result.error?.message || ee("access.text016"))
      return
    }
    setMailSetting(result.data)
    setMailForm(mailSettingToForm(result.data))
    toast.success(ee("access.text017"))
  }, [mailForm, setMailForm, setMailSetting, setSavingMail])

  const testMail = useCallback(async () => {
    const recipient = mailForm.replyTo.trim() || mailForm.fromAddress.trim()
    if (!recipient) {
      toast.error(ee("access.text018"))
      return
    }
    setTestingMail(true)
    const result = await sendNotificationTestMail(recipient)
    setTestingMail(false)
    if (result.success) toast.success(ee("access.text019", { value0: recipient }))
    else toast.error(result.error?.message || ee("access.text020"))
  }, [mailForm.fromAddress, mailForm.replyTo, setTestingMail])

  const runConnectorTest = useCallback(async (id: number) => {
    setTestingConnector(id)
    const result = await testConnector(id)
    setTestingConnector(null)
    if (result.success) {
      toast.success(ee("access.text021"))
      await load()
    } else {
      toast.error(result.error?.message || ee("access.text022"))
    }
  }, [load, setTestingConnector])

  const refreshMQTTStatus = useCallback(async (connector: ConnectorListItem) => {
    setTestingConnector(connector.id)
    const result = await fetchConnectorStatus(connector.id)
    setTestingConnector(null)
    if (!result.success || !result.data) {
      toast.error(result.error?.message || ee("access.text023"))
      return
    }
    setConnectors((current) => current.map((item) => item.id === connector.id ? {
      ...item,
      health_status: result.data?.status || item.health_status,
      last_health_check_at: result.data?.lastChecked || item.last_health_check_at,
      last_message_at: result.data?.lastMessageAt || item.last_message_at,
    } : item))
    toast.success(result.data.lastMessageAt ? ee("access.text024") : ee("access.text025"))
  }, [setConnectors, setTestingConnector])

  const submitConnector = useCallback(async () => {
    if (!newConnector.name.trim() || !newConnector.baseUrl.trim()) {
      toast.error(ee("access.text026"))
      return
    }
    let authType = newConnector.authType
    let authConfig = newConnector.authConfig.trim()
    let fieldMapping: string | undefined
    if (newConnector.connectorType === "mqtt") {
      const topics = mqttTopics(mqttForm.topics)
      const keepAliveSeconds = Number(mqttForm.keepAliveSeconds)
      const incompleteMetric = mqttForm.metrics.some((metric) => Boolean(metric.key.trim()) !== Boolean(metric.path.trim()))
      const metricEntries = mqttForm.metrics
        .map((metric) => [metric.key.trim(), metric.path.trim()] as const)
        .filter(([key, path]) => key && path)
      const duplicateMetric = new Set(metricEntries.map(([key]) => key)).size !== metricEntries.length
      if (topics.length === 0) {
        toast.error(ee("access.text027"))
        return
      }
      if (!mqttForm.clientId.trim()) {
        toast.error(ee("access.text028"))
        return
      }
      if (!Number.isInteger(keepAliveSeconds) || keepAliveSeconds < 5 || keepAliveSeconds > 3600) {
        toast.error(ee("access.text029"))
        return
      }
      if (!mqttForm.deviceSerial.trim()) {
        toast.error(ee("access.text030"))
        return
      }
      if (mqttForm.password && !mqttForm.username.trim()) {
        toast.error(ee("access.text031"))
        return
      }
      if (incompleteMetric || duplicateMetric) {
        toast.error(incompleteMetric ? ee("access.text032") : ee("access.text033"))
        return
      }
      authType = mqttForm.username.trim() || mqttForm.password ? "basic" : "none"
      authConfig = JSON.stringify({
        username: mqttForm.username.trim(),
        password: mqttForm.password,
        clientId: mqttForm.clientId.trim(),
      })
      fieldMapping = JSON.stringify({
        topics,
        qos: Number(mqttForm.qos),
        keepAliveSeconds,
        cleanSession: mqttForm.cleanSession,
        unknownDevicePolicy: mqttForm.unknownDevicePolicy,
        deviceSerial: mqttForm.deviceSerial.trim(),
        faultCode: mqttForm.faultCode.trim(),
        severity: mqttForm.severity.trim(),
        alarmMessage: mqttForm.alarmMessage.trim(),
        recordedAt: mqttForm.recordedAt.trim(),
        metrics: Object.fromEntries(metricEntries),
      })
    }
    setCreating(true)
    const result = await createConnector({
      name: newConnector.name.trim(),
      connectorType: newConnector.connectorType,
      baseUrl: newConnector.baseUrl.trim(),
      authType,
      authConfig,
      fieldMapping,
    })
    setCreating(false)
    if (!result.success) {
      toast.error(result.error?.message || ee("access.text034"))
      return
    }
    setCreateOpen(false)
    setNewConnector({ name: "", connectorType: "api", baseUrl: "", authType: "bearer", authConfig: "" })
    setMQTTForm(resetMQTTForm())
    toast.success(ee("access.text035"))
    await load()
  }, [load, mqttForm, newConnector, setCreateOpen, setCreating, setMQTTForm, setNewConnector])

  const openFeishuConfiguration = useCallback(() => {
    setFeishuForm({
      ...EMPTY_FEISHU_FORM,
      region: feishuConnector?.base_url.includes("larksuite.com") ? "global" : "china",
    })
    setFeishuOpen(true)
  }, [feishuConnector, setFeishuForm, setFeishuOpen])

  const submitFeishu = useCallback(async () => {
    const appId = feishuForm.appId.trim()
    const appSecret = feishuForm.appSecret.trim()
    if (!appId || !appSecret) {
      toast.error(ee("access.text036"))
      return
    }
    const payload = {
      name: ee("access.text037"),
      connectorType: "feishu",
      baseUrl: FEISHU_ENDPOINTS[feishuForm.region],
      authType: "app_credentials",
      authConfig: JSON.stringify({ appId, appSecret }),
    }
    setSavingFeishu(true)
    const saveResult = feishuConnector
      ? await updateConnector(feishuConnector.id, payload)
      : await createConnector(payload)
    if (!saveResult.success || !saveResult.data) {
      setSavingFeishu(false)
      toast.error(saveResult.error?.message || ee("access.text038"))
      return
    }
    const connectorId = saveResult.data.id || feishuConnector?.id
    const testResult = connectorId ? await testConnector(connectorId) : null
    setSavingFeishu(false)
    setFeishuOpen(false)
    setFeishuForm(EMPTY_FEISHU_FORM)
    await load()
    if (testResult?.success && testResult.data?.success !== false) {
      toast.success(ee("access.text039"))
    } else {
      toast.warning(ee("access.text040"))
    }
  }, [feishuConnector, feishuForm, load, setFeishuForm, setFeishuOpen, setSavingFeishu])

  return (
    <PageShell
      title={ee("access.text041")}
      breadcrumb={useRouteBreadcrumbItems()}
      className="rhd-railops-access-page"
      actions={(
        <>
          <IconButton
            icon={<RefreshCwIcon className={cn("size-4", loading && "animate-spin")} />}
            tooltip={ee("access.text042")}
            aria-label={ee("access.text042")}
            onClick={() => void load()}
            disabled={loading}
          />
          <Link href="/enterprise/audit">
            <RailopsButton size="small">
              <ShieldCheckIcon className="size-4" />{ee("access.text043")}</RailopsButton>
          </Link>
        </>
      )}
    >

      <section className="rhd-railops-access-metrics" aria-label={ee("access.text044")}>
        <StatusMetric icon={<MailIcon />} label={ee("access.text045")} value={!canManageMail ? ee("access.text046") : mailSetting?.connected ? ee("access.text047") : ee("access.text048")} meta={!canManageMail ? ee("access.text049") : mailSetting?.connected ? `${mailSetting.smtp_host}:${mailSetting.smtp_port}` : "SMTP"} tone={!canManageMail ? "slate" : mailSetting?.connected ? "green" : "amber"} />
        <StatusMetric icon={<CloudCogIcon />} label={ee("access.text050")} value={`${connectorSummary.healthy}/${connectorSummary.total}`} meta={ee("access.text051")} tone={connectorSummary.attention > 0 ? "amber" : "blue"} />
        <StatusMetric icon={<CircleAlertIcon />} label={ee("access.text052")} value={String(connectorSummary.attention)} meta={ee("access.text053")} tone={connectorSummary.attention > 0 ? "red" : "green"} />
        <StatusMetric icon={<KeyRoundIcon />} label={ee("access.text054")} value={ee("access.text055")} meta={ee("access.text056")} tone="slate" />
      </section>

      <ContentModule className="rhd-railops-access-switcher">
        <FilterTabs
          ariaLabel={ee("access.text057")}
          items={accessTabs}
          value={view}
          onChange={(value) => setView(value as AccessView)}
        />
      </ContentModule>

      {view === "channels" ? (
        <div className="space-y-4">
          {!canManageMail ? (
            <AccessState title={ee("access.text058")} />
          ) : (
            <ContentModule
              className={PANEL_CLASS}
              title={(
                <span className="rhd-railops-access-module-title">
                  <MailIcon className="size-4" />{ee("access.text059")}</span>
              )}
              extra={(
                <StatusTag tone={mailSetting?.connected ? "success" : "warning"}>
                  {mailSetting?.connected ? ee("access.text060") : ee("access.text061")}
                </StatusTag>
              )}
            >

              {loading && !mailLoaded ? <MailFormSkeleton /> : mailError ? (
                <ErrorState title={ee("access.text062")} description={mailError} action={{ label: ee("access.text063"), onClick: load }} className="rounded-none border-0" />
              ) : (
                <div className="grid xl:grid-cols-[minmax(0,1fr)_320px]">
                  <div className="grid gap-4 p-4 sm:grid-cols-2">
                    <FormField label={ee("access.text064")}>
                      <Input value={mailForm.fromAddress} onChange={(event) => setMailForm((value) => ({ ...value, fromAddress: event.target.value }))} placeholder="service@example.com" />
                    </FormField>
                    <FormField label={ee("access.text065")}>
                      <Input value={mailForm.fromName} onChange={(event) => setMailForm((value) => ({ ...value, fromName: event.target.value }))} placeholder={ee("access.text066")} />
                    </FormField>
                    <FormField label={ee("access.text067")}>
                      <Input value={mailForm.host} onChange={(event) => setMailForm((value) => ({ ...value, host: event.target.value }))} placeholder="smtp.example.com" />
                    </FormField>
                    <FormField label={ee("access.text068")}>
                      <Input inputMode="numeric" value={mailForm.port} onChange={(event) => setMailForm((value) => ({ ...value, port: event.target.value }))} placeholder="465" />
                    </FormField>
                    <FormField label={ee("access.text069")}>
                      <Input autoComplete="username" value={mailForm.username} onChange={(event) => setMailForm((value) => ({ ...value, username: event.target.value }))} placeholder="service@example.com" />
                    </FormField>
                    <FormField label={ee("access.text070")}>
                      <Input type="password" autoComplete="new-password" value={mailForm.password} onChange={(event) => setMailForm((value) => ({ ...value, password: event.target.value }))} placeholder={mailSetting?.has_password ? ee("access.text071") : ee("access.text072")} />
                    </FormField>
                    <FormField label={ee("access.text073")}>
                      <Input value={mailForm.replyTo} onChange={(event) => setMailForm((value) => ({ ...value, replyTo: event.target.value }))} placeholder="support@example.com" />
                    </FormField>
                    <SelectField
                      label={ee("access.text074")}
                      style={{ marginBottom: 0 }}
                      selectProps={{
                        value: mailForm.retryPolicy,
                        onChange: (value) => setMailForm((current) => ({ ...current, retryPolicy: value ?? "retry_3_10m" })),
                        options: RETRY_OPTIONS,
                        style: { width: "100%" },
                      }}
                    />
                    <div className="flex items-center justify-between rounded-md border border-border bg-muted px-3 py-2.5 sm:col-span-2">
                      <div><div className="text-sm font-medium text-foreground">{ee("access.text075")}</div><div className="mt-0.5 text-xs text-muted-foreground">{mailForm.useTls ? ee("access.text055") : ee("access.text076")}</div></div>
                      <Switch checked={mailForm.useTls} onCheckedChange={(checked) => setMailForm((value) => ({ ...value, useTls: checked }))} aria-label={ee("access.text075")} />
                    </div>
                    <div className="flex flex-col gap-2 sm:col-span-2 sm:flex-row sm:justify-end">
                      <RailopsButton onClick={() => void testMail()} disabled={testingMail || !mailSetting?.connected}>
                        <SendIcon className="size-4" />{testingMail ? ee("access.text077") : ee("access.text078")}
                      </RailopsButton>
                      <RailopsButton onClick={() => void saveMail()} disabled={savingMail}>
                        <ServerCogIcon className="size-4" />{savingMail ? ee("access.text079") : ee("access.text080")}
                      </RailopsButton>
                    </div>
                  </div>
                  <aside className="border-t border-border bg-muted/60 p-4 xl:border-l xl:border-t-0">
                    <h3 className="text-xs font-semibold text-foreground">{ee("access.text081")}</h3>
                    <dl className="mt-4 space-y-4">
                      <StatusTerm label={ee("access.text082")} value={mailForm.useTls ? "SMTP over TLS" : "SMTP"} />
                      <StatusTerm label={ee("access.text083")} value={mailForm.host ? `${mailForm.host}:${mailForm.port}` : ee("access.text061")} />
                      <StatusTerm label={ee("access.text084")} value={mailForm.fromAddress || ee("access.text061")} />
                      <StatusTerm label={ee("access.text085")} value={mailSetting?.has_password ? ee("access.text086") : ee("access.text087")} />
                      <StatusTerm label={ee("access.text088")} value={RETRY_OPTIONS.find((item) => item.value === mailForm.retryPolicy)?.label || "-"} />
                    </dl>
                  </aside>
                </div>
              )}
            </ContentModule>
          )}

          <ContentModule className={PANEL_CLASS} title={ee("access.text089")}>
            <ChannelRow icon={<SendIcon />} name={ee("access.text090")} scope={ee("access.text091")} />
            <ChannelRow icon={<WebhookIcon />} name={ee("access.text092")} scope={ee("access.text093")} />
            <ChannelRow
              icon={<MessagesSquareIcon />}
              name={ee("access.text009")}
              scope={feishuConnector ? ee("access.text094", { value0: feishuConnector.base_url.includes("larksuite.com") ? ee("access.text132") : ee("access.text131") }) : ee("access.text095")}
              status={!feishuConnector ? ee("access.text061") : healthLabel(feishuConnector.health_status)}
              statusTone={!feishuConnector ? "disabled" : healthTone(feishuConnector.health_status)}
              action={(
                <div className="flex items-center gap-1.5">
                  {feishuConnector ? (
                    <IconButton
                      icon={<RotateCwIcon className={cn("size-4", testingConnector === feishuConnector.id && "animate-spin")} />}
                      tooltip={ee("access.text096")}
                      aria-label={ee("access.text096")}
                      disabled={!canTestConnector || testingConnector === feishuConnector.id}
                      onClick={() => void runConnectorTest(feishuConnector.id)}
                    />
                  ) : null}
                  <RailopsButton
                    size="small"
                    disabled={feishuConnector ? !canTestConnector : !canCreateConnector}
                    onClick={openFeishuConfiguration}
                  >
                    {feishuConnector ? <Settings2Icon className="size-4" /> : <PlusIcon className="size-4" />}
                    {feishuConnector ? ee("access.text097") : ee("access.text098")}
                  </RailopsButton>
                </div>
              )}
            />
          </ContentModule>
        </div>
      ) : (
        <ContentModule
          className={PANEL_CLASS}
          title={ee("access.text099")}
          extra={(
            <RailopsButton size="small" onClick={() => setCreateOpen(true)} disabled={!canCreateConnector}>
              <PlusIcon className="size-4" />{ee("access.text100")}</RailopsButton>
          )}
        >
          {loading && !connectorsLoaded ? <ConnectorSkeleton /> : connectorError ? (
            <ErrorState title={ee("access.text101")} description={connectorError} action={{ label: ee("access.text063"), onClick: load }} className="rounded-none border-0" />
          ) : businessConnectors.length === 0 ? (
            <AccessState title={ee("access.text102")} className="rounded-none border-0" />
          ) : (
            <div className="divide-y divide-border">
              {businessConnectors.map((connector) => (
                <div key={connector.id} className="grid gap-3 px-4 py-3 lg:grid-cols-[minmax(0,1.2fr)_150px_150px_180px_auto] lg:items-center">
                  <div className="flex min-w-0 items-center gap-3">
                    <span className={SURFACE_ICON_CLASS}>{connector.connector_type === "mqtt" ? <RadioTowerIcon className="size-4" /> : <DatabaseZapIcon className="size-4" />}</span>
                    <div className="min-w-0"><div className="truncate text-sm font-medium text-foreground">{connector.name}</div><div className="mt-0.5 truncate text-xs text-muted-foreground">{connector.base_url || ee("access.text103")}</div></div>
                  </div>
                  <div><div className="text-rhd-xs text-muted-foreground">{ee("access.text104")}</div><div className="mt-0.5 text-xs font-medium text-foreground">{connectorTypeLabel(connector.connector_type)}</div></div>
                  <div><div className="text-rhd-xs text-muted-foreground">{ee("access.text105")}</div><div className="mt-0.5 text-xs font-medium text-foreground">{connector.auth_type || ee("access.text106")}</div></div>
                  <div><div className="text-rhd-xs text-muted-foreground">{connector.connector_type === "mqtt" ? ee("access.text107") : ee("access.text108")}</div><div className="mt-0.5 text-xs font-medium text-foreground">{displayTime(connector.connector_type === "mqtt" ? connector.last_message_at : connector.last_health_check_at)}</div></div>
                  <div className="flex items-center justify-between gap-2 lg:justify-end">
                    <StatusTag tone={healthTone(connector.health_status)}>{healthLabel(connector.health_status)}</StatusTag>
                    <IconButton
                      icon={<RotateCwIcon className={cn("size-4", testingConnector === connector.id && "animate-spin")} />}
                      tooltip={connector.connector_type === "mqtt" ? ee("access.text109") : ee("access.text110")}
                      aria-label={connector.connector_type === "mqtt" ? ee("access.text111", { value0: connector.name }) : ee("access.text112", { value0: connector.name })}
                      disabled={!canTestConnector || testingConnector === connector.id}
                      onClick={() => connector.connector_type === "mqtt" ? void refreshMQTTStatus(connector) : void runConnectorTest(connector.id)}
                    />
                  </div>
                </div>
              ))}
            </div>
          )}
        </ContentModule>
      )}

      <StandardModal
        open={createOpen}
        onCancel={() => !creating && setCreateOpen(false)}
        title={ee("access.text113")}
        width={768}
        rootClassName="rhd-railops-access-create-modal"
        footer={
          <>
            <RailopsButton onClick={() => !creating && setCreateOpen(false)} disabled={creating}>{ee("access.text114")}</RailopsButton>
            <RailopsButton onClick={() => void submitConnector()} disabled={creating}>{creating ? ee("access.text115") : ee("access.text116")}</RailopsButton>

          </>
        }
      >
          <div className="grid gap-4 sm:grid-cols-2">
            <FormField label={ee("access.text117")}><Input aria-label={ee("access.text117")} value={newConnector.name} onChange={(event) => setNewConnector((value) => ({ ...value, name: event.target.value }))} placeholder={newConnector.connectorType === "mqtt" ? ee("access.text118") : ee("access.text119")} /></FormField>
            <SelectField
              label={ee("access.text120")}
              style={{ marginBottom: 0 }}
              selectProps={{
                "aria-label": ee("access.text120"),
                value: newConnector.connectorType,
                onChange: (value) => setNewConnector((current) => ({ ...current, connectorType: value ?? "api", baseUrl: value === "mqtt" && current.connectorType !== "mqtt" ? "mqtts://" : current.baseUrl })),
                options: CONNECTOR_TYPES,
                style: { width: "100%" },
              }}
            />
            <FormField label={newConnector.connectorType === "mqtt" ? ee("access.text121") : ee("access.text122")} className="sm:col-span-2">
              <Input aria-label={newConnector.connectorType === "mqtt" ? ee("access.text121") : ee("access.text122")} value={newConnector.baseUrl} onChange={(event) => setNewConnector((value) => ({ ...value, baseUrl: event.target.value }))} placeholder={newConnector.connectorType === "mqtt" ? "mqtts://broker.example.com:8883" : "https://api.example.com"} />
            </FormField>
            {newConnector.connectorType === "mqtt" ? (
              <MQTTConnectorFields value={mqttForm} onChange={setMQTTForm} />
            ) : (
              <>
                <SelectField
                  label={ee("access.text123")}
                  style={{ marginBottom: 0 }}
                  selectProps={{
                    "aria-label": ee("access.text123"),
                    value: newConnector.authType,
                    onChange: (value) => setNewConnector((current) => ({ ...current, authType: value ?? "bearer" })),
                    options: [
                      { value: "bearer", label: "Bearer Token" },
                      { value: "api_key", label: "API Key" },
                      { value: "basic", label: "Basic Auth" },
                      { value: "none", label: ee("access.text124") },
                    ],
                    style: { width: "100%" },
                  }}
                />
                <FormField label={ee("access.text125")}><Input aria-label={ee("access.text125")} type="password" autoComplete="new-password" value={newConnector.authConfig} onChange={(event) => setNewConnector((value) => ({ ...value, authConfig: event.target.value }))} placeholder={ee("access.text126")} /></FormField>
              </>
            )}
          </div>
          
      </StandardModal>

      <StandardModal
        open={feishuOpen}
        onCancel={() => !savingFeishu && setFeishuOpen(false)}
        title={feishuConnector ? ee("access.text127") : ee("access.text098")}
        width={512}
        footer={
          <>
            <RailopsButton onClick={() => !savingFeishu && setFeishuOpen(false)} disabled={savingFeishu}>{ee("access.text114")}</RailopsButton>
            <RailopsButton onClick={() => void submitFeishu()} disabled={savingFeishu}>
              {savingFeishu ? <RotateCwIcon className="size-4 animate-spin" /> : <MessagesSquareIcon className="size-4" />}
              {savingFeishu ? ee("access.text128") : ee("access.text129")}
            </RailopsButton>

          </>
        }
      >
          <div className="grid gap-4">
            <SelectField
              label={ee("access.text130")}
              style={{ marginBottom: 0 }}
              selectProps={{
                value: feishuForm.region,
                onChange: (value) => setFeishuForm((current) => ({ ...current, region: (value ?? "china") as FeishuRegion })),
                options: [
                  { value: "china", label: ee("access.text131") },
                  { value: "global", label: ee("access.text132") },
                ],
                style: { width: "100%" },
              }}
            />
            <FormField label="App ID">
              <Input value={feishuForm.appId} onChange={(event) => setFeishuForm((value) => ({ ...value, appId: event.target.value }))} placeholder="cli_xxxxxxxxxxxxxxxx" autoComplete="off" />
            </FormField>
            <FormField label="App Secret">
              <Input type="password" value={feishuForm.appSecret} onChange={(event) => setFeishuForm((value) => ({ ...value, appSecret: event.target.value }))} placeholder={ee("access.text133")} autoComplete="new-password" />
            </FormField>
          </div>

      </StandardModal>
    </PageShell>
  )
}

function AccessState({ className, title }: { className?: string; title: string }) {
  return (
    <div className={cn("grid min-h-40 place-items-center border border-dashed p-8 text-center", className)}>
      <div className="text-sm font-medium text-foreground">{title}</div>
    </div>
  )
}

function MQTTConnectorFields({ value, onChange }: { value: MQTTForm; onChange: Dispatch<SetStateAction<MQTTForm>> }) {
  const updateMetric = (id: string, field: "key" | "path", nextValue: string) => {
    onChange((current) => ({
      ...current,
      metrics: current.metrics.map((metric) => metric.id === id ? { ...metric, [field]: nextValue } : metric),
    }))
  }

  return (
    <>
      <FormField label={ee("access.text134")}>
        <Input aria-label={ee("access.text135")} autoComplete="username" value={value.username} onChange={(event) => onChange((current) => ({ ...current, username: event.target.value }))} placeholder={ee("access.text136")} />
      </FormField>
      <FormField label={ee("access.text137")}>
        <Input aria-label={ee("access.text138")} type="password" autoComplete="new-password" value={value.password} onChange={(event) => onChange((current) => ({ ...current, password: event.target.value }))} placeholder={ee("access.text139")} />
      </FormField>
      <FormField label="Client ID">
        <Input aria-label="MQTT Client ID" autoComplete="off" value={value.clientId} onChange={(event) => onChange((current) => ({ ...current, clientId: event.target.value }))} placeholder="remotehelpdesk-tenant-1" />
      </FormField>
      <SelectField
        label="QoS"
        style={{ marginBottom: 0 }}
        selectProps={{
          "aria-label": "MQTT QoS",
          value: value.qos,
          onChange: (qos) => onChange((current) => ({ ...current, qos: (qos ?? "1") as MQTTForm["qos"] })),
          options: [
            { value: "0", label: "QoS 0" },
            { value: "1", label: "QoS 1" },
            { value: "2", label: "QoS 2" },
          ],
          style: { width: "100%" },
        }}
      />
      <FormField label={ee("access.text140")} className="sm:col-span-2">
        <Input.TextArea aria-label={ee("access.text141")} className="min-h-20 resize-y" value={value.topics} onChange={(event) => onChange((current) => ({ ...current, topics: event.target.value }))} placeholder={"devices/+/telemetry\ndevices/+/events"} />
      </FormField>
      <FormField label={ee("access.text142")}>
        <Input aria-label={ee("access.text143")} type="number" inputMode="numeric" min={5} max={3600} step={1} value={value.keepAliveSeconds} onChange={(event) => onChange((current) => ({ ...current, keepAliveSeconds: event.target.value }))} />
      </FormField>
      <div className="flex min-w-0 items-center justify-between gap-3 border-b border-border py-2 sm:self-end" role="group" aria-label="MQTT Clean Session">
        <div className="min-w-0"><div className="text-xs font-medium text-foreground">Clean Session</div><div className="mt-0.5 text-xs text-muted-foreground">{value.cleanSession ? ee("access.text144") : ee("access.text145")}</div></div>
        <Switch aria-label="MQTT Clean Session" checked={value.cleanSession} onCheckedChange={(cleanSession) => onChange((current) => ({ ...current, cleanSession }))} />
      </div>

      <div className="border-t border-border pt-3 sm:col-span-2">
        <h3 className="text-sm font-semibold text-foreground">{ee("access.text146")}</h3>
      </div>
      <FormField label={ee("access.text147")}>
        <Input aria-label={ee("access.text147")} value={value.deviceSerial} onChange={(event) => onChange((current) => ({ ...current, deviceSerial: event.target.value }))} placeholder="$.device_id" />
      </FormField>
      <FormField label={ee("access.text148")}>
        <Input aria-label={ee("access.text148")} value={value.faultCode} onChange={(event) => onChange((current) => ({ ...current, faultCode: event.target.value }))} placeholder="$.fault_code" />
      </FormField>
      <FormField label={ee("access.text149")}>
        <Input aria-label={ee("access.text149")} value={value.severity} onChange={(event) => onChange((current) => ({ ...current, severity: event.target.value }))} placeholder="$.severity" />
      </FormField>
      <FormField label={ee("access.text150")}>
        <Input aria-label={ee("access.text150")} value={value.alarmMessage} onChange={(event) => onChange((current) => ({ ...current, alarmMessage: event.target.value }))} placeholder="$.message" />
      </FormField>
      <FormField label={ee("access.text151")}>
        <Input aria-label={ee("access.text151")} value={value.recordedAt} onChange={(event) => onChange((current) => ({ ...current, recordedAt: event.target.value }))} placeholder="$.recorded_at" />
      </FormField>
      <SelectField
        label={ee("access.text152")}
        style={{ marginBottom: 0 }}
        selectProps={{
          "aria-label": ee("access.text153"),
          value: value.unknownDevicePolicy,
          onChange: (policy) => onChange((current) => ({ ...current, unknownDevicePolicy: (policy ?? "dead_letter") as MQTTForm["unknownDevicePolicy"] })),
          options: [
            { value: "dead_letter", label: ee("access.text154") },
            { value: "accept_unbound", label: ee("access.text155") },
          ],
          style: { width: "100%" },
        }}
      />

      <section className="min-w-0 border-t border-border pt-3 sm:col-span-2" aria-label={ee("access.text156")}>
        <div className="flex items-center justify-between gap-3">
          <h3 className="text-sm font-semibold text-foreground">{ee("access.text157")}</h3>
          <RailopsButton
            size="small"
            onClick={() => onChange((current) => ({ ...current, metrics: [...current.metrics, { id: `metric-${Date.now()}`, key: "", path: "" }] }))}
          >
            <PlusIcon className="size-4" />{ee("access.text158")}</RailopsButton>
        </div>
        <div className="mt-3 space-y-2">
          {value.metrics.length === 0 ? <p className="text-xs text-muted-foreground">{ee("access.text159")}</p> : value.metrics.map((metric, index) => (
            <div key={metric.id} className="grid min-w-0 gap-2 sm:grid-cols-[minmax(0,0.7fr)_minmax(0,1.3fr)_32px] sm:items-end">
              <FormField label={ee("access.text160", { value0: index + 1 })}>
                <Input aria-label={ee("access.text161", { value0: index + 1 })} value={metric.key} onChange={(event) => updateMetric(metric.id, "key", event.target.value)} placeholder="temperature" />
              </FormField>
              <FormField label={ee("access.text162", { value0: index + 1 })}>
                <Input aria-label={ee("access.text163", { value0: index + 1 })} value={metric.path} onChange={(event) => updateMetric(metric.id, "path", event.target.value)} placeholder="$.metrics.temperature" />
              </FormField>
              <IconButton
                icon={<Trash2Icon className="size-4" />}
                tooltip={ee("access.text164")}
                aria-label={ee("access.text165", { value0: index + 1 })}
                danger
                onClick={() => onChange((current) => ({ ...current, metrics: current.metrics.filter((item) => item.id !== metric.id) }))}
              />
            </div>
          ))}
        </div>
      </section>
    </>
  )
}

function StatusMetric({ icon, label, value, meta, tone }: { icon: ReactNode; label: string; value: string; meta: string; tone: "green" | "blue" | "amber" | "red" | "slate" }) {
  const toneClass = { green: "text-primary", blue: "text-primary", amber: "text-foreground", red: "text-destructive", slate: "text-muted-foreground" }[tone]
  return <div className="rhd-railops-access-metric"><span className={cn("rhd-railops-access-metric-icon", toneClass)}>{icon}</span><div className="min-w-0"><div className="text-xs text-muted-foreground">{label}</div><div className={cn("mt-0.5 truncate text-lg font-semibold", toneClass)}>{value}</div><div className="truncate text-rhd-xs text-muted-foreground">{meta}</div></div></div>
}

function FormField({ label, children, className }: { label: string; children: ReactNode; className?: string }) {
  return <label className={cn("block", className)}><span className={FIELD_LABEL_CLASS}>{label}</span>{children}</label>
}

function StatusTerm({ label, value }: { label: string; value: string }) {
  return <div className="flex items-start justify-between gap-4"><dt className="shrink-0 text-xs text-muted-foreground">{label}</dt><dd className="min-w-0 break-all text-right text-xs font-medium text-foreground">{value}</dd></div>
}

function ChannelRow({ icon, name, scope, status = ee("access.text166"), statusTone = "disabled", action }: { icon: ReactNode; name: string; scope: string; status?: string; statusTone?: StatusTagTone; action?: ReactNode }) {
  return <div className="flex flex-col gap-3 border-b border-border px-4 py-3 last:border-b-0 sm:flex-row sm:items-center"><div className="flex min-w-0 flex-1 items-center gap-3"><span className={SURFACE_ICON_CLASS}>{icon}</span><div className="min-w-0 flex-1"><div className="text-sm font-medium text-foreground">{name}</div><div className="mt-0.5 truncate text-xs text-muted-foreground">{scope}</div></div></div><div className="flex shrink-0 items-center justify-between gap-2 sm:justify-end"><StatusTag tone={statusTone}>{status}</StatusTag>{action}</div></div>
}

function MailFormSkeleton() {
  return <div className="grid gap-4 p-4 sm:grid-cols-2" role="status" aria-busy="true" aria-label={ee("access.text167")}>{Array.from({ length: 8 }).map((_, index) => <div key={index}><Skeleton.Node active className="mb-2" style={{ width: 80, height: 12 }} /><Skeleton.Node active className="w-full" style={{ width: "100%", height: 36 }} /></div>)}</div>
}

function ConnectorSkeleton() {
  return <div className="divide-y divide-border" role="status" aria-busy="true" aria-label={ee("access.text168")}>{Array.from({ length: 3 }).map((_, index) => <div key={index} className="flex items-center gap-3 p-4"><Skeleton.Node active style={{ width: 36, height: 36 }} /><div className="flex-1"><Skeleton.Node active style={{ width: 192, height: 16 }} /><Skeleton.Node active className="mt-2 max-w-full" style={{ width: 288, height: 12 }} /></div><Skeleton.Node active style={{ width: 64, height: 28 }} /></div>)}</div>
}
