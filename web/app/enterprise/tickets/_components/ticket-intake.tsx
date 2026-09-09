"use client"

import { useEffect, useState } from "react"
import { Checkbox, Input, Select } from "antd"
import { PlusIcon, SaveIcon, Settings2Icon, Trash2Icon } from "lucide-react"
import { FormField, IconButton, RailopsButton, StandardModal } from "@railops/ui"
import { translateCurrentMessage } from "@/i18n/messages"
import { useAuth } from "@/components/auth-provider"
import { completeTicketIntake, fetchTicketIntakePolicy, fetchTicketCustomerOptions, updateTicketIntakePolicy, type TicketCustomerOption } from "@/lib/api/enterprise-tickets"
import { listAllProducts, getProductDevices, type ProductDevice } from "@/lib/api/enterprise-products"
import type { TicketAggregateDTO } from "@/lib/api/types"
import { EMPTY_INTAKE, missingPhoneContext, type IntakeDraft, type TicketIntakePolicy } from "@/lib/ticket-intake"

export const intakeLabel = (key: string) => translateCurrentMessage(`ticketIntake.${key}`)

export function IntakeFields({ value, onChange, policy, customerId, original, showDeviceContext = true }: {
  value: IntakeDraft; onChange: (value: IntakeDraft) => void; policy: TicketIntakePolicy; customerId: number
  original?: IntakeDraft; showDeviceContext?: boolean
}) {
  const [products, setProducts] = useState<Array<{ id: number; name: string }>>([])
  const [deviceResult, setDeviceResult] = useState<{ productId: number; items: ProductDevice[]; error?: string }>({ productId: 0, items: [] })
  const [error, setError] = useState("")
  const loading = Boolean(value.product_id && showDeviceContext && deviceResult.productId !== value.product_id)
  const devices = deviceResult.productId === value.product_id ? deviceResult.items : []
  useEffect(() => {
    if (!showDeviceContext) return
    let active = true
    listAllProducts().then((items) => { if (active) setProducts(items) }).catch(() => { if (active) setError(intakeLabel("loadError")) })
    return () => { active = false }
  }, [showDeviceContext])
  useEffect(() => {
    if (!value.product_id || !showDeviceContext) return
    let active = true
    async function load() {
      const items: ProductDevice[] = []
      for (let page = 1; page <= 100; page++) {
        const result = await getProductDevices(value.product_id, { page, page_size: 100 })
        if (!active) return
        if (!result.success || !result.data) throw new Error(intakeLabel("loadError"))
        items.push(...result.data.items)
        if (result.data.items.length < 100) break
      }
      if (active) setDeviceResult({ productId: value.product_id, items })
    }
    load().catch(() => { if (active) setDeviceResult({ productId: value.product_id, items: [], error: intakeLabel("loadError") }) })
    return () => { active = false }
  }, [value.product_id, showDeviceContext])
  const set = (key: keyof IntakeDraft, next: string | number) => onChange({ ...value, [key]: next })
  const rules = policy.rules.filter((rule) => rule.channel === "phone")
  const missing = missingPhoneContext(value, policy, customerId)
  const required = rules.find((rule) => rule.project_key === value.project_key && rule.ticket_type === value.ticket_type)?.required_fields ?? []
  return <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 [&_.ant-form-item]:mb-0">
    <FormField label={intakeLabel("project_key")}>
      <Select className="w-full" showSearch allowClear value={value.project_key || undefined} options={[...new Set(rules.map((rule) => rule.project_key))].map((key) => ({ value: key, label: key }))} onChange={(key) => onChange({ ...value, project_key: key ?? "", ticket_type: "" })} />
    </FormField>
    <FormField label={intakeLabel("ticket_type")}>
      <Select className="w-full" showSearch allowClear value={value.ticket_type || undefined} options={rules.filter((rule) => rule.project_key === value.project_key).map((rule) => ({ value: rule.ticket_type, label: rule.ticket_type }))} onChange={(key) => set("ticket_type", key ?? "")} />
    </FormField>
    {(["caller_name", "caller_phone", "service_region"] as const).map((key) => <FormField key={key} label={intakeLabel(key)} required={required.includes(key)}>
      <Input value={value[key]} maxLength={key === "caller_name" ? 120 : 64} type={key === "caller_phone" ? "tel" : "text"} disabled={Boolean(original && key !== "service_region" && original[key])} onChange={(event) => set(key, event.target.value)} />
    </FormField>)}
    {!original && <>
      <FormField label={intakeLabel("source_record_id")}><Input value={value.source_record_id} maxLength={160} placeholder={intakeLabel("generated")} onChange={(event) => set("source_record_id", event.target.value)} /></FormField>
      <FormField label={intakeLabel("received_at")}><Input type="datetime-local" value={value.received_at} onChange={(event) => set("received_at", event.target.value)} /></FormField>
    </>}
    {showDeviceContext && <>
      <FormField label={intakeLabel("product_id")} required={required.includes("product_id")}><Select className="w-full" showSearch optionFilterProp="label" allowClear value={value.product_id || undefined} options={products.map((product) => ({ value: product.id, label: product.name }))} onChange={(id) => onChange({ ...value, product_id: id ?? 0, device_id: 0 })} /></FormField>
      <FormField label={intakeLabel("device_id")} required={required.includes("device_id")}><Select className="w-full" showSearch optionFilterProp="label" allowClear loading={loading} disabled={!value.product_id} value={value.device_id || undefined} options={devices.map((device) => ({ value: device.id, label: device.device_no || device.serial_no }))} onChange={(id) => set("device_id", id ?? 0)} /></FormField>
    </>}
    {(error || deviceResult.error) && <div className="text-sm text-destructive sm:col-span-2">{error || deviceResult.error}</div>}
    {missing.length > 0 && <div role="status" className="border-l-2 border-amber-500 bg-amber-50 p-3 text-sm text-amber-950 sm:col-span-2">
      <div>{intakeLabel("incomplete")}</div><div className="mt-1 break-words">{intakeLabel("missing")}: {missing.map(intakeLabel).join(", ")}</div>
    </div>}
  </div>
}

