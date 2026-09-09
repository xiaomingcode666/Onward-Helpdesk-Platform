import MarkdownIt from "markdown-it"

import { translateCurrentMessage } from "@/i18n/messages"

export type MessageAssetPayload = {
  assetId?: string
  filename?: string
  fileSize?: number
  mimeType?: string
  url?: string
  durationSeconds?: number
  pending?: boolean
  failed?: boolean
  localTransfer?: boolean
}

export type KnowledgeMessageCitation = {
  documentId?: number
  documentTitle?: string
  faqId?: number
  faqQuestion?: string
  chunkNo?: number
  title?: string
  sectionPath?: string
  snippet?: string
}

type MediaTransferPayloadInput = Pick<
  MessageAssetPayload,
  "filename" | "fileSize" | "mimeType" | "durationSeconds"
>

let pendingMessageSequence = 0

export function createPendingMessageId() {
  pendingMessageSequence = (pendingMessageSequence + 1) % 1000
  return Date.now() * 1000 + pendingMessageSequence
}

export function buildMediaTransferPayload(
  input: MediaTransferPayloadInput,
  state: "pending" | "failed" = "pending",
) {
  return JSON.stringify({
    ...input,
    localTransfer: true,
    pending: state === "pending",
    failed: state === "failed",
  } satisfies MessageAssetPayload)
}

const messageMarkdown = new MarkdownIt({
  html: false,
  linkify: true,
  breaks: true,
})

function t(key: string) {
  return translateCurrentMessage(key)
}

export function parseMessageAssetPayload(payload?: string): MessageAssetPayload | null {
  if (!payload?.trim()) {
    return null
  }
  try {
    const parsed = JSON.parse(payload) as MessageAssetPayload
    if (!parsed?.assetId?.trim() && !parsed?.url?.trim() && !parsed?.pending && !parsed?.failed) {
      return null
    }
    return parsed
  } catch {
    return null
  }
}

export function parseKnowledgeMessageCitations(payload?: string): KnowledgeMessageCitation[] {
  if (!payload?.trim()) return []
  try {
    const parsed = JSON.parse(payload) as Record<string, unknown>
    if (parsed?.kind !== "knowledge_answer" || !Array.isArray(parsed.knowledgeCitations)) {
      return []
    }
    return parsed.knowledgeCitations
      .filter((item): item is Record<string, unknown> => Boolean(item) && typeof item === "object" && !Array.isArray(item))
      .map((item) => ({
        documentId: finiteNumber(item.documentId),
        documentTitle: trimmedText(item.documentTitle),
        faqId: finiteNumber(item.faqId),
        faqQuestion: trimmedText(item.faqQuestion),
        chunkNo: finiteNumber(item.chunkNo),
        title: trimmedText(item.title),
        sectionPath: trimmedText(item.sectionPath),
        snippet: trimmedText(item.snippet),
      }))
      .filter((item) => Boolean(item.documentTitle || item.faqQuestion || item.title || item.sectionPath || item.snippet))
      .slice(0, 3)
  } catch {
    return []
  }
}

function finiteNumber(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined
}

function trimmedText(value: unknown) {
  return typeof value === "string" && value.trim() ? value.trim() : undefined
}

export function isLocalMediaTransferPayload(payload?: string) {
  return Boolean(parseMessageAssetPayload(payload)?.localTransfer)
}

export function renderIMMessageHTML(message: {
  messageType: string
  content: string
  payload?: string
}) {
  if (message.messageType === "html") {
    return message.content
  }

  const asset = parseMessageAssetPayload(message.payload)

  if (asset?.pending || asset?.failed) {
    return renderMediaTransferState(message.messageType, message.content, asset)
  }

  if (
    message.messageType === "audio" ||
    (message.messageType === "attachment" && asset?.mimeType?.toLowerCase().startsWith("audio/"))
  ) {
    if (asset?.assetId || asset?.url) {
      const title = escapeHTML(asset.filename || t("supportChat.audioSummary"))
      const duration = formatAudioDuration(asset.durationSeconds ?? 0)
      const source = renderMediaSourceAttributes(asset)
      return `<div class="im-audio" data-media-state="${asset.assetId ? "loading" : "ready"}">${renderMediaLoadingStatus(Boolean(asset.assetId))}<audio class="${asset.assetId ? "hidden" : ""}" controls preload="metadata" ${source} aria-label="${escapeHTMLAttr(title)}"></audio><div class="im-audio-meta"><span>${title}</span>${duration ? `<span>${duration}</span>` : ""}</div></div>`
    }
    return `<p>${escapeHTML(t("supportChat.audioSummary"))}</p>`
  }
  if (message.messageType === "image") {
    if (asset?.assetId || asset?.url) {
      return `<div class="im-image" data-media-state="${asset.assetId ? "loading" : "ready"}">${renderMediaLoadingStatus(Boolean(asset.assetId))}<img class="${asset.assetId ? "hidden" : ""}" ${renderMediaSourceAttributes(asset)} data-media-state="${asset.assetId ? "loading" : "ready"}" alt="${escapeHTMLAttr(
        asset.filename || "image"
      )}"></div>`
    }
    return `<p>${escapeHTML(t("supportChat.imageSummary"))}</p>`
  }

  if (message.messageType === "attachment") {
    if (asset?.mimeType?.toLowerCase().startsWith("video/") && (asset.assetId || asset.url)) {
      const title = escapeHTML(asset.filename || t("supportChat.videoSummary"))
      return `<div class="im-video" data-media-state="${asset.assetId ? "loading" : "ready"}">${renderMediaLoadingStatus(Boolean(asset.assetId))}<video class="${asset.assetId ? "hidden" : ""}" controls preload="metadata" ${renderMediaSourceAttributes(asset)} aria-label="${escapeHTMLAttr(title)}"></video><div class="im-audio-meta"><span>${title}</span></div></div>`
    }
    if (asset?.assetId || asset?.url) {
      const title = escapeHTML(asset.filename || message.content || t("supportChat.attachmentSummary"))
      const meta = formatFileSize(asset.fileSize ?? 0)
      const metaHTML = meta ? `<div class="im-attachment-meta">${escapeHTML(meta)}</div>` : ""
      const href = asset.url ? ` href="${escapeHTMLAttr(asset.url)}" target="_blank" rel="noreferrer"` : ""
      const assetAttr = asset.assetId ? ` data-asset-id="${escapeHTMLAttr(asset.assetId)}"` : ""
      return `<div class="im-attachment"><a${href}${assetAttr} download="${escapeHTMLAttr(
        asset.filename || ""
      )}" class="im-attachment-link"><span class="im-attachment-icon" aria-hidden="true">${getAttachmentIconSVG()}</span><span class="im-attachment-content"><span class="im-attachment-title">${title}</span>${metaHTML}</span></a></div>`
    }
    return `<p>${escapeHTML(message.content || t("supportChat.attachmentSummary"))}</p>`
  }
  return renderTextMessageHTML(message.content || "")
}

