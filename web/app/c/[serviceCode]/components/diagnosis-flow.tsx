"use client"

import { useState, useRef, useCallback } from "react"
import {
  ChevronRightIcon,
  ImagePlusIcon,
  WrenchIcon,
  CheckCircle2Icon,
  XCircleIcon,
  MessageCircleIcon,
  Loader2Icon,
  SearchIcon,
  FileTextIcon,
  ThumbsUpIcon,
  ThumbsDownIcon,
} from "lucide-react"

import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent } from "@/components/ui/card"
import { Textarea } from "@/components/ui/textarea"
import { Input } from "@/components/ui/input"
import { Separator } from "@/components/ui/separator"
import { useI18n } from "@/i18n/provider"

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type DiagnosisStep = "input" | "diagnosing" | "result" | "feedback"

type ProgressPhrase = "retrieving" | "analyzing" | "generating"

interface CauseItem {
  id: number
  description: string
  confidence: "high" | "medium" | "low"
}

interface ActionStep {
  id: number
  order: number
  instruction: string
}

interface DiagnosisResult {
  causes: CauseItem[]
  actions: ActionStep[]
  summary: string
}

interface DiagnosisFlowProps {
  /** Called when user confirms the issue is resolved */
  onResolved?: () => void
  /** Called when user requests human transfer */
  onTransferToHuman?: (feedback?: string) => void
  /** Called when the diagnosis process starts */
  onDiagnose?: (symptoms: string, faultCode?: string) => Promise<DiagnosisResult>
}

// ---------------------------------------------------------------------------
// Stepper component
// ---------------------------------------------------------------------------

const STEPS = [
  { key: "input", labelKey: "portalExtract.serviceCode.diagnosis.steps.input" },
  { key: "diagnosing", labelKey: "portalExtract.serviceCode.diagnosis.steps.diagnosing" },
  { key: "result", labelKey: "portalExtract.serviceCode.diagnosis.steps.result" },
  { key: "feedback", labelKey: "portalExtract.serviceCode.diagnosis.steps.feedback" },
] as const

