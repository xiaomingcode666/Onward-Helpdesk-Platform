"use client"

import { useEffect, useRef, useState } from "react"
import { FileAudioIcon, Loader2Icon, SendIcon } from "lucide-react"
import { Input } from "antd"
import { RailopsButton, StandardModal } from "@railops/ui"

import { VoiceRecorderButton } from "@/components/chat/voice-recorder-button"
import { useI18n } from "@/i18n/provider"
import {
  submitCustomerVoiceTicket,
  transcribeCustomerVoice,
  type CustomerVoiceTicketResult,
} from "@/lib/api/customer-voice-tickets"
import { convertAudioToPCM16Wav } from "@/lib/audio/pcm-wav"
import { generateUUID } from "@/lib/utils"

const VOICE_TICKET_TRANSCRIPT_MAX = 4000

type CustomerVoiceTicketDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSubmitted: (result: CustomerVoiceTicketResult) => void
}

function errorText(error: unknown, fallback: string) {
  return error instanceof Error && error.message ? error.message : fallback
}

// Standalone voice ticket: the recording is transcribed, reviewed by the
// customer and submitted as a new ticket that is not tied to any conversation.
export function CustomerVoiceTicketDialog({ open, onOpenChange, onSubmitted }: CustomerVoiceTicketDialogProps) {
  const t = useI18n()
  const vt = (key: string) => t(`customerEntryExtract.chat.voiceTicket${key}`)
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const [audio, setAudio] = useState<File | null>(null)
  const [transcript, setTranscript] = useState("")
  const [requestKey, setRequestKey] = useState("")
  const [transcribing, setTranscribing] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState("")
  const busy = transcribing || submitting

  useEffect(() => {
    if (!open) return
    setAudio(null)
    setTranscript("")
    setRequestKey(generateUUID())
    setTranscribing(false)
    setSubmitting(false)
    setError("")
  }, [open])

  async function processAudio(file: File) {
    setTranscribing(true)
    setError("")
    setTranscript("")
    setAudio(file)
    try {
      let wav: File
      try {
        wav = await convertAudioToPCM16Wav(file)
      } catch {
        setError(vt("DecodeFailed"))
        return
      }
      const result = await transcribeCustomerVoice(wav)
      setTranscript(result.text.slice(0, VOICE_TICKET_TRANSCRIPT_MAX))
    } catch (processError) {
      setError(errorText(processError, vt("TranscribeFailed")))
    } finally {
      setTranscribing(false)
    }
  }

  async function submit() {
    if (!audio || !transcript.trim() || busy) return
    setSubmitting(true)
    setError("")
    try {
      const result = await submitCustomerVoiceTicket({ transcript: transcript.trim(), requestKey, audio })
      onOpenChange(false)
      onSubmitted(result)
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
            disabled={!audio || !transcript.trim() || busy}
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
            onRecorded={(file) => processAudio(file)}
            onError={(message) => setError(message)}
          />
          {audio ? <span className="text-xs text-muted-foreground">{audio.name}</span> : null}
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
