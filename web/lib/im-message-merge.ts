export type MergeableImMessage = {
  id: number
  conversationId: number
  clientMsgId?: string
  senderType: string
  senderId?: number
  senderName?: string
  senderAvatar?: string
  messageType: string
  content: string
  payload?: string
  sendStatus: number
  sentAt?: string
  deliveredAt?: string
  readAt?: string
  customerRead: boolean
  customerReadAt?: string
  agentRead: boolean
  agentReadAt?: string
  recalledAt?: string
  quotedMessageId?: number
}

export function mergeImMessagesByIdAsc<T extends MergeableImMessage>(
  a: T[],
  b: T[]
): T[] {
  const byId = new Map<number, T>()
  for (const message of a) {
    byId.set(message.id, message)
  }
  for (const message of b) {
    const existing = byId.get(message.id)
    byId.set(message.id, existing ? mergeImMessage(existing, message) : message)
  }
  const persistedClientIds = new Set(Array.from(byId.values())
    .filter(message => message.id > 0).map(message => message.clientMsgId).filter(Boolean))
  return Array.from(byId.values())
    .filter(message => message.id > 0 || !message.clientMsgId || !persistedClientIds.has(message.clientMsgId))
    .sort((x, y) => (x.id > 0 && y.id > 0) ? x.id - y.id : x.id > 0 ? -1 : y.id > 0 ? 1 : y.id - x.id)
}

export function upsertImMessageReplacingClientMessage<T extends MergeableImMessage>(
  messages: T[],
  incoming: T,
): T[] {
  const clientMsgId = incoming.clientMsgId?.trim()
  const withoutOptimistic = clientMsgId
    ? messages.filter((message) => message.clientMsgId?.trim() !== clientMsgId)
    : messages
  return mergeImMessagesByIdAsc(withoutOptimistic, [incoming])
}

export function mergeImMessagesReplacingClientMessages<T extends MergeableImMessage>(
  current: T[],
  incoming: T[],
): T[] {
  const incomingClientIds = new Set(
    incoming.map((message) => message.clientMsgId?.trim()).filter(Boolean),
  )
  const withoutReplacedMessages = current.filter((message) => {
    const clientMsgId = message.clientMsgId?.trim()
    return !clientMsgId || !incomingClientIds.has(clientMsgId)
  })
  return mergeImMessagesByIdAsc(withoutReplacedMessages, incoming)
}

export function replaceLoadedImMessagesPreservingLocalTransfers<
  T extends MergeableImMessage,
>(current: T[], incoming: T[]): T[] {
  const incomingClientIds = new Set(
    incoming.map((message) => message.clientMsgId?.trim()).filter(Boolean),
  )
  const localTransfers = current.filter((message) => {
    const clientMsgId = message.clientMsgId?.trim()
    return (isLocalMediaTransfer(message.payload) || isLiveAIStream(message)) &&
      (!clientMsgId || !incomingClientIds.has(clientMsgId))
  })
  return mergeImMessagesByIdAsc(incoming, localTransfers)
}

function isLocalMediaTransfer(payload?: string) {
  if (!payload?.trim()) return false
  try {
    return Boolean((JSON.parse(payload) as { localTransfer?: boolean }).localTransfer)
  } catch {
    return false
  }
}

function isLiveAIStream(message: MergeableImMessage) {
  if (message.id >= 0 || !message.payload) return false
  try {
    const payload = JSON.parse(message.payload)
    return payload.kind === "ai_stream" && payload.expiresAt > Date.now()
  } catch { return false }
}

export function parseImMessageCursorId(cursor: string): number {
  const value = Number.parseInt(cursor, 10)
  return Number.isFinite(value) && value > 0 ? value : 0
}

export function cursorFromLoadedImMessages<T extends Pick<MergeableImMessage, "id">>(
  messages: T[]
): string {
  const id = minImMessageId(messages)
  return id === null ? "" : String(id)
}

export function hasMoreAfterLatestImMessageMerge<
  T extends Pick<MergeableImMessage, "id">,
>(args: {
  previousMessages: T[]
  previousHasMore: boolean
  merged: T[]
  apiHasMore: boolean
}): boolean {
  const prevMin = minImMessageId(args.previousMessages)
  const mergedMin = minImMessageId(args.merged)

  if (mergedMin === null) {
    return Boolean(args.apiHasMore)
  }

  if (!args.previousHasMore && prevMin !== null && mergedMin >= prevMin) {
    return false
  }

  return args.previousHasMore || Boolean(args.apiHasMore)
}

function minImMessageId<T extends Pick<MergeableImMessage, "id">>(
  messages: T[]
): number | null {
  const ids = messages.map(message => message.id).filter(id => id > 0)
  return ids.length ? Math.min(...ids) : null
}

export function mergeImMessage<T extends MergeableImMessage>(
  existing: T,
  incoming: T
): T {
  return isSameImMessage(existing, incoming) ? existing : incoming
}

function isSameImMessage(a: MergeableImMessage, b: MergeableImMessage): boolean {
  const aKeys = Object.keys(a)
  const bKeys = Object.keys(b)
  if (aKeys.length !== bKeys.length) {
    return false
  }
  return aKeys.every((key) => {
    const field = key as keyof MergeableImMessage
    return Object.prototype.hasOwnProperty.call(b, key) && a[field] === b[field]
  })
}
