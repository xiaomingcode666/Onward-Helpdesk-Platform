import { normalizeLocale, SUPPORTED_LOCALES } from "@/i18n/config"
import { matchMessage, translateMessage } from "@/i18n/messages"

type TranslateFn = (key: string, values?: Record<string, string | number>) => string

export type ConversationSystemText = {
  key: string
  params?: Record<string, string | number>
}

const systemKeys = new Set([
  "ticketCreated", "ticketCreatedWithDevice", "ticketCreatedSummary",
  "supplierInvited", "supplierInvitedWithModule", "supplierJoined", "supplierJoinedSummary",
  "supplierJoinedGeneric", "supplierJoinedSummaryGeneric",
].map((name) => `conversationSystem.${name}`))

function validSystemText(value: unknown): value is ConversationSystemText {
  if (!value || typeof value !== "object") return false
  const text = value as ConversationSystemText
  return systemKeys.has(text.key) && (text.params === undefined || (
    text.params !== null && typeof text.params === "object" && !Array.isArray(text.params) &&
    Object.values(text.params).every((item) => typeof item === "string" || typeof item === "number")
  ))
}

export function getConversationSystemMessageText(message: {
  senderType?: string
  messageType: string
  payload?: string
  recalledAt?: string
  sendStatus?: number
}, kind: "content" | "summary"): ConversationSystemText | undefined {
  if (message.senderType !== "system" || message.messageType !== "text" || message.recalledAt || message.sendStatus === 6) return undefined
  try {
    const text: unknown = JSON.parse(message.payload || "{}").systemText?.[kind]
    return validSystemText(text) ? text : undefined
  } catch {
    return undefined
  }
}

const summaryKey = (name: string) => `portalExtract.customerMobile.conversation.summaryEvents.${name}`
const eventKey = (name: string) => `customerEntryExtract.event.${name}`

// Historical summaries store rendered text. Recognize complete templates from
// every locale, keeping business values intact and longer templates first.
const eventSummaries: [string, string][] = [
  ["ticketCreatedDescriptionWithTicketNoGeneral", "ticketCreatedWithTicketNo"],
  ["ticketCreatedDescriptionWithTicketNo", "ticketCreatedWithTicketNo"],
  ["ticketCreatedDescriptionGeneral", "ticketCreated"],
  ["ticketCreatedDescription", "ticketCreated"],
  ["supplierJoinedDescriptionWithCompany", "supplierJoinedWithCompany"],
  ["repairConclusionDescriptionWaitingGeneral", "repairSubmittedWaiting"],
  ["videoScheduledDescriptionWithTime", "engineerVideoScheduled"],
  ["videoScheduledDescription", "engineerVideoScheduled"],
]

const summaryNames = [
  "supplierInvitedWithModule", "supplierInvitedWithCompany", "supplierJoinedWithCompany",
  "supplierResolvedWithResolution", "videoEndedWithRoomDuration", "videoEndedWithDuration",
  "videoEnded", "videoStartedWithRoom", "videoStarted", "engineerVideoStarted",
  "engineerVideoScheduled", "feedbackSubmitted", "repairSubmittedWaiting",
  "repairSubmittedWithSummary", "ticketCreatedWithTicketNo", "ticketCreated",
]

const durationNames = ["minutesSeconds", "minutes", "seconds"]
const durationEvents = ["durationMinutesSeconds", "durationMinutes", "durationSeconds"]

function matchDuration(key: string, duration: string) {
  for (const locale of SUPPORTED_LOCALES) {
    const names: string[] = []
    const pattern = translateMessage(locale, key).replace(/\s+/g, "").split(/(\{\w+\})/).map((part) => {
      const parameter = part.match(/^\{(\w+)\}$/)
      if (parameter) {
        names.push(parameter[1])
        return "(\\d+)"
      }
      return part.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")
    }).join("")
    const match = duration.replace(/\s+/g, "").match(new RegExp(`^${pattern}$`))
    if (match && names.length) return Object.fromEntries(names.map((name, index) => [name, match[index + 1]]))
  }
  return null
}

function localizeDuration(duration: string, t: TranslateFn) {
  for (let i = 0; i < durationNames.length; i++) {
    const key = `portalExtract.customerMobile.time.${durationNames[i]}`
    const values = matchDuration(eventKey(durationEvents[i]), duration) ?? matchDuration(key, duration)
    if (values && Object.values(values).every((value) => /^\d+$/.test(value))) {
      return t(key, values)
    }
  }
  return duration
}

export function renderCustomerConversationSummary(
  summary: string | null | undefined,
  t: TranslateFn,
  fallback = "",
  systemText?: ConversationSystemText | null,
) {
  if (validSystemText(systemText)) return t(systemText.key, systemText.params)
  const text = summary ?? ""
  if (!text.trim()) return fallback

  const templates: [string, string][] = [
    ...eventSummaries.map(([source, target]): [string, string] => [eventKey(source), summaryKey(target)]),
    [summaryKey("repairSubmittedWaitingService"), summaryKey("repairSubmittedWaiting")],
    [eventKey("feedbackSubmittedDescriptionWithComment"), eventKey("feedbackSubmittedDescriptionWithComment")],
    ...summaryNames.map((name): [string, string] => [summaryKey(name), summaryKey(name)]),
  ]
  for (const [source, target] of templates) {
    const values = matchMessage(source, text)
    if (!values) continue
    // A free-form sentence after the ticket number is not a system template.
    if (values.ticketNo && !/^#?[A-Za-z0-9][A-Za-z0-9_-]*$/.test(values.ticketNo)) continue
    if (values.rating && !/^[1-5]$/.test(values.rating)) continue
    if (values.duration) values.duration = localizeDuration(values.duration, t)
    return t(target, values)
  }

  for (const name of ["image", "audio", "attachment"]) {
    const key = summaryKey(name)
    for (const locale of SUPPORTED_LOCALES) {
      const label = translateMessage(locale, key)
      const trimmed = text.trim()
      if (trimmed === label) return t(key)
      if (trimmed.startsWith(`${label} `)) return `${t(key)} ${trimmed.slice(label.length).trimStart()}`
    }
  }

  return text
}

export function localizeConversationSummary(summary: string | null | undefined, locale: string, fallback = "", systemText?: ConversationSystemText | null) {
  return renderCustomerConversationSummary(summary, (key, values) => translateMessage(normalizeLocale(locale), key, values), fallback, systemText)
}
