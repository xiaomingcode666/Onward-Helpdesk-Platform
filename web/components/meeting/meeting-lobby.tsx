"use client"

import { translateCurrentMessage } from "@/i18n/messages"
import { useCallback, useEffect, useRef, useState } from "react"

import { IconButton, RailopsButton } from "@railops/ui"
import { cn } from "@/lib/utils"
import {
  CameraIcon,
  CameraOffIcon,
  Loader2Icon,
  MicIcon,
  MicOffIcon,
  PhoneIcon,
  TriangleAlertIcon,
} from "lucide-react"
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


interface MeetingLobbyProps {
  onJoin: () => void | Promise<void>
  isJoining?: boolean
  joinRequirementText?: string
  className?: string
}

type MediaAccessIssueKind = "permission-denied" | "not-found" | "in-use" | "unsupported" | "unknown"

type MediaAccessIssue = {
  kind: MediaAccessIssueKind
  message: string
}

export function MeetingLobby({
  onJoin,
  isJoining = false,
  joinRequirementText = "",
  className,
}: MeetingLobbyProps) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const streamRef = useRef<MediaStream | null>(null)
  const mountedRef = useRef(true)
  const [stream, setStream] = useState<MediaStream | null>(null)
  const [audioEnabled, setAudioEnabled] = useState(false)
  const [videoEnabled, setVideoEnabled] = useState(false)
  const [cameraIssue, setCameraIssue] = useState<MediaAccessIssue | null>(null)
  const [micIssue, setMicIssue] = useState<MediaAccessIssue | null>(null)
  const [checkingMedia, setCheckingMedia] = useState(false)

  const replaceMedia = useCallback((mediaStream: MediaStream | null) => {
    streamRef.current?.getTracks().forEach((track) => track.stop())
    streamRef.current = mediaStream
    setStream(mediaStream)
    if (videoRef.current) {
      videoRef.current.srcObject = mediaStream
    }
  }, [])

  const resolveMediaIssue = useCallback((error: unknown, deviceName: string): MediaAccessIssue => {
    const errorName = error instanceof Error ? error.name : ""
    if (errorName === "NotAllowedError" || errorName === "PermissionDeniedError") {
      return { kind: "permission-denied", message: ee("meetingLobby.text001", { value0: deviceName }) }
    }
    if (errorName === "NotFoundError" || errorName === "DevicesNotFoundError") {
      return {
        kind: "not-found",
        message: deviceName === ee("meetingLobby.text002")
          ? ee("meetingLobby.text003")
          : ee("meetingLobby.text004"),
      }
    }
    if (errorName === "NotReadableError" || errorName === "TrackStartError") {
      return { kind: "in-use", message: ee("meetingLobby.text005", { value0: deviceName }) }
    }
    return { kind: "unknown", message: ee("meetingLobby.text006", { value0: deviceName }) }
  }, [])

  const startMedia = useCallback(async () => {
    setCheckingMedia(true)
    setCameraIssue(null)
    setMicIssue(null)

    try {
      if (!navigator.mediaDevices?.getUserMedia) {
        replaceMedia(null)
        setCameraIssue({ kind: "unsupported", message: ee("meetingLobby.text007") })
        setMicIssue({ kind: "unsupported", message: ee("meetingLobby.text008") })
        return
      }

      const [cameraResult, micResult] = await Promise.allSettled([
        navigator.mediaDevices.getUserMedia({ video: true, audio: false }),
        navigator.mediaDevices.getUserMedia({ video: false, audio: true }),
      ])
      const mediaStream = new MediaStream()
      const nextCameraIssue = cameraResult.status === "rejected"
        ? resolveMediaIssue(cameraResult.reason, ee("meetingLobby.text002"))
        : null
      const nextMicIssue = micResult.status === "rejected"
        ? resolveMediaIssue(micResult.reason, ee("meetingLobby.text009"))
        : null

      if (cameraResult.status === "fulfilled") {
        cameraResult.value.getVideoTracks().forEach((track) => mediaStream.addTrack(track))
      }
      if (micResult.status === "fulfilled") {
        micResult.value.getAudioTracks().forEach((track) => mediaStream.addTrack(track))
      }

      if (!mountedRef.current) {
        mediaStream.getTracks().forEach((track) => track.stop())
        return
      }
      const hasAudio = mediaStream.getAudioTracks().length > 0
      const hasVideo = mediaStream.getVideoTracks().length > 0
      setCameraIssue(nextCameraIssue)
      setMicIssue(nextMicIssue)
      setAudioEnabled(hasAudio)
      setVideoEnabled(hasVideo)
      replaceMedia(hasAudio || hasVideo ? mediaStream : null)
    } finally {
      if (mountedRef.current) {
        setCheckingMedia(false)
      }
    }
  }, [replaceMedia, resolveMediaIssue])

  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
      streamRef.current?.getTracks().forEach((track) => track.stop())
      streamRef.current = null
    }
  }, [])

  useEffect(() => {
    if (videoRef.current) {
      videoRef.current.srcObject = stream
    }
  }, [stream, videoEnabled])

  const toggleAudio = useCallback(() => {
    if (stream) {
      const audioTrack = stream.getAudioTracks()[0]
      if (audioTrack) {
        audioTrack.enabled = !audioTrack.enabled
        setAudioEnabled(audioTrack.enabled)
      }
    }
  }, [stream])

  const toggleVideo = useCallback(() => {
    if (stream) {
      const videoTrack = stream.getVideoTracks()[0]
      if (videoTrack) {
        videoTrack.enabled = !videoTrack.enabled
        setVideoEnabled(videoTrack.enabled)
      }
    }
  }, [stream])

  const retryPermissions = useCallback(() => {
    void startMedia()
  }, [startMedia])
  const hasAudioTrack = Boolean(stream?.getAudioTracks().length)
  const hasVideoTrack = Boolean(stream?.getVideoTracks().length)
  const mediaIssueKinds = new Set([cameraIssue?.kind, micIssue?.kind].filter(Boolean))
  const mediaIssueTitle = mediaIssueKinds.has("permission-denied")
    ? ee("meetingLobby.text010")
    : cameraIssue?.kind === "not-found" && !micIssue
      ? ee("meetingLobby.text011")
      : micIssue?.kind === "not-found" && !cameraIssue
        ? ee("meetingLobby.text012")
        : ee("meetingLobby.text013")
  const mediaIssueGuidance = mediaIssueKinds.has("permission-denied")
    ? ee("meetingLobby.text014")
    : mediaIssueKinds.has("in-use")
      ? ee("meetingLobby.text015")
      : mediaIssueKinds.has("not-found")
        ? ee("meetingLobby.text016")
        : mediaIssueKinds.has("unsupported")
          ? ee("meetingLobby.text017")
          : ee("meetingLobby.text018")

  return (
    <section className={cn("min-w-0", className)}>
      <div className="mb-3 flex items-center justify-between gap-3">
        <h3 className="text-sm font-semibold text-foreground">{ee("meetingLobby.text019")}</h3>
        <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
          <span className={cn("size-2 rounded-full", checkingMedia ? "bg-amber-500" : stream ? "bg-emerald-500" : "bg-muted-foreground/40")} />
          {checkingMedia ? ee("meetingLobby.text025") : stream ? ee("meetingLobby.text023") : ee("meetingLobby.text026")}
        </div>
      </div>

      <div className="space-y-3">
        <div className="relative aspect-video min-h-[220px] max-h-[360px] overflow-hidden rounded-lg bg-muted sm:min-h-[260px] xl:min-h-[240px] 2xl:min-h-[260px]">
          {stream && videoEnabled ? (
            <video
              ref={videoRef}
              autoPlay
              muted
              playsInline
              className="w-full h-full object-cover"
            />
          ) : stream ? (
            <div className="w-full h-full flex items-center justify-center text-muted-foreground">
              <CameraOffIcon className="w-12 h-12" />
            </div>
          ) : (
            <div className="absolute inset-0 flex flex-col items-center justify-center gap-4 px-6 pb-14 text-center text-foreground">
              <span className="grid size-14 place-items-center rounded-full bg-background/80"><CameraIcon className="size-6" /></span>
              <div><div className="font-medium">{ee("meetingLobby.text023")}</div><div className="mt-1 text-sm text-muted-foreground">{ee("meetingLobby.text024")}</div></div>
              <RailopsButton onClick={() => void startMedia()} disabled={checkingMedia}>
                {checkingMedia ? <Loader2Icon className="mr-2 size-4 animate-spin" /> : null}
                {checkingMedia ? ee("meetingLobby.text025") : ee("meetingLobby.text026")}
              </RailopsButton>
            </div>
          )}
          <div className="absolute bottom-3 left-1/2 flex -translate-x-1/2 gap-2 rounded-lg bg-black/55 p-1.5 backdrop-blur-sm">
            <IconButton
              danger={!(hasAudioTrack && audioEnabled)}
              icon={audioEnabled ? <MicIcon className="size-5" /> : <MicOffIcon className="size-5" />}
              onClick={toggleAudio}
              disabled={!hasAudioTrack}
              tooltip={!hasAudioTrack ? ee("meetingLobby.text027") : audioEnabled ? ee("meetingLobby.text028") : ee("meetingLobby.text029")}
              aria-label={!hasAudioTrack ? ee("meetingLobby.text027") : audioEnabled ? ee("meetingLobby.text028") : ee("meetingLobby.text029")}
            />
            <IconButton
              danger={!(hasVideoTrack && videoEnabled)}
              icon={videoEnabled ? <CameraIcon className="size-5" /> : <CameraOffIcon className="size-5" />}
              onClick={toggleVideo}
              disabled={!hasVideoTrack}
              tooltip={!hasVideoTrack ? ee("meetingLobby.text030") : videoEnabled ? ee("meetingLobby.text031") : ee("meetingLobby.text032")}
              aria-label={!hasVideoTrack ? ee("meetingLobby.text030") : videoEnabled ? ee("meetingLobby.text031") : ee("meetingLobby.text032")}
            />
          </div>
        </div>

        {(cameraIssue || micIssue) && (
          <div className="flex flex-wrap items-center gap-3 rounded-lg border border-amber-500/20 bg-amber-500/5 px-3 py-2.5 text-sm text-foreground">
            <TriangleAlertIcon className="mt-0.5 size-5 shrink-0 text-amber-500" />
            <div className="min-w-0 flex-1">
              <p className="font-medium">{mediaIssueTitle}</p>
              <p className="mt-0.5 text-xs leading-5 text-muted-foreground">{mediaIssueGuidance}</p>
            </div>
            <RailopsButton size="small" onClick={retryPermissions}>{ee("meetingLobby.text034")}</RailopsButton>
          </div>
        )}

        <div className="flex flex-col gap-3 border-t border-border pt-3 2xl:flex-row 2xl:items-center">
          {joinRequirementText ? (
            <p className="min-w-0 flex-1 text-xs leading-5 text-muted-foreground" role="note">
              {joinRequirementText}
            </p>
          ) : <span className="flex-1" />}
          <RailopsButton
            variant="primary"
            className="h-11 w-full shrink-0 font-semibold 2xl:w-auto 2xl:min-w-60"
            size="large"
            onClick={onJoin}
            disabled={isJoining}
          >
            {isJoining ? (
              <><Loader2Icon className="mr-2 size-4 animate-spin" />{ee("meetingLobby.text035")}</>
            ) : (
              <><PhoneIcon className="mr-2 size-4" />{ee("meetingLobby.text036")}</>
            )}
          </RailopsButton>
        </div>
      </div>
    </section>
  )
}
