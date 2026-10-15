"use client"

import { useEffect, useRef, useState } from "react"
import { FileAudioIcon, Loader2Icon, UploadIcon } from "lucide-react"
import { Input } from "antd"
import { RailopsButton, StandardModal } from "@railops/ui"
import { toast } from "sonner"

import { VoiceRecorderButton } from "@/components/chat/voice-recorder-button"
import { useI18n } from "@/i18n/provider"
import {
  sendAgentMessage,
  transcribeAgentConversationAudio,
  uploadAgentConversationAudio,
  type AgentConversation,
} from "@/lib/api/agent"
import { createTicket } from "@/lib/api/enterprise-tickets"
import { convertAudioToPCM16Wav } from "@/lib/audio/pcm-wav"
import type { TicketListItem } from "@/lib/api/types"

type VoiceTicketDialogProps = {
  open: boolean
  conversation: AgentConversation | null
  currentAssigneeId?: number
  onOpenChange: (open: boolean) => void
  onCreated?: (ticket: TicketListItem) => void | Promise<void>
}

export function VoiceTicketDialog({ open, conversation, currentAssigneeId, onOpenChange, onCreated }: VoiceTicketDialogProps) {
  const t = useI18n()
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const [file, setFile] = useState<File | null>(null)
  const [durationSeconds, setDurationSeconds] = useState(0)
  const [transcript, setTranscript] = useState("")
  const [title, setTitle] = useState("")
  const [transcribing, setTranscribing] = useState(false)
  const [creating, setCreating] = useState(false)
  const [uploadedAssetId, setUploadedAssetId] = useState("")
  const [createKey, setCreateKey] = useState("")

  useEffect(() => {
    if (!open) return
    setFile(null)
    setDurationSeconds(0)
    setTranscript("")
    setTitle("")
    setTranscribing(false)
    setCreating(false)
    setUploadedAssetId("")
    setCreateKey(crypto.randomUUID())
  }, [open, conversation?.id])

  async function transcribe(fileToProcess: File, duration = durationSeconds) {
    if (!conversation) return
    setFile(fileToProcess)
    setDurationSeconds(duration)
    setTranscribing(true)
    try {
      const wav = await convertAudioToPCM16Wav(fileToProcess)
      const result = await transcribeAgentConversationAudio(conversation.id, wav)
      setTranscript(result.text)
      setTitle((current) => current.trim() || result.text.trim().slice(0, 80))
      setDurationSeconds(result.durationSeconds || duration)
    } catch (error) {
      setTranscript("")
      toast.error(error instanceof Error ? error.message : t("voiceTicket.transcribeFailed"))
    } finally {
      setTranscribing(false)
    }
  }

  async function createVoiceTicket() {
    if (!conversation || !file || !transcript.trim() || !title.trim() || creating) return
    setCreating(true)
    try {
      let assetId = uploadedAssetId
      let filename = file.name
      if (!assetId) {
        const asset = await uploadAgentConversationAudio(conversation.id, file)
        assetId = asset.assetId
        filename = asset.filename
        await sendAgentMessage({
          conversationId: conversation.id,
          messageType: "audio",
          content: filename,
          payload: JSON.stringify({ assetId, durationSeconds }),
          clientMsgId: `voice_ticket_${crypto.randomUUID()}`,
        })
        setUploadedAssetId(assetId)
      }

      const result = await createTicket({
        source: "conversation",
        channel: "im",
        conversation_id: conversation.id,
        customer_id: conversation.customerId || undefined,
        current_assignee_id: currentAssigneeId || undefined,
        title: title.trim(),
        description: transcript.trim(),
        priority: "medium",
        idempotency_key: createKey,
      })
      if (!result.success || !result.data) throw new Error(result.error?.message || t("voiceTicket.createFailed"))
      await onCreated?.(result.data)
      toast.success(t("voiceTicket.created"))
      onOpenChange(false)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t("voiceTicket.createFailed"))
    } finally {
      setCreating(false)
    }
  }

  return (
    <StandardModal
      open={open}
      destroyOnHidden
      centered
      width={620}
      title={t("voiceTicket.title")}
      onCancel={() => { if (!creating && !transcribing) onOpenChange(false) }}
      footer={(
        <>
          <RailopsButton onClick={() => onOpenChange(false)} disabled={creating || transcribing}>{t("voiceTicket.cancel")}</RailopsButton>
          <RailopsButton variant="primary" onClick={() => void createVoiceTicket()} disabled={!conversation || !file || !transcript.trim() || !title.trim() || creating || transcribing}>
            {creating ? <Loader2Icon className="size-4 animate-spin" /> : <UploadIcon className="size-4" />}
            {creating ? t("voiceTicket.creating") : t("voiceTicket.create")}
          </RailopsButton>
        </>
      )}
    >
      <div className="grid gap-4">
        <div className="flex flex-wrap items-center gap-2">
          <input ref={fileInputRef} type="file" accept="audio/*" className="hidden" onChange={(event) => { const selected = event.target.files?.[0]; event.target.value = ""; if (selected) void transcribe(selected) }} />
          <RailopsButton onClick={() => fileInputRef.current?.click()} disabled={transcribing || creating}><FileAudioIcon className="size-4" /> {t("voiceTicket.chooseFile")}</RailopsButton>
          <VoiceRecorderButton disabled={transcribing || creating} className="border border-border" onRecorded={(recordedFile, seconds) => transcribe(recordedFile, seconds)} onError={(message) => toast.error(message)} />
          {file ? <span className="text-xs text-muted-foreground">{file.name}</span> : null}
        </div>
        {transcribing ? <div className="flex items-center gap-2 text-sm text-muted-foreground"><Loader2Icon className="size-4 animate-spin" /> {t("voiceTicket.transcribing")}</div> : null}
        <label className="grid gap-1.5 text-sm font-medium text-foreground">{t("voiceTicket.ticketTitle")}<Input value={title} maxLength={200} onChange={(event) => setTitle(event.target.value)} placeholder={t("voiceTicket.ticketTitlePlaceholder")} /></label>
        <label className="grid gap-1.5 text-sm font-medium text-foreground">{t("voiceTicket.transcript")}<Input.TextArea value={transcript} rows={6} maxLength={4000} showCount onChange={(event) => setTranscript(event.target.value)} placeholder={t("voiceTicket.transcriptPlaceholder")} /></label>
        <p className="text-xs text-muted-foreground">{t("voiceTicket.recordingHint")}</p>
      </div>
    </StandardModal>
  )
}
