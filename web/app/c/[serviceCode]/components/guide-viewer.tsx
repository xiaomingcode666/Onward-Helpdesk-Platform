"use client"

import { useState, useCallback, useRef } from "react"
import {
  ChevronLeftIcon,
  ChevronRightIcon,
  CheckCircle2Icon,
  SkipForwardIcon,
  CheckIcon,
  ImageIcon,
  PlayCircleIcon,
} from "lucide-react"

import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { useI18n } from "@/i18n/provider"

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface GuideStep {
  id: number
  order?: number
  title: string
  description: string
  imageUrl?: string
  videoUrl?: string
}

interface GuideViewerProps {
  /** Ordered list of guide steps */
  steps?: GuideStep[]
  /** Called when the user completes the entire guide */
  onComplete?: () => void
  /** Called when the user skips the guide */
  onSkip?: () => void
}

const DEFAULT_STEPS: GuideStep[] = []

// ---------------------------------------------------------------------------
// Progress bar
// ---------------------------------------------------------------------------

function StepProgressBar({
  current,
  total,
  t,
}: {
  current: number
  total: number
  t: ReturnType<typeof useI18n>
}) {
  const pct = total > 0 ? ((current + 1) / total) * 100 : 0

  return (
    <div className="space-y-1.5 px-4 pt-4">
      <div className="flex items-center justify-between text-xs text-muted-foreground">
        <span>
          {t("portalExtract.serviceCode.guideViewer.stepProgress", {
            current: current + 1,
            total,
          })}
        </span>
        <span>{Math.round(pct)}%</span>
      </div>
      <div className="h-2 w-full overflow-hidden rounded-full bg-muted">
        <div
          className="h-full rounded-full bg-primary transition-all duration-500 ease-out"
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Step dot indicator
// ---------------------------------------------------------------------------

function StepDots({ current, total }: { current: number; total: number }) {
  return (
    <div className="flex items-center justify-center gap-1.5 pb-2">
      {Array.from({ length: total }).map((_, i) => (
        <span
          key={i}
          className={`block rounded-full transition-all duration-300 ${
            i === current
              ? "h-2 w-6 bg-primary"
              : i < current
                ? "h-2 w-2 bg-primary/50"
                : "h-2 w-2 bg-muted-foreground/25"
          }`}
        />
      ))}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Media placeholder
// ---------------------------------------------------------------------------

function MediaPlaceholder({
  step,
  t,
}: {
  step: GuideStep
  t: ReturnType<typeof useI18n>
}) {
  if (step.videoUrl) {
    return (
      <div className="relative flex aspect-video items-center justify-center rounded-lg bg-muted">
        <PlayCircleIcon className="size-12 text-muted-foreground/50" />
        <p className="absolute bottom-2 left-2 text-rhd-2xs text-muted-foreground">
          {t("portalExtract.serviceCode.guideViewer.videoTutorial")}
        </p>
      </div>
    )
  }

  if (step.imageUrl) {
    return (
      <div className="relative aspect-video overflow-hidden rounded-lg bg-muted">
        {/* eslint-disable-next-line @next/next/no-img-element */}
        <img
          src={step.imageUrl}
          alt={step.title}
          className="size-full object-cover"
        />
      </div>
    )
  }

  // Placeholder for missing media
  return (
    <div className="flex aspect-video items-center justify-center rounded-lg bg-muted">
      <div className="flex flex-col items-center gap-1 text-muted-foreground">
        <ImageIcon className="size-8" />
        <p className="text-rhd-2xs">
          {t("portalExtract.serviceCode.guideViewer.operationImage")}
        </p>
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Main component
// ---------------------------------------------------------------------------

export function GuideViewer({
  steps = DEFAULT_STEPS,
  onComplete,
  onSkip,
}: GuideViewerProps) {
  const t = useI18n()
  const [currentStepIndex, setCurrentStepIndex] = useState(0)
  const [completedSteps, setCompletedSteps] = useState<Set<number>>(new Set())
  const [isCompleted, setIsCompleted] = useState(false)
  const touchStartX = useRef<number | null>(null)

  const currentStep = steps[currentStepIndex]
  const isFirst = currentStepIndex === 0
  const isLast = currentStepIndex === steps.length - 1
  const isStepCompleted = currentStep
    ? completedSteps.has(currentStep.id)
    : false

  // ---- Navigation ----
  const goNext = useCallback(() => {
    if (currentStepIndex < steps.length - 1) {
      setCurrentStepIndex((i) => i + 1)
    }
  }, [currentStepIndex, steps.length])

  const goPrev = useCallback(() => {
    if (currentStepIndex > 0) {
      setCurrentStepIndex((i) => i - 1)
    }
  }, [currentStepIndex])

  // ---- Mark current step complete ----
  const handleToggleComplete = useCallback(() => {
    if (!currentStep) return
    setCompletedSteps((prev) => {
      const next = new Set(prev)
      if (next.has(currentStep.id)) {
        next.delete(currentStep.id)
      } else {
        next.add(currentStep.id)
      }
      return next
    })
  }, [currentStep])

  // ---- Finish guide ----
  const handleFinish = useCallback(() => {
    // Mark all steps complete
    const all = new Set(steps.map((s) => s.id))
    setCompletedSteps(all)
    setIsCompleted(true)
    onComplete?.()
  }, [steps, onComplete])

  // ---- Skip ----
  const handleSkip = useCallback(() => {
    onSkip?.()
  }, [onSkip])

  // ---- Touch swipe ----
  const handleTouchStart = useCallback((e: React.TouchEvent) => {
    touchStartX.current = e.touches[0].clientX
  }, [])

  const handleTouchEnd = useCallback(
    (e: React.TouchEvent) => {
      if (touchStartX.current === null) return
      const diff = e.changedTouches[0].clientX - touchStartX.current
      const threshold = 60
      if (Math.abs(diff) > threshold) {
        if (diff < 0) goNext()
        else goPrev()
      }
      touchStartX.current = null
    },
    [goNext, goPrev]
  )

  // ---- Render ----

  if (!currentStep) {
    return (
      <Card className="mx-4 mt-4">
        <CardContent className="flex flex-col items-center gap-3 py-10">
          <CheckCircle2Icon className="size-12 text-muted-foreground" />
          <p className="text-base font-medium">
            {t("portalExtract.serviceCode.guideViewer.emptyTitle")}
          </p>
        </CardContent>
      </Card>
    )
  }

  if (isCompleted) {
    return (
      <Card className="mx-4 mt-4">
        <CardContent className="flex flex-col items-center gap-3 py-10">
          <CheckCircle2Icon className="size-12 text-primary" />
          <p className="text-base font-medium">
            {t("portalExtract.serviceCode.guideViewer.allCompleted")}
          </p>
          <p className="text-center text-xs text-muted-foreground">
            {t("portalExtract.serviceCode.guideViewer.allCompletedDesc")}
          </p>
          <Button type="button" variant="outline" onClick={handleSkip}>
            {t("portalExtract.serviceCode.guideViewer.close")}
          </Button>
        </CardContent>
      </Card>
    )
  }

  return (
    <div className="flex flex-col">
      {/* Progress bar */}
      <StepProgressBar current={currentStepIndex} total={steps.length} t={t} />
      <StepDots current={currentStepIndex} total={steps.length} />

      {/* Step content */}
      <div
        className="flex-1 px-4"
        onTouchStart={handleTouchStart}
        onTouchEnd={handleTouchEnd}
      >
        <Card>
          <CardContent className="space-y-3 p-4">
            {/* Media */}
            <MediaPlaceholder step={currentStep} t={t} />

            {/* Title */}
            <div className="flex items-start justify-between gap-2">
              <h3 className="text-base font-medium leading-snug">
                {currentStep.order ?? currentStepIndex + 1}. {currentStep.title}
              </h3>
              {isStepCompleted && (
                <Badge variant="default" className="shrink-0 text-rhd-2xs">
                  <CheckIcon className="mr-0.5 size-3" />
                  {t("portalExtract.serviceCode.guideViewer.done")}
                </Badge>
              )}
            </div>

            {/* Description */}
            <p className="text-sm leading-relaxed text-muted-foreground">
              {currentStep.description}
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Action buttons */}
      <div className="space-y-2 px-4 pb-4 pt-3">
        {/* Navigation row */}
        <div className="flex gap-2">
          <Button
            type="button"
            variant="outline"
            size="lg"
            className="flex-1"
            disabled={isFirst}
            onClick={goPrev}
          >
            <ChevronLeftIcon className="mr-1 size-4" />
            {t("portalExtract.serviceCode.guideViewer.prev")}
          </Button>

          {!isLast ? (
            <Button
              type="button"
              variant="outline"
              size="lg"
              className="flex-1"
              onClick={goNext}
            >
              {t("portalExtract.serviceCode.guideViewer.next")}
              <ChevronRightIcon className="ml-1 size-4" />
            </Button>
          ) : (
            <Button
              type="button"
              size="lg"
              className="flex-1"
              onClick={handleFinish}
            >
              <CheckCircle2Icon className="mr-1 size-4" />
              {t("portalExtract.serviceCode.guideViewer.complete")}
            </Button>
          )}
        </div>

        {/* Secondary row */}
        <div className="flex gap-2">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="flex-1 text-xs text-muted-foreground"
            onClick={handleToggleComplete}
          >
            {isStepCompleted ? (
              <>{t("portalExtract.serviceCode.guideViewer.unmark")}</>
            ) : (
              <>
                <CheckIcon className="mr-1 size-3" />
                {t("portalExtract.serviceCode.guideViewer.markDone")}
              </>
            )}
          </Button>

          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="flex-1 text-xs text-muted-foreground"
            onClick={handleSkip}
          >
            <SkipForwardIcon className="mr-1 size-3" />
            {t("portalExtract.serviceCode.guideViewer.skip")}
          </Button>
        </div>
      </div>
    </div>
  )
}
