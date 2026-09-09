export type LiveTranscriptEvent = {
	provider?: "jitsi_caption" | "jitsi_chat"
	providerEventId: string
  participantId: string
  speakerName: string
  language: string
  text: string
  isFinal: boolean
  startedAtMs: number
  endedAtMs: number
  confidence: number
  translatedLanguage?: string
  translatedText?: string
  translationProvider?: string
  translationStatus?: string
  translationError?: string
}

type JitsiTranscriptionChunk = {
  messageID?: string
  language?: string
  participant?: {
    id?: string
    name?: string
  }
  final?: string
  stable?: string
  unstable?: string
}

export function parseJitsiTranscriptionChunk(payload: unknown): LiveTranscriptEvent | null {
  if (!payload || typeof payload !== "object") return null
  const direct = payload as JitsiTranscriptionChunk & { data?: JitsiTranscriptionChunk }
  const chunk = direct.data && typeof direct.data === "object" ? direct.data : direct
  const text = firstText(chunk.final, chunk.stable, chunk.unstable)
  if (!text) return null

  const now = Date.now()
  const participantId = cleanText(chunk.participant?.id) || "unknown"
  return {
    providerEventId: cleanText(chunk.messageID) || `${participantId}-${now}`,
    participantId,
    speakerName: cleanText(chunk.participant?.name),
    language: cleanText(chunk.language) || "zh-CN",
    text,
    isFinal: Boolean(cleanText(chunk.final)),
    startedAtMs: now,
    endedAtMs: now,
    confidence: 0,
  }
}

function firstText(...values: unknown[]) {
  for (const value of values) {
    const text = cleanText(value)
    if (text) return text
  }
  return ""
}

function cleanText(value: unknown) {
  return typeof value === "string" ? value.trim() : ""
}
