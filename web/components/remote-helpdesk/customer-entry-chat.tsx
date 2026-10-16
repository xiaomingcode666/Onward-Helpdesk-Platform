"use client"

import { useCallback, useEffect, useRef, useState } from "react"
import {
  AlertTriangleIcon,
	  CheckCircle2Icon,
	  Clock3Icon,
	  HeadphonesIcon,
	  ImageIcon,
	  InfoIcon,
	  Loader2Icon,
	  PaperclipIcon,
	  RotateCwIcon,
	  SendIcon,
	  ShieldCheckIcon,
	  WrenchIcon,
	} from "lucide-react"

import { Input } from "antd"
import { IconButton, RailopsButton, StatusTag } from "@railops/ui"

import { CustomerConversationTimeline } from "@/components/remote-helpdesk/customer-conversation-timeline"
import { VoiceRecorderButton } from "@/components/chat/voice-recorder-button"
import { ModuleLoading } from "@/components/shared/loading-states"
import { translateCurrentMessage } from "@/i18n/messages"
import { useI18n } from "@/i18n/provider"
import {
	  createCustomerEntryConversation,
	  exchangeCustomerEntrySession,
	  fetchCustomerEntryConversation,
	  fetchCustomerEntryMessages,
	  requestCustomerEntryHumanSupport,
	  sendCustomerEntryMessage,
	  uploadCustomerEntryAttachment,
	  uploadCustomerEntryAudio,
	  uploadCustomerEntryImage,
	  type CustomerEntryChat,
	} from "@/lib/api/customer-entry"
import type { ImConversation, ImMessage } from "@/lib/api/im"
import { createCustomerSessionWebSocket } from "@/lib/api/websocket"
import {
  expireRealtimePresenceActor,
  expireRealtimeTypingActor,
  mergeRealtimePresenceActor,
  presenceFromRealtimeTyping,
  realtimePresenceExpiryDelay,
  TYPING_IDLE_TIMEOUT_MS,
  type RealtimePresencePayload,
  type RealtimePresenceSnapshotPayload,
  type RealtimeTypingPayload,
  type RealtimeTypingState,
  updateRealtimeTypingState,
} from "@/lib/im-realtime-state"
import { CUSTOMER_GUEST_QUESTION_LIMIT } from "@/lib/customer-guest-demo"
import { createRealtimeConnectionManager } from "@/lib/realtime-connection"
import { cn, generateUUID } from "@/lib/utils"

type CustomerEntryChatPanelProps = {
  chat: CustomerEntryChat
  onContextChanged?: () => void
}

const customerEntryChatI18nPrefix = "customerEntryExtract.chat."
type CustomerEntryChatT = ReturnType<typeof useI18n>
const cec = (t: CustomerEntryChatT, key: string, values?: Record<string, string | number>) => t(`${customerEntryChatI18nPrefix}${key}`, values)

const QUICK_PROMPTS = [
  { labelKey: "quickPromptDeviceWontStart", icon: AlertTriangleIcon },
  { labelKey: "quickPromptFaultCode", icon: InfoIcon },
  { labelKey: "quickPromptAbnormalAlert", icon: WrenchIcon },
  { labelKey: "quickPromptNeedEngineer", icon: HeadphonesIcon },
]

const SELF_SERVICE_QUICK_PROMPTS = [
  { labelKey: "quickPromptDeviceWontStart", icon: AlertTriangleIcon },
  { labelKey: "quickPromptFaultCode", icon: InfoIcon },
  { labelKey: "quickPromptAbnormalAlert", icon: WrenchIcon },
  { labelKey: "quickPromptAddPhenomenon", icon: WrenchIcon },
]

const HANDOFF_REASON_KEYS = [
  "handoffReasonSuggestHuman",
  "handoffReasonStepsNotResolved",
  "handoffReasonSafetyRisk",
]

type SupportStage = "ai" | "queued" | "human" | "closed"

function supportStage(conversation: ImConversation | null): SupportStage {
  switch (conversation?.status) {
    case 2:
      return "queued"
    case 3:
      return "human"
    case 4:
      return "closed"
    default:
      return "ai"
  }
}

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error && error.message.trim() ? error.message : fallback
}

function mergeMessages(current: ImMessage[], incoming: ImMessage[]) {
  const byID = new Map<number, ImMessage>()
  for (const message of [...current, ...incoming]) {
    byID.set(message.id, message)
  }
  return Array.from(byID.values()).sort((left, right) => left.id - right.id)
}

