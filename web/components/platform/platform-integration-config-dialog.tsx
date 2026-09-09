"use client"

import { useEffect, useMemo, useState } from "react"
import { LoaderCircleIcon, PlugZapIcon, SaveIcon } from "lucide-react"
import { toast } from "sonner"

import { SelectField } from "@railops/ui"
import { ProjectDialog } from "@/components/project-dialog"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Switch } from "@/components/ui/switch"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { useAppLocale, useI18n } from "@/i18n/provider"
import {
  testPlatformIntegration,
  updatePlatformIntegration,
  type PlatformIntegrationKind,
  type PlatformIntegrationMutation,
  type PlatformIntegrationSettings,
  type PlatformIntegrationTestResult,
} from "@/lib/api/platform"

type IntegrationDraft = {
  jitsiUrl: string
  jitsiDomain: string
  jitsiRequireAuth: boolean
  jitsiAppId: string
  jitsiAppSecret: string
  jitsiWebhookSecret: string
  jitsiTokenTtlMinutes: number
  jitsiTimeout: string
  jitsiMaxRetries: number
  speechProvider: "disabled" | "mock" | "xfyun" | "aliyun"
  xfyunAppId: string
  xfyunApiKey: string
  xfyunEndpoint: string
  xfyunDomain: string
  aliyunApiKey: string
  aliyunWorkspaceId: string
  aliyunEndpoint: string
  aliyunModel: string
  jigasiEnabled: boolean
  jigasiSharedSecret: string
  jigasiMaxParticipantStreams: number
  jigasiMaxConcurrentStreams: number
  translationEnabled: boolean
  translationApiSecret: string
  translationEndpoint: string
  translationTargetLanguage: string
  arProvider: string
  arEndpoint: string
  arApiKey: string
  arHealthPath: string
  sub2apiHost: string
  sub2apiAdminApiKey: string
  sub2apiTranslationApiKey: string
  sub2apiDefaultModel: string
  smtpHost: string
  smtpPort: number
  smtpUsername: string
  smtpPassword: string
  smtpFromAddress: string
  smtpFromName: string
  smtpUseTLS: boolean
}

type SMTPProviderPreset = {
  key: string
  label: string
  host: string
  port: number
  useTLS: boolean
  usernamePlaceholder: string
}

function settingsToDraft(settings: PlatformIntegrationSettings): IntegrationDraft {
  return {
    jitsiUrl: settings.jitsi.url,
    jitsiDomain: settings.jitsi.jitsiDomain,
    jitsiRequireAuth: settings.jitsi.requireAuth,
    jitsiAppId: settings.jitsi.appId,
    jitsiAppSecret: "",
    jitsiWebhookSecret: "",
    jitsiTokenTtlMinutes: settings.jitsi.tokenTtlMinutes || 15,
    jitsiTimeout: settings.jitsi.timeout || "10s",
    jitsiMaxRetries: settings.jitsi.maxRetries || 3,
    speechProvider: settings.speech.provider === "disabled" ? "xfyun" : settings.speech.provider,
    xfyunAppId: settings.speech.xfyunAppId,
    xfyunApiKey: "",
    xfyunEndpoint: settings.speech.xfyunEndpoint,
    xfyunDomain: settings.speech.xfyunDomain,
    aliyunApiKey: "",
    aliyunWorkspaceId: settings.speech.aliyunWorkspaceId,
    aliyunEndpoint: settings.speech.aliyunEndpoint,
    aliyunModel: settings.speech.aliyunModel || "qwen-audio-3.0-asr-flash-streaming",
    jigasiEnabled: settings.speech.jigasiEnabled,
    jigasiSharedSecret: "",
    jigasiMaxParticipantStreams: settings.speech.jigasiMaxParticipantStreams || 8,
    jigasiMaxConcurrentStreams: settings.speech.jigasiMaxConcurrentStreams || 32,
    translationEnabled: settings.speech.translationEnabled,
    translationApiSecret: "",
    translationEndpoint: settings.speech.translationEndpoint,
    translationTargetLanguage: settings.speech.translationTargetLanguage || "en",
    arProvider: settings.ar.provider,
    arEndpoint: settings.ar.endpoint,
    arApiKey: "",
    arHealthPath: settings.ar.healthPath || "/health",
    sub2apiHost: settings.sub2api.host,
    sub2apiAdminApiKey: "",
    sub2apiTranslationApiKey: "",
    sub2apiDefaultModel: settings.sub2api.defaultLlmModel,
    smtpHost: settings.smtp.smtpHost,
    smtpPort: settings.smtp.smtpPort || 465,
    smtpUsername: settings.smtp.username,
    smtpPassword: "",
    smtpFromAddress: settings.smtp.fromAddress || settings.smtp.username,
    smtpFromName: settings.smtp.fromName || "RemoteHelpDesk",
    smtpUseTLS: settings.smtp.smtpHost ? settings.smtp.useTls : true,
  }
}

