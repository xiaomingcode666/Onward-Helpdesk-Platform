"use client"

import { translateCurrentMessage } from "@/i18n/messages"
import { useCallback, useEffect, useRef, useState } from "react"
import { CastIcon, CheckIcon, Loader2Icon, ScanSearchIcon, XIcon } from "lucide-react"

import { Input } from "antd"
import { IconButton } from "@railops/ui"

import { createMeetingAnnotation, detectMeetingFrame, fetchMeetingAnnotations } from "@/lib/api/meetings"
import type { MeetingARAnnotation } from "@/lib/api/types"
function ee(key: string, values?: Record<string, unknown>) {
  if (!values) {
    return translateCurrentMessage(`enterpriseExtract.${key}`)
  }
  return translateCurrentMessage(
    `enterpriseExtract.${key}`,
    Object.fromEntries(
      Object.entries(values).map(([name, value]) => [
        name,
        typeof value === "string" || typeof value === "number" ? value : String(value ?? ""),
      ]),
    ),
  )
}


type Bounds = { x: number; y: number; width: number; height: number }

type MeetingARWorkspaceProps = {
  meetingId: string
  frameAssetId: number
  frameDataUrl: string
  onApply?: (annotations: MeetingARAnnotation[]) => void
  onClose: () => void
}

export function MeetingARWorkspace({ meetingId, frameAssetId, frameDataUrl, onApply, onClose }: MeetingARWorkspaceProps) {
  const frameRef = useRef<HTMLDivElement>(null)
  const dragStartRef = useRef<{ x: number; y: number } | null>(null)
  const [draft, setDraft] = useState<Bounds | null>(null)
  const [label, setLabel] = useState(ee("meetingAr.text001"))
  const [annotations, setAnnotations] = useState<MeetingARAnnotation[]>([])
  const [saving, setSaving] = useState(false)
  const [detecting, setDetecting] = useState(false)
  const [error, setError] = useState("")

  const detect = useCallback(async () => {
    if (detecting) return
    setDetecting(true)
    setError("")
    const result = await detectMeetingFrame(meetingId, frameAssetId)
    if (result.success && result.data) {
      setAnnotations(result.data)
    } else {
      const existing = await fetchMeetingAnnotations(meetingId)
      if (existing.success && existing.data) {
        const frameAnnotations = existing.data.filter((item) => item.frameAssetId === frameAssetId)
        setAnnotations(frameAnnotations)
      }
      setError(result.error?.message || ee("meetingAr.text002"))
    }
    setDetecting(false)
  }, [detecting, frameAssetId, meetingId])

  useEffect(() => {
    const timer = window.setTimeout(() => void detect(), 0)
    return () => window.clearTimeout(timer)
    // Run once for each newly captured frame. The button below handles retries.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [frameAssetId, meetingId])

  const pointFromPointer = useCallback((clientX: number, clientY: number) => {
    const rect = frameRef.current?.getBoundingClientRect()
    if (!rect) return null
    return {
      x: Math.max(0, Math.min(1, (clientX - rect.left) / rect.width)),
      y: Math.max(0, Math.min(1, (clientY - rect.top) / rect.height)),
    }
  }, [])

  const handlePointerDown = (event: React.PointerEvent<HTMLDivElement>) => {
    const point = pointFromPointer(event.clientX, event.clientY)
    if (!point) return
    dragStartRef.current = point
    event.currentTarget.setPointerCapture(event.pointerId)
    setDraft({ x: point.x, y: point.y, width: 0, height: 0 })
  }

  const handlePointerMove = (event: React.PointerEvent<HTMLDivElement>) => {
    const start = dragStartRef.current
    const point = pointFromPointer(event.clientX, event.clientY)
    if (!start || !point) return
    setDraft({
      x: Math.min(start.x, point.x),
      y: Math.min(start.y, point.y),
      width: Math.abs(point.x - start.x),
      height: Math.abs(point.y - start.y),
    })
  }

  const handlePointerUp = () => {
    dragStartRef.current = null
    setDraft((current) => current && current.width >= 0.01 && current.height >= 0.01 ? current : null)
  }

  const save = async () => {
    if (!draft || !label.trim() || saving) return
    setSaving(true)
    setError("")
    const result = await createMeetingAnnotation(meetingId, {
      frameAssetId,
      detectionProvider: "manual",
      externalDetectionId: "",
      label: label.trim(),
      partCode: "",
      confidence: 1,
      bounds: draft,
      color: "#ef4444",
      note: "",
      metadataJson: "{}",
    })
    if (result.success && result.data) {
      setAnnotations((current) => [...current, result.data])
      setDraft(null)
    } else {
      setError(result.error?.message || ee("meetingAr.text003"))
    }
    setSaving(false)
  }

  return (
    <div className="absolute inset-0 z-40 flex flex-col bg-zinc-950 text-white">
      <div className="flex min-h-14 items-center gap-2 border-b border-white/15 px-3">
        <Input
          value={label}
          onChange={(event) => setLabel(event.target.value)}
          className="h-9 max-w-64 border-white/20 bg-white/10 text-white"
          aria-label={ee("meetingAr.text004")}
        />
        <IconButton icon={saving ? <Loader2Icon className="size-4 animate-spin" /> : <CheckIcon className="size-4" />} tooltip={ee("meetingAr.text005")} aria-label={ee("meetingAr.text005")} onClick={save} disabled={!draft || saving} />
		<IconButton icon={detecting ? <Loader2Icon className="size-4 animate-spin" /> : <ScanSearchIcon className="size-4" />} tooltip={ee("meetingAr.text006")} aria-label={ee("meetingAr.text006")} onClick={() => void detect()} disabled={detecting} />
		<IconButton icon={<CastIcon className="size-4" />} tooltip={ee("meetingAr.text007")} aria-label={ee("meetingAr.text007")} onClick={() => onApply?.(annotations)} disabled={annotations.length === 0} />
        {error ? <span className="text-xs text-red-300">{error}</span> : null}
        <IconButton variant="text" icon={<XIcon className="size-4" />} className="ml-auto text-white" tooltip={ee("meetingAr.text008")} aria-label={ee("meetingAr.text008")} onClick={onClose} />
      </div>
      <div className="flex min-h-0 flex-1 items-center justify-center overflow-hidden p-3">
        <div
          ref={frameRef}
          className="relative inline-block max-h-full max-w-full touch-none cursor-crosshair select-none"
          onPointerDown={handlePointerDown}
          onPointerMove={handlePointerMove}
          onPointerUp={handlePointerUp}
          onPointerCancel={handlePointerUp}
        >
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={frameDataUrl} alt={ee("meetingAr.text009")} className="block max-h-[calc(100vh-10rem)] max-w-full object-contain" draggable={false} />
          {annotations.map((annotation) => (
            <AnnotationBox key={annotation.id} bounds={annotation.bounds} label={annotation.label} color={annotation.color} />
          ))}
          {draft ? <AnnotationBox bounds={draft} label={label} color="#facc15" /> : null}
        </div>
      </div>
    </div>
  )
}

function AnnotationBox({ bounds, label, color }: { bounds: Bounds; label: string; color: string }) {
  return (
    <div
      className="pointer-events-none absolute border-2"
      style={{
        left: `${bounds.x * 100}%`, top: `${bounds.y * 100}%`,
        width: `${bounds.width * 100}%`, height: `${bounds.height * 100}%`, borderColor: color,
      }}
    >
      <span className="absolute left-0 top-0 max-w-48 -translate-y-full truncate px-1 py-0.5 text-xs text-white" style={{ backgroundColor: color }}>
        {label}
      </span>
    </div>
  )
}