export function IntakePolicyButton({ onSaved }: { onSaved?: (policy: TicketIntakePolicy) => void }) {
  const [open, setOpen] = useState(false)
  const [policy, setPolicy] = useState<TicketIntakePolicy>({ rules: [] })
  const [loaded, setLoaded] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState("")
  async function load() {
    setOpen(true); setLoaded(false); setError("")
    try {
      const result = await fetchTicketIntakePolicy()
      if (!result.success || !result.data) throw new Error(result.error?.message || intakeLabel("loadError"))
      setPolicy({ rules: result.data.rules ?? [] }); setLoaded(true)
    } catch (error) { setError(error instanceof Error ? error.message : intakeLabel("loadError")) }
  }
  async function save() {
    setSaving(true); setError("")
    try {
      const result = await updateTicketIntakePolicy(policy)
      if (!result.success) throw new Error(result.error?.message || intakeLabel("saveError"))
      onSaved?.(policy)
      setOpen(false)
    } catch (error) { setError(error instanceof Error ? error.message : intakeLabel("saveError")) }
    finally { setSaving(false) }
  }
  return <>
    <IconButton icon={<Settings2Icon />} tooltip={intakeLabel("policy")} aria-label={intakeLabel("policy")} onClick={() => void load()} />
    <StandardModal open={open} width={720} styles={{ body: { maxHeight: "calc(100dvh - 180px)", overflowY: "auto" } }} title={intakeLabel("policy")} onCancel={() => !saving && setOpen(false)} footer={<RailopsButton disabled={!loaded || saving} onClick={() => void save()}><SaveIcon size={16} />{intakeLabel("save")}</RailopsButton>}>
      <div className="grid gap-4">
        {policy.rules.map((rule, index) => <div key={index} className="grid grid-cols-1 gap-3 border-b pb-4 sm:grid-cols-3">
          {(["project_key", "channel", "ticket_type"] as const).map((key) => <FormField key={key} label={intakeLabel(key)} required>
            {key === "channel" ? <Select className="w-full" value={rule.channel} options={["phone", "email", "monitoring_alert", "api", "webhook", "whatsapp", "chatbot_handoff"].map((value) => ({ value, label: value === "phone" ? intakeLabel("phone") : value }))} onChange={(value) => setPolicy({ rules: policy.rules.map((item, i) => i === index ? { ...item, channel: value } : item) })} /> : <Input maxLength={64} value={rule[key]} onChange={(event) => setPolicy({ rules: policy.rules.map((item, i) => i === index ? { ...item, [key]: event.target.value } : item) })} />}
          </FormField>)}
          <div className="sm:col-span-3"><FormField label={intakeLabel("requiredFields")}><Checkbox.Group value={rule.required_fields} options={["caller_name", "caller_phone", "customer_id", "product_id", "device_id", "service_region"].map((value) => ({ value, label: intakeLabel(value) }))} onChange={(fields) => setPolicy({ rules: policy.rules.map((item, i) => i === index ? { ...item, required_fields: fields as string[] } : item) })} /></FormField></div>
          <IconButton icon={<Trash2Icon />} tooltip={intakeLabel("removeRule")} aria-label={intakeLabel("removeRule")} onClick={() => setPolicy({ rules: policy.rules.filter((_, i) => i !== index) })} />
        </div>)}
        <RailopsButton disabled={!loaded || saving} onClick={() => setPolicy({ rules: [...policy.rules, { project_key: "", channel: "phone", ticket_type: "", required_fields: [] }] })}><PlusIcon size={16} />{intakeLabel("addRule")}</RailopsButton>
        {error && <div role="alert" className="text-sm text-destructive">{error}{!loaded && <RailopsButton onClick={() => void load()}>{intakeLabel("retry")}</RailopsButton>}</div>}
      </div>
    </StandardModal>
  </>
}

