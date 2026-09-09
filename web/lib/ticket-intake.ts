export type TicketIntakeRule = {
  project_key: string
  channel: string
  ticket_type: string
  required_fields: string[]
}

export type TicketIntakePolicy = { rules: TicketIntakeRule[] }

export type IntakeDraft = {
  source_record_id: string
  project_key: string
  ticket_type: string
  caller_name: string
  caller_phone: string
  received_at: string
  product_id: number
  device_id: number
  service_region: string
}

export const EMPTY_INTAKE: IntakeDraft = {
  source_record_id: "", project_key: "", ticket_type: "", caller_name: "", caller_phone: "",
  received_at: "", product_id: 0, device_id: 0, service_region: "",
}

export function phoneIntakePayload(channel: string, draft: IntakeDraft) {
  if (channel !== "phone") return {}
  const receivedAt = draft.received_at ? new Date(draft.received_at) : null
  if (receivedAt && !Number.isFinite(receivedAt.getTime())) throw new Error("Invalid received_at")
  return {
    source_record_id: draft.source_record_id.trim(), project_key: draft.project_key.trim(),
    ticket_type: draft.ticket_type.trim(), caller_name: draft.caller_name.trim(), caller_phone: draft.caller_phone.trim(),
    ...(receivedAt ? { received_at: receivedAt.toISOString() } : {}),
    product_id: draft.product_id || undefined, device_id: draft.device_id || undefined,
    region_code: draft.service_region.trim(),
  }
}

export function missingPhoneContext(draft: IntakeDraft, policy: TicketIntakePolicy, customerId: number): string[] {
  const missing: string[] = []
  if (!draft.project_key) missing.push("project_key")
  if (!draft.ticket_type) missing.push("ticket_type")
  const rule = policy.rules.find((rule) => rule.project_key === draft.project_key && rule.channel === "phone" && rule.ticket_type === draft.ticket_type)
  if (!rule) return [...missing, "projectPolicy"]
  const values: Record<string, unknown> = { ...draft, customer_id: customerId }
  for (const field of rule.required_fields) {
    const value = values[field]
    if (typeof value === "string" ? !value.trim() : !value) missing.push(field)
  }
  return missing
}
