"use client"

import { useState, useRef, useEffect, useCallback } from "react"
import {
  MicIcon,
  MicOffIcon,
  SendHorizonalIcon,
  SquareIcon,
  Loader2Icon,
} from "lucide-react"

import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { useAppLocale } from "@/i18n/provider"

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type VoiceState = "idle" | "recording" | "processing" | "result"

type BrowserSpeechRecognitionEvent = Event & {
  resultIndex: number
  results: SpeechRecognitionResultList
}

type BrowserSpeechRecognition = EventTarget & {
  lang: string
  continuous: boolean
  interimResults: boolean
  onresult: ((event: BrowserSpeechRecognitionEvent) => void) | null
  onerror: (() => void) | null
  onend: (() => void) | null
  start: () => void
  stop: () => void
}

type BrowserSpeechRecognitionConstructor = new () => BrowserSpeechRecognition

type BrowserWindowWithSpeechRecognition = Window & {
  SpeechRecognition?: BrowserSpeechRecognitionConstructor
  webkitSpeechRecognition?: BrowserSpeechRecognitionConstructor
}

interface VoiceInputProps {
  /** Optional callback when a transcript is ready to be sent */
  onSend?: (text: string) => void
  /** Optional callback when recording is cancelled */
  onCancel?: () => void
}

// ---------------------------------------------------------------------------
// Waveform bar component
// ---------------------------------------------------------------------------

const BAR_COUNT = 5