export function CustomerEntryChatPanel({
  chat,
  onContextChanged,
}: CustomerEntryChatPanelProps) {
  const t = useI18n()
  const [token, setToken] = useState("")
  const [conversation, setConversation] = useState<ImConversation | null>(null)
  const [messages, setMessages] = useState<ImMessage[]>([])
  const [draft, setDraft] = useState("")
	  const [loading, setLoading] = useState(true)
	  const [sending, setSending] = useState(false)
	  const [uploadingAsset, setUploadingAsset] = useState(false)
	  const [requestingHuman, setRequestingHuman] = useState(false)
  const [humanRequestAccepted, setHumanRequestAccepted] = useState(false)
  const [handoffPanelOpen, setHandoffPanelOpen] = useState(false)
  const [handoffReason, setHandoffReason] = useState("")
  const [connected, setConnected] = useState(false)
  const [presence, setPresence] = useState<Record<string, RealtimePresencePayload>>({})
  const [remoteTyping, setRemoteTyping] = useState<RealtimeTypingState>({})
  const [error, setError] = useState("")
	  const [retryNonce, setRetryNonce] = useState(0)
	  const [guestQuestionCount, setGuestQuestionCount] = useState(0)
	  const imageInputRef = useRef<HTMLInputElement>(null)
	  const attachmentInputRef = useRef<HTMLInputElement>(null)
	  const messageEndRef = useRef<HTMLDivElement>(null)
  const socketRef = useRef<WebSocket | null>(null)
  const typingExpiryRef = useRef(new Map<string, ReturnType<typeof setTimeout>>())
  const onContextChangedRef = useRef(onContextChanged)

  useEffect(() => {
    onContextChangedRef.current = onContextChanged
  }, [onContextChanged])

  const refreshMessages = useCallback(async (sessionToken: string, conversationID: number) => {
    const page = await fetchCustomerEntryMessages(sessionToken, conversationID)
    setMessages((current) => mergeMessages(current, page.results ?? []))
  }, [])

  const refreshConversation = useCallback(async (sessionToken: string, conversationID: number) => {
    const nextConversation = await fetchCustomerEntryConversation(sessionToken, conversationID)
    setConversation(nextConversation)
    if (nextConversation.status === 2 || nextConversation.status === 3) {
      setHumanRequestAccepted(true)
    }
  }, [])

  const refreshChat = useCallback(async (sessionToken: string, conversationID: number) => {
    await Promise.all([
      refreshMessages(sessionToken, conversationID),
      refreshConversation(sessionToken, conversationID),
    ])
  }, [refreshConversation, refreshMessages])

  useEffect(() => {
    let active = true
    let realtime: ReturnType<typeof createRealtimeConnectionManager> | null = null
    let pollTimer: ReturnType<typeof setInterval> | null = null
    let refreshTimer: ReturnType<typeof setTimeout> | null = null
    let contextRefreshTimer: ReturnType<typeof setTimeout> | null = null
    let refreshInFlight = false
    let refreshRequested = false
    let realtimeConnected = false
    let reconciliationTick = 0
    const typingExpiry = typingExpiryRef.current
    const presenceExpiry = new Map<string, {
      expiresAt: string
      timer: ReturnType<typeof setTimeout>
    }>()

    const clearPresenceExpiry = (actorId: string) => {
      const lease = presenceExpiry.get(actorId)
      if (lease) clearTimeout(lease.timer)
      presenceExpiry.delete(actorId)
    }

    const schedulePresenceExpiry = (payload: RealtimePresencePayload) => {
      if (!payload.actorId) return
      if (!payload.online) {
        clearPresenceExpiry(payload.actorId)
        return
      }
      const delay = realtimePresenceExpiryDelay(payload.expiresAt)
      if (delay === null || !payload.expiresAt) return
      const currentLease = presenceExpiry.get(payload.actorId)
      if (
        currentLease &&
        Date.parse(currentLease.expiresAt) >= Date.parse(payload.expiresAt)
      ) {
        return
      }
      clearPresenceExpiry(payload.actorId)
      const expectedExpiresAt = payload.expiresAt
      presenceExpiry.set(payload.actorId, {
        expiresAt: expectedExpiresAt,
        timer: setTimeout(() => {
          setPresence((current) => expireRealtimePresenceActor(
            current,
            payload.actorId,
            expectedExpiresAt,
          ))
          presenceExpiry.delete(payload.actorId)
        }, delay),
      })
    }

    const bootstrap = async () => {
      setLoading(true)
      setError("")
      try {
        const session = await exchangeCustomerEntrySession({
          entrySessionId: chat.entrySessionId,
          visitorId: chat.visitorId,
          visitorToken: chat.visitorToken,
        })
        if (!active) return
        const nextConversation = await createCustomerEntryConversation(
          session.customerSessionToken,
          chat.entrySessionId
        )
        if (!active) return
        setToken(session.customerSessionToken)
        setConversation(nextConversation)
        await refreshChat(session.customerSessionToken, nextConversation.id)
        if (!active) return

        let realtimeToken = session.customerSessionToken
        const notifyContextChanged = () => {
          if (!active || contextRefreshTimer) return
          contextRefreshTimer = setTimeout(() => {
            contextRefreshTimer = null
            if (active) onContextChangedRef.current?.()
          }, 120)
        }
        const runScheduledRefresh = async () => {
          refreshTimer = null
          if (!active) return
          if (refreshInFlight) {
            refreshRequested = true
            return
          }
          refreshInFlight = true
          refreshRequested = false
          try {
            await refreshChat(realtimeToken, nextConversation.id)
            notifyContextChanged()
          } finally {
            refreshInFlight = false
            if (active && refreshRequested && !refreshTimer) {
              refreshTimer = setTimeout(
                () => void runScheduledRefresh().catch(() => undefined),
                120,
              )
            }
          }
        }
        const scheduleRefresh = (delay = 80) => {
          if (!active) return
          refreshRequested = true
          if (refreshTimer || refreshInFlight) return
          refreshTimer = setTimeout(
            () => void runScheduledRefresh().catch(() => undefined),
            delay,
          )
        }
        const handleRealtimeMessage = (event: MessageEvent) => {
          if (!active) return
          let envelope: { type?: string; data?: unknown }
          try {
            envelope = JSON.parse(event.data) as typeof envelope
          } catch {
            return
          }
          const payload = envelope.data as RealtimePresencePayload & RealtimeTypingPayload
          if (envelope.type === "customer_session.refresh") {
            const refreshedToken = (payload as unknown as { customerSessionToken?: string })?.customerSessionToken?.trim()
            if (refreshedToken) {
              realtimeToken = refreshedToken
              setToken(refreshedToken)
            }
            return
          }
          if (envelope.type === "presence.changed" && payload?.actorId && payload.conversationId === nextConversation.id) {
            if (payload.participantType !== "customer") {
              setPresence((current) => mergeRealtimePresenceActor(current, payload))
              schedulePresenceExpiry(payload)
            }
            return
          }
          if (envelope.type === "presence.snapshot") {
            const snapshot = envelope.data as RealtimePresenceSnapshotPayload | undefined
            if (snapshot?.conversationId === nextConversation.id) {
              presenceExpiry.forEach((lease) => clearTimeout(lease.timer))
              presenceExpiry.clear()
              const participants = (snapshot.participants ?? [])
                .filter((item) => item.participantType !== "customer" && item.actorId && item.online)
              setPresence(Object.fromEntries(participants.map((item) => [item.actorId, item])))
              participants.forEach(schedulePresenceExpiry)
            }
            return
          }
          if (envelope.type === "typing.changed" && payload?.actorId && payload.conversationId === nextConversation.id) {
            if (payload.participantType !== "customer") {
              setRemoteTyping((current) => updateRealtimeTypingState(current, payload))
              const activePresence = presenceFromRealtimeTyping(payload)
              if (activePresence) {
                setPresence((current) => mergeRealtimePresenceActor(current, activePresence))
                schedulePresenceExpiry(activePresence)
              }
              const existingTimer = typingExpiry.get(payload.actorId)
              if (existingTimer) clearTimeout(existingTimer)
              typingExpiry.delete(payload.actorId)
              if (payload.typing) {
                const expectedActorID = payload.actorId
                typingExpiry.set(expectedActorID, setTimeout(() => {
                  setRemoteTyping((current) => expireRealtimeTypingActor(current, expectedActorID))
                  typingExpiry.delete(expectedActorID)
                }, 5_200))
              }
            }
            return
          }
          scheduleRefresh()
        }
        const protocol = window.location.protocol === "https:" ? "wss:" : "ws:"
        realtime = createRealtimeConnectionManager({
          createSocket: () => createCustomerSessionWebSocket(
            `${protocol}//${window.location.host}/api/ws/open`,
            realtimeToken,
          ),
          canReconnect: () => active,
          onSocketChange: (nextSocket) => {
            socketRef.current = nextSocket
          },
          onStatusChange: (status) => {
            if (!active) return
            realtimeConnected = status === "connected"
            setConnected(status === "connected")
            if (status === "disconnected") {
              presenceExpiry.forEach((lease) => clearTimeout(lease.timer))
              presenceExpiry.clear()
              setPresence((current) => Object.fromEntries(
                Object.entries(current).map(([key, value]) => [key, { ...value, online: false }]),
              ))
              setRemoteTyping({})
              scheduleRefresh(0)
            }
          },
          onOpen: (nextSocket) => {
            nextSocket.send(JSON.stringify({
              type: "subscribe",
              topics: [`conversation:${nextConversation.id}`],
            }))
          },
          onMessage: handleRealtimeMessage,
          reconnectBaseDelayMs: 500,
          reconnectMaxDelayMs: 5_000,
        })
        realtime.connect()
        pollTimer = setInterval(() => {
          if (!active || document.visibilityState !== "visible") return
          reconciliationTick += 1
          if (!realtimeConnected || reconciliationTick % 6 === 0) {
            scheduleRefresh(realtimeConnected ? 120 : 0)
          }
        }, 5_000)
      } catch (bootstrapError) {
        if (active) {
          setError(errorMessage(bootstrapError, translateCurrentMessage(`${customerEntryChatI18nPrefix}bootstrapFailed`)))
        }
      } finally {
        if (active) setLoading(false)
      }
    }

    void bootstrap()
    return () => {
      active = false
      socketRef.current = null
      realtime?.disconnect()
      if (pollTimer) clearInterval(pollTimer)
      if (refreshTimer) clearTimeout(refreshTimer)
      if (contextRefreshTimer) clearTimeout(contextRefreshTimer)
      typingExpiry.forEach((timer) => clearTimeout(timer))
      typingExpiry.clear()
      presenceExpiry.forEach((lease) => clearTimeout(lease.timer))
      presenceExpiry.clear()
    }
  }, [chat.entrySessionId, chat.visitorId, chat.visitorToken, refreshChat, retryNonce])

  const activeConversationID = conversation?.id ?? 0

  useEffect(() => {
    const customerMessages = messages.filter((message) => message.senderType === "customer").length
    setGuestQuestionCount((current) => Math.max(current, customerMessages))
  }, [messages])

  useEffect(() => {
    const socket = socketRef.current
    if (!socket || socket.readyState !== WebSocket.OPEN || activeConversationID <= 0) return
    const typing = draft.trim().length > 0
    socket.send(JSON.stringify({ type: "typing", conversationId: activeConversationID, typing }))
    if (!typing) return
    const timer = setTimeout(() => {
      if (socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({ type: "typing", conversationId: activeConversationID, typing: false }))
      }
    }, TYPING_IDLE_TIMEOUT_MS)
    return () => clearTimeout(timer)
  }, [activeConversationID, draft])

  useEffect(() => {
    messageEndRef.current?.scrollIntoView({ behavior: "smooth", block: "end" })
  }, [messages])

	  const sendMessage = useCallback(async () => {
	    const content = draft.trim()
	    if (!content || !token || !conversation || sending) return
    if (guestQuestionCount >= CUSTOMER_GUEST_QUESTION_LIMIT) {
      setError(cec(t, "guestQuestionLimit", { count: CUSTOMER_GUEST_QUESTION_LIMIT }))
      return
    }
    setSending(true)
    setError("")
    try {
      const message = await sendCustomerEntryMessage(token, {
        conversationId: conversation.id,
        content,
        clientMsgId: `entry_${generateUUID()}`,
      })
      setMessages((current) => mergeMessages(current, [message]))
      setGuestQuestionCount((current) => current + 1)
      setDraft("")
      if (socketRef.current?.readyState === WebSocket.OPEN) {
        socketRef.current.send(JSON.stringify({ type: "typing", conversationId: conversation.id, typing: false }))
      }
    } catch (sendError) {
      setError(errorMessage(sendError, cec(t, "sendFailed")))
    } finally {
      setSending(false)
	    }
	  }, [conversation, draft, guestQuestionCount, sending, t, token])

	  const sendMediaMessage = useCallback(async (
	    file: File,
	    messageType: "image" | "attachment" | "audio",
	    durationSeconds?: number,
	  ) => {
	    if (!token || !conversation || conversation.status === 4 || sending || uploadingAsset) return
	    if (guestQuestionCount >= CUSTOMER_GUEST_QUESTION_LIMIT) {
	      setError(cec(t, "guestQuestionLimit", { count: CUSTOMER_GUEST_QUESTION_LIMIT }))
	      return
	    }
	    setUploadingAsset(true)
	    setError("")
	    try {
	      const asset = messageType === "image"
	        ? await uploadCustomerEntryImage(token, conversation.id, file)
	        : messageType === "audio"
	          ? await uploadCustomerEntryAudio(token, conversation.id, file)
	          : await uploadCustomerEntryAttachment(token, conversation.id, file)
	      const message = await sendCustomerEntryMessage(token, {
	        conversationId: conversation.id,
	        content: asset.filename,
	        clientMsgId: `entry_${messageType}_${generateUUID()}`,
	        messageType,
	        payload: JSON.stringify({
	          assetId: asset.assetId,
	          ...(durationSeconds ? { durationSeconds } : {}),
	        }),
	      })
	      setMessages((current) => mergeMessages(current, [message]))
	      setGuestQuestionCount((current) => current + 1)
	    } catch (sendError) {
	      const fallback = messageType === "image"
	        ? cec(t, "sendImageFailed")
	        : messageType === "audio"
	          ? cec(t, "sendVoiceFailed")
	          : cec(t, "sendAttachmentFailed")
	      setError(errorMessage(sendError, fallback))
	    } finally {
	      setUploadingAsset(false)
	    }
	  }, [conversation, guestQuestionCount, sending, t, token, uploadingAsset])

  const requestHuman = useCallback(async () => {
    if (!token || !conversation || requestingHuman || conversation.humanHandoffEnabled === false) return
    setRequestingHuman(true)
    setError("")
    try {
      await requestCustomerEntryHumanSupport(
        token,
        conversation.id,
        handoffReason.trim() || cec(t, "handoffFallbackReason")
      )
      setHumanRequestAccepted(true)
      setHandoffPanelOpen(false)
      await refreshChat(token, conversation.id).catch(() => undefined)
      onContextChanged?.()
    } catch (requestError) {
      setError(errorMessage(requestError, cec(t, "handoffFailed")))
    } finally {
      setRequestingHuman(false)
    }
  }, [conversation, handoffReason, onContextChanged, refreshChat, requestingHuman, t, token])

  if (loading) {
    return (
      <div className="flex h-[max(320px,calc(100svh-330px))] max-h-[620px] flex-col overflow-hidden border bg-background">
        <div className="border-b bg-muted/20 px-4 py-3">
          <div className="flex items-center justify-between gap-3">
            <div className="flex min-w-0 items-center gap-2">
              <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary">
                <Loader2Icon className="size-4 animate-spin" />
              </span>
              <div className="min-w-0">
                <p className="truncate text-sm font-semibold">{cec(t, "serviceSession")}</p>
                <p className="mt-0.5 truncate text-xs text-muted-foreground">{cec(t, "connecting")}</p>
              </div>
            </div>
            <StatusTag tone="neutral" className="shrink-0">{cec(t, "loading")}</StatusTag>
          </div>
          <div className="mt-3 grid grid-cols-3 gap-1" aria-label={cec(t, "progressLoadingAria")}>
            <SupportStep label={cec(t, "stepOnline")} state="active" />
            <SupportStep label={cec(t, "stepHuman")} state="pending" />
            <SupportStep label={cec(t, "stepTracking")} state="pending" />
          </div>
        </div>
        <div className="min-h-0 flex-1 overflow-hidden px-4 py-4">
          <ModuleLoading variant="list" count={4} label={cec(t, "conversationLoading")} />
        </div>
        <div className="border-t bg-background px-4 pb-4 pt-3">
          <div className="flex items-end gap-2">
            <Input.TextArea className="min-h-11 max-h-28 resize-none" placeholder={cec(t, "inputPlaceholder")} disabled />
            <IconButton icon={<SendIcon className="size-4" />} className="size-11 shrink-0" disabled aria-label={cec(t, "sendMessage")} />
          </div>
        </div>
      </div>
    )
  }

  const conversationStage = supportStage(conversation)
  const stage = humanRequestAccepted && conversationStage === "ai"
    ? "queued"
    : conversationStage
  const humanRequested = humanRequestAccepted || stage === "queued" || stage === "human"
  const humanHandoffEnabled = conversation?.humanHandoffEnabled !== false
  const showHumanService = humanHandoffEnabled || humanRequested
  const conversationClosed = stage === "closed"
  const lastMessage = messages.length > 0 ? messages[messages.length - 1] : null
  const hasCustomerMessage = messages.some((message) => message.senderType === "customer")
  const waitingForReply = Boolean(
    lastMessage?.senderType === "customer" && !sending && !conversationClosed
  )
  const agentPresence = Object.values(presence).filter((item) => item.participantType === "agent")
  const assignedActorID = conversation?.currentAssigneeId
    ? `user:${conversation.currentAssigneeId}`
    : ""
  const assignedPresence = assignedActorID ? presence[assignedActorID] : undefined
  const partnerOnlineCount = Object.values(presence).filter(
    (item) => item.participantType === "partner" && item.online,
  ).length
  const humanOnline = assignedActorID
    ? Boolean(assignedPresence?.online)
    : agentPresence.some((item) => item.online)
  const remoteTypists = Object.values(remoteTyping)
	  const remoteTypingLabel = remoteTypists.length > 1
	    ? cec(t, "remoteTypingCount", { count: remoteTypists.length })
	    : remoteTypists[0]
	      ? cec(t, "remoteTypingName", {
	          name: remoteTypists[0].displayName || (remoteTypists[0].participantType === "partner" ? cec(t, "supplier") : cec(t, "engineer")),
	        })
	      : ""
	  const guestLimitReached = guestQuestionCount >= CUSTOMER_GUEST_QUESTION_LIMIT
	  const mediaDisabled = !conversation || sending || uploadingAsset || conversationClosed || guestLimitReached

  const stageCopy = {
    ai: {
      title: cec(t, "stageAi"),
      detail: humanHandoffEnabled
        ? connected ? cec(t, "stageAiHumanAvailable") : cec(t, "connecting")
        : connected ? cec(t, "stageAiContinue") : cec(t, "connecting"),
      icon: ShieldCheckIcon,
    },
    queued: {
      title: conversation?.currentAssigneeName
        ? cec(t, "assignedTo", { name: conversation.currentAssigneeName })
        : conversation?.currentAssigneeId
          ? cec(t, "assignedEngineer")
          : cec(t, "humanQueue"),
      detail: cec(t, "waitingAccept"),
      icon: Clock3Icon,
    },
    human: {
      title: conversation?.currentAssigneeName
        ? cec(t, "engineerJoined", { name: conversation.currentAssigneeName })
        : cec(t, "engineerJoinedFallback"),
      detail: humanOnline
        ? partnerOnlineCount > 0
          ? cec(t, "engineerOnlineWithSuppliers", { count: partnerOnlineCount })
          : cec(t, "engineerOnline")
        : cec(t, "engineerOffline"),
      icon: HeadphonesIcon,
    },
    closed: {
      title: cec(t, "serviceEndedTitle"),
      detail: cec(t, "serviceEndedDetail"),
      icon: CheckCircle2Icon,
    },
  }[stage]
  const StageIcon = stageCopy.icon

  return (
    <div className="flex h-[max(320px,calc(100svh-330px))] max-h-[620px] flex-col overflow-hidden border bg-background">
      <div className="border-b bg-muted/20 px-4 py-3">
        <div className="flex items-center justify-between gap-3">
          <div className="flex min-w-0 items-center gap-2">
            <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary">
              <StageIcon className="size-4" />
            </span>
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <p className="truncate text-sm font-semibold">{stageCopy.title}</p>
                <StatusTag tone={stage === "ai" || stage === "closed" ? "neutral" : "blue"}>
                  {stage === "ai" ? cec(t, "tagReceiving") : stage === "queued" ? (conversation?.currentAssigneeId ? cec(t, "tagPendingAssign") : cec(t, "tagQueued")) : stage === "human" ? cec(t, "tagHuman") : cec(t, "tagEnded")}
                </StatusTag>
              </div>
              <p className="mt-0.5 truncate text-xs text-muted-foreground">{stageCopy.detail}</p>
            </div>
          </div>
          {showHumanService ? (
            <RailopsButton
              variant={humanRequested || conversationClosed ? undefined : "primary"}
              size="small"
              className={cn(
                "shrink-0 gap-1.5",
                !humanRequested && !conversationClosed && "bg-primary text-primary-foreground shadow-sm hover:bg-primary/90",
              )}
              disabled={!conversation || requestingHuman || humanRequested || conversationClosed}
              aria-label={humanRequested ? cec(t, "handoffRequestedAria") : cec(t, "handoffAria")}
              onClick={() => {
                if (!humanRequested) {
                  setHandoffPanelOpen(true)
                  if (!handoffReason.trim()) {
                    setHandoffReason(cec(t, "handoffReasonStepsNotResolved"))
                  }
                }
              }}
            >
              {conversationClosed ? (
                <CheckCircle2Icon className="size-4" />
              ) : requestingHuman ? (
                <Loader2Icon className="size-4 animate-spin" />
              ) : humanRequested ? (
                <Clock3Icon className="size-4" />
              ) : (
                <HeadphonesIcon className="size-4" />
              )}
              {conversationClosed ? cec(t, "serviceEnded") : requestingHuman ? cec(t, "transferring") : humanRequested ? cec(t, "handoffRequested") : cec(t, "requestHuman")}
            </RailopsButton>
          ) : (
            <StatusTag tone="neutral" className="shrink-0 gap-1.5">
              <ShieldCheckIcon className="size-3.5" />
              {cec(t, "selfService")}
            </StatusTag>
          )}
        </div>

        {handoffPanelOpen && humanHandoffEnabled && !humanRequested && !conversationClosed ? (
          <div className="mt-3 rounded-lg border bg-background p-3 shadow-sm">
            <div className="flex items-start gap-2">
              <span className="mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary">
                <HeadphonesIcon className="size-3.5" />
              </span>
              <div className="min-w-0 flex-1">
                <p className="text-sm font-semibold">{cec(t, "handoffConfirmTitle")}</p>
                <div className="mt-2 flex flex-wrap gap-2">
                  {HANDOFF_REASON_KEYS.map((reasonKey) => {
                    const label = cec(t, reasonKey)
                    return (
                      <button
                        key={reasonKey}
                        type="button"
                        className={cn(
                          "rounded-full border px-2.5 py-1 text-xs transition",
                          handoffReason === label
                            ? "border-primary bg-primary/10 text-primary"
                            : "bg-background text-muted-foreground hover:border-primary/50 hover:text-foreground"
                        )}
                        onClick={() => setHandoffReason(label)}
                      >
                        {label}
                      </button>
                    )
                  })}
                </div>
                <Input.TextArea
                  className="mt-2 min-h-20 resize-none text-sm"
                  value={handoffReason}
                  onChange={(event) => setHandoffReason(event.target.value)}
                  placeholder={cec(t, "handoffDetailPlaceholder")}
                />
                <div className="mt-2 flex justify-end">
                  <div className="flex items-center gap-2">
                    <RailopsButton
                      variant="text"
                      size="small"
                      disabled={requestingHuman}
                      onClick={() => setHandoffPanelOpen(false)}
                    >
                      {cec(t, "handoffNotNow")}
                    </RailopsButton>
                    <RailopsButton
                      size="small"
                      disabled={requestingHuman}
                      onClick={() => void requestHuman()}
                    >
                      {requestingHuman ? <Loader2Icon className="size-4 animate-spin" /> : <HeadphonesIcon className="size-4" />}
                      {cec(t, "handoffConfirm")}
                    </RailopsButton>
                  </div>
                </div>
              </div>
            </div>
          </div>
        ) : null}

        <div className={cn("mt-3 grid gap-1", showHumanService ? "grid-cols-3" : "grid-cols-2")} aria-label={cec(t, "progressAria")}>
          <SupportStep label={cec(t, "stepOnline")} state={stage === "ai" ? "active" : "done"} />
          {showHumanService ? (
            <SupportStep
              label={cec(t, "stepHuman")}
              state={stage === "queued" || stage === "human" ? "active" : stage === "closed" ? "done" : "pending"}
            />
          ) : null}
          <SupportStep label={cec(t, "stepTracking")} state={stage === "closed" ? "done" : "pending"} />
        </div>
      </div>

      <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto px-4 py-4">
        {messages.length === 0 && !error ? (
          <div className="flex flex-1 flex-col items-center justify-center py-6 text-center">
            <span className="flex size-11 items-center justify-center rounded-full bg-primary/10 text-primary">
              <WrenchIcon className="size-5" />
            </span>
            <p className="mt-3 text-sm font-medium text-foreground">{cec(t, "emptyTitle")}</p>
            <p className="mt-1 max-w-xs text-xs leading-5 text-muted-foreground">
              {cec(t, "emptyHint")}
            </p>
          </div>
        ) : null}
        <CustomerConversationTimeline
          messages={messages}
          variant="entry"
          completion={stage === "closed" ? {
            title: cec(t, "serviceCompleted"),
          } : undefined}
        />
        {!hasCustomerMessage && !error ? (
          <div className="ml-9 space-y-2">
            <div className="grid grid-cols-2 gap-2">
              {(humanHandoffEnabled ? QUICK_PROMPTS : SELF_SERVICE_QUICK_PROMPTS).map((prompt) => {
                const PromptIcon = prompt.icon
                return (
                  <RailopsButton
                    key={prompt.labelKey}
                    size="small"
                    className="h-9 justify-start px-3 text-xs"
                    disabled={!conversation || conversationClosed || guestLimitReached}
                    onClick={() => setDraft(cec(t, prompt.labelKey))}
                  >
                    <PromptIcon className="size-3.5" />
                    {cec(t, prompt.labelKey)}
                  </RailopsButton>
                )
              })}
            </div>
          </div>
        ) : null}
        {remoteTypingLabel ? (
          <div className="flex items-center gap-2 text-xs text-muted-foreground" aria-live="polite">
            <span className="flex size-7 items-center justify-center rounded-full bg-primary/10 text-primary">
              <HeadphonesIcon className="size-3.5" />
            </span>
            <span className="rounded-md bg-muted px-3 py-2">
              {remoteTypingLabel}
            </span>
          </div>
        ) : waitingForReply ? (
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <span className="flex size-7 items-center justify-center rounded-full bg-primary/10 text-primary">
              {stage === "human" ? <HeadphonesIcon className="size-3.5" /> : <ShieldCheckIcon className="size-3.5" />}
            </span>
            <span className="flex items-center gap-1.5 rounded-md bg-muted px-3 py-2">
              <Loader2Icon className="size-3 animate-spin" />
              {stage === "human" ? cec(t, "engineerViewing") : stage === "queued" ? cec(t, "waitingEngineer") : cec(t, "processing")}
            </span>
          </div>
        ) : null}
        <div ref={messageEndRef} />
      </div>

      {error ? (
        <div className="mx-4 mb-2 flex items-center justify-between gap-3 rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
          <span>{error}</span>
          {!conversation ? (
            <RailopsButton size="small" onClick={() => setRetryNonce((value) => value + 1)}>
              <RotateCwIcon className="size-3.5" />
              {cec(t, "retry")}
            </RailopsButton>
          ) : null}
        </div>
      ) : null}

	      <div className="border-t bg-background px-4 pb-4 pt-3">
	        <div className="flex items-end gap-2">
	          <Input.TextArea
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter" && !event.shiftKey) {
                event.preventDefault()
                void sendMessage()
              }
            }}
            placeholder={guestLimitReached ? cec(t, "guestQuestionLimit", { count: CUSTOMER_GUEST_QUESTION_LIMIT }) : stage === "human" ? cec(t, "humanInputPlaceholder") : cec(t, "defaultInputPlaceholder")}
            className="min-h-11 max-h-28 resize-none"
            disabled={!conversation || sending || conversationClosed || guestLimitReached}
          />
          <IconButton
            icon={sending ? (
              <Loader2Icon className="size-4 animate-spin" />
            ) : (
              <SendIcon className="size-4" />
            )}
            className="size-11 shrink-0"
            disabled={!conversation || !draft.trim() || sending || conversationClosed || guestLimitReached}
            tooltip={cec(t, "sendMessage")}
            aria-label={cec(t, "sendMessage")}
	            onClick={() => void sendMessage()}
	          />
	        </div>
	        <div className="mt-2 flex flex-wrap items-center gap-2">
	          <input
	            ref={imageInputRef}
	            type="file"
	            accept="image/*"
	            className="hidden"
	            onChange={(event) => {
	              const file = event.target.files?.[0]
	              event.target.value = ""
	              if (file) void sendMediaMessage(file, "image")
	            }}
	          />
	          <input
	            ref={attachmentInputRef}
	            type="file"
	            className="hidden"
	            onChange={(event) => {
	              const file = event.target.files?.[0]
	              event.target.value = ""
	              if (file) void sendMediaMessage(file, "attachment")
	            }}
	          />
	          <RailopsButton
	            size="small"
	            className="gap-1.5"
	            disabled={mediaDisabled}
	            onClick={() => imageInputRef.current?.click()}
	          >
	            {uploadingAsset ? <Loader2Icon className="size-3.5 animate-spin" /> : <ImageIcon className="size-3.5" />}
	            {uploadingAsset ? cec(t, "mediaUploading") : cec(t, "sendImage")}
	          </RailopsButton>
	          <RailopsButton
	            size="small"
	            className="gap-1.5"
	            disabled={mediaDisabled}
	            onClick={() => attachmentInputRef.current?.click()}
	          >
	            {uploadingAsset ? <Loader2Icon className="size-3.5 animate-spin" /> : <PaperclipIcon className="size-3.5" />}
	            {uploadingAsset ? cec(t, "mediaUploading") : cec(t, "sendAttachment")}
	          </RailopsButton>
	          <VoiceRecorderButton
	            disabled={mediaDisabled}
	            className="border border-border"
	            onRecorded={(file, durationSeconds) => sendMediaMessage(file, "audio", durationSeconds)}
	            onError={(message) => setError(message)}
	          />
	        </div>
	        <div className="mt-2 flex items-center gap-1.5 text-rhd-xs text-muted-foreground">
	          <ShieldCheckIcon className="size-3.5" />
          {cec(t, "guestCounter", {
            count: Math.min(guestQuestionCount, CUSTOMER_GUEST_QUESTION_LIMIT),
            total: CUSTOMER_GUEST_QUESTION_LIMIT,
          })}
        </div>
      </div>
    </div>
  )
}

function SupportStep({
  label,
  state,
}: {
  label: string
  state: "active" | "done" | "pending"
}) {
  return (
    <div
      className={cn(
        "flex h-7 items-center justify-center gap-1 rounded-md text-rhd-xs font-medium",
        state === "active" && "bg-primary text-primary-foreground",
        state === "done" && "bg-primary/10 text-primary",
        state === "pending" && "bg-muted text-muted-foreground"
      )}
    >
      {state === "done" ? <CheckCircle2Icon className="size-3" /> : null}
      {state === "active" ? <span className="size-1.5 rounded-full bg-current" /> : null}
      {label}
    </div>
  )
}
