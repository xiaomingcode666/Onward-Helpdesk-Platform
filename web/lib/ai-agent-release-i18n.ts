import { normalizeLocale } from "@/i18n/config"
import { matchMessage, translateMessage } from "@/i18n/messages"

const SYSTEM_COMMENT_KEYS = [
  "configurationSynced",
  "initialized",
  "collaborationSynced",
  "aiOnlySynced",
].map((key) => `aiAgentRelease.systemComments.${key}`)

// Older releases store rendered system comments. Translate only recognized
// complete messages so custom review comments and additions stay intact.
export function localizeAIAgentReleaseReviewComment(value: string | null | undefined, locale: string) {
  const comment = value ?? ""
  const key = SYSTEM_COMMENT_KEYS.find((key) => matchMessage(key, comment) !== null)
  return key ? translateMessage(normalizeLocale(locale), key) : comment
}