export function buildPlatformIntegrationMutation(
  kind: PlatformIntegrationKind,
  settings: PlatformIntegrationSettings,
  draft = settingsToDraft(settings),
): PlatformIntegrationMutation {
  if (kind === "jitsi") {
    return {
      integration: kind,
      jitsi: {
        url: draft.jitsiUrl.trim(),
        jitsiDomain: draft.jitsiDomain.trim(),
        requireAuth: draft.jitsiRequireAuth,
        appId: draft.jitsiAppId.trim(),
        appSecret: draft.jitsiAppSecret.trim(),
        webhookSecret: draft.jitsiWebhookSecret.trim(),
        tokenTtlMinutes: draft.jitsiTokenTtlMinutes,
        timeout: draft.jitsiTimeout.trim(),
        maxRetries: draft.jitsiMaxRetries,
      },
    }
  }
  if (kind === "speech" || kind === "translation") {
    return {
      integration: kind,
      speech: {
        provider: draft.speechProvider,
        xfyunAppId: draft.xfyunAppId.trim(),
        xfyunApiKey: draft.xfyunApiKey.trim(),
        xfyunEndpoint: draft.xfyunEndpoint.trim(),
        xfyunDomain: draft.xfyunDomain.trim(),
        aliyunApiKey: draft.aliyunApiKey.trim(),
        aliyunWorkspaceId: draft.aliyunWorkspaceId.trim(),
        aliyunEndpoint: draft.aliyunEndpoint.trim(),
        aliyunModel: draft.aliyunModel.trim(),
        jigasiEnabled: draft.jigasiEnabled,
        jigasiSharedSecret: draft.jigasiSharedSecret.trim(),
        jigasiMaxParticipantStreams: draft.jigasiMaxParticipantStreams,
        jigasiMaxConcurrentStreams: draft.jigasiMaxConcurrentStreams,
        xfyunTranslationEnabled: draft.translationEnabled,
        xfyunTranslationApiSecret: draft.translationApiSecret.trim(),
        xfyunTranslationEndpoint: draft.translationEndpoint.trim(),
        xfyunTranslationTargetLanguage: draft.translationTargetLanguage.trim(),
      },
    }
  }
  if (kind === "ar") {
    return {
      integration: kind,
      ar: {
        provider: draft.arProvider,
        endpoint: draft.arEndpoint.trim(),
        apiKey: draft.arApiKey.trim(),
        healthPath: draft.arHealthPath.trim(),
      },
    }
  }
  if (kind === "smtp") {
    return {
      integration: kind,
      smtp: {
        smtpHost: draft.smtpHost.trim(),
        smtpPort: draft.smtpPort,
        username: draft.smtpUsername.trim(),
        password: draft.smtpPassword.trim(),
        fromAddress: draft.smtpFromAddress.trim(),
        fromName: draft.smtpFromName.trim(),
        useTls: draft.smtpUseTLS,
      },
    }
  }
  return {
    integration: kind,
    sub2api: {
      host: draft.sub2apiHost.trim(),
      adminApiKey: draft.sub2apiAdminApiKey.trim(),
      translationApiKey: draft.sub2apiTranslationApiKey.trim(),
      defaultLlmModel: draft.sub2apiDefaultModel.trim(),
    },
  }
}