export function IntakeCompletion({ aggregate, onSaved, showDeviceContext }: { aggregate: TicketAggregateDTO; onSaved: (value: TicketAggregateDTO) => void; showDeviceContext: boolean }) {
  const { session } = useAuth()
  const [open, setOpen] = useState(false)
  const [draft, setDraft] = useState(EMPTY_INTAKE)
  const [original, setOriginal] = useState(EMPTY_INTAKE)
  const [customerId, setCustomerId] = useState(0)
  const [customers, setCustomers] = useState<TicketCustomerOption[]>([])
  const [policy, setPolicy] = useState<TicketIntakePolicy>({ rules: [] })
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState("")
  if (aggregate.ticket.channel !== "phone" || !aggregate.ticket.source_record_id || !session?.permissions?.includes("ticket.update")) return null
  async function start() {
    const ticket = aggregate.ticket
    const value = { ...EMPTY_INTAKE, project_key: ticket.project_key ?? "", ticket_type: ticket.ticket_type ?? "", caller_name: ticket.caller_name ?? "", caller_phone: ticket.caller_phone ?? "", product_id: ticket.product_id, device_id: ticket.device_id ?? 0, service_region: ticket.service_region ?? "" }
    setDraft(value); setOriginal(value); setCustomerId(aggregate.customer.id); setCustomers([]); setError(""); setOpen(true)
    try {
      const result = await fetchTicketIntakePolicy()
      if (!result.success || !result.data) throw new Error(result.error?.message || intakeLabel("loadError"))
      setPolicy({ rules: result.data.rules ?? [] })
      if (!aggregate.customer.id) {
        const customerResult = await fetchTicketCustomerOptions()
        if (!customerResult.success) throw new Error(customerResult.error?.message || intakeLabel("loadError"))
        setCustomers(customerResult.data ?? [])
      }
    } catch (error) { setError(error instanceof Error ? error.message : intakeLabel("loadError")) }
  }
  async function save() {
    setSaving(true); setError("")
    try {
      const result = await completeTicketIntake(aggregate.ticket.id, draft, customerId)
      if (!result.success || !result.data) throw new Error(result.error?.message || intakeLabel("saveError"))
      onSaved(result.data); setOpen(false)
    } catch (error) { setError(error instanceof Error ? error.message : intakeLabel("saveError")) }
    finally { setSaving(false) }
  }
  return <>
    <RailopsButton onClick={() => void start()}>{intakeLabel("supplement")}</RailopsButton>
    <StandardModal open={open} width={640} styles={{ body: { maxHeight: "calc(100dvh - 180px)", overflowY: "auto" } }} title={intakeLabel("supplement")} onCancel={() => !saving && setOpen(false)} footer={<RailopsButton disabled={saving} onClick={() => void save()}><SaveIcon size={16} />{intakeLabel("save")}</RailopsButton>}>
      <IntakeFields value={draft} onChange={setDraft} policy={policy} customerId={customerId} original={original} showDeviceContext={showDeviceContext} />
      {!aggregate.customer.id && <FormField label={intakeLabel("customer_id")}><Select className="w-full" showSearch optionFilterProp="label" allowClear value={customerId || undefined} options={customers.map((customer) => ({ value: customer.customer_id, label: [customer.display_name, customer.customer_org_name].filter(Boolean).join(" / ") }))} onChange={(id) => setCustomerId(id ?? 0)} /></FormField>}
      {error && <div role="alert" className="mt-3 text-sm text-destructive">{error}</div>}
    </StandardModal>
  </>
}
