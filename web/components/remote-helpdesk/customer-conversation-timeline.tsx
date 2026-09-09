import {
  HeadphonesIcon,
  LanguagesIcon,
  Loader2Icon,
  ShieldCheckIcon,
  UserRoundIcon,
  UsersRoundIcon,
} from "lucide-react"

import { ImMessageHTML } from "@/components/im-message-html"
import { KnowledgeMessageCitations } from "@/components/remote-helpdesk/knowledge-message-citations"
import {
  ConversationPhaseDivider,
  ConversationServiceEvent,
  parseConversationServiceEvent,
  type ConversationServiceEventData,
  type ConversationServicePhase,
} from "@/components/remote-helpdesk/conversation-service-event"
import { useI18n } from "@/i18n/provider"
import type { ImMessage } from "@/lib/api/im"
import type { ConversationMessageTranslationDTO } from "@/lib/api/types"
import { renderIMMessageHTML } from "@/lib/im-message"
import { cn } from "@/lib/utils"

type CustomerConversationTimelineProps = {
  messages: ImMessage[]
  variant?: "portal" | "entry"
  hasDeviceConcept?: boolean
  completion?: {
    title: string
    description?: string
  }
  translatedMessages?: Record<number, ConversationMessageTranslationDTO>
  translatingMessageId?: number | null
  onTranslateMessage?: (message: ImMessage) => Promise<void> | void
}

const customerEntryTimelineI18nPrefix = "customerEntryExtract.timeline."
type CustomerConversationTimelineT = ReturnType<typeof useI18n>
const cet = (t: CustomerConversationTimelineT, key: string, values?: Record<string, string | number>) => t(`${customerEntryTimelineI18nPrefix}${key}`, values)

const managedMessageKeys: Record<string, string> = {
  "正在为你接入人工客服，请稍候。": "handoffWaiting",
  "We are connecting you to a human support agent. Please wait.": "handoffWaiting",
  "Te estamos conectando con un agente de soporte. Espera un momento.": "handoffWaiting",
  "您好，我是企业知识服务助手。请直接告诉我你想了解的问题。": "welcomeKnowledgeSupport",
  "Hello, I'm your company's knowledge assistant. Tell me what you'd like to know.": "welcomeKnowledgeSupport",
  "Hola, soy el asistente de conocimiento de tu empresa. Dime que te gustaria saber.": "welcomeKnowledgeSupport",
  "您好，我是企业服务助手。你可以直接咨询，也可以描述产品或设备问题。": "welcomeGeneralService",
  "Hello, I'm your company's service assistant. You can ask a question directly or describe a product or equipment issue.": "welcomeGeneralService",
  "Hola, soy el asistente de servicio de tu empresa. Puedes hacer una consulta directamente o describir un problema con un producto o equipo.": "welcomeGeneralService",
}

