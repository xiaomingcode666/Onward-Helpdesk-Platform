"use client"

import { useEffect, useRef, useState } from "react"
import { FileAudioIcon, Loader2Icon, SendIcon } from "lucide-react"
import { Input } from "antd"
import { RailopsButton, StandardModal } from "@railops/ui"

import { VoiceRecorderButton } from "@/components/chat/voice-recorder-button"
import { useI18n } from "@/i18n/provider"
import type { CustomerVoiceTicketResult, CustomerVoiceTranscription, ImAsset, ImMessage } from "@/lib/api/im"
import { convertAudioToPCM16Wav } from "@/lib/audio/pcm-wav"
import { generateUUID } from "@/lib/utils"

const VOICE_TICKET_TRANSCRIPT_MAX = 4000

// Customer voice ticket calls, bound to the caller's session (guest entry
// session or logged-in customer portal).
export type CustomerVoiceTicketApi = {
  uploadAudio: (conversationId: number, file: File) => Promise<ImAsset>
  sendMessage: (payload: { conversationId: number; content: string; clientMsgId: string; messageType: string; payload: string }) => Promise<ImMessage>
  transcribe: (conversationId: number, wav: File) => Promise<CustomerVoiceTranscription>
  submit: (body: { conversationId: number; transcript: string; audioAssetId: string }) => Promise<CustomerVoiceTicketResult>
}

type CustomerVoiceTicketDialogProps = {
  open: boolean
  api: CustomerVoiceTicketApi
  conversationId: number
  onOpenChange: (open: boolean) => void
  onMessageSent: (message: ImMessage) => void
  onSubmitted: (result: CustomerVoiceTicketResult) => void
}

function errorText(error: unknown, fallback: string) {
  return error instanceof Error && error.message ? error.message : fallback
}

export function CustomerVoiceTicketDialog({
  open,
  api,
  conversationId,
  onOpenChange,
  onMessageSent,
  onSubmitted,
}: CustomerVoiceTicketDialogProps) {
  const t = useI18n()
  const vt = (key: string, values?: Record<string, string | number>) => t(`customerEntryExtract.chat.voiceTicket${key}`, values)
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const [audioAssetId, setAudioAssetId] = useState("")
  const [transcript, setTranscript] = useState("")
  const [transcribing, setTranscribing] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState("")
  const busy = transcribing || submitting

  useEffect(() => {
    if (!open) return
    setAudioAssetId("")
    setTranscript("")
    setTranscribing(false)
    setSubmitting(false)
    setError("")
  }, [open, conversationId])

  async function processAudio(file: File, durationSeconds?: number) {
    setTranscribing(true)
    setError("")
    setTranscript("")
    try {
      // Send the original recording first so it is kept in the conversation.
      const asset = await api.uploadAudio(conversationId, file)
      const message = await api.sendMessage({
        conversationId,
        content: asset.filename,
        clientMsgId: `entry_voice_ticket_${generateUUID()}`,
        messageType: "audio",
        payload: JSON.stringify({ assetId: asset.assetId, ...(durationSeconds ? { durationSeconds } : {}) }),
      })
      onMessageSent(message)
      setAudioAssetId(asset.assetId)

      let wav: File
      try {
        wav = await convertAudioToPCM16Wav(file)
      } catch {
        setError(vt("DecodeFailed"))
        return
      }
      const result = await api.transcribe(conversationId, wav)
      setTranscript(result.text.slice(0, VOICE_TICKET_TRANSCRIPT_MAX))
    } catch (processError) {
      setError(errorText(processError, vt("TranscribeFailed")))
    } finally {
      setTranscribing(false)
    }
  }

  async function submit() {
    if (!audioAssetId || !transcript.trim() || busy) return
    setSubmitting(true)
    setError("")
    try {
      const result = await api.submit({ conversationId, transcript: transcript.trim(), audioAssetId })
      onSubmitted(result)
      onOpenChange(false)
    } catch (submitError) {
      setError(errorText(submitError, vt("SubmitFailed")))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <StandardModal
      open={open}
      destroyOnHidden
      centered
      width={560}
      title={vt("Title")}
      onCancel={() => { if (!busy) onOpenChange(false) }}
      footer={(
        <>
          <RailopsButton onClick={() => onOpenChange(false)} disabled={busy}>{vt("Cancel")}</RailopsButton>
          <RailopsButton
            variant="primary"
            data-testid="voice-ticket-submit"
            onClick={() => void submit()}
            disabled={!audioAssetId || !transcript.trim() || busy}
          >
            {submitting ? <Loader2Icon className="size-4 animate-spin" /> : <SendIcon className="size-4" />}
            {submitting ? vt("Submitting") : vt("Submit")}
          </RailopsButton>
        </>
      )}
    >
      <div className="grid gap-4">
        <div className="flex flex-wrap items-center gap-2">
          <input
            ref={fileInputRef}
            type="file"
            accept="audio/*"
            className="hidden"
            data-testid="voice-ticket-file"
            onChange={(event) => {
              const selected = event.target.files?.[0]
              event.target.value = ""
              if (selected) void processAudio(selected)
            }}
          />
          <RailopsButton onClick={() => fileInputRef.current?.click()} disabled={busy}>
            <FileAudioIcon className="size-4" /> {vt("ChooseFile")}
          </RailopsButton>
          <VoiceRecorderButton
            disabled={busy}
            className="border border-border"
            onRecorded={(file, seconds) => processAudio(file, seconds)}
            onError={(message) => setError(message)}
          />
        </div>
        {transcribing ? (
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Loader2Icon className="size-4 animate-spin" /> {vt("Transcribing")}
          </div>
        ) : null}
        <label className="grid gap-1.5 text-sm font-medium text-foreground">
          {vt("Transcript")}
          <Input.TextArea
            data-testid="voice-ticket-transcript"
            value={transcript}
            rows={6}
            maxLength={VOICE_TICKET_TRANSCRIPT_MAX}
            showCount
            disabled={busy}
            onChange={(event) => setTranscript(event.target.value)}
            placeholder={vt("TranscriptPlaceholder")}
          />
        </label>
        {error ? <p className="text-sm text-destructive" role="alert">{error}</p> : null}
        <p className="text-xs text-muted-foreground">{vt("Hint")}</p>
      </div>
    </StandardModal>
  )
}
