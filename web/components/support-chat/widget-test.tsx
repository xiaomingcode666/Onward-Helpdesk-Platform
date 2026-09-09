"use client"

import { SignJWT } from "jose"
import { CheckIcon, CopyIcon } from "lucide-react"
import { useCallback, useEffect, useMemo, useState } from "react"

import type { RemoteHelpDeskWidgetConfig } from "@/lib/sdk/config-types"
import { useI18n } from "@/i18n/provider"

const STORAGE_KEY = "remote-helpdesk-web-widget-test-config"
const LEGACY_STORAGE_KEY = "agent-desk-web-widget-test-config"
const DEFAULT_JWT_TTL_MINUTES = "30"
const INITIAL_CONFIG: RemoteHelpDeskWidgetConfig = {
  channelId: "",
  baseUrl: "",
  apiBaseUrl: "",
}

type WidgetTestConfig = RemoteHelpDeskWidgetConfig & {
  jwtSecret?: string
  jwtUserId?: string
  jwtName?: string
  jwtTtlMinutes?: string
}

function getDefaultConfig(defaultName: string): WidgetTestConfig {
  if (typeof window === "undefined") {
    return INITIAL_CONFIG
  }

  const savedText =
    window.localStorage.getItem(STORAGE_KEY) ??
    window.localStorage.getItem(LEGACY_STORAGE_KEY)
  const savedConfig = savedText
    ? (JSON.parse(savedText) as Partial<WidgetTestConfig>)
    : {}
  const query = new URLSearchParams(window.location.search)

  return {
    channelId: query.get("channelId") ?? savedConfig.channelId ?? "",
    baseUrl: "",
    apiBaseUrl: "",
    jwtSecret: savedConfig.jwtSecret ?? "",
    jwtUserId: query.get("userId") ?? savedConfig.jwtUserId ?? "web-customer-001",
    jwtName: query.get("name") ?? savedConfig.jwtName ?? defaultName,
    jwtTtlMinutes: savedConfig.jwtTtlMinutes ?? DEFAULT_JWT_TTL_MINUTES,
  }
}

function removeMountedWidget() {
  if (typeof window === "undefined") {
    return
  }

  window.RemoteHelpDeskWidget?.destroy()
  document
    .querySelectorAll(
      '[data-agent-desk-widget="launcher"], [data-agent-desk-widget="frame"], [data-agent-desk-widget="script"]'
      + ', [data-remote-help-desk-widget="launcher"], [data-remote-help-desk-widget="frame"], [data-remote-help-desk-widget="script"]'
    )
    .forEach((node) => node.remove())

  delete window.RemoteHelpDeskConfig
  delete window.AgentDeskConfig
  delete window.__REMOTE_HELP_DESK_WIDGET_CONFIG__
  delete window.__REMOTE_HELP_DESK_WIDGET_STATE__
  delete window.__CS_AI_AGENT_WIDGET_CONFIG__
  delete window.__CS_AI_AGENT_WIDGET_STATE__
  delete window.RemoteHelpDeskWidget
  delete window.AgentDeskWidget
}

function injectWidget(config: RemoteHelpDeskWidgetConfig) {
  removeMountedWidget()
  window.RemoteHelpDeskConfig = config

  const script = document.createElement("script")
  script.async = true
  script.src = `${window.location.origin}/sdk/remote-helpdesk-sdk.min.js`
  script.dataset.remoteHelpDeskWidget = "script"
  document.body.appendChild(script)
}

function buildWidgetConfig(config: WidgetTestConfig): RemoteHelpDeskWidgetConfig {
  const nextConfig: RemoteHelpDeskWidgetConfig = {
    channelId: config.channelId.trim(),
    baseUrl: "",
    apiBaseUrl: "",
  }
  nextConfig.getUserToken = undefined
  return nextConfig
}

async function signUserToken(config: WidgetTestConfig, t: (key: string) => string) {
  const userId = (config.jwtUserId || "").trim()
  const name = (config.jwtName || "").trim()
  const secret = (config.jwtSecret || "").trim()
  const ttl = Number(config.jwtTtlMinutes || DEFAULT_JWT_TTL_MINUTES)

  if (!userId) {
    throw new Error(t("widgetTest.missingUserId"))
  }
  if (!name) {
    throw new Error(t("widgetTest.missingName"))
  }
  if (!secret) {
    throw new Error(t("widgetTest.missingSecret"))
  }
  if (!Number.isFinite(ttl) || ttl <= 0) {
    throw new Error(t("widgetTest.invalidTtl"))
  }

  return new SignJWT({ userId, name })
    .setProtectedHeader({ alg: "HS256", typ: "JWT" })
    .setIssuedAt()
    .setExpirationTime(`${ttl}m`)
    .sign(new TextEncoder().encode(secret))
}