function localizeManagedMessage(message: ImMessage, t: CustomerConversationTimelineT) {
  const senderType = message.senderType.toLowerCase()
  if (senderType !== "ai" && !senderType.includes("bot")) return message.content
  const normalizedContent = message.content.trim().replace(/(?:(?:&#x20;|&#32;|&nbsp;)\s*)+$/gi, "").trim()
  const key = managedMessageKeys[normalizedContent]
  return key ? cet(t, key) : message.content
}

const customerTranslationLanguageKeys: Record<string, string> = {
  "zh-CN": "languageZhCN",
  en: "languageEn",
  de: "languageDe",
  es: "languageEs",
  fr: "languageFr",
  pt: "languagePt",
  ja: "languageJa",
  ko: "languageKo",
}

function customerTranslationLanguageLabel(t: CustomerConversationTimelineT, language: string) {
  return customerTranslationLanguageKeys[language] ? cet(t, customerTranslationLanguageKeys[language]) : language
}

function formatMessageTime(value?: string) {
  if (!value) return ""
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ""
  return new Intl.DateTimeFormat("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(date)
}

function senderLabel(message: ImMessage, t: CustomerConversationTimelineT) {
  const senderType = message.senderType.toLowerCase()
  if (senderType === "ai" || senderType.includes("bot")) return cet(t, "senderOnlineAssistant")
  if (senderType === "agent" || senderType.includes("engineer")) {
    return message.senderName?.trim() || cet(t, "senderAfterSalesEngineer")
  }
  if (senderType === "partner") {
    return message.senderName?.trim() || cet(t, "senderSupplierTechSupport")
  }
  return message.senderName?.trim() || cet(t, "senderServiceNotice")
}

function messageTone(senderType: string) {
  switch (senderType.toLowerCase()) {
    case "customer":
      return "border-primary/20 bg-primary/10 text-foreground"
    case "agent":
      return "border-primary/20 bg-primary/10 text-primary"
    case "partner":
      return "border-border bg-muted text-muted-foreground"
    default:
      return "border-border bg-card text-foreground"
  }
}

function SenderAvatarIcon({ senderType }: { senderType: string }) {
  const normalized = senderType.toLowerCase()
  if (normalized === "customer") return <UserRoundIcon className="size-3.5" />
  if (normalized === "partner") return <UsersRoundIcon className="size-3.5" />
  if (normalized === "agent" || normalized.includes("engineer")) {
    return <HeadphonesIcon className="size-3.5" />
  }
  return <ShieldCheckIcon className="size-3.5" />
}

function CustomerConversationMessage({
  message,
  event,
  variant,
  hasDeviceConcept,
  translation,
  translating,
  onTranslateMessage,
}: {
  message: ImMessage
  event: ConversationServiceEventData | null
  variant: "portal" | "entry"
  hasDeviceConcept: boolean
  translation?: ConversationMessageTranslationDTO
  translating: boolean
  onTranslateMessage?: (message: ImMessage) => Promise<void> | void
}) {
  const t = useI18n()
  const senderType = message.senderType.toLowerCase()
  const fromCustomer = senderType === "customer"
  const fromSystem = senderType === "system"
  const messageTime = formatMessageTime(message.sentAt)
  const canTranslate = Boolean(onTranslateMessage && message.id > 0 && message.content.trim())
  const compact = variant === "portal"
  const localizedContent = localizeManagedMessage(message, t)
  const localizedMessage = localizedContent === message.content ? message : { ...message, content: localizedContent }

  if (event) {
    return <ConversationServiceEvent audience="customer" density={compact ? "compact" : "default"} event={event} time={messageTime} hasDeviceConcept={hasDeviceConcept} />
  }

  if (fromSystem) {
    return (
      <div className="flex justify-center py-1" data-sender-type="system">
        <div className={cn("max-w-[90%] rounded-md bg-muted px-3 py-1.5 text-center text-muted-foreground", compact ? "text-rhd-xs leading-5" : "text-xs leading-5")}>
          <ImMessageHTML html={renderIMMessageHTML(localizedMessage)} className={compact ? "text-rhd-xs leading-5" : "text-xs leading-5"} />
          {messageTime ? <time className="ml-2 text-rhd-2xs text-muted-foreground">{messageTime}</time> : null}
        </div>
      </div>
    )
  }

  return (
    <div
      data-message-id={message.id}
      data-sender-type={senderType}
      aria-label={cet(t, "messageAria", { sender: fromCustomer ? cet(t, "customer") : senderLabel(message, t) })}
      className={cn("flex gap-2.5", fromCustomer ? "justify-end" : "justify-start")}
    >
      {!fromCustomer ? (
        <span className={cn(
          "mt-5 flex shrink-0 items-center justify-center rounded-full border border-border bg-card text-muted-foreground shadow-sm",
          compact ? "size-7" : "size-7",
        )}>
          <SenderAvatarIcon senderType={senderType} />
        </span>
      ) : null}
      <div className={cn(variant === "portal" ? "max-w-[78%]" : "max-w-[84%]", fromCustomer && "text-right")}>
        <div className={cn("mb-1 flex items-center gap-2 text-rhd-xs text-muted-foreground", compact && "text-rhd-2xs", fromCustomer && "justify-end")}>
          <span className="font-medium">{fromCustomer ? cet(t, "self") : senderLabel(message, t)}</span>
          {messageTime ? <time>{messageTime}</time> : null}
        </div>
        <div className={cn(
          "break-words rounded-lg border text-left shadow-[0_1px_1px_rgba(15,23,42,0.03)]",
          compact ? "px-3 py-2 text-rhd-md leading-5" : "px-4 py-3 text-sm leading-6",
          messageTone(senderType),
        )}>
          <ImMessageHTML html={renderIMMessageHTML(localizedMessage)} className="text-current [&_a]:text-primary" />
        </div>
        {!fromCustomer ? <KnowledgeMessageCitations payload={message.payload} /> : null}
        {translation ? (
          <div className={cn("mt-1 rounded-lg border border-primary/20 bg-primary/10 px-3 py-2 text-left leading-5 text-foreground", compact ? "text-rhd-xs" : "text-xs")} data-testid="customer-message-translation">
            <div className="mb-1 text-rhd-2xs font-semibold uppercase text-primary">
              {cet(t, "translationLabel", { language: customerTranslationLanguageLabel(t, translation.target_language) })}
            </div>
            {translation.translated_text}
          </div>
        ) : null}
        {canTranslate ? (
          <div className={cn("mt-1 flex items-center", fromCustomer ? "justify-end" : "justify-start")}>
            <button
              type="button"
              className="flex size-7 items-center justify-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground disabled:opacity-50"
              onClick={() => void onTranslateMessage?.(message)}
              disabled={translating}
              aria-label={cet(t, "translate")}
              title={cet(t, "translate")}
              data-testid="customer-translate-message-button"
            >
              {translating ? (
                <Loader2Icon className="size-3.5 animate-spin" />
              ) : (
                <LanguagesIcon className="size-3.5" />
              )}
            </button>
          </div>
        ) : null}
      </div>
      {fromCustomer ? (
        <span className={cn(
          "mt-5 flex shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary",
          variant === "portal" ? "size-8" : "size-7",
        )}>
          <SenderAvatarIcon senderType={senderType} />
        </span>
      ) : null}
    </div>
  )
}

export function CustomerConversationTimeline({
  messages,
  variant = "portal",
  hasDeviceConcept = true,
  completion,
  translatedMessages = {},
  translatingMessageId = null,
  onTranslateMessage,
}: CustomerConversationTimelineProps) {
  const compact = variant === "portal"
  const timeline = messages.reduce<{
    currentPhase: ConversationServicePhase
    rows: Array<{
      message: ImMessage
      event: ConversationServiceEventData | null
      phase?: ConversationServicePhase
    }>
  }>((result, message) => {
    const event = parseConversationServiceEvent(message)
    const nextPhase = event?.phase
    return {
      currentPhase: nextPhase || result.currentPhase,
      rows: [
        ...result.rows,
        {
          message,
          event,
          phase: nextPhase && nextPhase !== result.currentPhase ? nextPhase : undefined,
        },
      ],
    }
  }, { currentPhase: "consultation", rows: [] })
  const completionEvent: ConversationServiceEventData | null = completion
    ? {
        kind: "completed",
        eventType: "conversation_closed",
        title: completion.title,
        description: completion.description ?? "",
        phase: "video-completion",
      }
    : null

  return (
    <div className={cn(compact ? "space-y-4" : "space-y-3")}>
      {messages.length > 0 ? <ConversationPhaseDivider density={compact ? "compact" : "default"} phase="consultation" /> : null}
      {timeline.rows.map((row) => (
          <div key={row.message.id}>
            {row.phase ? <ConversationPhaseDivider density={compact ? "compact" : "default"} phase={row.phase} /> : null}
            <CustomerConversationMessage
              message={row.message}
              event={row.event}
              variant={variant}
              hasDeviceConcept={hasDeviceConcept}
              translation={translatedMessages[row.message.id]}
              translating={row.message.id === translatingMessageId}
              onTranslateMessage={onTranslateMessage}
            />
          </div>
      ))}
      {completionEvent ? (
        <>
          {timeline.currentPhase !== completionEvent.phase && completionEvent.phase ? (
            <ConversationPhaseDivider density={compact ? "compact" : "default"} phase={completionEvent.phase} />
          ) : null}
          <ConversationServiceEvent audience="customer" density={compact ? "compact" : "default"} event={completionEvent} hasDeviceConcept={hasDeviceConcept} />
        </>
      ) : null}
    </div>
  )
}
