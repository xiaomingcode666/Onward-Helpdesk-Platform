import { mergeImMessagesByIdAsc, type MergeableImMessage } from "@/lib/im-message-merge"

export type AIMessageStreamPayload = {
  conversationId: number
  sourceMessageId: number
  streamId: string
  clientMsgId: string
  sequence: number
  content: string
  sentAt: string
  startedAt: number
  streamExpiresAt: number
  done: boolean
}

export function readAIStreamMetadata(message: MergeableImMessage) {
  if (message.id >= 0 || !message.payload) return null
  try {
    const value = JSON.parse(message.payload)
    return value.kind === "ai_stream" ? value as {
      kind: string; streamId: string; sequence: number; startedAt: number; expiresAt: number
    } : null
  } catch { return null }
}

export function applyAIMessageStream<T extends MergeableImMessage>(
  messages: T[], update: Partial<AIMessageStreamPayload> | undefined, activeConversationId: number,
): T[] {
  if (!update || update.conversationId !== activeConversationId || activeConversationId <= 0 ||
    !Number.isSafeInteger(update.sourceMessageId) || (update.sourceMessageId ?? 0) <= 0 ||
    !update.streamId || !update.clientMsgId || !Number.isFinite(update.startedAt) ||
    !Number.isSafeInteger(update.sequence) || (update.sequence ?? 0) <= 0 ||
    !Number.isFinite(update.streamExpiresAt) || (update.streamExpiresAt ?? 0) <= Date.now() ||
    typeof update.content !== "string") return messages

  const existing = messages.find(message => message.clientMsgId === update.clientMsgId)
  const previous = existing && readAIStreamMetadata(existing)
  if (existing && !previous) return messages
  if (previous && (previous.streamId === update.streamId
    ? previous.sequence >= update.sequence!
    : previous.startedAt >= update.startedAt!)) return messages
  // A terminal event from an earlier attempt must not remove the current attempt.
  if (update.done && (!previous || previous.streamId !== update.streamId)) return messages
  const remaining = messages.filter(message => message.clientMsgId !== update.clientMsgId)
  if (update.done) return remaining
  const draft = {
    id: -update.sourceMessageId!, conversationId: activeConversationId,
    clientMsgId: update.clientMsgId, senderType: "ai", senderId: 0, senderName: "AI",
    messageType: "text", content: update.content, sentAt: update.sentAt,
    sendStatus: 1, customerRead: false, agentRead: false,
    payload: JSON.stringify({ kind: "ai_stream", streamId: update.streamId,
      sequence: update.sequence, startedAt: update.startedAt, expiresAt: update.streamExpiresAt }),
  } as T
  return mergeImMessagesByIdAsc(remaining, [draft])
}