function WaveformBars({ active }: { active: boolean }) {
  return (
    <div className="flex items-end justify-center gap-[3px]">
      {Array.from({ length: BAR_COUNT }).map((_, i) => (
        <span
          key={i}
          className="block w-[3px] rounded-full bg-primary transition-all duration-150"
          style={{
            height: active ? `${12 + (i % 3) * 4}px` : "4px",
            animation: active
              ? `voice-wave-${i} 0.6s ease-in-out infinite alternate`
              : "none",
          }}
        />
      ))}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Main component
// ---------------------------------------------------------------------------

export function VoiceInput({ onSend, onCancel }: VoiceInputProps) {
  const { locale, t } = useAppLocale()
  const [state, setState] = useState<VoiceState>("idle")
  const [transcript, setTranscript] = useState("")
  const [interimTranscript, setInterimTranscript] = useState("")
  const [unsupportedMessage, setUnsupportedMessage] = useState("")
  const recognitionRef = useRef<BrowserSpeechRecognition | null>(null)
  const holdTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const [isHolding, setIsHolding] = useState(false)

  // ------ Speech recognition setup ------

  const startRecording = useCallback(() => {
    const speechWindow = window as BrowserWindowWithSpeechRecognition
    if (!speechWindow.webkitSpeechRecognition && !speechWindow.SpeechRecognition) {
      setUnsupportedMessage(
        t("portalExtract.serviceCode.voiceInput.unsupported")
      )
      setState("idle")
      return
    }
    setUnsupportedMessage("")

    const SpeechRecognitionAPI =
      speechWindow.SpeechRecognition ?? speechWindow.webkitSpeechRecognition
    if (!SpeechRecognitionAPI) {
      return
    }
    const recognition = new SpeechRecognitionAPI()
    recognition.lang = locale
    recognition.continuous = true
    recognition.interimResults = true

    recognition.onresult = (event: BrowserSpeechRecognitionEvent) => {
      let final = ""
      let interim = ""
      for (let i = event.resultIndex; i < event.results.length; i++) {
        const result = event.results[i]
        if (result.isFinal) {
          final += result[0].transcript
        } else {
          interim += result[0].transcript
        }
      }
      setTranscript((prev) => prev + final)
      setInterimTranscript(interim)
    }

    recognition.onerror = () => {
      setState("idle")
    }

    recognition.onend = () => {
      if (state === "recording") {
        setState("result")
      }
    }

    recognitionRef.current = recognition
    recognition.start()
    setState("recording")
  }, [locale, state, t])

  const stopRecording = useCallback(() => {
    if (recognitionRef.current) {
      recognitionRef.current.stop()
      recognitionRef.current = null
    }
    if (holdTimerRef.current) {
      clearTimeout(holdTimerRef.current)
      holdTimerRef.current = null
    }
    setIsHolding(false)
    if (transcript || interimTranscript) {
      setTranscript((prev) => prev + interimTranscript)
      setInterimTranscript("")
      setState("result")
    } else {
      setState("idle")
    }
  }, [transcript, interimTranscript])

  const cancelRecording = useCallback(() => {
    if (recognitionRef.current) {
      recognitionRef.current.stop()
      recognitionRef.current = null
    }
    setTranscript("")
    setInterimTranscript("")
    setState("idle")
    onCancel?.()
  }, [onCancel])

  const handleSend = useCallback(() => {
    if (transcript.trim()) {
      onSend?.(transcript.trim())
    }
    setTranscript("")
    setInterimTranscript("")
    setState("idle")
  }, [transcript, onSend])

  const handleRetry = useCallback(() => {
    setTranscript("")
    setInterimTranscript("")
    setState("idle")
  }, [])

  // ------ Hold-to-talk handlers ------

  const handlePointerDown = useCallback(() => {
    holdTimerRef.current = setTimeout(() => {
      setIsHolding(true)
      startRecording()
    }, 150) // small threshold to avoid accidental triggers
  }, [startRecording])

  const handlePointerUp = useCallback(() => {
    if (holdTimerRef.current) {
      clearTimeout(holdTimerRef.current)
      holdTimerRef.current = null
    }
    if (isHolding && state === "recording") {
      stopRecording()
    }
    setIsHolding(false)
  }, [isHolding, state, stopRecording])

  // ------ Cleanup ------

  useEffect(() => {
    return () => {
      if (recognitionRef.current) {
        recognitionRef.current.stop()
      }
      if (holdTimerRef.current) {
        clearTimeout(holdTimerRef.current)
      }
    }
  }, [])

  // ------ Inject keyframes once ------

  useEffect(() => {
    if (typeof document === "undefined") return
    const existing = document.getElementById("voice-input-keyframes")
    if (existing) return
    const style = document.createElement("style")
    style.id = "voice-input-keyframes"
    style.textContent = Array.from({ length: BAR_COUNT })
      .map(
        (_, i) =>
          `@keyframes voice-wave-${i} {
            0%   { height: 8px; }
            100% { height: ${20 + i * 6}px; }
          }`
      )
      .join("\n")
    document.head.appendChild(style)
    return () => {
      document.head.removeChild(style)
    }
  }, [])

  // ------ Derived display text ------

  const displayText = transcript + interimTranscript

  // ------ Render ------

  return (
    <Card className="overflow-hidden border-0 shadow-lg">
      <CardContent className="flex flex-col items-center gap-4 p-6">
        {/* ---- State indicator ---- */}
        {state === "idle" && (
          <p className="text-center text-sm text-muted-foreground">
            {t("portalExtract.serviceCode.voiceInput.tapToRecord")}
          </p>
        )}
        {unsupportedMessage ? (
          <p className="rounded-md bg-muted px-3 py-2 text-center text-sm text-muted-foreground">
            {unsupportedMessage}
          </p>
        ) : null}

        {state === "recording" && (
          <div className="flex flex-col items-center gap-2">
            <WaveformBars active />
            <p className="text-xs text-destructive animate-pulse">
              {t("portalExtract.serviceCode.voiceInput.recording")}
            </p>
          </div>
        )}

        {state === "processing" && (
          <div className="flex flex-col items-center gap-2">
            <Loader2Icon className="size-6 animate-spin text-primary" />
            <p className="text-xs text-muted-foreground">
              {t("portalExtract.serviceCode.voiceInput.processing")}
            </p>
          </div>
        )}

        {state === "result" && (
          <div className="flex flex-col items-center gap-2">
            <MicIcon className="size-6 text-primary" />
            <p className="text-xs text-primary">
              {t("portalExtract.serviceCode.voiceInput.recognized")}
            </p>
          </div>
        )}

        {/* ---- Transcript display ---- */}
        {(displayText || state === "result") && (
          <div className="w-full rounded-lg bg-muted p-3 text-sm leading-relaxed">
            {displayText || transcript}
            {interimTranscript && (
              <span className="text-muted-foreground">{interimTranscript}</span>
            )}
            {!displayText && state === "result" && (
              <span className="italic text-muted-foreground">
                {t("portalExtract.serviceCode.voiceInput.noSpeech")}
              </span>
            )}
          </div>
        )}

        {/* ---- Record button ---- */}
        <div className="flex items-center gap-4">
          {state === "idle" && (
            <button
              type="button"
              onPointerDown={handlePointerDown}
              onPointerUp={handlePointerUp}
              onPointerLeave={handlePointerUp}
              className="flex size-20 items-center justify-center rounded-full
                         bg-primary text-primary-foreground shadow-lg
                         active:scale-95 active:bg-primary/90
                         transition-transform touch-none select-none"
              aria-label={t("portalExtract.serviceCode.voiceInput.startRecording")}
            >
              <MicIcon className="size-8" />
            </button>
          )}

          {state === "recording" && (
            <button
              type="button"
              onClick={cancelRecording}
              className="flex size-20 items-center justify-center rounded-full
                         bg-destructive text-destructive-foreground shadow-lg
                         active:scale-95 transition-transform"
              aria-label={t("portalExtract.serviceCode.voiceInput.cancel")}
            >
              <SquareIcon className="size-6" />
            </button>
          )}

          {state === "result" && (
            <>
              <Button
                type="button"
                variant="outline"
                size="lg"
                onClick={handleRetry}
                className="rounded-full"
              >
                <MicOffIcon className="mr-2 size-4" />
                {t("portalExtract.serviceCode.voiceInput.retry")}
              </Button>

              <Button
                type="button"
                size="lg"
                onClick={handleSend}
                disabled={!transcript.trim()}
                className="rounded-full"
              >
                <SendHorizonalIcon className="mr-2 size-4" />
                {t("portalExtract.serviceCode.voiceInput.send")}
              </Button>
            </>
          )}
        </div>

        {/* ---- Hint text ---- */}
        {state === "idle" && (
          <p className="text-rhd-2xs text-muted-foreground">
            {t("portalExtract.serviceCode.voiceInput.tapHint")}
          </p>
        )}
      </CardContent>
    </Card>
  )
}
