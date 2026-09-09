"use client"

import { useCallback, useEffect, useState } from "react"
import { useParams, useRouter, useSearchParams } from "next/navigation"
import { ArrowLeftIcon, Loader2Icon, RefreshCwIcon } from "lucide-react"

import { RailopsButton } from "@railops/ui"

import { MeetingLiveRoom } from "@/components/meeting/meeting-live-room"
import {
	getMeetingDisplaySubject,
	getMeetingReturnPath,
	getMeetingTicketId,
} from "@/components/meeting/meeting-room-context"
import { useI18n } from "@/i18n/provider"
import { readSession } from "@/lib/auth"
import { isStaticExportRouteParam } from "@/lib/static-export-route"
import {
  confirmMeetingJoined,
  confirmMeetingLeft,
  fetchMeetingJoinConfig,
  fetchMeetingStatus,
  fetchMeetingTranscripts,
  fetchMeetingTranscriptPage,
  ingestMeetingTranscript,
  heartbeatMeeting,
} from "@/lib/api/meetings"
import type { MeetingJoinConfig, MeetingTranscriptSegment } from "@/lib/api/types"
import { publishMeetingSync } from "@/lib/meeting-sync"

type I18nT = ReturnType<typeof useI18n>

const mr = (t: I18nT, key: string, values?: Record<string, string | number>) =>
  t(`meetingRoomExtract.roomClient.${key}`, values)

type EnterpriseMeetingRoomPageProps = {
  initialMeetingId?: string
}