export function SupportWidgetTest() {
  const t = useI18n()
  const [config, setConfig] = useState<WidgetTestConfig>({
    ...INITIAL_CONFIG,
    jwtSecret: "",
    jwtUserId: "web-customer-001",
    jwtName: t("widgetTest.defaultName"),
    jwtTtlMinutes: DEFAULT_JWT_TTL_MINUTES,
  })
  const [status, setStatus] = useState(t("widgetTest.missingChannel"))
  const [origin, setOrigin] = useState("")
  const [generatedToken, setGeneratedToken] = useState("")
  const [latestDirectChatUrl, setLatestDirectChatUrl] = useState("")
  const [copied, setCopied] = useState(false)
  const [snippetCopied, setSnippetCopied] = useState(false)

  const mountWidget = useCallback(async (configToMount: WidgetTestConfig) => {
    const cleanConfig = {
      ...configToMount,
      channelId: configToMount.channelId.trim(),
      baseUrl: "",
      apiBaseUrl: "",
      getUserToken: undefined,
    }
    const nextConfig = buildWidgetConfig(cleanConfig)
    nextConfig.getUserToken = () => signUserToken(cleanConfig, t)
    setConfig(cleanConfig)
    setGeneratedToken("")
    setLatestDirectChatUrl("")
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(cleanConfig))

    if (!nextConfig.channelId) {
      removeMountedWidget()
      setStatus(t("widgetTest.missingChannel"))
      return
    }

    injectWidget(nextConfig)
    setStatus(t("widgetTest.mountedJwt"))
  }, [t])

  useEffect(() => {
    const timer = window.setTimeout(() => {
      const initialConfig = getDefaultConfig(t("widgetTest.defaultName"))
      setOrigin(window.location.origin)
      setConfig(initialConfig)
      setStatus(initialConfig.channelId ? t("widgetTest.mounted") : t("widgetTest.missingChannel"))

      if (initialConfig.channelId) {
        void mountWidget(initialConfig).catch((error) => {
          removeMountedWidget()
          setGeneratedToken("")
          setStatus(error instanceof Error ? error.message : t("widgetTest.mountFailed"))
        })
      }
    }, 0)

    return () => {
      window.clearTimeout(timer)
      removeMountedWidget()
    }
  }, [mountWidget, t])

  const snippet = useMemo(() => {
    const scriptSrc = origin
      ? `${origin}/sdk/remote-helpdesk-sdk.min.js`
      : "/sdk/remote-helpdesk-sdk.min.js"

    const configLines = [`    channelId: "${config.channelId || ""}"`]
    configLines.push(`    async getUserToken() {
      const res = await fetch("/api/support/user-token", { credentials: "include" });
      const data = await res.json();
      return data.userToken;
    }`)

    return `<script>
  window.RemoteHelpDeskConfig = {
${configLines.join(",\n")}
  };
</script>
<script async src="${scriptSrc}"></script>`
  }, [config.channelId, origin])

  function updateField<K extends keyof WidgetTestConfig>(
    key: K,
    value: WidgetTestConfig[K]
  ) {
    setConfig((current) => ({ ...current, [key]: value }))
  }

  async function handleMount() {
    try {
      await mountWidget(config)
    } catch (error) {
      removeMountedWidget()
      setGeneratedToken("")
      setStatus(error instanceof Error ? error.message : t("widgetTest.mountFailed"))
    }
  }

  async function handleCopyDirectUrl() {
    const widget = window.RemoteHelpDeskWidget
    if (!widget || typeof navigator === "undefined") {
      return
    }
    try {
      const url = await widget.getChatUrl()
      setLatestDirectChatUrl(url)
      setGeneratedToken(new URL(url).searchParams.get("userToken") || "")
      await navigator.clipboard.writeText(url)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1600)
    } catch (error) {
      setStatus(error instanceof Error ? error.message : t("widgetTest.linkFailed"))
    }
  }

  async function handleOpenDirectChat() {
    const widget = window.RemoteHelpDeskWidget
    if (!widget) {
      return
    }
    try {
      const url = await widget.getChatUrl()
      setLatestDirectChatUrl(url)
      setGeneratedToken(new URL(url).searchParams.get("userToken") || "")
      window.open(url, "_blank", "noopener,noreferrer")
    } catch (error) {
      setStatus(error instanceof Error ? error.message : t("widgetTest.linkFailed"))
    }
  }

  async function handleCopySnippet() {
    if (typeof navigator === "undefined") {
      return
    }
    await navigator.clipboard.writeText(snippet)
    setSnippetCopied(true)
    window.setTimeout(() => setSnippetCopied(false), 1600)
  }

  return (
    <main className="min-h-svh bg-slate-50 px-6 py-8 text-slate-950">
      <div className="mx-auto grid max-w-6xl gap-6 lg:grid-cols-[360px_minmax(0,1fr)]">
        <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
          <div className="text-base font-semibold">{t("widgetTest.title")}</div>
          <div className="mt-1 text-sm text-slate-500">{status}</div>

          <div className="mt-5 grid gap-3">
            <TextField
              label="channelId"
              value={config.channelId}
              onChange={(value) => updateField("channelId", value)}
            />
            <div className="grid gap-3 rounded-md border border-slate-200 p-3">
                <TextField
                  label="userId"
                  value={config.jwtUserId}
                  onChange={(value) => updateField("jwtUserId", value)}
                />
                <TextField
                  label="name"
                  value={config.jwtName}
                  onChange={(value) => updateField("jwtName", value)}
                />
                <TextField
                  label="JWT Secret"
                  value={config.jwtSecret}
                  onChange={(value) => updateField("jwtSecret", value)}
                  type="password"
                />
                <TextField
                  label={t("widgetTest.ttlMinutes")}
                  value={config.jwtTtlMinutes}
                  onChange={(value) => updateField("jwtTtlMinutes", value)}
                  type="number"
                />
            </div>
          </div>

          <div className="mt-5 flex gap-2">
            <button
              type="button"
              onClick={() => void handleMount()}
              className="rounded-md bg-slate-950 px-4 py-2 text-sm font-medium text-white"
            >
              {t("widgetTest.mount")}
            </button>
            <button
              type="button"
              onClick={() => {
                removeMountedWidget()
                setStatus(t("widgetTest.unmounted"))
              }}
              className="rounded-md border border-slate-200 bg-white px-4 py-2 text-sm font-medium"
            >
              {t("widgetTest.unmount")}
            </button>
          </div>
        </section>

        <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
          <div className="text-base font-semibold">{t("widgetTest.snippetTitle")}</div>
          <div className="mt-2 rounded-md bg-amber-50 px-3 py-2 text-sm text-amber-800">
            {t("widgetTest.jwtNotice")}
          </div>
          <div className="relative mt-4">
            <button
              type="button"
              onClick={() => void handleCopySnippet()}
              className="absolute right-2 top-2 inline-flex size-8 items-center justify-center rounded-md border border-white/10 bg-white/10 text-slate-200 transition hover:bg-white/20 hover:text-white"
              aria-label={snippetCopied ? t("widgetTest.copiedSnippet") : t("widgetTest.copySnippet")}
              title={snippetCopied ? t("widgetTest.copied") : t("widgetTest.copyCode")}
            >
              {snippetCopied ? (
                <CheckIcon className="size-4" />
              ) : (
                <CopyIcon className="size-4" />
              )}
            </button>
            <pre className="overflow-x-auto rounded-md bg-slate-950 p-4 pr-12 text-xs leading-5 text-slate-100">
              <code>{snippet}</code>
            </pre>
          </div>
          <div className="mt-5">
            <div className="text-sm font-medium text-slate-700">{t("widgetTest.directChat")}</div>
            <div className="mt-2 flex flex-col gap-2 sm:flex-row">
              <input
                readOnly
                value={latestDirectChatUrl || t("widgetTest.directChatPlaceholder")}
                className="h-9 min-w-0 flex-1 rounded-md border border-slate-200 px-3 font-mono text-xs outline-none"
              />
              <div className="flex gap-2">
                <button
                  type="button"
                  disabled={!config.channelId}
                  onClick={() => void handleCopyDirectUrl()}
                  className="rounded-md border border-slate-200 bg-white px-3 py-2 text-sm font-medium disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {copied ? t("widgetTest.copied") : t("widgetTest.copy")}
                </button>
                <button
                  type="button"
                  disabled={!config.channelId}
                  onClick={() => void handleOpenDirectChat()}
                  className="rounded-md bg-slate-950 px-3 py-2 text-sm font-medium text-white disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {t("widgetTest.openNewWindow")}
                </button>
              </div>
            </div>
          </div>
          {generatedToken ? (
            <div className="mt-4">
              <div className="text-sm font-medium text-slate-700">{t("widgetTest.currentToken")}</div>
              <textarea
                readOnly
                value={generatedToken}
                className="mt-2 h-28 w-full resize-none rounded-md border border-slate-200 p-3 font-mono text-xs outline-none"
              />
            </div>
          ) : null}
        </section>
      </div>
    </main>
  )
}

function TextField({
  label,
  value,
  onChange,
  type = "text",
}: {
  label: string
  value?: string
  onChange: (value: string) => void
  type?: string
}) {
  return (
    <label className="grid gap-1.5 text-sm">
      <span className="font-medium text-slate-700">{label}</span>
      <input
        type={type}
        value={value || ""}
        onChange={(event) => onChange(event.target.value)}
        className="h-9 rounded-md border border-slate-200 px-3 text-sm outline-none focus:border-slate-400"
      />
    </label>
  )
}
