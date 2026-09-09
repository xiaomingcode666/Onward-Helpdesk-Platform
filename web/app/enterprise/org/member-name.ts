import { translateCurrentMessage } from "@/i18n/messages"

type AgentMemberIdentity = {
  userId?: number
  nickname?: string
  displayName?: string
  username?: string
  agentCode?: string
}

export function agentMemberName(
  item: AgentMemberIdentity | null | undefined,
  fallback = "",
) {
  if (!item) return fallback
  return item.displayName?.trim()
    || item.nickname?.trim()
    || item.username?.trim()
    || item.agentCode?.trim()
    || (item.userId ? translateCurrentMessage("orgExtract.orgMembers.memberNameFallback", { id: item.userId }) : fallback)
}
