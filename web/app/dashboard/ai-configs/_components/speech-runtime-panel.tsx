"use client"

import { useCallback, useEffect, useState, type FormEvent } from "react"
import { AudioLines, LoaderCircle, RefreshCw, Save } from "lucide-react"
import { toast } from "sonner"

import { useI18n } from "@/i18n/provider"
import {
  fetchSpeechRuntime,
  updateSpeechRuntime,
  type SpeechRuntimeProvider,
  type SpeechRuntimeStatus,
} from "@/lib/api/admin"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Field, FieldContent, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Switch } from "@/components/ui/switch"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"

type SpeechRuntimeForm = {
  provider: SpeechRuntimeProvider
  xfyunAppId: string
  xfyunApiKey: string
  xfyunEndpoint: string
  xfyunDomain: string
  aliyunApiKey: string
  aliyunWorkspaceId: string
  aliyunEndpoint: string
  aliyunModel: string
  xfyunTranslationEnabled: boolean
  xfyunTranslationApiSecret: string
  xfyunTranslationEndpoint: string
  xfyunTranslationTargetLanguage: string
  mockSegmentDurationMs: number
}

const emptyForm: SpeechRuntimeForm = {
  provider: "disabled",
  xfyunAppId: "",
  xfyunApiKey: "",
  xfyunEndpoint: "",
  xfyunDomain: "",
  aliyunApiKey: "",
  aliyunWorkspaceId: "",
  aliyunEndpoint: "",
  aliyunModel: "qwen-audio-3.0-asr-flash-streaming",
  xfyunTranslationEnabled: false,
  xfyunTranslationApiSecret: "",
  xfyunTranslationEndpoint: "",
  xfyunTranslationTargetLanguage: "en",
  mockSegmentDurationMs: 2000,
}