function StepIndicator({
  current,
  t,
}: {
  current: number
  t: ReturnType<typeof useI18n>
}) {
  return (
    <div className="flex items-center justify-center gap-1 px-4 py-3">
      {STEPS.map((step, i) => {
        const isActive = i <= current
        const isCurrent = i === current
        return (
          <div key={step.key} className="flex items-center gap-1">
            <div
              className={`flex size-7 items-center justify-center rounded-full text-rhd-2xs font-medium transition-colors ${
                isActive
                  ? "bg-primary text-primary-foreground"
                  : "bg-muted text-muted-foreground"
              } ${isCurrent ? "ring-2 ring-primary/30" : ""}`}
            >
              {i + 1}
            </div>
            <span
              className={`hidden text-rhd-2xs sm:inline ${
                isCurrent ? "font-medium text-foreground" : "text-muted-foreground"
              }`}
            >
              {t(step.labelKey)}
            </span>
            {i < STEPS.length - 1 && (
              <ChevronRightIcon className="size-3 text-muted-foreground/40" />
            )}
          </div>
        )
      })}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Progress animation
// ---------------------------------------------------------------------------

const PROGRESS_PHRASES: Record<ProgressPhrase, { icon: typeof Loader2Icon; textKey: string }> = {
  retrieving: { icon: SearchIcon, textKey: "portalExtract.serviceCode.diagnosis.progress.retrieving" },
  analyzing: { icon: SearchIcon, textKey: "portalExtract.serviceCode.diagnosis.progress.analyzing" },
  generating: { icon: FileTextIcon, textKey: "portalExtract.serviceCode.diagnosis.progress.generating" },
}

function DiagnosingAnimation({
  phrase,
  t,
}: {
  phrase: ProgressPhrase
  t: ReturnType<typeof useI18n>
}) {
  const { icon: PhraseIcon, textKey } = PROGRESS_PHRASES[phrase]

  return (
    <div className="flex flex-col items-center gap-4 py-8">
      <div className="relative">
        <div className="absolute inset-0 animate-ping rounded-full bg-primary/20" />
        <div className="relative flex size-16 items-center justify-center rounded-full bg-primary/10">
          <PhraseIcon className="size-7 animate-pulse text-primary" />
        </div>
      </div>
      <div className="flex items-center gap-2">
        <Loader2Icon className="size-4 animate-spin text-muted-foreground" />
        <p className="text-sm text-muted-foreground">{t(textKey)}</p>
      </div>
      {/* Progress dots */}
      <div className="flex gap-1.5">
        {Array.from({ length: 3 }).map((_, i) => (
          <span
            key={i}
            className="block size-2 rounded-full bg-primary animate-bounce"
            style={{ animationDelay: `${i * 0.2}s` }}
          />
        ))}
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Main component
// ---------------------------------------------------------------------------

export function DiagnosisFlow({ onResolved, onTransferToHuman, onDiagnose }: DiagnosisFlowProps) {
  const t = useI18n()

  // ---- State ----
  const [step, setStep] = useState<DiagnosisStep>("input")
  const stepIndex = STEPS.findIndex((s) => s.key === step)

  const [symptoms, setSymptoms] = useState("")
  const [faultCode, setFaultCode] = useState("")
  const [images, setImages] = useState<string[]>([])
  const fileInputRef = useRef<HTMLInputElement>(null)

  const [progressPhrase, setProgressPhrase] = useState<ProgressPhrase>("retrieving")
  const [result, setResult] = useState<DiagnosisResult | null>(null)
  const [diagnosisError, setDiagnosisError] = useState("")

  const [feedback, setFeedback] = useState<"resolved" | "unresolved" | null>(null)
  const [feedbackText, setFeedbackText] = useState("")

  // ---- Image upload ----
  const handleImageUpload = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files
    if (!files) return
    Array.from(files).forEach((file) => {
      const reader = new FileReader()
      reader.onloadend = () => {
        setImages((prev) => [...prev, reader.result as string])
      }
      reader.readAsDataURL(file)
    })
    e.target.value = ""
  }, [])

  const removeImage = useCallback((index: number) => {
    setImages((prev) => prev.filter((_, i) => i !== index))
  }, [])

  // ---- Start diagnosis ----
  const handleStartDiagnosis = useCallback(async () => {
    if (!symptoms.trim()) return
    if (!onDiagnose) {
      setDiagnosisError(
        t("portalExtract.serviceCode.diagnosis.unavailable")
      )
      return
    }
    setStep("diagnosing")
    setProgressPhrase("retrieving")
    setDiagnosisError("")

    try {
      const diagnosisResult = await onDiagnose(symptoms, faultCode)
      setResult(diagnosisResult)
      setStep("result")
    } catch (error) {
      setDiagnosisError(
        error instanceof Error && error.message.trim()
          ? error.message
          : t("portalExtract.serviceCode.diagnosis.failed")
      )
      setResult(null)
      setStep("input")
    }
  }, [symptoms, faultCode, onDiagnose, t])

  // ---- Feedback handling ----
  const handleResolved = useCallback(() => {
    setFeedback("resolved")
    setStep("feedback")
    onResolved?.()
  }, [onResolved])

  const handleNotResolved = useCallback(() => {
    setFeedback("unresolved")
    setStep("feedback")
  }, [])

  const handleTransfer = useCallback(() => {
    onTransferToHuman?.(feedbackText)
  }, [onTransferToHuman, feedbackText])

  // ---- Confidence badge color ----
  const confidenceVariant = (c: string) => {
    if (c === "high") return "default"
    if (c === "medium") return "secondary"
    return "outline"
  }

  const confidenceLabel = (c: string) => {
    if (c === "high") return t("portalExtract.serviceCode.diagnosis.confidence.high")
    if (c === "medium") return t("portalExtract.serviceCode.diagnosis.confidence.medium")
    return t("portalExtract.serviceCode.diagnosis.confidence.low")
  }

  // ---- Render helpers ----

  const renderInput = () => (
    <div className="space-y-4">
      {/* Symptoms */}
      <div>
        <label className="mb-1.5 block text-sm font-medium">
          {t("portalExtract.serviceCode.diagnosis.symptoms")}
        </label>
        <Textarea
          value={symptoms}
          onChange={(e) => setSymptoms(e.target.value)}
          placeholder={t("portalExtract.serviceCode.diagnosis.symptomsPlaceholder")}
          rows={4}
          className="resize-none"
        />
      </div>

      {/* Fault code */}
      <div>
        <label className="mb-1.5 block text-sm font-medium">
          {t("portalExtract.serviceCode.diagnosis.faultCode")}
        </label>
        <Input
          value={faultCode}
          onChange={(e) => setFaultCode(e.target.value)}
          placeholder={t("portalExtract.serviceCode.diagnosis.faultCodePlaceholder")}
        />
      </div>

      {/* Image upload */}
      <div>
        <label className="mb-1.5 block text-sm font-medium">
          {t("portalExtract.serviceCode.diagnosis.images")}
        </label>
        <div className="flex flex-wrap gap-2">
          {images.map((img, i) => (
            <div key={i} className="relative size-16 overflow-hidden rounded-lg border">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src={img}
                alt={`Upload ${i}`}
                className="size-full object-cover"
              />
              <button
                type="button"
                onClick={() => removeImage(i)}
                className="absolute right-0 top-0 flex size-5 items-center justify-center
                           rounded-bl-lg bg-black/50 text-white text-rhd-2xs"
                aria-label={t("portalExtract.serviceCode.diagnosis.removeImage")}
                title={t("portalExtract.serviceCode.diagnosis.removeImage")}
              >
                &times;
              </button>
            </div>
          ))}
          <button
            type="button"
            onClick={() => fileInputRef.current?.click()}
            className="flex size-16 items-center justify-center rounded-lg border-2
                       border-dashed border-muted-foreground/30 text-muted-foreground
                       hover:border-primary/50 hover:text-primary transition-colors"
            aria-label={t("portalExtract.serviceCode.diagnosis.uploadImage")}
            title={t("portalExtract.serviceCode.diagnosis.uploadImage")}
          >
            <ImagePlusIcon className="size-6" />
          </button>
          <input
            ref={fileInputRef}
            type="file"
            accept="image/*"
            multiple
            className="hidden"
            onChange={handleImageUpload}
          />
        </div>
      </div>

      <Separator />

      {diagnosisError ? (
        <div className="rounded-md bg-destructive/10 px-3 py-2 text-sm text-destructive">
          {diagnosisError}
        </div>
      ) : null}

      <Button
        type="button"
        size="lg"
        className="w-full"
        disabled={!symptoms.trim()}
        onClick={handleStartDiagnosis}
      >
        <WrenchIcon className="mr-2 size-4" />
        {t("portalExtract.serviceCode.diagnosis.start")}
      </Button>
    </div>
  )

  const renderDiagnosing = () => <DiagnosingAnimation phrase={progressPhrase} t={t} />

  const renderResult = () => {
    if (!result) return null
    return (
      <div className="space-y-4">
        {/* Summary */}
        <Card>
          <CardContent className="p-4 text-sm leading-relaxed">{result.summary}</CardContent>
        </Card>

        {/* Possible causes */}
        <div>
          <h4 className="mb-2 flex items-center gap-1.5 text-sm font-medium">
            <SearchIcon className="size-4 text-primary" />
            {t("portalExtract.serviceCode.diagnosis.possibleCauses")}
          </h4>
          <div className="space-y-2">
            {result.causes.map((cause) => (
              <div
                key={cause.id}
                className="flex items-start gap-2 rounded-lg border p-3 text-sm"
              >
                <span className="mt-0.5 size-2 shrink-0 rounded-full bg-amber-400" />
                <span className="flex-1">{cause.description}</span>
                <Badge
                  variant={confidenceVariant(cause.confidence)}
                  className="shrink-0 text-rhd-2xs"
                >
                  {confidenceLabel(cause.confidence)}
                </Badge>
              </div>
            ))}
          </div>
        </div>

        {/* Action steps */}
        <div>
          <h4 className="mb-2 flex items-center gap-1.5 text-sm font-medium">
            <WrenchIcon className="size-4 text-primary" />
            {t("portalExtract.serviceCode.diagnosis.actionSteps")}
          </h4>
          <div className="space-y-2">
            {result.actions.map((action) => (
              <div
                key={action.id}
                className="flex items-start gap-3 rounded-lg border bg-muted/30 p-3 text-sm"
              >
                <span className="flex size-6 shrink-0 items-center justify-center rounded-full bg-primary text-rhd-xs font-medium text-primary-foreground">
                  {action.order}
                </span>
                <span className="pt-0.5">{action.instruction}</span>
              </div>
            ))}
          </div>
        </div>

        <Separator />

        {/* Resolve / Transfer */}
        <div className="space-y-2">
          <Button
            type="button"
            size="lg"
            className="w-full"
            onClick={handleResolved}
          >
            <CheckCircle2Icon className="mr-2 size-4" />
            {t("portalExtract.serviceCode.diagnosis.resolved")}
          </Button>
          <Button
            type="button"
            variant="outline"
            size="lg"
            className="w-full"
            onClick={handleNotResolved}
          >
            <XCircleIcon className="mr-2 size-4" />
            {t("portalExtract.serviceCode.diagnosis.notResolved")}
          </Button>
        </div>
      </div>
    )
  }

  const renderFeedback = () => (
    <div className="space-y-4">
      {feedback === "resolved" ? (
        <Card>
          <CardContent className="flex flex-col items-center gap-3 py-8">
            <ThumbsUpIcon className="size-10 text-primary" />
            <p className="text-center text-sm font-medium text-primary">
              {t("portalExtract.serviceCode.diagnosis.feedbackThanks")}
            </p>
            <p className="text-center text-xs text-muted-foreground">
              {t("portalExtract.serviceCode.diagnosis.feedbackHelped")}
            </p>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          <Card>
            <CardContent className="flex flex-col items-center gap-3 py-8">
              <ThumbsDownIcon className="size-10 text-destructive" />
              <p className="text-center text-sm font-medium text-destructive">
                {t("portalExtract.serviceCode.diagnosis.feedbackUnresolved")}
              </p>
              <p className="text-center text-xs text-muted-foreground">
                {t("portalExtract.serviceCode.diagnosis.feedbackDescribe")}
              </p>
            </CardContent>
          </Card>

          <Textarea
            value={feedbackText}
            onChange={(e) => setFeedbackText(e.target.value)}
            placeholder={t("portalExtract.serviceCode.diagnosis.feedbackPlaceholder")}
            rows={3}
            className="resize-none"
          />

          <div className="flex gap-2">
            <Button
              type="button"
              variant="outline"
              size="lg"
              className="flex-1"
              onClick={() => setStep("result")}
            >
              {t("portalExtract.serviceCode.diagnosis.back")}
            </Button>
            <Button
              type="button"
              size="lg"
              className="flex-1"
              onClick={handleTransfer}
            >
              <MessageCircleIcon className="mr-2 size-4" />
              {t("portalExtract.serviceCode.diagnosis.transfer")}
            </Button>
          </div>
        </div>
      )}
    </div>
  )

  // ---- Main render ----

  return (
    <div className="flex flex-col">
      <StepIndicator current={stepIndex} t={t} />
      <div className="px-4 pb-4">
        {step === "input" && renderInput()}
        {step === "diagnosing" && renderDiagnosing()}
        {step === "result" && renderResult()}
        {step === "feedback" && renderFeedback()}
      </div>
    </div>
  )
}
