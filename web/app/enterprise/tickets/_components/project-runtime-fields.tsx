"use client"

import { Checkbox, ConfigProvider, Input, InputNumber, Select } from "antd"
import { FormField, RailopsButton } from "@railops/ui"
import type { ProjectConfiguration, ProjectRuntime } from "@/lib/api/project-configuration"
import { replaceProjectRuntime } from "@/lib/project-configuration-editing"

export function ProjectRuntimeFields({ document, onChange, disabled = false }: { document: ProjectConfiguration; onChange: (document: ProjectConfiguration) => void; disabled?: boolean }) {
  const r = document.runtime
  if (!r) return null
  const set = (patch: Partial<ProjectRuntime>) => onChange(replaceProjectRuntime(document, { ...r, ...patch }))
  const field = (name: string, value: string, update: (value: string) => void) => <FormField label={name}><Input aria-label={name} value={value} onChange={e => update(e.target.value)} /></FormField>
  const number = (name: string, value: number, update: (value: number) => void) => <FormField label={name}><InputNumber aria-label={name} min={0} className="!w-full" value={value} onChange={v => update(v ?? 0)} /></FormField>
  return <ConfigProvider componentDisabled={disabled}><fieldset disabled={disabled} className="grid min-w-0 gap-4">
    <p className="text-sm">以下设置随本次配置一起生效。恢复旧版本也会恢复这些设置；已创建工单的 SLA 继续使用创建时的版本。开启渠道只允许该来源录单，不会自动开通外部账号。</p>
    <details open><summary className="font-medium">公司服务设置</summary><div className="mt-3 grid gap-3 sm:grid-cols-2">
      {field("默认时区", r.timezone, timezone => set({ timezone }))}
      <FormField label="默认语言"><Select aria-label="默认语言" className="w-full" value={r.locale} options={["zh-CN", "en", "en-US", "ar", "ar-SA"].map(value => ({ value }))} onChange={locale => set({ locale })} /></FormField>
      <FormField label="支持语言"><Select mode="multiple" className="w-full" value={r.locales} options={["zh-CN", "en", "en-US", "ar", "ar-SA"].map(value => ({ value }))} onChange={locales => set({ locales })} /></FormField>
      <FormField label="服务模式"><Select className="w-full" value={r.service_scene} options={[{ value: "knowledge_support", label: "知识管理服务" }, { value: "equipment_after_sales", label: "设备售后服务" }]} onChange={service_scene => set({ service_scene })} /></FormField>
    </div></details>
    <details><summary className="font-medium">工作日历</summary><div className="mt-3 grid gap-3">
      {(r.calendars ?? []).map((c, i) => {
        const change = (patch: Partial<typeof c>) => set({ calendars: r.calendars.map((item, j) => i === j ? { ...item, ...patch } : item) })
        return <div key={i} className="grid gap-3 rounded border p-3 sm:grid-cols-2">
          {field(`日历标识 ${i + 1}`, c.key, key => change({ key }))}{field(`日历时区 ${i + 1}`, c.timezone, timezone => change({ timezone }))}
          {field(`上班时间 ${i + 1}`, c.start, start => change({ start }))}{field(`下班时间 ${i + 1}`, c.end, end => change({ end }))}
          <div className="sm:col-span-2"><Checkbox.Group value={c.work_days ?? []} options={["周日", "周一", "周二", "周三", "周四", "周五", "周六"].map((label, value) => ({ label, value }))} onChange={work_days => change({ work_days })} /></div>
          {field(`休息日期 ${i + 1}（逗号分隔）`, (c.holidays ?? []).join(","), value => change({ holidays: value.split(",").map(v => v.trim()).filter(Boolean) }))}
          <RailopsButton onClick={() => set({ calendars: r.calendars.filter((_, j) => i !== j) })}>删除此日历</RailopsButton>
        </div>
      })}
      <RailopsButton onClick={() => set({ calendars: [...r.calendars, { key: "", timezone: r.timezone, work_days: [1, 2, 3, 4, 5], start: "09:00", end: "18:00", holidays: [] }] })}>添加工作日历</RailopsButton>
    </div></details>
    <details><summary className="font-medium">工单处理时限（SLA）</summary><div className="mt-3 grid gap-3">
      {(r.targets ?? []).map((t, i) => {
        const change = (patch: Partial<typeof t>) => set({ targets: r.targets.map((item, j) => i === j ? { ...item, ...patch } : item) })
        return <div key={i} className="grid gap-3 rounded border p-3 sm:grid-cols-2">
          <FormField label="适用服务项目"><Select className="w-full" value={t.project_key} options={[{ value: "*", label: "全部项目的默认规则" }, ...document.projects.map(p => ({ value: p.key, label: p.name }))]} onChange={project_key => change({ project_key })} /></FormField>
          <FormField label="服务档次"><Select className="w-full" value={t.profile} options={[{ value: "standard", label: "标准" }, { value: "enhanced", label: "增强" }, { value: "mission_critical", label: "关键服务" }]} onChange={profile => change({ profile })} /></FormField>
          <FormField label="工单优先级"><Select className="w-full" value={t.priority} options={["p0", "p1", "p2", "p3", "p4"].map(value => ({ value }))} onChange={priority => change({ priority })} /></FormField>
          <FormField label="工作日历"><Select className="w-full" value={t.calendar_key} options={r.calendars.map(c => ({ value: c.key }))} onChange={calendar_key => change({ calendar_key })} /></FormField>
          {number(`首次回复分钟 ${i + 1}`, t.response_minutes, response_minutes => change({ response_minutes }))}
          {number(`工程师接单分钟 ${i + 1}`, t.assignment_minutes, assignment_minutes => change({ assignment_minutes }))}
          {number(`解决问题分钟 ${i + 1}`, t.resolution_minutes, resolution_minutes => change({ resolution_minutes }))}
          <RailopsButton onClick={() => set({ targets: r.targets.filter((_, j) => i !== j) })}>删除此时限规则</RailopsButton>
        </div>
      })}
      <RailopsButton onClick={() => set({ targets: [...r.targets, { project_key: "*", profile: "standard", priority: "p2", calendar_key: r.calendars[0]?.key ?? "", response_minutes: 0, assignment_minutes: 0, resolution_minutes: 0 }] })}>添加时限规则</RailopsButton>
    </div></details>
    <details><summary className="font-medium">允许的工单来源</summary><div className="mt-3 flex flex-wrap gap-3">{(r.channels ?? []).map((c, i) => <Checkbox key={c.name} checked={c.enabled} onChange={e => set({ channels: r.channels.map((item, j) => i === j ? { ...item, enabled: e.target.checked } : item) })}>{({ manual: "页面录入", phone: "人工电话受理", email: "邮件入站", api: "接口", webhook: "事件推送", monitoring_alert: "监控告警", whatsapp: "WhatsApp", chatbot_handoff: "机器人转人工" } as Record<string, string>)[c.name] ?? c.name}</Checkbox>)}</div></details>
    <details><summary className="font-medium">邮件发送</summary><div className="mt-3 grid gap-3 sm:grid-cols-2">
      <Checkbox checked={r.mail.enabled} onChange={e => set({ mail: { ...r.mail, enabled: e.target.checked } })}>启用邮件发送</Checkbox>
      <Checkbox checked={r.mail.use_tls} onChange={e => set({ mail: { ...r.mail, use_tls: e.target.checked } })}>加密连接（TLS）</Checkbox>
      {field("SMTP 地址", r.mail.host, host => set({ mail: { ...r.mail, host } }))}{number("SMTP 端口", r.mail.port, port => set({ mail: { ...r.mail, port } }))}
      {field("发信账号", r.mail.username, username => set({ mail: { ...r.mail, username } }))}{field("邮件密钥引用", r.mail.password_ref, password_ref => set({ mail: { ...r.mail, password_ref } }))}
      {field("发信邮箱地址", r.mail.from_address, from_address => set({ mail: { ...r.mail, from_address } }))}{field("发信人名称", r.mail.from_name, from_name => set({ mail: { ...r.mail, from_name } }))}
      {field("回复邮箱", r.mail.reply_to, reply_to => set({ mail: { ...r.mail, reply_to } }))}
      <FormField label="发信失败重试"><Select className="w-full" value={r.mail.retry_policy} options={[{value:"retry_3_10m",label:"重试3次，间隔10分钟"},{value:"retry_1_5m",label:"重试1次，间隔5分钟"},{value:"no_retry",label:"不重试"}]} onChange={retry_policy=>set({mail:{...r.mail,retry_policy}})} /></FormField>
      <p className="text-sm sm:col-span-2">密钥引用填写工程师提供的 secret://名称，不填密码。旧邮箱密码不会导出，迁移时需要准备对应密钥文件。</p>
    </div></details>
    <details><summary className="font-medium">外部系统接入</summary><div className="mt-3 grid gap-3">
      {(r.integrations ?? []).map((c, i) => {
        const change = (patch: Partial<typeof c>) => set({ integrations: r.integrations.map((item, j) => i === j ? { ...item, ...patch } : item) })
        return <div key={i} className="grid gap-3 rounded border p-3 sm:grid-cols-2">
          {field(`接入标识 ${i + 1}`, c.provider, provider => change({ provider }))}{field(`接入地址 ${i + 1}`, c.base_url, base_url => change({ base_url }))}
          {field(`应用编号 ${i + 1}`, c.app_id, app_id => change({ app_id }))}{field(`接入密钥引用 ${i + 1}`, c.secret_ref, secret_ref => change({ secret_ref }))}
          {field(`应用 Key 引用 ${i + 1}`, c.key_ref, key_ref => change({ key_ref }))}
          {field(`接入附加设置 JSON ${i + 1}`, c.metadata_json, metadata_json => change({ metadata_json }))}
          <Checkbox checked={c.enabled} onChange={e => change({ enabled: e.target.checked })}>启用此接入</Checkbox><RailopsButton onClick={() => set({ integrations: r.integrations.filter((_, j) => i !== j) })}>删除此接入</RailopsButton>
        </div>
      })}
      <RailopsButton onClick={() => set({ integrations: [...r.integrations, { provider: "", enabled: false, base_url: "", app_id: "", secret_ref: "", key_ref: "", metadata_json: "{}" }] })}>添加接入设置</RailopsButton>
    </div></details>
    <details><summary className="font-medium">数据保存与自动关闭</summary><div className="mt-3 grid gap-3 sm:grid-cols-2">
      {field("数据区域", r.retention.data_region, data_region => set({ retention: { ...r.retention, data_region } }))}
      <Checkbox checked={r.retention.gdpr_region} onChange={e => set({retention:{...r.retention,gdpr_region:e.target.checked}})}>适用 GDPR 地区</Checkbox>
      <Checkbox checked={r.retention.ccpa_region} onChange={e => set({retention:{...r.retention,ccpa_region:e.target.checked}})}>适用 CCPA 地区</Checkbox>
      <p className="text-sm sm:col-span-2">保存策略作用于现有日志、通知和授权记录清理任务。地区是配置标记，不会自动迁移数据库所在地。</p>
      {number("保存天数", r.retention.days, days => set({ retention: { ...r.retention, days } }))}{number("归档天数（0 为不归档）", r.retention.archive_after_days, archive_after_days => set({ retention: { ...r.retention, archive_after_days } }))}
      <Checkbox checked={r.retention.auto_delete} onChange={e => set({ retention: { ...r.retention, auto_delete: e.target.checked } })}>按保存期限自动清理</Checkbox>
      <Checkbox checked={r.retention.legal_hold} onChange={e => set({ retention: { ...r.retention, legal_hold: e.target.checked } })}>暂停本公司自动清理（需要保留证据）</Checkbox>
      <Checkbox checked={r.auto_close.enabled} onChange={e => set({ auto_close: { ...r.auto_close, enabled: e.target.checked } })}>启用已解决工单自动关闭</Checkbox>
      {number("自动关闭等待天数", r.auto_close.days, days => set({ auto_close: { ...r.auto_close, days } }))}
    </div></details>
  </fieldset></ConfigProvider>
}
