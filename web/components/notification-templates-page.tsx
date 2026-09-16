"use client"

import { useCallback, useEffect, useState } from "react"
import {
  BadgeCheckIcon,
  FileTextIcon,
  PlusIcon,
  RefreshCwIcon,
  SendIcon,
  ShieldAlertIcon,
  XCircleIcon,
} from "lucide-react"
import { toast } from "sonner"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { DashboardPage, DashboardToolbar } from "@/components/dashboard-page"
import { Input } from "@/components/ui/input"
import { useI18n } from "@/i18n/provider"
import { readSession } from "@/lib/auth"
import {
  approveNotificationTemplate,
  createNotificationTemplate,
  deleteNotificationTemplate,
  fetchNotificationDeliveryAttempts,
  fetchNotificationTemplates,
  previewNotificationTemplate,
  retireNotificationTemplate,
  seedNotificationTemplates,
  updateNotificationTemplate,
  type NotificationDeliveryAttempt,
  type NotificationTemplate,
  type NotificationTemplatePreview,
} from "@/lib/api/notification-templates"

const CHANNELS = ["in_app", "email", "wxwork", "sms"]
const LANGUAGES = ["zh-CN", "en-US", "es-ES"]
const APPROVAL_STATUSES = ["draft", "approved", "retired"]
const PAGE_SIZE = 20

type TemplateForm = {
  id: number | null
  code: string
  name: string
  channel: string
  language: string
  titleTemplate: string
  contentTemplate: string
}

const EMPTY_FORM: TemplateForm = {
  id: null,
  code: "",
  name: "",
  channel: "in_app",
  language: "zh-CN",
  titleTemplate: "",
  contentTemplate: "",
}