export function PlatformIntegrationConfigDialog({
  open,
  kind,
  settings,
  onOpenChange,
  onSaved,
}: {
  open: boolean
  kind: PlatformIntegrationKind
  settings: PlatformIntegrationSettings
  onOpenChange: (open: boolean) => void
  onSaved: (settings: PlatformIntegrationSettings) => void
}) {
  const t = useI18n()
  const { locale } = useAppLocale()
  const [draft, setDraft] = useState(() => settingsToDraft(settings))
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<PlatformIntegrationTestResult | null>(null)
  const meta = {
    jitsi: { title: t("platformExtract.integration.dialog.jitsiTitle"), short: t("platformExtract.access.integration.jitsi") },
    speech: { title: t("platformExtract.integration.dialog.speechTitle"), short: t("platformExtract.access.integration.speech") },
    translation: { title: t("platformExtract.integration.dialog.translationTitle"), short: t("platformExtract.integration.field.defaultTargetLanguage") },
    ar: { title: t("platformExtract.integration.dialog.arTitle"), short: t("platformExtract.integration.field.recognitionProvider") },
    sub2api: { title: t("platformExtract.integration.dialog.sub2apiTitle"), short: "Sub2API" },
    smtp: { title: t("platformExtract.integration.dialog.smtpTitle"), short: "SMTP" },
  }[kind]

  useEffect(() => {
    if (!open) return
    setDraft(settingsToDraft(settings))
    setTestResult(null)
  }, [kind, open, settings])

  const payload = useMemo(
    () => buildPlatformIntegrationMutation(kind, settings, draft),
    [draft, kind, settings],
  )

  async function handleSave() {
    setSaving(true)
    try {
      const next = await updatePlatformIntegration(payload)
      onSaved(next)
      setDraft(settingsToDraft(next))
      toast.success(t("platformExtract.integration.saveSuccess"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("platformExtract.integration.saveFailed"))
    } finally {
      setSaving(false)
    }
  }

  async function handleTest() {
    setTesting(true)
    setTestResult(null)
    try {
      const result = await testPlatformIntegration(payload)
      setTestResult(result)
      if (result.success) toast.success(t("platformExtract.integration.connectionHealthyToast", { name: meta.short }))
      else toast.error(result.message || t("platformExtract.integration.testFailedToast"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("platformExtract.integration.testFailedToast"))
    } finally {
      setTesting(false)
    }
  }

  return (
    <ProjectDialog
      open={open}
      onOpenChange={onOpenChange}
      title={meta.title}
      size="lg"
      footer={
        <>
          <Button variant="outline" onClick={() => void handleTest()} disabled={saving || testing}>
            {testing ? <LoaderCircleIcon className="animate-spin" /> : <PlugZapIcon />}
            {testing ? t("platformExtract.integration.testing") : t("platformExtract.integration.testConnection")}
          </Button>
          <Button onClick={() => void handleSave()} disabled={!settings.canUpdate || saving || testing}>
            {saving ? <LoaderCircleIcon className="animate-spin" /> : <SaveIcon />}
            {saving ? t("platformExtract.common.saving") : t("platformExtract.integration.saveConfig")}
          </Button>
        </>
      }
    >
      <div data-locale={locale}>
        {kind === "jitsi" ? <JitsiFields draft={draft} setDraft={setDraft} settings={settings} /> : null}
        {kind === "speech" ? <SpeechFields draft={draft} setDraft={setDraft} settings={settings} /> : null}
        {kind === "translation" ? <TranslationFields draft={draft} setDraft={setDraft} settings={settings} /> : null}
        {kind === "ar" ? <ARFields draft={draft} setDraft={setDraft} settings={settings} /> : null}
        {kind === "sub2api" ? <Sub2APIFields draft={draft} setDraft={setDraft} settings={settings} /> : null}
        {kind === "smtp" ? <SMTPFields draft={draft} setDraft={setDraft} settings={settings} /> : null}
        {testResult ? (
          <div className={testResult.success ? "border border-primary/20 bg-primary/10 px-3 py-2 text-sm text-primary" : "border border-destructive/20 bg-destructive/10 px-3 py-2 text-sm text-destructive"}>
            <div className="font-medium">{testResult.success ? t("platformExtract.integration.connectionHealthy") : t("platformExtract.integration.connectionFailed")}</div>
            <div className="mt-1 text-xs">{testResult.message} · {testResult.latencyMs} ms</div>
          </div>
        ) : null}
      </div>
    </ProjectDialog>
  )
}

type DraftFieldsProps = {
  draft: IntegrationDraft
  setDraft: React.Dispatch<React.SetStateAction<IntegrationDraft>>
  settings: PlatformIntegrationSettings
}

function JitsiFields({ draft, setDraft, settings }: DraftFieldsProps) {
  const t = useI18n()
  return (
    <div className="grid gap-4 sm:grid-cols-2">
      <ConfigField label={t("platformExtract.integration.field.serviceUrl")} wide><Input value={draft.jitsiUrl} onChange={(event) => setDraft((value) => ({ ...value, jitsiUrl: event.target.value }))} placeholder="https://meet.example.com" /></ConfigField>
      <ConfigField label={t("platformExtract.integration.field.frontendDomain")}><Input value={draft.jitsiDomain} onChange={(event) => setDraft((value) => ({ ...value, jitsiDomain: event.target.value }))} placeholder="meet.example.com" /></ConfigField>
      <ConfigField label="App ID"><Input value={draft.jitsiAppId} onChange={(event) => setDraft((value) => ({ ...value, jitsiAppId: event.target.value }))} placeholder="RemoteHelpDesk" /></ConfigField>
      <ConfigField label="App Secret"><Input type="password" autoComplete="new-password" value={draft.jitsiAppSecret} onChange={(event) => setDraft((value) => ({ ...value, jitsiAppSecret: event.target.value }))} placeholder={settings.jitsi.appSecretConfigured ? t("platformExtract.integration.placeholder.savedLeaveBlank") : t("platformExtract.integration.placeholder.signingSecret")} /></ConfigField>
      <ConfigField label="Webhook Secret"><Input type="password" autoComplete="new-password" value={draft.jitsiWebhookSecret} onChange={(event) => setDraft((value) => ({ ...value, jitsiWebhookSecret: event.target.value }))} placeholder={settings.jitsi.webhookSecretConfigured ? t("platformExtract.integration.placeholder.savedLeaveBlank") : t("platformExtract.integration.placeholder.webhookSecret")} /></ConfigField>
      <ConfigField label={t("platformExtract.integration.field.jwtTtlMinutes")}><Input type="number" min={1} max={1440} value={draft.jitsiTokenTtlMinutes} onChange={(event) => setDraft((value) => ({ ...value, jitsiTokenTtlMinutes: Number(event.target.value) }))} /></ConfigField>
      <ConfigField label={t("platformExtract.integration.field.requestTimeout")}><Input value={draft.jitsiTimeout} onChange={(event) => setDraft((value) => ({ ...value, jitsiTimeout: event.target.value }))} placeholder="10s" /></ConfigField>
      <ConfigField label={t("platformExtract.integration.field.maxRetries")}><Input type="number" min={0} max={10} value={draft.jitsiMaxRetries} onChange={(event) => setDraft((value) => ({ ...value, jitsiMaxRetries: Number(event.target.value) }))} /></ConfigField>
      <ToggleField label={t("platformExtract.integration.field.enableJwtPrivateMeeting")} checked={draft.jitsiRequireAuth} onCheckedChange={(checked) => setDraft((value) => ({ ...value, jitsiRequireAuth: checked }))} />
    </div>
  )
}

function SpeechFields({ draft, setDraft, settings }: DraftFieldsProps) {
  const t = useI18n()
  return (
    <div className="space-y-5">
      <div>
        <div className="mb-2 text-xs font-medium text-muted-foreground">{t("platformExtract.integration.field.speechService")}</div>
        <ToggleGroup multiple={false} value={[draft.speechProvider]} onValueChange={(values) => values[0] && setDraft((value) => ({ ...value, speechProvider: values[0] as IntegrationDraft["speechProvider"] }))} variant="outline" size="sm">
          <ToggleGroupItem value="disabled">{t("platformExtract.common.disabled")}</ToggleGroupItem><ToggleGroupItem value="xfyun">{t("platformExtract.integration.speechProvider.xfyunRtasr")}</ToggleGroupItem><ToggleGroupItem value="aliyun">{t("platformExtract.integration.speechProvider.aliyunAsr")}</ToggleGroupItem><ToggleGroupItem value="mock">{t("platformExtract.integration.speechProvider.mock")}</ToggleGroupItem>
        </ToggleGroup>
      </div>
      {draft.speechProvider === "xfyun" ? (
        <div className="grid gap-4 sm:grid-cols-2">
          <ConfigField label="APPID"><Input value={draft.xfyunAppId} onChange={(event) => setDraft((value) => ({ ...value, xfyunAppId: event.target.value }))} placeholder="e250082b" /></ConfigField>
          <ConfigField label={t("platformExtract.integration.field.apiKey")}><Input type="password" autoComplete="new-password" value={draft.xfyunApiKey} onChange={(event) => setDraft((value) => ({ ...value, xfyunApiKey: event.target.value }))} placeholder={settings.speech.xfyunApiKeyConfigured ? t("platformExtract.integration.placeholder.savedLeaveBlank") : t("platformExtract.integration.placeholder.webapiKey")} /></ConfigField>
          <ConfigField label={t("platformExtract.integration.field.websocketUrl")} wide><Input value={draft.xfyunEndpoint} onChange={(event) => setDraft((value) => ({ ...value, xfyunEndpoint: event.target.value }))} placeholder="wss://rtasr.xfyun.cn/v1/ws" /></ConfigField>
          <ConfigField label={t("platformExtract.integration.field.domain")}><Input value={draft.xfyunDomain} onChange={(event) => setDraft((value) => ({ ...value, xfyunDomain: event.target.value }))} placeholder="iat" /></ConfigField>
        </div>
      ) : null}
      {draft.speechProvider === "aliyun" ? (
        <div className="grid gap-4 sm:grid-cols-2">
          <ConfigField label={t("platformExtract.integration.field.apiKey")}><Input type="password" autoComplete="new-password" value={draft.aliyunApiKey} onChange={(event) => setDraft((value) => ({ ...value, aliyunApiKey: event.target.value }))} placeholder={settings.speech.aliyunApiKeyConfigured ? t("platformExtract.integration.placeholder.savedLeaveBlank") : t("platformExtract.integration.placeholder.webapiKey")} /></ConfigField>
          <ConfigField label={t("platformExtract.integration.field.workspaceId")}><Input value={draft.aliyunWorkspaceId} onChange={(event) => setDraft((value) => ({ ...value, aliyunWorkspaceId: event.target.value }))} placeholder={t("platformExtract.integration.placeholder.aliyunWorkspaceId")} /></ConfigField>
          <ConfigField label={t("platformExtract.integration.field.websocketUrl")} wide><Input value={draft.aliyunEndpoint} onChange={(event) => setDraft((value) => ({ ...value, aliyunEndpoint: event.target.value }))} placeholder="wss://dashscope.aliyuncs.com/api-ws/v1/inference" /></ConfigField>
          <ConfigField label={t("platformExtract.integration.field.model")} wide><Input value={draft.aliyunModel} onChange={(event) => setDraft((value) => ({ ...value, aliyunModel: event.target.value }))} placeholder="qwen-audio-3.0-asr-flash-streaming" /></ConfigField>
        </div>
      ) : null}
      <div className="border-t border-border pt-5">
        <div className="grid gap-4 sm:grid-cols-2">
          <ToggleField label={t("platformExtract.integration.field.enableJigasiBridge")} checked={draft.jigasiEnabled} onCheckedChange={(checked) => setDraft((value) => ({ ...value, jigasiEnabled: checked }))} />
          <ConfigField label={t("platformExtract.integration.field.jigasiSharedSecret")}><Input type="password" autoComplete="new-password" value={draft.jigasiSharedSecret} onChange={(event) => setDraft((value) => ({ ...value, jigasiSharedSecret: event.target.value }))} placeholder={settings.speech.jigasiSharedSecretConfigured ? t("platformExtract.integration.placeholder.savedLeaveBlank") : t("platformExtract.integration.placeholder.atLeast32Chars")} /></ConfigField>
          <ConfigField label={t("platformExtract.integration.field.maxParticipantStreams")}><Input type="number" min={1} max={64} value={draft.jigasiMaxParticipantStreams} onChange={(event) => setDraft((value) => ({ ...value, jigasiMaxParticipantStreams: Number(event.target.value) }))} /></ConfigField>
          <ConfigField label={t("platformExtract.integration.field.maxConcurrentStreams")}><Input type="number" min={1} max={512} value={draft.jigasiMaxConcurrentStreams} onChange={(event) => setDraft((value) => ({ ...value, jigasiMaxConcurrentStreams: Number(event.target.value) }))} /></ConfigField>
        </div>
      </div>
    </div>
  )
}

function TranslationFields({ draft, setDraft, settings }: DraftFieldsProps) {
  const t = useI18n()
  return (
    <div className="grid gap-4 sm:grid-cols-2">
      <ToggleField label={t("platformExtract.integration.field.enableRealtimeTranslation")} checked={draft.translationEnabled} onCheckedChange={(checked) => setDraft((value) => ({ ...value, translationEnabled: checked }))} />
      <ConfigField label={t("platformExtract.integration.field.translationApiSecret")}><Input type="password" autoComplete="new-password" value={draft.translationApiSecret} onChange={(event) => setDraft((value) => ({ ...value, translationApiSecret: event.target.value }))} placeholder={settings.speech.translationConfigured ? t("platformExtract.integration.placeholder.savedLeaveBlank") : t("platformExtract.integration.placeholder.translationApiSecret")} /></ConfigField>
      <ConfigField label={t("platformExtract.integration.field.translationEndpoint")} wide><Input value={draft.translationEndpoint} onChange={(event) => setDraft((value) => ({ ...value, translationEndpoint: event.target.value }))} placeholder="https://itrans.xfyun.cn/v2/its" /></ConfigField>
      <ConfigField label={t("platformExtract.integration.field.defaultTargetLanguage")}><Input value={draft.translationTargetLanguage} onChange={(event) => setDraft((value) => ({ ...value, translationTargetLanguage: event.target.value }))} placeholder="en" /></ConfigField>
    </div>
  )
}

function ARFields({ draft, setDraft, settings }: DraftFieldsProps) {
  const t = useI18n()
  return (
    <div className="space-y-5">
      <div><div className="mb-2 text-xs font-medium text-muted-foreground">{t("platformExtract.integration.field.recognitionProvider")}</div><ToggleGroup multiple={false} value={[draft.arProvider]} onValueChange={(values) => values[0] && setDraft((value) => ({ ...value, arProvider: values[0] }))} variant="outline" size="sm"><ToggleGroupItem value="disabled">{t("platformExtract.common.disabled")}</ToggleGroupItem><ToggleGroupItem value="http">HTTP API</ToggleGroupItem></ToggleGroup></div>
      {draft.arProvider === "http" ? <div className="grid gap-4 sm:grid-cols-2">
        <ConfigField label={t("platformExtract.integration.field.recognitionEndpoint")} wide><Input value={draft.arEndpoint} onChange={(event) => setDraft((value) => ({ ...value, arEndpoint: event.target.value }))} placeholder="https://vision.example.com/v1/detect" /></ConfigField>
        <ConfigField label={t("platformExtract.integration.field.apiKey")}><Input type="password" autoComplete="new-password" value={draft.arApiKey} onChange={(event) => setDraft((value) => ({ ...value, arApiKey: event.target.value }))} placeholder={settings.ar.apiKeyConfigured ? t("platformExtract.integration.placeholder.savedLeaveBlank") : t("platformExtract.integration.placeholder.bearerToken")} /></ConfigField>
        <ConfigField label={t("platformExtract.integration.field.healthPath")}><Input value={draft.arHealthPath} onChange={(event) => setDraft((value) => ({ ...value, arHealthPath: event.target.value }))} placeholder="/health" /></ConfigField>
      </div> : null}
    </div>
  )
}

function Sub2APIFields({ draft, setDraft, settings }: DraftFieldsProps) {
  const t = useI18n()
  const fingerprint = settings.sub2api.adminApiKeyFingerprint
  const fingerprintLabel = fingerprint
    ? `${fingerprint.slice(0, 12)}...${fingerprint.slice(-8)}`
    : t("platformExtract.integration.notGenerated")
  const translationFingerprint = settings.sub2api.translationApiKeyFingerprint
  const translationFingerprintLabel = translationFingerprint
    ? `${translationFingerprint.slice(0, 12)}...${translationFingerprint.slice(-8)}`
    : t("platformExtract.integration.notGenerated")
  return (
    <div className="grid gap-4 sm:grid-cols-2">
      <ConfigField label={t("platformExtract.integration.field.gatewayUrl")} wide><Input value={draft.sub2apiHost} onChange={(event) => setDraft((value) => ({ ...value, sub2apiHost: event.target.value }))} placeholder="https://sub2api.example.com" /></ConfigField>
      <ConfigField label={t("platformExtract.integration.field.currentAdminApiKey")}>
        <div className="min-h-9 rounded-md border border-border bg-muted px-3 py-2">
          <code className="block text-xs font-semibold text-foreground">
            {settings.sub2api.adminApiKeyMasked || t("platformExtract.access.status.notConfigured")}
          </code>
          <span className="mt-1 block truncate text-rhd-xs text-muted-foreground" title={fingerprint}>
            {t("platformExtract.integration.fingerprint", { fingerprint: fingerprintLabel })}
          </span>
        </div>
      </ConfigField>
      <ConfigField label={t("platformExtract.integration.field.rotateAdminCredential")}><Input type="password" autoComplete="new-password" value={draft.sub2apiAdminApiKey} onChange={(event) => setDraft((value) => ({ ...value, sub2apiAdminApiKey: event.target.value }))} placeholder={settings.sub2api.adminApiKeyConfigured ? t("platformExtract.integration.placeholder.newKeyLeaveBlank") : t("platformExtract.integration.placeholder.adminApiKey")} /></ConfigField>
      <ConfigField label={t("platformExtract.integration.field.defaultModel")}><Input value={draft.sub2apiDefaultModel} onChange={(event) => setDraft((value) => ({ ...value, sub2apiDefaultModel: event.target.value }))} placeholder="gpt-5.4-mini" /></ConfigField>
      <ConfigField label={t("platformExtract.integration.field.currentTranslationApiKey")}>
        <div className="min-h-9 rounded-md border border-border bg-muted px-3 py-2">
          <code className="block text-xs font-semibold text-foreground">
            {settings.sub2api.translationApiKeyMasked || t("platformExtract.access.status.notConfigured")}
          </code>
          <span className="mt-1 block truncate text-rhd-xs text-muted-foreground" title={translationFingerprint}>
            {t("platformExtract.integration.fingerprint", { fingerprint: translationFingerprintLabel })}
          </span>
        </div>
      </ConfigField>
      <ConfigField label={t("platformExtract.integration.field.translationFallbackKey")}>
        <Input
          type="password"
          autoComplete="new-password"
          value={draft.sub2apiTranslationApiKey}
          onChange={(event) => setDraft((value) => ({ ...value, sub2apiTranslationApiKey: event.target.value }))}
          placeholder={settings.sub2api.translationApiKeyConfigured ? t("platformExtract.integration.placeholder.newKeyLeaveBlank") : t("platformExtract.integration.placeholder.translationFallbackKey")}
        />
      </ConfigField>
    </div>
  )
}

function SMTPFields({ draft, setDraft, settings }: DraftFieldsProps) {
  const t = useI18n()
  const smtpProviderPresets: SMTPProviderPreset[] = [
    { key: "qq", label: t("platformExtract.integration.preset.qq"), host: "smtp.qq.com", port: 465, useTLS: true, usernamePlaceholder: "name@qq.com" },
    { key: "tencent-enterprise", label: t("platformExtract.integration.preset.tencentEnterprise"), host: "smtp.exmail.qq.com", port: 465, useTLS: true, usernamePlaceholder: "name@company.com" },
    { key: "aliyun-enterprise", label: t("platformExtract.integration.preset.aliyunEnterprise"), host: "smtp.qiye.aliyun.com", port: 465, useTLS: true, usernamePlaceholder: "name@company.com" },
    { key: "gmail", label: t("platformExtract.integration.preset.gmail"), host: "smtp.gmail.com", port: 465, useTLS: true, usernamePlaceholder: "name@gmail.com" },
    { key: "microsoft365", label: t("platformExtract.integration.preset.microsoft365"), host: "smtp.office365.com", port: 587, useTLS: false, usernamePlaceholder: "name@company.com" },
    { key: "outlook", label: t("platformExtract.integration.preset.outlook"), host: "smtp-mail.outlook.com", port: 587, useTLS: false, usernamePlaceholder: "name@outlook.com" },
    { key: "netease-163", label: t("platformExtract.integration.preset.netease163"), host: "smtp.163.com", port: 465, useTLS: true, usernamePlaceholder: "name@163.com" },
    { key: "netease-126", label: t("platformExtract.integration.preset.netease126"), host: "smtp.126.com", port: 465, useTLS: true, usernamePlaceholder: "name@126.com" },
    { key: "zoho", label: t("platformExtract.integration.preset.zoho"), host: "smtp.zoho.com", port: 465, useTLS: true, usernamePlaceholder: "name@company.com" },
  ]
  const activePreset = getSMTPPreset(draft, smtpProviderPresets)
  const [smtpHostChoice, setSMTPHostChoice] = useState(activePreset?.key || "custom")
  const usernamePlaceholder = activePreset?.usernamePlaceholder || "name@example.com"

  useEffect(() => {
    setSMTPHostChoice(activePreset?.key || "custom")
  }, [activePreset?.key, settings.smtp.smtpHost, settings.smtp.smtpPort, settings.smtp.useTls])

  function applySMTPPreset(preset: SMTPProviderPreset) {
    setDraft((value) => ({
      ...value,
      smtpHost: preset.host,
      smtpPort: preset.port,
      smtpUseTLS: preset.useTLS,
      smtpUsername: value.smtpUsername || value.smtpFromAddress,
      smtpFromAddress: value.smtpFromAddress || value.smtpUsername,
    }))
  }

  function handleSMTPHostChoice(nextValue: string) {
    setSMTPHostChoice(nextValue)
    const next = smtpProviderPresets.find((preset) => preset.key === nextValue)
    if (next) applySMTPPreset(next)
  }

  return (
    <div className="grid gap-4 sm:grid-cols-2">
      <SelectField
        label="SMTP Host"
        style={{ marginBottom: 0 }}
        selectProps={{
          "aria-label": "SMTP Host",
          value: smtpHostChoice,
          onChange: (value) => handleSMTPHostChoice(value),
          placeholder: t("platformExtract.integration.placeholder.selectSmtpHost"),
          options: [
            ...smtpProviderPresets.map((preset) => ({
              value: preset.key,
              label: `${preset.label} · ${preset.host}`,
            })),
            { value: "custom", label: t("platformExtract.integration.preset.custom") },
          ],
          style: { width: "100%" },
        }}
      />
      {smtpHostChoice === "custom" ? (
        <ConfigField label={t("platformExtract.integration.field.customHost")}><Input value={draft.smtpHost} onChange={(event) => setDraft((value) => ({ ...value, smtpHost: event.target.value }))} placeholder="smtp.example.com" /></ConfigField>
      ) : null}
      <ConfigField label={t("platformExtract.access.card.port")}><Input type="number" min={1} max={65535} value={draft.smtpPort} onChange={(event) => setDraft((value) => ({ ...value, smtpPort: Number(event.target.value) }))} /></ConfigField>
      <ToggleField label={t("platformExtract.integration.field.sslTlsDirect")} checked={draft.smtpUseTLS} onCheckedChange={(checked) => setDraft((value) => ({ ...value, smtpUseTLS: checked, smtpPort: value.smtpPort || (checked ? 465 : 587) }))} />
      <ConfigField label={t("platformExtract.integration.field.username")}><Input value={draft.smtpUsername} onChange={(event) => setDraft((value) => ({ ...value, smtpUsername: event.target.value, smtpFromAddress: value.smtpFromAddress || event.target.value }))} placeholder={usernamePlaceholder} /></ConfigField>
      <ConfigField label={t("platformExtract.integration.field.passwordOrAuthCode")}><Input type="password" autoComplete="new-password" value={draft.smtpPassword} onChange={(event) => setDraft((value) => ({ ...value, smtpPassword: event.target.value }))} placeholder={settings.smtp.passwordConfigured ? t("platformExtract.integration.placeholder.savedLeaveBlank") : t("platformExtract.integration.placeholder.smtpPassword")} /></ConfigField>
      <ConfigField label={t("platformExtract.access.smtp.fromAddress")}><Input value={draft.smtpFromAddress} onChange={(event) => setDraft((value) => ({ ...value, smtpFromAddress: event.target.value }))} placeholder={usernamePlaceholder} /></ConfigField>
      <ConfigField label={t("platformExtract.integration.field.fromName")}><Input value={draft.smtpFromName} onChange={(event) => setDraft((value) => ({ ...value, smtpFromName: event.target.value }))} placeholder="RemoteHelpDesk" /></ConfigField>
    </div>
  )
}

function getSMTPPreset(draft: IntegrationDraft, smtpProviderPresets: SMTPProviderPreset[]) {
  const host = draft.smtpHost.trim().toLowerCase()
  return smtpProviderPresets.find((preset) =>
    preset.host === host &&
    preset.port === draft.smtpPort &&
    preset.useTLS === draft.smtpUseTLS
  )
}

function ConfigField({ label, children, wide = false }: { label: string; children: React.ReactNode; wide?: boolean }) {
  return <label className={wide ? "space-y-2 sm:col-span-2" : "space-y-2"}><span className="text-xs font-medium text-muted-foreground">{label}</span>{children}</label>
}

function ToggleField({ label, checked, onCheckedChange }: { label: string; checked: boolean; onCheckedChange: (checked: boolean) => void }) {
  return <div className="flex min-h-8 items-center justify-between gap-4"><span className="text-xs font-medium text-muted-foreground">{label}</span><Switch checked={checked} onCheckedChange={onCheckedChange} /></div>
}