export function SpeechRuntimePanel() {
  const t = useI18n()
  const [status, setStatus] = useState<SpeechRuntimeStatus | null>(null)
  const [form, setForm] = useState<SpeechRuntimeForm>(emptyForm)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)

  const loadStatus = useCallback(async () => {
    setLoading(true)
    try {
      const next = await fetchSpeechRuntime()
      setStatus(next)
      setForm((current) => ({
        ...current,
        provider: next.provider,
        xfyunAppId: "",
        xfyunApiKey: "",
        xfyunEndpoint: next.xfyunEndpoint,
        xfyunDomain: next.xfyunDomain,
        aliyunApiKey: "",
        aliyunWorkspaceId: next.aliyunWorkspaceId,
        aliyunEndpoint: next.aliyunEndpoint,
        aliyunModel: next.aliyunModel || "qwen-audio-3.0-asr-flash-streaming",
        xfyunTranslationEnabled: next.xfyunTranslationEnabled,
        xfyunTranslationApiSecret: "",
        xfyunTranslationEndpoint: next.xfyunTranslationEndpoint,
        xfyunTranslationTargetLanguage: next.xfyunTranslationTargetLanguage || "en",
        mockSegmentDurationMs: next.mockSegmentDurationMs || 2000,
      }))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("aiConfig.speechLoadFailed"))
    } finally {
      setLoading(false)
    }
  }, [t])

  useEffect(() => {
    void loadStatus()
  }, [loadStatus])

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!status?.canUpdate) return
    setSaving(true)
    try {
      const next = await updateSpeechRuntime({
        provider: form.provider,
        xfyunAppId: form.xfyunAppId.trim(),
        xfyunApiKey: form.xfyunApiKey.trim(),
        xfyunEndpoint: form.xfyunEndpoint.trim(),
        xfyunDomain: form.xfyunDomain.trim(),
        aliyunApiKey: form.aliyunApiKey.trim(),
        aliyunWorkspaceId: form.aliyunWorkspaceId.trim(),
        aliyunEndpoint: form.aliyunEndpoint.trim(),
        aliyunModel: form.aliyunModel.trim(),
        xfyunTranslationEnabled: form.xfyunTranslationEnabled,
        xfyunTranslationApiSecret: form.xfyunTranslationApiSecret.trim(),
        xfyunTranslationEndpoint: form.xfyunTranslationEndpoint.trim(),
        xfyunTranslationTargetLanguage: form.xfyunTranslationTargetLanguage.trim(),
        mockSegmentDurationMs: form.mockSegmentDurationMs,
      })
      setStatus(next)
      setForm((current) => ({
        ...current,
        xfyunApiKey: "",
        xfyunTranslationApiSecret: "",
        xfyunTranslationEnabled: next.xfyunTranslationEnabled,
        xfyunEndpoint: next.xfyunEndpoint,
        xfyunDomain: next.xfyunDomain,
        aliyunApiKey: "",
        aliyunWorkspaceId: next.aliyunWorkspaceId,
        aliyunEndpoint: next.aliyunEndpoint,
        aliyunModel: next.aliyunModel || "qwen-audio-3.0-asr-flash-streaming",
        xfyunTranslationEndpoint: next.xfyunTranslationEndpoint,
        xfyunTranslationTargetLanguage: next.xfyunTranslationTargetLanguage || "en",
        mockSegmentDurationMs: next.mockSegmentDurationMs || 2000,
      }))
      toast.success(t("aiConfig.speechSaved"))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("aiConfig.speechSaveFailed"))
    } finally {
      setSaving(false)
    }
  }

  const providerLabels: Record<SpeechRuntimeProvider, string> = {
    disabled: t("aiConfig.speechProviderDisabled"),
    mock: t("aiConfig.speechProviderMock"),
    xfyun: t("aiConfig.speechProviderXfyun"),
    aliyun: t("aiConfig.speechProviderAliyun"),
  }
  const readOnly = status !== null && !status.canUpdate

  return (
    <section className="border-y border-border/70 bg-background py-4" aria-labelledby="speech-runtime-title">
      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex min-w-0 items-center gap-2">
            <AudioLines className="size-4 shrink-0 text-muted-foreground" />
            <h2 id="speech-runtime-title" className="text-sm font-semibold">
              {t("aiConfig.speechTitle")}
            </h2>
            {status ? (
              <>
                <Badge variant={status.configured ? "default" : "outline"}>
                  {status.configured
                    ? t("aiConfig.speechConfigured")
                    : t("aiConfig.speechNotConfigured")}
                </Badge>
                <Badge variant={status.enabled ? "secondary" : "outline"}>
                  {status.enabled
                    ? t("aiConfig.speechEnabled")
                    : t("aiConfig.speechInactive")}
                </Badge>
              </>
            ) : null}
          </div>
          <div className="flex items-center gap-2">
            <Button
              type="button"
              variant="ghost"
              size="icon"
              onClick={() => void loadStatus()}
              disabled={loading || saving}
              aria-label={t("aiConfig.speechRefresh")}
              title={t("aiConfig.speechRefresh")}
            >
              <RefreshCw className={loading ? "animate-spin" : undefined} />
            </Button>
            {readOnly ? <Badge variant="outline">{t("aiConfig.speechReadOnly")}</Badge> : null}
            <Button type="submit" disabled={loading || saving || readOnly}>
              {saving ? <LoaderCircle className="animate-spin" /> : <Save />}
              {saving ? t("aiConfig.speechSaving") : t("aiConfig.speechSave")}
            </Button>
          </div>
        </div>

        <Field>
          <FieldLabel>{t("aiConfig.speechProvider")}</FieldLabel>
          <FieldContent>
            <ToggleGroup
              multiple={false}
              value={[form.provider]}
              onValueChange={(value) => {
                const provider = value[0] as SpeechRuntimeProvider | undefined
                if (provider) setForm((current) => ({ ...current, provider }))
              }}
              variant="outline"
              size="sm"
              disabled={readOnly}
              aria-label={t("aiConfig.speechProvider")}
            >
              {(Object.keys(providerLabels) as SpeechRuntimeProvider[]).map((provider) => (
                <ToggleGroupItem key={provider} value={provider}>
                  {providerLabels[provider]}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
          </FieldContent>
        </Field>

        {form.provider === "mock" ? (
          <div className="max-w-sm">
            <Field>
              <FieldLabel htmlFor="speech-mock-segment-duration">
                {t("aiConfig.speechMockSegmentDuration")}
              </FieldLabel>
              <FieldContent>
                <Input
                  id="speech-mock-segment-duration"
                  type="number"
                  min={500}
                  max={30000}
                  step={100}
                  value={form.mockSegmentDurationMs}
                  disabled={readOnly}
                  onChange={(event) =>
                    setForm((current) => ({
                      ...current,
                      mockSegmentDurationMs: Number(event.target.value),
                    }))
                  }
                />
              </FieldContent>
            </Field>
          </div>
        ) : null}

        {form.provider === "xfyun" ? (
          <div className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <Field>
              <FieldLabel htmlFor="speech-xfyun-app-id">
                {t("aiConfig.speechXfyunAppId")}
              </FieldLabel>
              <FieldContent>
                <Input
                  id="speech-xfyun-app-id"
                  autoComplete="off"
                  value={form.xfyunAppId}
                  disabled={readOnly}
                  placeholder={
                    status?.xfyunCredentialsConfigured
                      ? t("aiConfig.speechKeepExisting")
                      : t("aiConfig.speechXfyunAppIdPlaceholder")
                  }
                  onChange={(event) =>
                    setForm((current) => ({ ...current, xfyunAppId: event.target.value }))
                  }
                />
              </FieldContent>
            </Field>
            <Field>
              <FieldLabel htmlFor="speech-xfyun-api-key">
                {t("aiConfig.speechXfyunApiKey")}
              </FieldLabel>
              <FieldContent>
                <Input
                  id="speech-xfyun-api-key"
                  type="password"
                  autoComplete="new-password"
                  value={form.xfyunApiKey}
                  disabled={readOnly}
                  placeholder={
                    status?.xfyunCredentialsConfigured
                      ? t("aiConfig.speechKeepExisting")
                      : t("aiConfig.speechXfyunApiKeyPlaceholder")
                  }
                  onChange={(event) =>
                    setForm((current) => ({ ...current, xfyunApiKey: event.target.value }))
                  }
                />
              </FieldContent>
            </Field>
            <Field>
              <FieldLabel htmlFor="speech-xfyun-endpoint">
                {t("aiConfig.speechXfyunEndpoint")}
              </FieldLabel>
              <FieldContent>
                <Input
                  id="speech-xfyun-endpoint"
                  inputMode="url"
                  value={form.xfyunEndpoint}
                  disabled={readOnly}
                  placeholder={t("aiConfig.speechKeepExisting")}
                  onChange={(event) =>
                    setForm((current) => ({ ...current, xfyunEndpoint: event.target.value }))
                  }
                />
              </FieldContent>
            </Field>
            <Field>
              <FieldLabel htmlFor="speech-xfyun-domain">
                {t("aiConfig.speechXfyunDomain")}
              </FieldLabel>
              <FieldContent>
                <Input
                  id="speech-xfyun-domain"
                  value={form.xfyunDomain}
                  disabled={readOnly}
                  placeholder={t("aiConfig.speechXfyunDomainPlaceholder")}
                  onChange={(event) =>
                    setForm((current) => ({ ...current, xfyunDomain: event.target.value }))
                  }
                />
              </FieldContent>
            </Field>
          </div>
          <div className="border-t pt-4">
            <div className="mb-4 flex items-center justify-between gap-4">
              <div>
                <div className="text-sm font-medium">{t("aiConfig.speechTranslationTitle")}</div>
              </div>
              <div className="flex items-center gap-2">
                <Badge variant={status?.xfyunTranslationConfigured ? "secondary" : "outline"}>
                  {status?.xfyunTranslationConfigured ? t("aiConfig.speechConfigured") : t("aiConfig.speechNotConfigured")}
                </Badge>
                <Switch
                  checked={form.xfyunTranslationEnabled}
                  disabled={readOnly}
                  onCheckedChange={(checked) => setForm((current) => ({ ...current, xfyunTranslationEnabled: checked }))}
                  aria-label={t("aiConfig.speechTranslationEnabled")}
                />
              </div>
            </div>
            <div className="grid gap-4 md:grid-cols-3">
              <Field>
                <FieldLabel htmlFor="speech-xfyun-translation-secret">{t("aiConfig.speechXfyunApiSecret")}</FieldLabel>
                <FieldContent>
                  <Input
                    id="speech-xfyun-translation-secret"
                    type="password"
                    autoComplete="new-password"
                    value={form.xfyunTranslationApiSecret}
                    disabled={readOnly}
                    placeholder={status?.xfyunTranslationConfigured ? t("aiConfig.speechKeepExisting") : t("aiConfig.speechXfyunApiSecretPlaceholder")}
                    onChange={(event) => setForm((current) => ({ ...current, xfyunTranslationApiSecret: event.target.value }))}
                  />
                </FieldContent>
              </Field>
              <Field>
                <FieldLabel htmlFor="speech-xfyun-translation-endpoint">{t("aiConfig.speechTranslationEndpoint")}</FieldLabel>
                <FieldContent>
                  <Input
                    id="speech-xfyun-translation-endpoint"
                    inputMode="url"
                    value={form.xfyunTranslationEndpoint}
                    disabled={readOnly}
                    placeholder="https://itrans.xfyun.cn/v2/its"
                    onChange={(event) => setForm((current) => ({ ...current, xfyunTranslationEndpoint: event.target.value }))}
                  />
                </FieldContent>
              </Field>
              <Field>
                <FieldLabel htmlFor="speech-xfyun-translation-target">{t("aiConfig.speechTranslationTargetLanguage")}</FieldLabel>
                <FieldContent>
                  <Input
                    id="speech-xfyun-translation-target"
                    value={form.xfyunTranslationTargetLanguage}
                    disabled={readOnly}
                    placeholder="en"
                    onChange={(event) => setForm((current) => ({ ...current, xfyunTranslationTargetLanguage: event.target.value }))}
                  />
                </FieldContent>
              </Field>
            </div>
          </div>
          </div>
        ) : null}
        {form.provider === "aliyun" ? (
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <Field>
              <FieldLabel htmlFor="speech-aliyun-api-key">{t("aiConfig.speechAliyunApiKey")}</FieldLabel>
              <FieldContent>
                <Input
                  id="speech-aliyun-api-key"
                  type="password"
                  autoComplete="new-password"
                  value={form.aliyunApiKey}
                  disabled={readOnly}
                  placeholder={status?.aliyunApiKeyConfigured ? t("aiConfig.speechKeepExisting") : t("aiConfig.speechAliyunApiKeyPlaceholder")}
                  onChange={(event) => setForm((current) => ({ ...current, aliyunApiKey: event.target.value }))}
                />
              </FieldContent>
            </Field>
            <Field>
              <FieldLabel htmlFor="speech-aliyun-workspace-id">{t("aiConfig.speechAliyunWorkspaceId")}</FieldLabel>
              <FieldContent>
                <Input id="speech-aliyun-workspace-id" value={form.aliyunWorkspaceId} disabled={readOnly} placeholder={t("aiConfig.speechAliyunWorkspaceIdPlaceholder")} onChange={(event) => setForm((current) => ({ ...current, aliyunWorkspaceId: event.target.value }))} />
              </FieldContent>
            </Field>
            <Field>
              <FieldLabel htmlFor="speech-aliyun-endpoint">{t("aiConfig.speechAliyunEndpoint")}</FieldLabel>
              <FieldContent>
                <Input id="speech-aliyun-endpoint" inputMode="url" value={form.aliyunEndpoint} disabled={readOnly} placeholder="wss://dashscope.aliyuncs.com/api-ws/v1/inference" onChange={(event) => setForm((current) => ({ ...current, aliyunEndpoint: event.target.value }))} />
              </FieldContent>
            </Field>
            <Field>
              <FieldLabel htmlFor="speech-aliyun-model">{t("aiConfig.speechAliyunModel")}</FieldLabel>
              <FieldContent>
                <Input id="speech-aliyun-model" value={form.aliyunModel} disabled={readOnly} placeholder="qwen-audio-3.0-asr-flash-streaming" onChange={(event) => setForm((current) => ({ ...current, aliyunModel: event.target.value }))} />
              </FieldContent>
            </Field>
          </div>
        ) : null}
      </form>
    </section>
  )
}