export function NotificationTemplatesPage() {
  const t = useI18n()
  const session = readSession()
  const canManage = session?.permissions.includes("notification.update") ?? false

  const [tab, setTab] = useState<"templates" | "attempts">("templates")
  const [items, setItems] = useState<NotificationTemplate[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [channel, setChannel] = useState("")
  const [language, setLanguage] = useState("")
  const [status, setStatus] = useState("")
  const [keyword, setKeyword] = useState("")
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState("")
  const [form, setForm] = useState<TemplateForm | null>(null)
  const [preview, setPreview] = useState<NotificationTemplatePreview | null>(null)
  const [saving, setSaving] = useState(false)
  const [seeding, setSeeding] = useState(false)

  const [attempts, setAttempts] = useState<NotificationDeliveryAttempt[]>([])
  const [attemptTotal, setAttemptTotal] = useState(0)
  const [attemptPage, setAttemptPage] = useState(1)
  const [attemptStatus, setAttemptStatus] = useState("")
  const [attemptsBusy, setAttemptsBusy] = useState(false)
  const [attemptsError, setAttemptsError] = useState("")

  const load = useCallback(async () => {
    setBusy(true)
    setError("")
    const res = await fetchNotificationTemplates({
      channel: channel || undefined,
      language: language || undefined,
      approval_status: status || undefined,
      keyword: keyword || undefined,
      page,
      page_size: PAGE_SIZE,
    })
    if (res.success && res.data) {
      setItems(res.data.items ?? [])
      setTotal(res.data.total ?? 0)
    } else {
      setItems([])
      setTotal(0)
      setError(res.error?.message || t("notificationTemplates.loadFailed"))
    }
    setBusy(false)
  }, [channel, keyword, language, page, status, t])

  const loadAttempts = useCallback(async () => {
    setAttemptsBusy(true)
    setAttemptsError("")
    const res = await fetchNotificationDeliveryAttempts({
      status: attemptStatus || undefined,
      page: attemptPage,
      page_size: PAGE_SIZE,
    })
    if (res.success && res.data) {
      setAttempts(res.data.items ?? [])
      setAttemptTotal(res.data.total ?? 0)
    } else {
      setAttempts([])
      setAttemptTotal(0)
      setAttemptsError(res.error?.message || t("notificationTemplates.loadFailed"))
    }
    setAttemptsBusy(false)
  }, [attemptPage, attemptStatus, t])

  useEffect(() => {
    const timer = window.setTimeout(() => void load(), 150)
    return () => window.clearTimeout(timer)
  }, [load])

  useEffect(() => {
    if (tab !== "attempts") return
    const timer = window.setTimeout(() => void loadAttempts(), 0)
    return () => window.clearTimeout(timer)
  }, [loadAttempts, tab])

  function openCreate() {
    setPreview(null)
    setForm({ ...EMPTY_FORM })
  }

  function openEdit(item: NotificationTemplate) {
    setPreview(null)
    setForm({
      id: item.id,
      code: item.code,
      name: item.name,
      channel: item.channel,
      language: item.language,
      titleTemplate: item.title_template,
      contentTemplate: item.content_template,
    })
  }

  function payload() {
    if (!form) return null
    return {
      code: form.code.trim(),
      name: form.name.trim(),
      channel: form.channel,
      language: form.language,
      titleTemplate: form.titleTemplate,
      contentTemplate: form.contentTemplate,
    }
  }

  async function save() {
    if (!form || !canManage) return
    const body = payload()
    if (!body) return
    if (!body.code) {
      toast.error(t("notificationTemplates.codeRequired"))
      return
    }
    setSaving(true)
    const res = form.id
      ? await updateNotificationTemplate(form.id, body)
      : await createNotificationTemplate(body)
    setSaving(false)
    if (res.success && res.data) {
      toast.success(form.id ? t("notificationTemplates.saved") : t("notificationTemplates.created"))
      setForm(null)
      setPreview(null)
      await load()
    } else {
      toast.error(res.error?.message || t("notificationTemplates.saveFailed"))
    }
  }

  async function runPreview() {
    if (!form) return
    const body = payload()
    if (!body) return
    const res = await previewNotificationTemplate(body)
    if (res.success && res.data) {
      setPreview(res.data)
    } else {
      toast.error(res.error?.message || t("notificationTemplates.previewFailed"))
    }
  }

  async function approve(item: NotificationTemplate) {
    const res = await approveNotificationTemplate(item.id)
    if (res.success) {
      toast.success(t("notificationTemplates.approved"))
      await load()
    } else {
      toast.error(res.error?.message || t("notificationTemplates.actionFailed"))
    }
  }

  async function retire(item: NotificationTemplate) {
    const res = await retireNotificationTemplate(item.id)
    if (res.success) {
      toast.success(t("notificationTemplates.retired"))
      await load()
    } else {
      toast.error(res.error?.message || t("notificationTemplates.actionFailed"))
    }
  }

  async function remove(item: NotificationTemplate) {
    const res = await deleteNotificationTemplate(item.id)
    if (res.success) {
      toast.success(t("notificationTemplates.deleted"))
      if (form?.id === item.id) setForm(null)
      await load()
    } else {
      toast.error(res.error?.message || t("notificationTemplates.actionFailed"))
    }
  }

  async function seedDefaults() {
    setSeeding(true)
    const res = await seedNotificationTemplates()
    setSeeding(false)
    if (res.success && res.data) {
      toast.success(t("notificationTemplates.seeded", { count: res.data.created }))
      await load()
    } else {
      toast.error(res.error?.message || t("notificationTemplates.actionFailed"))
    }
  }

  const statusTone = (value: string) =>
    value === "approved" ? "default" : value === "retired" ? "outline" : "secondary"

  return (
    <DashboardPage>
      <DashboardToolbar
        actions={
          <>
            {canManage && (
              <Button variant="outline" disabled={seeding} onClick={() => void seedDefaults()}>
                <FileTextIcon />{t("notificationTemplates.seed")}
              </Button>
            )}
            {canManage && (
              <Button variant="outline" onClick={openCreate}>
                <PlusIcon />{t("notificationTemplates.newTemplate")}
              </Button>
            )}
            <Button variant="outline" disabled={busy} onClick={() => void load()}>
              <RefreshCwIcon />{t("notificationTemplates.refresh")}
            </Button>
          </>
        }
      >
        <div>
          <h1 className="flex items-center gap-2 text-xl font-semibold">
            <BadgeCheckIcon />{t("notificationTemplates.title")}
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">{t("notificationTemplates.description")}</p>
        </div>
      </DashboardToolbar>

      <div className="flex gap-2">
        {(["templates", "attempts"] as const).map((key) => (
          <Button
            key={key}
            size="sm"
            variant={tab === key ? "default" : "outline"}
            onClick={() => setTab(key)}
          >
            {t(`notificationTemplates.tabs.${key}`)}
          </Button>
        ))}
      </div>

      {tab === "templates" && (
        <>
          <div className="flex flex-wrap gap-3">
            <Input
              className="max-w-sm"
              aria-label={t("notificationTemplates.search")}
              placeholder={t("notificationTemplates.search")}
              value={keyword}
              onChange={(event) => {
                setKeyword(event.target.value)
                setPage(1)
              }}
            />
            <select
              className="h-9 rounded-md border bg-background px-3 text-sm"
              aria-label={t("notificationTemplates.channel")}
              value={channel}
              onChange={(event) => {
                setChannel(event.target.value)
                setPage(1)
              }}
            >
              <option value="">{t("notificationTemplates.allChannels")}</option>
              {CHANNELS.map((value) => (
                <option key={value} value={value}>{t(`notificationTemplates.channels.${value}`)}</option>
              ))}
            </select>
            <select
              className="h-9 rounded-md border bg-background px-3 text-sm"
              aria-label={t("notificationTemplates.language")}
              value={language}
              onChange={(event) => {
                setLanguage(event.target.value)
                setPage(1)
              }}
            >
              <option value="">{t("notificationTemplates.allLanguages")}</option>
              {LANGUAGES.map((value) => (
                <option key={value} value={value}>{t(`notificationTemplates.languages.${value}`)}</option>
              ))}
            </select>
            <select
              className="h-9 rounded-md border bg-background px-3 text-sm"
              aria-label={t("notificationTemplates.approval")}
              value={status}
              onChange={(event) => {
                setStatus(event.target.value)
                setPage(1)
              }}
            >
              <option value="">{t("notificationTemplates.allStatuses")}</option>
              {APPROVAL_STATUSES.map((value) => (
                <option key={value} value={value}>{t(`notificationTemplates.statuses.${value}`)}</option>
              ))}
            </select>
          </div>
          {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
          <div className="overflow-x-auto rounded-md border" aria-busy={busy}>
            <table className="w-full text-left text-sm">
              <thead className="bg-muted/50">
                <tr>
                  {["code", "name", "channel", "language", "approval", "updatedAt", "actions"].map((key) => (
                    <th key={key} className="whitespace-nowrap p-3 font-medium">
                      {t(`notificationTemplates.columns.${key}`)}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {items.map((item) => (
                  <tr key={item.id} className="border-t align-top">
                    <td className="p-3 font-mono text-xs">{item.code}</td>
                    <td className="p-3">
                      <p>{item.name}</p>
                      <p className="text-xs text-muted-foreground">
                        {item.source === "platform"
                          ? t("notificationTemplates.platformTemplate")
                          : t("notificationTemplates.tenantTemplate")}
                        {" · #"}{item.id}
                      </p>
                    </td>
                    <td className="p-3">{t(`notificationTemplates.channels.${item.channel}`)}</td>
                    <td className="p-3">{t(`notificationTemplates.languages.${item.language}`)}</td>
                    <td className="p-3">
                      <Badge variant={statusTone(item.approval_status)}>
                        {t(`notificationTemplates.statuses.${item.approval_status}`)}
                      </Badge>
                    </td>
                    <td className="whitespace-nowrap p-3">
                      {item.updated_at ? new Date(item.updated_at).toLocaleString() : "—"}
                    </td>
                    <td className="p-3">
                      <div className="flex flex-wrap gap-2">
                        {item.editable ? (
                          <>
                            <Button size="sm" variant="outline" disabled={!canManage} onClick={() => openEdit(item)}>
                              {t("notificationTemplates.edit")}
                            </Button>
                            {item.approval_status !== "approved" && (
                              <Button size="sm" disabled={!canManage} onClick={() => void approve(item)}>
                                {t("notificationTemplates.approve")}
                              </Button>
                            )}
                            {item.approval_status === "approved" && (
                              <Button size="sm" variant="outline" disabled={!canManage} onClick={() => void retire(item)}>
                                {t("notificationTemplates.retire")}
                              </Button>
                            )}
                            <Button size="sm" variant="outline" disabled={!canManage} onClick={() => void remove(item)}>
                              {t("notificationTemplates.delete")}
                            </Button>
                          </>
                        ) : (
                          <span className="text-xs text-muted-foreground">{t("notificationTemplates.readOnly")}</span>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
                {!items.length && (
                  <tr>
                    <td colSpan={7} className="p-8 text-center text-muted-foreground">
                      {busy ? t("notificationTemplates.loading") : t("notificationTemplates.empty")}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
          <div className="flex items-center justify-between text-sm">
            <span>{t("notificationTemplates.total", { total })}</span>
            <div className="flex items-center gap-3">
              <Button variant="outline" disabled={page === 1 || busy} onClick={() => setPage((value) => value - 1)}>
                {t("notificationTemplates.previous")}
              </Button>
              <span>{page}</span>
              <Button
                variant="outline"
                disabled={page * PAGE_SIZE >= total || busy}
                onClick={() => setPage((value) => value + 1)}
              >
                {t("notificationTemplates.next")}
              </Button>
            </div>
          </div>

          {form && (
            <section className="space-y-3 rounded-md border p-4" aria-label={t("notificationTemplates.formTitle")}>
              <h2 className="font-semibold">
                {form.id ? t("notificationTemplates.formEditTitle") : t("notificationTemplates.formCreateTitle")}
              </h2>
              <div className="grid gap-3 sm:grid-cols-2">
                <label className="block text-sm">
                  <span className="mb-1 block text-xs text-muted-foreground">{t("notificationTemplates.code")}</span>
                  <Input
                    value={form.code}
                    disabled={form.id !== null}
                    aria-label={t("notificationTemplates.code")}
                    placeholder="ticket_assigned"
                    onChange={(event) => setForm({ ...form, code: event.target.value })}
                  />
                </label>
                <label className="block text-sm">
                  <span className="mb-1 block text-xs text-muted-foreground">{t("notificationTemplates.name")}</span>
                  <Input
                    value={form.name}
                    aria-label={t("notificationTemplates.name")}
                    onChange={(event) => setForm({ ...form, name: event.target.value })}
                  />
                </label>
                <label className="block text-sm">
                  <span className="mb-1 block text-xs text-muted-foreground">{t("notificationTemplates.channel")}</span>
                  <select
                    className="h-9 w-full rounded-md border bg-background px-3 text-sm"
                    aria-label={t("notificationTemplates.channel")}
                    value={form.channel}
                    onChange={(event) => setForm({ ...form, channel: event.target.value })}
                  >
                    {CHANNELS.map((value) => (
                      <option key={value} value={value}>{t(`notificationTemplates.channels.${value}`)}</option>
                    ))}
                  </select>
                </label>
                <label className="block text-sm">
                  <span className="mb-1 block text-xs text-muted-foreground">{t("notificationTemplates.language")}</span>
                  <select
                    className="h-9 w-full rounded-md border bg-background px-3 text-sm"
                    aria-label={t("notificationTemplates.language")}
                    value={form.language}
                    onChange={(event) => setForm({ ...form, language: event.target.value })}
                  >
                    {LANGUAGES.map((value) => (
                      <option key={value} value={value}>{t(`notificationTemplates.languages.${value}`)}</option>
                    ))}
                  </select>
                </label>
              </div>
              <label className="block text-sm">
                <span className="mb-1 block text-xs text-muted-foreground">{t("notificationTemplates.titleTemplate")}</span>
                <Input
                  value={form.titleTemplate}
                  aria-label={t("notificationTemplates.titleTemplate")}
                  placeholder={t("notificationTemplates.titlePlaceholder")}
                  onChange={(event) => setForm({ ...form, titleTemplate: event.target.value })}
                />
              </label>
              <label className="block text-sm">
                <span className="mb-1 block text-xs text-muted-foreground">{t("notificationTemplates.contentTemplate")}</span>
                <textarea
                  className="min-h-28 w-full rounded-md border bg-background p-3 text-sm"
                  value={form.contentTemplate}
                  aria-label={t("notificationTemplates.contentTemplate")}
                  placeholder={t("notificationTemplates.contentPlaceholder")}
                  onChange={(event) => setForm({ ...form, contentTemplate: event.target.value })}
                />
              </label>
              <p className="text-xs text-muted-foreground">{t("notificationTemplates.variableHint")}</p>
              <div className="flex flex-wrap gap-2">
                <Button disabled={saving || !canManage} onClick={() => void save()}>
                  {t("notificationTemplates.saveDraft")}
                </Button>
                <Button variant="outline" onClick={() => void runPreview()}>
                  {t("notificationTemplates.preview")}
                </Button>
                <Button variant="outline" onClick={() => { setForm(null); setPreview(null) }}>
                  {t("notificationTemplates.cancel")}
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">{t("notificationTemplates.approvalHint")}</p>
              {preview && (
                <div className="space-y-2 rounded-md border bg-muted/30 p-3 text-sm">
                  <p className="font-medium">{preview.title || "—"}</p>
                  <p className="whitespace-pre-wrap text-muted-foreground">{preview.content || "—"}</p>
                  <p className={`flex items-center gap-1 text-xs ${preview.blocked ? "text-destructive" : "text-muted-foreground"}`}>
                    {preview.blocked ? <ShieldAlertIcon className="size-3" /> : <SendIcon className="size-3" />}
                    {preview.blocked
                      ? t("notificationTemplates.previewBlocked", { rule: preview.rule_label || preview.rule_code })
                      : t("notificationTemplates.previewSafe")}
                  </p>
                </div>
              )}
            </section>
          )}
        </>
      )}

      {tab === "attempts" && (
        <>
          <div className="flex flex-wrap gap-3">
            <select
              className="h-9 rounded-md border bg-background px-3 text-sm"
              aria-label={t("notificationTemplates.attemptStatus")}
              value={attemptStatus}
              onChange={(event) => {
                setAttemptStatus(event.target.value)
                setAttemptPage(1)
              }}
            >
              <option value="">{t("notificationTemplates.allStatuses")}</option>
              {["sent", "failed", "blocked", "skipped"].map((value) => (
                <option key={value} value={value}>{t(`notificationTemplates.attemptStatuses.${value}`)}</option>
              ))}
            </select>
            <Button variant="outline" disabled={attemptsBusy} onClick={() => void loadAttempts()}>
              <RefreshCwIcon />{t("notificationTemplates.refresh")}
            </Button>
          </div>
          {attemptsError && <p role="alert" className="text-sm text-destructive">{attemptsError}</p>}
          <div className="overflow-x-auto rounded-md border" aria-busy={attemptsBusy}>
            <table className="w-full text-left text-sm">
              <thead className="bg-muted/50">
                <tr>
                  {["time", "channel", "status", "reason", "detail", "template"].map((key) => (
                    <th key={key} className="whitespace-nowrap p-3 font-medium">
                      {t(`notificationTemplates.attemptColumns.${key}`)}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {attempts.map((attempt) => (
                  <tr key={attempt.id} className="border-t align-top">
                    <td className="whitespace-nowrap p-3">
                      {attempt.created_at ? new Date(attempt.created_at).toLocaleString() : "—"}
                    </td>
                    <td className="p-3">
                      {t(`notificationTemplates.channels.${attempt.channel}`) || attempt.channel}
                    </td>
                    <td className="p-3">
                      <Badge variant={attempt.status === "sent" ? "default" : attempt.status === "blocked" ? "destructive" : attempt.status === "skipped" ? "outline" : "secondary"}>
                        {attempt.status === "sent" && <BadgeCheckIcon className="mr-1 size-3" />}
                        {attempt.status !== "sent" && <XCircleIcon className="mr-1 size-3" />}
                        {t(`notificationTemplates.attemptStatuses.${attempt.status}`) || attempt.status}
                      </Badge>
                    </td>
                    <td className="p-3">{attempt.reason || "—"}</td>
                    <td className="max-w-md break-words p-3 text-muted-foreground">{attempt.detail || "—"}</td>
                    <td className="p-3 text-xs">
                      <p>{attempt.template_code || "—"}</p>
                      <p className="text-muted-foreground">
                        {attempt.notification_id > 0 ? `#${attempt.notification_id}` : t("notificationTemplates.notDelivered")}
                      </p>
                    </td>
                  </tr>
                ))}
                {!attempts.length && (
                  <tr>
                    <td colSpan={6} className="p-8 text-center text-muted-foreground">
                      {attemptsBusy ? t("notificationTemplates.loading") : t("notificationTemplates.emptyAttempts")}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
          <div className="flex items-center justify-between text-sm">
            <span>{t("notificationTemplates.total", { total: attemptTotal })}</span>
            <div className="flex items-center gap-3">
              <Button variant="outline" disabled={attemptPage === 1 || attemptsBusy} onClick={() => setAttemptPage((value) => value - 1)}>
                {t("notificationTemplates.previous")}
              </Button>
              <span>{attemptPage}</span>
              <Button
                variant="outline"
                disabled={attemptPage * PAGE_SIZE >= attemptTotal || attemptsBusy}
                onClick={() => setAttemptPage((value) => value + 1)}
              >
                {t("notificationTemplates.next")}
              </Button>
            </div>
          </div>
        </>
      )}
    </DashboardPage>
  )
}