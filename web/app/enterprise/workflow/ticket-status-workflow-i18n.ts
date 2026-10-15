import type { AppLocale } from "@/i18n/config"
import { matchMessage, translateMessage } from "@/i18n/messages"

export type TicketWorkflowDetail = {
  role: string
  action: string
  group: string
  hint: string
}

export type TicketWorkflowCopy = {
  engineTitle: string
  ticketWorkspace: string
  aiWorkspace: string
  workflowTypeAria: string
  loadFailed: string
  incomplete: string
  saveFailed: string
  draftSaved: (id: number) => string
  publishFailed: string
  published: string
  historyFailed: string
  reload: string
  title: string
  instruction: string
  scope: string
  defaultScope: string
  version: (id: number) => string
  systemDefault: string
  versionNote: string
  assignmentNote: string
  deviceNote: string
  editReason: string
  editReasonPlaceholder: string
  edit: string
  saveDraft: string
  publish: string
  returnCurrent: string
  deploymentNote: string
  history: string
  historyNote: string
  current: string
  publishedHistory: string
  draft: string
  loadVersion: string
  loadedVersion: (id: number) => string
  loadedNotice: string
  loadEarlier: string
  viewOnly: string
  graphTitle: string
  relatedTransitions: string
  statusCount: (count: number) => string
  backOverview: string
  autoLayout: string
  collapseCanvas: string
  expandCanvas: string
  graphAria: string
  graphHintSelected: string
  graphHintOverview: string
  transitionSettingsAria: string
  viewStatus: string
  overview: string
  removeTransition: string
  requiredTransition: string
  saving: string
  adjustHint: string
  nextStep: string
  editTransitionHint: string
  currentTransitionHint: string
  noNextStep: string
  setupTitle: string
  setupDescription: string
  setupStep1: string
  setupStep2: string
  setupStep3: string
  prepared: string
  footer: string
  required: string
  details: Record<string, TicketWorkflowDetail>
}

const PREFIX = "ticketStatusWorkflow"
const STATUSES = ["new", "acknowledged", "in_triage", "assigned", "waiting", "restored", "resolved", "closure_pending", "closed", "cancelled"]
const TEXT_KEYS = [
  "engineTitle", "ticketWorkspace", "aiWorkspace", "workflowTypeAria", "loadFailed", "incomplete", "saveFailed",
  "publishFailed", "published", "historyFailed", "reload", "title", "instruction", "scope", "defaultScope",
  "systemDefault", "versionNote", "assignmentNote", "deviceNote", "editReason", "editReasonPlaceholder", "edit",
  "saveDraft", "publish", "returnCurrent", "deploymentNote", "history", "historyNote", "current", "publishedHistory",
  "draft", "loadVersion", "loadedNotice", "loadEarlier", "viewOnly", "graphTitle", "relatedTransitions",
  "backOverview", "autoLayout", "collapseCanvas", "expandCanvas", "graphAria", "graphHintSelected",
  "graphHintOverview", "transitionSettingsAria", "viewStatus", "overview", "removeTransition", "requiredTransition",
  "saving", "adjustHint", "nextStep", "editTransitionHint", "currentTransitionHint", "noNextStep", "setupTitle",
  "setupDescription", "setupStep1", "setupStep2", "setupStep3", "prepared", "footer", "required",
] as const

export function ticketWorkflowCopy(locale: AppLocale): TicketWorkflowCopy {
  const text = (key: string, values?: Record<string, string | number>) => translateMessage(locale, `${PREFIX}.${key}`, values)
  const copy = Object.fromEntries(TEXT_KEYS.map((key) => [key, text(key)])) as Omit<
    TicketWorkflowCopy,
    "draftSaved" | "version" | "loadedVersion" | "statusCount" | "details"
  >
  return {
    ...copy,
    draftSaved: (id) => text("draftSaved", { id }),
    version: (id) => text("version", { id }),
    loadedVersion: (id) => text("loadedVersion", { id }),
    statusCount: (count) => text("statusCount", { count }),
    details: Object.fromEntries(STATUSES.map((status) => [status, {
      role: text(`details.${status}.role`),
      action: text(`details.${status}.action`),
      group: text(`details.${status}.group`),
      hint: text(`details.${status}.hint`),
    }])),
  }
}

export function ticketWorkflowDestinationLabel(locale: AppLocale, count: number) {
  return count
    ? translateMessage(locale, `${PREFIX}.destinationCount`, { count })
    : translateMessage(locale, `${PREFIX}.endOfWorkflow`)
}

export function ticketWorkflowTransitionSettingsLabel(locale: AppLocale, status: string) {
  return translateMessage(locale, `${PREFIX}.transitionSettingsHint`, { status })
}

export function displayTicketWorkflowProjectName(name: string, locale: AppLocale) {
  const value = name.trim()
  if (matchMessage(`${PREFIX}.defaultScope`, value)) return translateMessage(locale, `${PREFIX}.defaultScope`)
  return value
}

// Version notes are stored as rendered text. Recognize the generated notes in any
// locale and render them again in the current one, keeping custom reasons as-is.
export function displayTicketWorkflowVersionNote(note: string, locale: AppLocale): string {
  const value = note.trim()
  const key = (name: string) => `${PREFIX}.versionNotes.${name}`

  const prefixed = matchMessage(key("adjustedFromPrefix"), value)
  if (prefixed) {
    return translateMessage(locale, key("adjustedFromPrefix"), {
      version: prefixed.version,
      rest: displayTicketWorkflowVersionNote(prefixed.rest, locale),
    })
  }
  const adjusted = matchMessage(key("adjustedOperationalConfig"), value)
  if (adjusted) return translateMessage(locale, key("adjustedOperationalConfig"), adjusted)
  const restored = matchMessage(`${PREFIX}.loadedVersion`, value)
  if (restored) return translateMessage(locale, `${PREFIX}.loadedVersion`, restored)
  for (const name of ["restoreDefault", "adoptExistingSettings", "reach"]) {
    if (matchMessage(key(name), value)) return translateMessage(locale, key(name))
  }
  const generated = matchMessage(key("ticketWorkflow"), value)
  if (generated) {
    return translateMessage(locale, key("ticketWorkflow"), {
      reason: displayTicketWorkflowVersionNote(generated.reason, locale),
      scope: generated.scope,
    })
  }
  return value
}
