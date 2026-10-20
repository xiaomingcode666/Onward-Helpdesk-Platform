import { normalizeLocale } from "@/i18n/config"
import { matchMessage, translateMessage } from "@/i18n/messages"

const EVIDENCE_KEYS_BY_CRITERION: Record<string, string[]> = {
  identity_confirmation: ["identity_complete", "identity_partial", "identity_missing"],
  information_completeness: ["information_complete", "information_missing", "information_unverified"],
  issue_classification: ["classification_complete", "classification_partial", "classification_missing"],
  priority: ["priority_complete", "priority_review", "priority_missing"],
  communication: ["communication_recorded", "communication_partial", "communication_missing"],
  data_protection: ["data_protection_review"],
  handoff: ["handoff_complete", "handoff_partial", "handoff_missing"],
  follow_up: ["follow_up_complete", "follow_up_partial", "follow_up_missing"],
  closure: ["closure_complete", "closure_partial", "closure_missing"],
}

// Historical reviews store the rendered assessment, not its message keys.
// Recognize only complete system-generated lines; keep manual evidence intact.
export function localizeTicketQualityEvidence(value: string | null | undefined, locale: string) {
  const targetLocale = normalizeLocale(locale)
  return (value ?? "").split(/(\r?\n)/).map((line) => {
    const parts = line.match(/^([^:：\r\n]+)[:：]\s*(.+)$/)
    if (!parts) return line
    for (const [criterion, evidenceKeys] of Object.entries(EVIDENCE_KEYS_BY_CRITERION)) {
      const criterionKey = `ticketQuality.review.criteria.${criterion}`
      if (!matchMessage(criterionKey, parts[1])) continue
      const evidence = [...evidenceKeys, "manual_only"].find((key) =>
        matchMessage(`ticketQuality.review.evidence.${key}`, parts[2])
      )
      if (!evidence) return line
      return `${translateMessage(targetLocale, criterionKey)}: ${translateMessage(targetLocale, `ticketQuality.review.evidence.${evidence}`)}`
    }
    return line
  }).join("")
}
