"use client"

import { Checkbox, ConfigProvider, Input, InputNumber, Select } from "antd"
import { FormField, RailopsButton } from "@railops/ui"
import type { ProjectConfiguration, ProjectRuntime } from "@/lib/api/project-configuration"
import { replaceProjectRuntime } from "@/lib/project-configuration-editing"
import { useI18n } from "@/i18n/provider"

export function ProjectRuntimeFields({ document, onChange, disabled = false, mailSecrets, onMailSecretsChange }: { document: ProjectConfiguration; onChange: (document: ProjectConfiguration) => void; disabled?: boolean; mailSecrets?: { smtpPassword: string; imapPassword: string }; onMailSecretsChange?: (secrets: { smtpPassword: string; imapPassword: string }) => void }) {
  const translate = useI18n()
  const pc = (key: string, values?: Record<string, string | number>) => translate(`projectConfiguration.${key}`, values)
  const r = document.runtime
  if (!r) return null
  // The editor passes these values for the newer mail-secret workflow. The
  // runtime editor still stores secret references, so keep the props available
  // without rendering or serializing the secret values here.
  void mailSecrets
  void onMailSecretsChange
  const knowledgeSupport = r.service_scene === "knowledge_support"
  const slaLabels = knowledgeSupport
    ? { response: pc("firstHumanReply"), assignment: pc("humanHandoff"), resolution: pc("problemResolution") }
    : { response: pc("firstReply"), assignment: pc("engineerAcceptance"), resolution: pc("resolution") }
  const imap = r.mail.imap ?? { enabled: false, host: "", port: 993, username: "", password_ref: "", project_key: "", ticket_type: "" }
  const setIMAP = (patch: Partial<typeof imap>) => set({ mail: { ...r.mail, imap: { ...imap, ...patch } } })
  const set = (patch: Partial<ProjectRuntime>) => onChange(replaceProjectRuntime(document, { ...r, ...patch }))
  const field = (name: string, value: string, update: (value: string) => void) => <FormField label={name}><Input aria-label={name} value={value} onChange={e => update(e.target.value)} /></FormField>
  const number = (name: string, value: number, update: (value: number) => void) => <FormField label={name}><InputNumber aria-label={name} min={0} className="!w-full" value={value} onChange={v => update(v ?? 0)} /></FormField>
  return <ConfigProvider componentDisabled={disabled}><fieldset disabled={disabled} className="grid min-w-0 gap-4">
    <p className="text-sm">{pc("intro")}</p>
    <details open><summary className="font-medium">{pc("serviceSettings")}</summary><div className="mt-3 grid gap-3 sm:grid-cols-2">
      {field(pc("defaultTimezone"), r.timezone, timezone => set({ timezone }))}
      <FormField label={pc("defaultLanguage")}><Select aria-label={pc("defaultLanguage")} className="w-full" value={r.locale} options={["zh-CN", "en", "en-US", "ar", "ar-SA"].map(value => ({ value }))} onChange={locale => set({ locale })} /></FormField>
      <FormField label={pc("supportedLanguages")}><Select mode="multiple" className="w-full" value={r.locales} options={["zh-CN", "en", "en-US", "ar", "ar-SA"].map(value => ({ value }))} onChange={locales => set({ locales })} /></FormField>
      <FormField label={pc("serviceMode")}><Select className="w-full" value={r.service_scene} options={[{ value: "knowledge_support", label: pc("knowledgeSupport") }, { value: "equipment_after_sales", label: pc("equipmentAfterSales") }]} onChange={service_scene => set({ service_scene })} /></FormField>
    </div></details>
    <details><summary className="font-medium">{pc("workCalendar")}</summary><div className="mt-3 grid gap-3">
      {(r.calendars ?? []).map((c, i) => {
        const change = (patch: Partial<typeof c>) => set({ calendars: r.calendars.map((item, j) => i === j ? { ...item, ...patch } : item) })
        return <div key={i} className="grid gap-3 rounded border p-3 sm:grid-cols-2">
          {field(`${pc("calendarIdentifier")} ${i + 1}`, c.key, key => change({ key }))}{field(`${pc("calendarTimezone")} ${i + 1}`, c.timezone, timezone => change({ timezone }))}
          {field(`${pc("startTime")} ${i + 1}`, c.start, start => change({ start }))}{field(`${pc("endTime")} ${i + 1}`, c.end, end => change({ end }))}
          <div className="sm:col-span-2"><Checkbox.Group value={c.work_days ?? []} options={[pc("sunday"), pc("monday"), pc("tuesday"), pc("wednesday"), pc("thursday"), pc("friday"), pc("saturday")].map((label, value) => ({ label, value }))} onChange={work_days => change({ work_days })} /></div>
          <div className="flex flex-wrap gap-2 sm:col-span-2">
            <RailopsButton onClick={() => change({ work_days: [0, 1, 2, 3, 4, 5, 6], start: "00:00", end: "24:00" })}>{pc("allDay")}</RailopsButton>
            <RailopsButton onClick={() => change({ work_days: [1, 2, 3, 4, 5], start: "09:00", end: "18:00" })}>{pc("businessHours")}</RailopsButton>
          </div>
          <RailopsButton onClick={() => set({ calendars: r.calendars.filter((_, j) => i !== j) })}>{pc("deleteCalendar")}</RailopsButton>
        </div>
      })}
      <RailopsButton onClick={() => set({ calendars: [...r.calendars, { key: "", timezone: r.timezone, work_days: [1, 2, 3, 4, 5], start: "09:00", end: "18:00", holidays: [] }] })}>{pc("addCalendar")}</RailopsButton>
    </div></details>
    <details><summary className="font-medium">{pc("slaTitle")}</summary><div className="mt-3 grid gap-3">
      <p className="text-sm">{pc("slaIntro")}</p>
      <p className="text-sm text-muted-foreground">{pc("slaDetail")}</p>
      {(r.targets ?? []).map((t, i) => {
        const change = (patch: Partial<typeof t>) => set({ targets: r.targets.map((item, j) => i === j ? { ...item, ...patch } : item) })
        return <div key={i} className="grid gap-3 rounded border p-3 sm:grid-cols-2">
          <FormField label={pc("serviceTier")}><Select aria-label={`${pc("serviceTier")} ${i + 1}`} className="w-full" value={t.profile} options={[{ value: "standard", label: pc("standardService") }, { value: "enhanced", label: pc("enhancedService") }, { value: "mission_critical", label: pc("missionCritical") }]} onChange={profile => {
            const preset = r.targets.find((item, j) => j !== i && item.profile === profile)
            change(preset
              ? { profile, response_minutes: preset.response_minutes, assignment_minutes: preset.assignment_minutes, resolution_minutes: preset.resolution_minutes }
              : { profile })
          }} /></FormField>
          <FormField label={pc("workCalendar")}><Select aria-label={`${pc("workCalendar")} ${i + 1}`} className="w-full" value={t.calendar_key} options={r.calendars.map(c => ({ value: c.key }))} onChange={calendar_key => change({ calendar_key })} /></FormField>
          {number(`${slaLabels.response} ${i + 1}`, t.response_minutes, response_minutes => change({ response_minutes }))}
          {number(`${slaLabels.assignment} ${i + 1}`, t.assignment_minutes, assignment_minutes => change({ assignment_minutes }))}
          {number(`${slaLabels.resolution} ${i + 1}`, t.resolution_minutes, resolution_minutes => change({ resolution_minutes }))}
          <div className="flex flex-wrap gap-2">
            <RailopsButton onClick={() => set({ targets: r.targets.filter((_, j) => i !== j) })}>{pc("deleteSla")}</RailopsButton>
          </div>
        </div>
      })}
      <RailopsButton onClick={() => set({ targets: [...r.targets, { project_key: "*", profile: "standard", priority: "", calendar_key: r.calendars[0]?.key ?? "", response_minutes: 0, assignment_minutes: 0, resolution_minutes: 0 }] })}>{pc("addSla")}</RailopsButton>
    </div></details>
    <details><summary className="font-medium">{pc("ticketSources")}</summary><div className="mt-3 flex flex-wrap gap-3">{(r.channels ?? []).map((c, i) => <Checkbox key={c.name} checked={c.enabled} onChange={e => set({ channels: r.channels.map((item, j) => i === j ? { ...item, enabled: e.target.checked } : item) })}>{({ manual: pc("sourceManual"), phone: pc("sourcePhone"), email: pc("sourceEmail"), api: pc("sourceApi"), webhook: pc("sourceWebhook"), monitoring_alert: pc("sourceMonitoring"), whatsapp: pc("sourceWhatsapp"), chatbot_handoff: pc("sourceChatbot") } as Record<string, string>)[c.name] ?? c.name}</Checkbox>)}</div></details>
    <details><summary className="font-medium">{pc("emailSending")}</summary><div className="mt-3 grid gap-3 sm:grid-cols-2">
      <Checkbox checked={r.mail.enabled} onChange={e => set({ mail: { ...r.mail, enabled: e.target.checked } })}>{pc("enableEmailSending")}</Checkbox>
      <Checkbox checked={r.mail.use_tls} onChange={e => set({ mail: { ...r.mail, use_tls: e.target.checked } })}>{pc("tlsEncryption")}</Checkbox>
      {field(pc("smtpAddress"), r.mail.host, host => set({ mail: { ...r.mail, host } }))}{number(pc("smtpPort"), r.mail.port, port => set({ mail: { ...r.mail, port } }))}
      {field(pc("senderAccount"), r.mail.username, username => set({ mail: { ...r.mail, username } }))}{field(pc("smtpPassword"), r.mail.password_ref, password_ref => set({ mail: { ...r.mail, password_ref } }))}
      {field(pc("senderEmail"), r.mail.from_address, from_address => set({ mail: { ...r.mail, from_address } }))}{field(pc("senderName"), r.mail.from_name, from_name => set({ mail: { ...r.mail, from_name } }))}
      {field(pc("replyEmail"), r.mail.reply_to, reply_to => set({ mail: { ...r.mail, reply_to } }))}
      <FormField label={pc("sendRetry")}><Select className="w-full" value={r.mail.retry_policy} options={[{value:"retry_3_10m",label:pc("retry3")},{value:"retry_1_5m",label:pc("retry1")},{value:"no_retry",label:pc("noRetry")}]} onChange={retry_policy=>set({mail:{...r.mail,retry_policy}})} /></FormField>
      <p className="text-sm sm:col-span-2">{pc("secretHelp")}</p>
    </div></details>
    <details><summary className="font-medium">{pc("automaticTicket")}</summary><div className="mt-3 grid gap-3 sm:grid-cols-2">
      <Checkbox checked={imap.enabled} onChange={e => setIMAP({ enabled: e.target.checked })}>{pc("enableAutoReceive")}</Checkbox>
      <p className="text-sm sm:col-span-2">{pc("autoReceiveHelp")}</p>
      {field(pc("imapAddress"), imap.host, host => setIMAP({ host }))}{number(pc("imapPort"), imap.port, port => setIMAP({ port }))}
      {field(pc("mailboxAccount"), imap.username, username => setIMAP({ username }))}{field(pc("imapPassword"), imap.password_ref, password_ref => setIMAP({ password_ref }))}
      {field(pc("defaultTicketType"), imap.ticket_type, ticket_type => setIMAP({ ticket_type }))}
    </div></details>
    <details><summary className="font-medium">{pc("externalSystems")}</summary><div className="mt-3 grid gap-3">
      {(r.integrations ?? []).map((c, i) => {
        const change = (patch: Partial<typeof c>) => set({ integrations: r.integrations.map((item, j) => i === j ? { ...item, ...patch } : item) })
        return <div key={i} className="grid gap-3 rounded border p-3 sm:grid-cols-2">
          {field(`${pc("integrationIdentifier")} ${i + 1}`, c.provider, provider => change({ provider }))}{field(`${pc("integrationAddress")} ${i + 1}`, c.base_url, base_url => change({ base_url }))}
          {field(`${pc("appId")} ${i + 1}`, c.app_id, app_id => change({ app_id }))}{field(`${pc("secretRef")} ${i + 1}`, c.secret_ref, secret_ref => change({ secret_ref }))}
          {field(`${pc("appKeyRef")} ${i + 1}`, c.key_ref, key_ref => change({ key_ref }))}
          {field(`${pc("metadataJson")} ${i + 1}`, c.metadata_json, metadata_json => change({ metadata_json }))}
          <Checkbox checked={c.enabled} onChange={e => change({ enabled: e.target.checked })}>{pc("enableIntegration")}</Checkbox><RailopsButton onClick={() => set({ integrations: r.integrations.filter((_, j) => i !== j) })}>{pc("deleteIntegration")}</RailopsButton>
        </div>
      })}
      <RailopsButton onClick={() => set({ integrations: [...r.integrations, { provider: "", enabled: false, base_url: "", app_id: "", secret_ref: "", key_ref: "", metadata_json: "{}" }] })}>{pc("addIntegration")}</RailopsButton>
    </div></details>
    <details><summary className="font-medium">{pc("retentionTitle")}</summary><div className="mt-3 grid gap-3 sm:grid-cols-2">
      <div className="sm:col-span-2">
        <p className="font-medium text-xs text-muted-foreground mb-2">
          {pc("retentionCategoriesTitle")}
        </p>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
          {number(pc("retentionTicket"), r.retention.ticket_days || r.retention.days || 365, v => set({ retention: { ...r.retention, ticket_days: v, days: v || r.retention.days || 365 } }))}
          {number(pc("retentionAttachment"), r.retention.attachment_days || 180, v => set({ retention: { ...r.retention, attachment_days: v } }))}
          {number(pc("retentionEvent"), r.retention.event_days || 90, v => set({ retention: { ...r.retention, event_days: v } }))}
          {number(pc("retentionAudit"), r.retention.audit_days || 180, v => set({ retention: { ...r.retention, audit_days: v } }))}
          {number(pc("retentionMetric"), r.retention.metric_days || 30, v => set({ retention: { ...r.retention, metric_days: v } }))}
          {number(pc("retentionLog"), r.retention.log_days || 30, v => set({ retention: { ...r.retention, log_days: v } }))}
          {number(pc("retentionReport"), r.retention.report_days || 90, v => set({ retention: { ...r.retention, report_days: v } }))}
          {number(pc("retentionBackup"), r.retention.backup_days || 30, v => set({ retention: { ...r.retention, backup_days: v } }))}
        </div>
      </div>
      <Checkbox checked={r.retention.auto_delete} onChange={e => set({ retention: { ...r.retention, auto_delete: e.target.checked } })}>{pc("autoDelete")}</Checkbox>
      <Checkbox checked={r.auto_close.enabled} onChange={e => set({ auto_close: { ...r.auto_close, enabled: e.target.checked } })}>{pc("autoClose")}</Checkbox>
      {number(pc("autoCloseWait"), r.auto_close.days, days => set({ auto_close: { ...r.auto_close, days } }))}
    </div></details>
  </fieldset></ConfigProvider>
}
