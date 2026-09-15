import { translateCurrentMessage } from "@/i18n/messages"
import { isCaseStatus } from "@/lib/ticket-lifecycle"

export function caseLabel(key: string, values?: Record<string, string | number>) {
  return translateCurrentMessage(`ticketCase.${key}`, values)
}

export function caseStatusLabel(status: string) {
  return isCaseStatus(status) ? caseLabel(`status.${status}`) : status
}

export function localizeCaseTimelineContent(content: string) {
  const transition = content.match(/^工单主状态：([a-z_]+) → ([a-z_]+)；([\s\S]*)$/)
  if (!transition || !isCaseStatus(transition[1]) || !isCaseStatus(transition[2])) return content
  const label = caseLabel("transition", { from: caseStatusLabel(transition[1]), to: caseStatusLabel(transition[2]) })
  return transition[3].trim() ? `${label} · ${transition[3].trim()}` : label
}
