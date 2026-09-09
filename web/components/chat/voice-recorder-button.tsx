"use client"

import { useEffect, useRef, useState } from "react"
import { Loader2Icon, MicIcon, SquareIcon, Trash2Icon } from "lucide-react"

import { Button } from "@/components/ui/button"
import { useI18n } from "@/i18n/provider"
import { cn } from "@/lib/utils"

const MAX_RECORDING_SECONDS = 120

type VoiceRecorderButtonProps = {
  disabled?: boolean
  className?: string
  holdToRecord?: boolean
  onRecorded: (file: File, durationSeconds: number) => Promise<void>
  onError?: (message: string) => void
}

type RecorderState = "idle" | "requesting" | "recording" | "sending"

export function VoiceRecorderButton({
  disabled = false,
  className,
  holdToRecord = false,
  onRecorded,
  onError,
}: VoiceRecorderButtonProps) {
  const t = useI18n()
  const [state, setState] = useState<RecorderState>("idle")
  const [elapsedSeconds, setElapsedSeconds] = useState(0)
  const recorderRef = useRef<MediaRecorder | null>(null)
  const streamRef = useRef<MediaStream | null>(null)
  const chunksRef = useRef<Blob[]>([])
  const startedAtRef = useRef(0)
  const cancelledRef = useRef(false)
  const stopAfterStartRef = useRef(false)
  const holdStartTimerRef = useRef<number | null>(null)
  const holdActiveRef = useRef(false)
  const timerRef = useRef<number | null>(null)
  const mountedRef = useRef(true)

  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
      cancelledRef.current = true
      clearHoldStartTimer()
      stopTimer()
      const recorder = recorderRef.current
      if (recorder && recorder.state !== "inactive") recorder.stop()
      stopStream()
    }
  }, [])

  function stopTimer() {
    if (timerRef.current !== null) {
      window.clearInterval(timerRef.current)
      timerRef.current = null
    }
  }

  function clearHoldStartTimer() {
    if (holdStartTimerRef.current !== null) {
      window.clearTimeout(holdStartTimerRef.current)
      holdStartTimerRef.current = null
    }
  }

  function stopStream() {
    streamRef.current?.getTracks().forEach((track) => track.stop())
    streamRef.current = null
  }

  function reportError(message: string) {
    onError?.(message)
  }

  async function startRecording() {
    if (disabled || state !== "idle") return
    if (!navigator.mediaDevices?.getUserMedia || typeof MediaRecorder === "undefined") {
      reportError(t("conversation.voiceUnsupported"))
      return
    }
    setState("requesting")
    cancelledRef.current = false
    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        audio: {
          echoCancellation: true,
          noiseSuppression: true,
          autoGainControl: true,
        },
      })
      if (!mountedRef.current) {
        stream.getTracks().forEach((track) => track.stop())
        return
      }
      streamRef.current = stream
      const mimeType = preferredAudioMimeType()
      const recorder = new MediaRecorder(stream, mimeType ? { mimeType } : undefined)
      recorderRef.current = recorder
      chunksRef.current = []
      recorder.ondataavailable = (event) => {
        if (event.data.size > 0) chunksRef.current.push(event.data)
      }
      recorder.onerror = () => {
        reportError(t("conversation.voiceRecordFailed"))
      }
      recorder.onstop = () => void finishRecording(recorder.mimeType || mimeType)
      recorder.start(250)
      startedAtRef.current = Date.now()
      setElapsedSeconds(0)
      setState("recording")
      if (stopAfterStartRef.current) {
        stopAfterStartRef.current = false
        window.setTimeout(stopAndSend, 0)
      }
      timerRef.current = window.setInterval(() => {
        const elapsed = Math.min(
          MAX_RECORDING_SECONDS,
          Math.max(0, Math.floor((Date.now() - startedAtRef.current) / 1000)),
        )
        setElapsedSeconds(elapsed)
        if (elapsed >= MAX_RECORDING_SECONDS) stopAndSend()
      }, 250)
    } catch {
      stopStream()
      if (mountedRef.current) setState("idle")
      reportError(t("conversation.microphoneDenied"))
    }
  }

  function stopAndSend() {
    const recorder = recorderRef.current
    if (!recorder || recorder.state === "inactive") {
      stopAfterStartRef.current = true
      return
    }
    cancelledRef.current = false
    stopTimer()
    recorder.stop()
  }

  function cancelRecording() {
    cancelledRef.current = true
    stopAfterStartRef.current = false
    holdActiveRef.current = false
    clearHoldStartTimer()
    stopTimer()
    const recorder = recorderRef.current
    if (recorder && recorder.state !== "inactive") recorder.stop()
    else {
      stopStream()
      if (mountedRef.current) setState("idle")
    }
  }

  async function finishRecording(mimeType: string) {
    stopTimer()
    stopStream()
    recorderRef.current = null
    const cancelled = cancelledRef.current
    cancelledRef.current = false
    const durationSeconds = Math.max(1, Math.ceil((Date.now() - startedAtRef.current) / 1000))
    const chunks = chunksRef.current
    chunksRef.current = []
    if (cancelled || !mountedRef.current) {
      if (mountedRef.current) setState("idle")
      return
    }
    const normalizedMimeType = mimeType || chunks[0]?.type || "audio/webm"
    const blob = new Blob(chunks, { type: normalizedMimeType })
    if (blob.size === 0) {
      setState("idle")
      reportError(t("conversation.voiceEmpty"))
      return
    }
    const extension = audioFileExtension(normalizedMimeType)
    const file = new File([blob], `voice-${Date.now()}.${extension}`, {
      type: normalizedMimeType,
      lastModified: Date.now(),
    })
    setState("sending")
    try {
      await onRecorded(file, Math.min(durationSeconds, MAX_RECORDING_SECONDS))
    } catch {
      reportError(t("conversation.sendVoiceFailed"))
    } finally {
      if (mountedRef.current) {
        setElapsedSeconds(0)
        setState("idle")
      }
    }
  }

  if (state === "recording") {
    return (
      <div className="flex h-9 min-w-[8.25rem] shrink-0 items-center gap-1 rounded-full border border-destructive/20 bg-destructive/10 px-1.5">
        <span className="ml-1 size-2 animate-pulse rounded-full bg-destructive" aria-hidden="true" />
        <span className="w-10 text-center font-mono text-xs tabular-nums text-destructive" aria-live="polite">
          {formatVoiceDuration(elapsedSeconds)}
        </span>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="size-7 text-destructive hover:bg-destructive/10 hover:text-destructive"
          onClick={stopAndSend}
          aria-label={t("conversation.stopAndSendVoice")}
          title={t("conversation.stopAndSendVoice")}
        >
          <SquareIcon className="size-3.5 fill-current" />
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="size-7 text-slate-500"
          onClick={cancelRecording}
          aria-label={t("conversation.cancelVoice")}
          title={t("conversation.cancelVoice")}
        >
          <Trash2Icon className="size-3.5" />
        </Button>
      </div>
    )
  }

  return (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      className={cn("size-8", className)}
      onMouseDown={(event) => event.preventDefault()}
      onPointerDown={holdToRecord ? (event) => {
        if (disabled || state !== "idle") return
        event.preventDefault()
        event.currentTarget.setPointerCapture?.(event.pointerId)
        holdActiveRef.current = false
        clearHoldStartTimer()
        holdStartTimerRef.current = window.setTimeout(() => {
          holdStartTimerRef.current = null
          holdActiveRef.current = true
          void startRecording()
        }, 260)
      } : undefined}
      onPointerUp={holdToRecord ? (event) => {
        event.preventDefault()
        clearHoldStartTimer()
        if (holdActiveRef.current) {
          holdActiveRef.current = false
          stopAndSend()
          return
        }
        void startRecording()
      } : undefined}
      onPointerCancel={holdToRecord ? () => {
        const wasHolding = holdActiveRef.current
        holdActiveRef.current = false
        clearHoldStartTimer()
        if (wasHolding) cancelRecording()
      } : undefined}
      onPointerLeave={holdToRecord ? () => {
        if (state === "requesting" && holdActiveRef.current) cancelRecording()
      } : undefined}
      onContextMenu={holdToRecord ? (event) => event.preventDefault() : undefined}
      onClick={holdToRecord ? undefined : () => void startRecording()}
      disabled={disabled || state !== "idle"}
      aria-label={state === "sending" ? t("conversation.voiceSending") : t("conversation.recordVoice")}
      title={state === "sending" ? t("conversation.voiceSending") : t("conversation.recordVoice")}
    >
      {state === "requesting" || state === "sending" ? (
        <Loader2Icon className="size-4 animate-spin" />
      ) : (
        <MicIcon className="size-4" />
      )}
    </Button>
  )
}

export function preferredAudioMimeType() {
  if (typeof MediaRecorder === "undefined" || typeof MediaRecorder.isTypeSupported !== "function") return ""
  return ["audio/webm;codecs=opus", "audio/mp4", "audio/ogg;codecs=opus", "audio/webm"].find(
    (mimeType) => MediaRecorder.isTypeSupported(mimeType),
  ) ?? ""
}

export function audioFileExtension(mimeType: string) {
  const normalized = mimeType.toLowerCase()
  if (normalized.includes("mp4")) return "m4a"
  if (normalized.includes("ogg")) return "ogg"
  return "webm"
}

export function formatVoiceDuration(seconds: number) {
  const safeSeconds = Math.max(0, Math.min(MAX_RECORDING_SECONDS, Math.floor(seconds)))
  return `${String(Math.floor(safeSeconds / 60)).padStart(2, "0")}:${String(safeSeconds % 60).padStart(2, "0")}`
}