export default function EnterpriseMeetingRoomPage({ initialMeetingId }: EnterpriseMeetingRoomPageProps) {
  const params = useParams<{ meetingId?: string }>()
  const router = useRouter()
  const searchParams = useSearchParams()
  const t = useI18n()
  const [config, setConfig] = useState<MeetingJoinConfig | null>(null)
  const [initialTranscripts, setInitialTranscripts] = useState<MeetingTranscriptSegment[]>([])
  const [transcriptLoadError, setTranscriptLoadError] = useState("")
  const [error, setError] = useState("")
  const routeMeetingId = params.meetingId
  const queryMeetingId = searchParams.get("meeting_id") || searchParams.get("meetingId")
  const meetingId = routeMeetingId && !isStaticExportRouteParam(routeMeetingId)
    ? routeMeetingId
    : initialMeetingId || queryMeetingId || ""

  const loadInitialTranscripts = useCallback(async (activeMeetingId = meetingId) => {
    if (!activeMeetingId) return
    setTranscriptLoadError("")
    try {
      const transcriptResult = await fetchMeetingTranscripts(activeMeetingId)
      if (transcriptResult.success) {
        setInitialTranscripts(transcriptResult.data ?? [])
      } else {
        setTranscriptLoadError(transcriptResult.error?.message || mr(t, "transcriptLoadFailed"))
      }
    } catch (transcriptError) {
      setTranscriptLoadError(transcriptError instanceof Error ? transcriptError.message : mr(t, "transcriptLoadFailed"))
    }
  }, [meetingId, t])

  const load = useCallback(async () => {
    setError("")
    setTranscriptLoadError("")
    if (!meetingId) {
      setError(mr(t, "missingMeetingId"))
      return
    }
    try {
      const result = await fetchMeetingJoinConfig(meetingId)
      if (result.success && result.data) {
        setConfig(result.data)
        const resolvedMeetingId = result.data.meetingId || result.data.meeting_id || meetingId
        void loadInitialTranscripts(resolvedMeetingId)
      } else {
        setError(result.error?.message || mr(t, "joinConfigUnavailable"))
      }
    } catch (configError) {
      setError(configError instanceof Error ? configError.message : mr(t, "joinConfigUnavailable"))
    }
  }, [loadInitialTranscripts, meetingId, t])

  useEffect(() => {
    const timer = window.setTimeout(() => void load(), 0)
    return () => window.clearTimeout(timer)
  }, [load])

  const session = readSession()
  const roomName = config?.roomName || config?.room_name
  const fallbackTicketId = searchParams.get("ticket_id")
  const ticketId = getMeetingTicketId(config, fallbackTicketId)
  const returnPath = getMeetingReturnPath(config, fallbackTicketId)
  const returnLabel = ticketId > 0 ? mr(t, "returnTicket") : mr(t, "returnVideoCenter")
  const ended = error.includes("会议已结束") || error.toLocaleLowerCase().includes("meeting has ended")

  if (error) {
    return (
      <div className="flex min-h-[60vh] flex-col items-center justify-center gap-4 px-6 text-center">
        <h1 className="text-lg font-semibold">{ended ? mr(t, "collaborationEnded") : mr(t, "cannotEnterCollaboration")}</h1>
        <p className="max-w-md text-sm text-muted-foreground">{error}</p>
        <div className="flex flex-wrap justify-center gap-2">
          <RailopsButton variant="primary" onClick={() => router.push(returnPath)}>
            <ArrowLeftIcon className="size-4" />
            {returnLabel}
          </RailopsButton>
          {!ended ? (
            <RailopsButton onClick={() => void load()}>
              <RefreshCwIcon className="size-4" />
              {mr(t, "retry")}
            </RailopsButton>
          ) : null}
        </div>
      </div>
    )
  }
  if (!config || !roomName) {
    return (
      <div className="h-[calc(100vh-5.5rem)] min-h-[36rem] overflow-hidden border bg-black text-white">
        <div className="flex h-full min-h-0 flex-col">
          <div className="flex h-14 shrink-0 items-center justify-between border-b border-white/10 px-4">
            <div className="min-w-0">
              <div className="h-4 w-40 rounded bg-white/15" />
              <div className="mt-2 h-3 w-24 rounded bg-white/10" />
            </div>
            <Loader2Icon className="size-5 animate-spin text-white/70" />
          </div>
          <div className="grid min-h-0 flex-1 place-items-center px-6 text-center text-sm text-white/70">
            {mr(t, "configLoading")}
          </div>
        </div>
      </div>
    )
  }
  const resolvedMeetingId = config.meetingId || config.meeting_id || meetingId

  return (
    <div className="h-[calc(100vh-5.5rem)] min-h-[36rem] overflow-hidden border bg-black">
      <MeetingLiveRoom
        domain={config.domain}
        jitsiUrl={config.jitsiUrl || config.jitsi_url}
        meetingId={resolvedMeetingId}
        roomName={roomName}
        subject={getMeetingDisplaySubject(config, roomName)}
        returnLabel={returnLabel}
        jwt={config.jwt}
        canEndMeeting={config.canEnd ?? config.can_end ?? false}
        transcriptionEnabled={config.transcriptionEnabled ?? config.transcription_enabled ?? false}
        transcriptionProvider={config.transcriptionProvider ?? config.transcription_provider}
        transcriptionReady={config.transcriptionReady ?? config.transcription_ready ?? false}
        initialTranscripts={initialTranscripts}
        fetchTranscripts={async () => {
          const result = await fetchMeetingTranscripts(resolvedMeetingId)
          if (!result.success) throw new Error(result.error?.message || mr(t, "transcriptLoadFailed"))
          return result.data ?? []
        }}
        fetchTranscriptPage={async (cursor) => {
          const result = await fetchMeetingTranscriptPage(
            resolvedMeetingId,
            cursor,
          )
          if (!result.success || !result.data) throw new Error(result.error?.message || mr(t, "transcriptLoadFailed"))
          return result.data
        }}
        transcriptionUnavailableReason={(config.transcriptionError ?? config.transcription_error) || transcriptLoadError || undefined}
        arEnabled={config.arDetectionEnabled ?? config.ar_detection_enabled ?? false}
        displayName={session?.user.nickname || session?.user.username || "Engineer"}
        avatar={session?.user.avatar}
        onConferenceJoined={async () => {
          await confirmMeetingJoined(resolvedMeetingId)
          publishMeetingSync({ type: "meeting-updated", meetingId, status: "active" })
        }}
        onConferenceLeft={async () => {
          await confirmMeetingLeft(resolvedMeetingId)
          publishMeetingSync({ type: "meeting-updated", meetingId })
        }}
        onConferenceHeartbeat={async () => {
          const result = await heartbeatMeeting(resolvedMeetingId)
          if (!result.success) throw new Error(result.error?.message || mr(t, "heartbeatFailed"))
        }}
        onTranscriptFinal={async (event) => {
          const result = await ingestMeetingTranscript(resolvedMeetingId, event)
          if (!result.success) throw new Error(result.error?.message || mr(t, "transcriptArchiveFailed"))
          return result.data ?? null
        }}
        fetchMeetingStatus={async () => {
          const result = await fetchMeetingStatus(resolvedMeetingId)
          return result.success ? result.data ?? null : null
        }}
        onMeetingEnd={() => {
          publishMeetingSync({ type: "meeting-updated", meetingId })
          router.push(returnPath)
        }}
        className="h-full min-h-0"
      />
    </div>
  )
}