function renderMediaSourceAttributes(asset: MessageAssetPayload) {
  if (asset.assetId?.trim()) {
    return `data-asset-id="${escapeHTMLAttr(asset.assetId.trim())}"`
  }
  return asset.url ? `src="${escapeHTMLAttr(asset.url)}"` : ""
}

function renderMediaLoadingStatus(visible: boolean) {
  return `<div class="im-media-status${visible ? "" : " hidden"}" role="status" aria-live="polite"><span class="im-media-spinner" aria-hidden="true"></span><span class="im-media-status-label">${escapeHTML(t("supportChat.mediaLoading"))}</span><button type="button" class="im-media-retry hidden">${escapeHTML(t("supportChat.mediaRetry"))}</button></div>`
}

function renderMediaTransferState(
  messageType: string,
  content: string,
  asset: MessageAssetPayload,
) {
  const failed = Boolean(asset.failed)
  const label = t(failed ? "supportChat.mediaSendFailed" : "supportChat.mediaSending")
  const title = escapeHTML(asset.filename || content || mediaSummary(messageType, asset.mimeType))
  const meta = formatFileSize(asset.fileSize ?? 0)
  return `<div class="im-media-transfer${failed ? " im-media-transfer-failed" : ""}" role="${failed ? "alert" : "status"}" aria-live="polite">${failed ? "" : '<span class="im-media-spinner" aria-hidden="true"></span>'}<span class="im-media-transfer-content"><span>${escapeHTML(label)}</span><span class="im-media-transfer-meta">${title}${meta ? ` - ${escapeHTML(meta)}` : ""}</span></span></div>`
}

function mediaSummary(messageType: string, mimeType?: string) {
  if (messageType === "image") return t("supportChat.imageSummary")
  if (messageType === "audio" || mimeType?.toLowerCase().startsWith("audio/")) {
    return t("supportChat.audioSummary")
  }
  if (mimeType?.toLowerCase().startsWith("video/")) return t("supportChat.videoSummary")
  return t("supportChat.attachmentSummary")
}

export function summarizeIMMessage(message: {
  messageType: string
  content: string
  payload?: string
}) {
  const asset = parseMessageAssetPayload(message.payload)
  if (
    message.messageType === "audio" ||
    (message.messageType === "attachment" && asset?.mimeType?.toLowerCase().startsWith("audio/"))
  ) {
    return t("supportChat.audioSummary")
  }
  if (message.messageType === "image") {
    return t("supportChat.imageSummary")
  }
  if (message.messageType === "attachment") {
    return asset?.filename?.trim()
      ? `${t("supportChat.attachmentSummary")} ${asset.filename.trim()}`
      : t("supportChat.attachmentSummary")
  }
  if (message.messageType === "html") {
    const text = extractTextFromHTML(message.content)
    if (text.trim()) {
      return text.substring(0, 100)
    }
    if (message.content.includes("<img")) {
      return t("supportChat.imageSummary")
    }
    return t("supportChat.messageSummary")
  }
  return message.content?.substring(0, 100) || t("supportChat.messageSummary")
}

export function formatFileSize(size: number) {
  if (!Number.isFinite(size) || size <= 0) {
    return ""
  }
  const units = ["B", "KB", "MB", "GB"]
  let value = size
  let index = 0
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024
    index += 1
  }
  const digits = value >= 10 || index === 0 ? 0 : 1
  return `${value.toFixed(digits)} ${units[index]}`
}

export function formatAudioDuration(seconds: number) {
  if (!Number.isFinite(seconds) || seconds <= 0) return ""
  const wholeSeconds = Math.floor(seconds)
  return `${Math.floor(wholeSeconds / 60)}:${String(wholeSeconds % 60).padStart(2, "0")}`
}

function extractTextFromHTML(html: string): string {
  if (typeof document === "undefined") {
    return ""
  }
  const div = document.createElement("div")
  div.innerHTML = html
  return div.textContent || div.innerText || ""
}

function renderTextMessageHTML(content: string) {
  const value = content.trim()
  if (!value) {
    return "<p></p>"
  }
  return messageMarkdown.render(value)
}

function escapeHTML(value: string) {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;")
    .replaceAll("\n", "<br>")
}

function escapeHTMLAttr(value: string) {
  return value
    .replaceAll("&", "&amp;")
    .replaceAll('"', "&quot;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
}

function getAttachmentIconSVG() {
  return `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><path d="M14 2v6h6"></path><path d="M9 15h6"></path><path d="M9 11h2"></path></svg>`
}
