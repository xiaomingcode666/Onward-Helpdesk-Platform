"use client"

import { translateCurrentMessage } from "@/i18n/messages"
import { useCallback, useEffect, useRef, useState, type CSSProperties } from "react"
import { toast } from "sonner"

import { MeetingARWorkspace } from "@/components/meeting/meeting-ar-workspace"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { IconButton, RailopsButton, StandardModal } from "@railops/ui"
import { endMeeting, uploadMeetingFrame } from "@/lib/api/meetings"
import type { CursorPage, MeetingARAnnotation, MeetingTranscriptSegment } from "@/lib/api/types"
import { publishMeetingSync } from "@/lib/meeting-sync"
import { cn } from "@/lib/utils"
import { parseJitsiTranscriptionChunk, type LiveTranscriptEvent } from "./meeting-transcription"
import {
  Loader2Icon,
  ArrowLeftIcon,
  MicIcon,
  MicOffIcon,
  CameraIcon,
  CameraOffIcon,
  MonitorIcon,
  PhoneOffIcon,
  LogOutIcon,
  WifiIcon,
	  WifiOffIcon,
	  CaptionsIcon,
	  ScrollTextIcon,
		LanguagesIcon,
  ScanLineIcon,
  CheckIcon,
  Grid2X2Icon,
  MessageSquareIcon,
  UserPlusIcon,
  UsersIcon,
  Settings2Icon,
  VideoIcon,
  XIcon,
  SendIcon,
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


type JitsiMediaDevice = {
  deviceId: string
  groupId?: string
  kind: string
  label: string
}

type JitsiAvailableDevices = {
  audioInput?: JitsiMediaDevice[]
  audioOutput?: JitsiMediaDevice[]
  videoInput?: JitsiMediaDevice[]
}

type JitsiCurrentDevices = {
  audioInput?: JitsiMediaDevice
  audioOutput?: JitsiMediaDevice
  videoInput?: JitsiMediaDevice
}

interface JitsiMeetExternalAPIImpl {
  dispose(): void
  executeCommand(command: string, ...args: unknown[]): void
  addListener(event: string, listener: (...args: unknown[]) => void): void
  removeListener(event: string, listener: (...args: unknown[]) => void): void
  isAudioMuted(): Promise<boolean>
  isVideoMuted(): Promise<boolean>
  getAvatarURL(): string
  getDisplayName(): string
  getEmail(): string
  getIFrame(): HTMLIFrameElement
  isDeviceListAvailable(): Promise<boolean>
  captureLargeVideoScreenshot?(): Promise<{ dataURL?: string }>
  getAvailableDevices(): Promise<JitsiAvailableDevices>
  getCurrentDevices(): Promise<JitsiCurrentDevices>
  setAudioInputDevice(label: string, deviceId: string): Promise<void>
}

interface MeetingLiveRoomProps {
  domain: string
  jitsiUrl?: string
  meetingId?: string
  roomName: string
  subject?: string
  jwt: string
  canEndMeeting?: boolean
  transcriptionEnabled?: boolean
  transcriptionProvider?: string
  transcriptionReady?: boolean
  transcriptionUnavailableReason?: string
	initialTranscripts?: MeetingTranscriptSegment[]
	fetchTranscripts?: () => Promise<MeetingTranscriptSegment[]>
	fetchTranscriptPage?: (cursor?: string) => Promise<CursorPage<MeetingTranscriptSegment>>
  arEnabled?: boolean
  customerInviteEnabled?: boolean
  mobileLayout?: boolean
  displayName: string
  avatar?: string
  onConferenceJoined?: () => void | Promise<void>
  onConferenceLeft?: () => void | Promise<void>
  onConferenceHeartbeat?: () => void | Promise<void>
  onTranscriptFinal?: (event: LiveTranscriptEvent) => Promise<MeetingTranscriptSegment | null>
  fetchMeetingStatus?: () => Promise<{ status?: string } | null>
  returnLabel?: string
  onMeetingEnd: () => void
  className?: string
}

function canUseWebRTCInCurrentContext() {
  if (typeof window === "undefined") return true
  return window.isSecureContext
}

type MeetingAROverlayMessage = {
  type: "rhd.ar_overlay"
  meetingId: string
  annotations: MeetingARAnnotation[]
  timestamp: number
}

function transcriptionProviderLabel(provider: string): string {
  switch (provider.trim().toLowerCase()) {
    case "xfyun":
    case "xfyun_rtasr":
      return ee("meetingLive.text001")
    case "disabled":
    case "":
      return ee("meetingLive.text002")
    default:
      return ee("meetingLive.text003")
  }
}

function transcriptionLanguageLabel(language: string): string {
	return language === "en-US" ? "English" : ee("meetingLive.text004")
}

function archivedTranscriptToLive(item: MeetingTranscriptSegment): LiveTranscriptEvent {
	return {
		providerEventId: item.providerEventId ?? item.id,
		participantId: item.participantId,
		speakerName: item.speakerName,
		language: item.language,
		text: item.text,
		isFinal: item.isFinal,
		startedAtMs: item.startedAtMs,
		endedAtMs: item.endedAtMs,
		confidence: item.confidence,
		translatedLanguage: item.translatedLanguage,
		translatedText: item.translatedText,
		translationProvider: item.translationProvider,
		translationStatus: item.translationStatus,
		translationError: item.translationError,
	}
}

function mergeArchivedTranscripts(
	current: LiveTranscriptEvent[],
	archived: MeetingTranscriptSegment[],
): LiveTranscriptEvent[] {
	const merged = new Map<string, LiveTranscriptEvent>()
	for (const item of current) merged.set(item.providerEventId, item)
	for (const item of archived.map(archivedTranscriptToLive)) merged.set(item.providerEventId, item)
	return [...merged.values()]
		.sort((left, right) => left.startedAtMs - right.startedAtMs)
}

async function archiveTranscriptWithRetry(
	handler: (event: LiveTranscriptEvent) => Promise<MeetingTranscriptSegment | null>,
	event: LiveTranscriptEvent,
	shouldStop: () => boolean,
): Promise<MeetingTranscriptSegment | null> {
	const retryDelays = [0, 1_000, 3_000]
	let lastError: unknown
	for (const delay of retryDelays) {
		if (shouldStop()) return null
		if (delay > 0) {
			await new Promise<void>((resolve) => window.setTimeout(resolve, delay))
			if (shouldStop()) return null
		}
		try {
			return await handler(event)
		} catch (error) {
			lastError = error
		}
	}
	throw lastError
}

export function MeetingLiveRoom({
  domain,
  jitsiUrl,
  meetingId,
  roomName,
  subject,
  jwt,
  canEndMeeting = false,
  transcriptionEnabled = false,
  transcriptionProvider = "disabled",
  transcriptionReady = false,
	transcriptionUnavailableReason = "",
	initialTranscripts,
	fetchTranscripts,
	fetchTranscriptPage,
  arEnabled = false,
  customerInviteEnabled = true,
  mobileLayout = false,
  displayName,
  avatar,
  onConferenceJoined,
  onConferenceLeft,
  onConferenceHeartbeat,
  onTranscriptFinal,
  fetchMeetingStatus,
  returnLabel = ee("meetingLive.text005"),
  onMeetingEnd,
  className,
}: MeetingLiveRoomProps) {
  const roomRootRef = useRef<HTMLDivElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const apiRef = useRef<JitsiMeetExternalAPIImpl | null>(null)
  const onConferenceJoinedRef = useRef(onConferenceJoined)
  const onConferenceLeftRef = useRef(onConferenceLeft)
  const onConferenceHeartbeatRef = useRef(onConferenceHeartbeat)
	const fetchMeetingStatusRef = useRef(fetchMeetingStatus)
	const onTranscriptFinalRef = useRef(onTranscriptFinal)
	const fetchTranscriptsRef = useRef(fetchTranscripts)
	const fetchTranscriptPageRef = useRef(fetchTranscriptPage)
	const transcriptPageInitializedRef = useRef(false)
	const olderTranscriptPaginationStartedRef = useRef(false)
		const preserveTranscriptScrollRef = useRef(false)
		const archivingTranscriptIDsRef = useRef(new Set<string>())
		const archivedTranscriptIDsRef = useRef(
			new Set((initialTranscripts ?? []).map((item) => item.providerEventId ?? item.id)),
		)
		const autoTranscriptionRequestedRef = useRef(false)
  const onMeetingEndRef = useRef(onMeetingEnd)
  const conferenceJoinedRef = useRef(false)
  const conferenceLeaveReportedRef = useRef(false)
  const roomExitRequestedRef = useRef(false)
  const [joinRequested, setJoinRequested] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const [isAudioMuted, setIsAudioMuted] = useState(true)
  const [isVideoMuted, setIsVideoMuted] = useState(true)
  const [isScreenSharing, setIsScreenSharing] = useState(false)
  const [isConferenceJoined, setIsConferenceJoined] = useState(false)
  const [audioInputDevices, setAudioInputDevices] = useState<JitsiMediaDevice[]>([])
  const [selectedAudioInputId, setSelectedAudioInputId] = useState("")
  const [connectionQuality, setConnectionQuality] = useState<"good" | "poor" | "very-poor" | null>(null)
  const transcriptionTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
	const meetingLoadTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)
	const transcriptScrollRef = useRef<HTMLDivElement>(null)
  const [transcriptionState, setTranscriptionState] = useState<"idle" | "connecting" | "active" | "stopping">("idle")
  const [transcriptionError, setTranscriptionError] = useState("")
	const [captions, setCaptions] = useState<LiveTranscriptEvent[]>(() =>
		(initialTranscripts ?? []).map(archivedTranscriptToLive),
	)
	const [olderTranscriptCursor, setOlderTranscriptCursor] = useState("")
	const [hasOlderTranscripts, setHasOlderTranscripts] = useState(false)
	const [loadingOlderTranscripts, setLoadingOlderTranscripts] = useState(false)
	const [transcriptionLanguage, setTranscriptionLanguage] = useState("zh-CN")
  const [showTranscriptPanel, setShowTranscriptPanel] = useState(false)
  const [compactPanelLayout, setCompactPanelLayout] = useState(false)
  const [compactControlsLayout, setCompactControlsLayout] = useState(false)
  const [showMobileChat, setShowMobileChat] = useState(false)
  const [chatDraft, setChatDraft] = useState("")
  const [chatMessages, setChatMessages] = useState<Array<{ id: string; sender: string; text: string }>>([])
  const [arFrame, setARFrame] = useState<{ dataUrl: string; assetId: number } | null>(null)
  const [arOverlay, setAROverlay] = useState<MeetingARAnnotation[]>([])
  const [capturingFrame, setCapturingFrame] = useState(false)
	const [arCaptureSupported, setARCaptureSupported] = useState(false)
  const [inviteCopied, setInviteCopied] = useState(false)
  const [endConfirmOpen, setEndConfirmOpen] = useState(false)
  const [endingMeeting, setEndingMeeting] = useState(false)
  const [meetingError, setMeetingError] = useState("")
	const [actionError, setActionError] = useState("")
	const transcriptionNeedsAudio =
		transcriptionEnabled &&
		transcriptionReady &&
		(transcriptionState === "connecting" || transcriptionState === "active") &&
		isAudioMuted

	useEffect(() => {
		const saved = window.localStorage.getItem("remotehelpdesk:meeting-transcription-language")
		if (saved === "zh-CN" || saved === "en-US") setTranscriptionLanguage(saved)
	}, [])

	useEffect(() => {
		const root = roomRootRef.current
		if (!root || typeof ResizeObserver === "undefined") return
		const updateLayout = () => {
			setCompactPanelLayout(!mobileLayout && root.clientWidth < 760)
			setCompactControlsLayout(!mobileLayout && root.clientWidth < 520)
		}
		updateLayout()
		const observer = new ResizeObserver(updateLayout)
		observer.observe(root)
		return () => observer.disconnect()
	}, [mobileLayout])

		useEffect(() => {
			if (!initialTranscripts) return
		for (const item of initialTranscripts) {
			archivedTranscriptIDsRef.current.add(item.providerEventId ?? item.id)
		}
		setCaptions((current) => {
			return mergeArchivedTranscripts(current, initialTranscripts)
		})
		}, [initialTranscripts])

		useEffect(() => {
			autoTranscriptionRequestedRef.current = false
		}, [meetingId, roomName])

  useEffect(() => {
    onMeetingEndRef.current = onMeetingEnd
  }, [onMeetingEnd])

  useEffect(() => {
    onConferenceJoinedRef.current = onConferenceJoined
  }, [onConferenceJoined])

  useEffect(() => {
    onConferenceLeftRef.current = onConferenceLeft
  }, [onConferenceLeft])

  useEffect(() => {
    onConferenceHeartbeatRef.current = onConferenceHeartbeat
  }, [onConferenceHeartbeat])

  useEffect(() => {
    fetchMeetingStatusRef.current = fetchMeetingStatus
  }, [fetchMeetingStatus])

	useEffect(() => {
		onTranscriptFinalRef.current = onTranscriptFinal
	}, [onTranscriptFinal])

	useEffect(() => {
		fetchTranscriptsRef.current = fetchTranscripts
	}, [fetchTranscripts])

	useEffect(() => {
		fetchTranscriptPageRef.current = fetchTranscriptPage
	}, [fetchTranscriptPage])

	useEffect(() => {
			if (!showTranscriptPanel || (!fetchTranscriptPageRef.current && !fetchTranscriptsRef.current)) return
		let disposed = false
		let refreshing = false
		const refresh = async () => {
				if (disposed || refreshing) return
				refreshing = true
				try {
					const page = fetchTranscriptPageRef.current
						? await fetchTranscriptPageRef.current()
						: null
					const archived = page?.results ?? (await fetchTranscriptsRef.current?.()) ?? []
					if (disposed) return
				for (const item of archived) {
					archivedTranscriptIDsRef.current.add(item.providerEventId ?? item.id)
				}
					setCaptions((current) => mergeArchivedTranscripts(current, archived))
					if (page && (!transcriptPageInitializedRef.current || !olderTranscriptPaginationStartedRef.current)) {
						transcriptPageInitializedRef.current = true
						setOlderTranscriptCursor(page.cursor ?? "")
						setHasOlderTranscripts(Boolean(page.hasMore))
					}
			} catch (error) {
				console.warn("Failed to refresh archived meeting transcripts:", error)
			} finally {
				refreshing = false
			}
		}
		void refresh()
		const timer = window.setInterval(() => void refresh(), 5_000)
		return () => {
			disposed = true
			window.clearInterval(timer)
		}
	}, [showTranscriptPanel])

	useEffect(() => {
		if (!showTranscriptPanel || !transcriptScrollRef.current) return
		if (preserveTranscriptScrollRef.current) return
		transcriptScrollRef.current.scrollTop = transcriptScrollRef.current.scrollHeight
	}, [captions, showTranscriptPanel])

	const loadOlderTranscripts = useCallback(async () => {
		const fetchPage = fetchTranscriptPageRef.current
		if (!fetchPage || !hasOlderTranscripts || loadingOlderTranscripts) return
		setLoadingOlderTranscripts(true)
		const scrollContainer = transcriptScrollRef.current
		const previousScrollHeight = scrollContainer?.scrollHeight ?? 0
		preserveTranscriptScrollRef.current = true
		try {
			const page = await fetchPage(olderTranscriptCursor)
			for (const item of page.results) {
				archivedTranscriptIDsRef.current.add(item.providerEventId ?? item.id)
			}
			setCaptions((current) => mergeArchivedTranscripts(current, page.results))
			setOlderTranscriptCursor(page.cursor ?? "")
			setHasOlderTranscripts(Boolean(page.hasMore))
			olderTranscriptPaginationStartedRef.current = true
			window.requestAnimationFrame(() => {
				if (scrollContainer) {
					scrollContainer.scrollTop += scrollContainer.scrollHeight - previousScrollHeight
				}
				preserveTranscriptScrollRef.current = false
			})
		} catch (error) {
			preserveTranscriptScrollRef.current = false
			setTranscriptionError(error instanceof Error ? error.message : ee("meetingLive.text006"))
		} finally {
			setLoadingOlderTranscripts(false)
		}
	}, [hasOlderTranscripts, loadingOlderTranscripts, olderTranscriptCursor])

  useEffect(() => {
    if (!isConferenceJoined || !onConferenceHeartbeatRef.current) return
    const heartbeat = () => {
      void Promise.resolve(onConferenceHeartbeatRef.current?.()).catch((error) => {
        console.warn("Failed to report meeting heartbeat:", error)
      })
    }
    const timer = window.setInterval(heartbeat, 30_000)
    const handleVisibility = () => {
      if (document.visibilityState === "visible") heartbeat()
    }
    document.addEventListener("visibilitychange", handleVisibility)
    return () => {
      window.clearInterval(timer)
      document.removeEventListener("visibilitychange", handleVisibility)
    }
  }, [isConferenceJoined])

  const reportConferenceLeft = useCallback(() => {
    if (!conferenceJoinedRef.current || conferenceLeaveReportedRef.current) return
    conferenceLeaveReportedRef.current = true
    void Promise.resolve(onConferenceLeftRef.current?.()).catch((error) => {
      console.warn("Failed to report meeting leave:", error)
    })
  }, [])

  const exitRoom = useCallback(() => {
    if (roomExitRequestedRef.current) return
    roomExitRequestedRef.current = true
    onMeetingEndRef.current()
  }, [])

  // 加载 Jitsi API
  const loadJitsiApi = useCallback(async () => {
    try {
      const { loadJitsiExternalApi } = await import("./meeting-utils")
      // The browser should load from the public meeting domain. The backend
      // service URL may point at an internal HTTP endpoint for health checks.
      const JitsiMeetExternalAPI = await loadJitsiExternalApi(domain || jitsiUrl)
      return JitsiMeetExternalAPI
    } catch (err) {
      console.error("Failed to load Jitsi API:", err)
      return null
    }
  }, [domain, jitsiUrl])

  // 初始化会议
  useEffect(() => {
		if (!joinRequested) return
		if (!canUseWebRTCInCurrentContext()) {
			setIsLoading(false)
				setMeetingError(ee("meetingLive.text007"))
			return
		}
    let disposed = false
		let conferenceJoinReported = false
		let joinedThisAPI = false
		let meetingAPI: JitsiMeetExternalAPIImpl | null = null
		conferenceJoinedRef.current = false
		conferenceLeaveReportedRef.current = false
		roomExitRequestedRef.current = false
		setIsConferenceJoined(false)
		setActionError("")

		const handleReadyToClose = () => {
			if (!disposed) {
				reportConferenceLeft()
				exitRoom()
			}
		}
		const clearMeetingLoadTimeout = () => {
			if (meetingLoadTimeoutRef.current) {
				clearTimeout(meetingLoadTimeoutRef.current)
				meetingLoadTimeoutRef.current = null
			}
		}
		const refreshAudioInputDevices = async () => {
			const activeAPI = meetingAPI
			if (!activeAPI || disposed) return
			try {
				const [available, current] = await Promise.all([
					activeAPI.getAvailableDevices(),
					activeAPI.getCurrentDevices(),
				])
				if (disposed || meetingAPI !== activeAPI) return
				setAudioInputDevices(available.audioInput ?? [])
				setSelectedAudioInputId(current.audioInput?.deviceId ?? "")
			} catch (error) {
				console.warn("Failed to read Jitsi audio devices:", error)
			}
		}
		const markConferenceJoined = () => {
			if (disposed) return
			clearMeetingLoadTimeout()
			setMeetingError("")
			setIsConferenceJoined(true)
			setIsLoading(false)
			joinedThisAPI = true
			conferenceJoinedRef.current = true
			conferenceLeaveReportedRef.current = false
			roomExitRequestedRef.current = false
			void refreshAudioInputDevices()
			if (!conferenceJoinReported) {
				conferenceJoinReported = true
				void Promise.resolve(onConferenceJoinedRef.current?.()).catch((error) => {
					console.warn("Failed to report meeting join:", error)
				})
			}
		}
		const handleConferenceLeft = () => {
			if (disposed) return
			setIsConferenceJoined(false)
			reportConferenceLeft()
			exitRoom()
		}
		const handlePrejoinScreenLoaded = () => {
			if (disposed) return
			clearMeetingLoadTimeout()
			setMeetingError("")
			setIsLoading(false)
		}
		const handleEndpointTextMessage = (payload: unknown) => {
			const message = parseAROverlayMessage(payload)
			if (!message || (message.meetingId && message.meetingId !== meetingId)) return
			setAROverlay(message.annotations)
		}
		const handleDeviceListChanged = () => {
			void refreshAudioInputDevices()
		}
    const handleTranscribingStatusChanged = (payload: unknown) => {
      const active =
        typeof payload === "object" && payload !== null && "on" in payload
          ? Boolean((payload as { on?: boolean }).on)
          : Boolean(payload)
      if (transcriptionTimeoutRef.current) {
        clearTimeout(transcriptionTimeoutRef.current)
        transcriptionTimeoutRef.current = null
      }
      setTranscriptionState(active ? "active" : "idle")
	      if (active) {
	        setTranscriptionError("")
	      }
    }
    const handleTranscriptionChunkReceived = (payload: unknown) => {
      const event = parseJitsiTranscriptionChunk(payload)
      if (!event) return
      if (transcriptionTimeoutRef.current) {
        clearTimeout(transcriptionTimeoutRef.current)
        transcriptionTimeoutRef.current = null
      }
      setTranscriptionState("active")
	      setCaptions((current) => {
			const archived = current.find((item) => item.providerEventId === event.providerEventId)
			const mergedEvent = archived ? {
				...event,
				translatedLanguage: archived.translatedLanguage,
				translatedText: archived.translatedText,
				translationProvider: archived.translationProvider,
				translationStatus: archived.translationStatus,
				translationError: archived.translationError,
			} : event
			return [...current.filter((item) => item.providerEventId !== event.providerEventId), mergedEvent]
				.sort((left, right) => left.startedAtMs - right.startedAtMs)
	      })
		  const archiveHandler = onTranscriptFinalRef.current
		  if (
			event.isFinal &&
			archiveHandler &&
			!archivedTranscriptIDsRef.current.has(event.providerEventId) &&
			!archivingTranscriptIDsRef.current.has(event.providerEventId)
		  ) {
			archivingTranscriptIDsRef.current.add(event.providerEventId)
			void archiveTranscriptWithRetry(archiveHandler, event, () => disposed).then((archived) => {
				if (!archived || disposed) return
				archivedTranscriptIDsRef.current.add(event.providerEventId)
				setCaptions((current) => mergeArchivedTranscripts(current, [archived]))
				setTranscriptionError((current) => current.startsWith(ee("meetingLive.text008")) ? "" : current)
			}).catch((error) => {
				if (disposed) return
				console.warn("Failed to archive final meeting transcript:", error)
				setTranscriptionError(ee("meetingLive.text009"))
			}).finally(() => {
				archivingTranscriptIDsRef.current.delete(event.providerEventId)
			})
		  }
    }

    const initMeeting = async () => {
      setMeetingError("")
      const JitsiMeetExternalAPI = await loadJitsiApi()
      if (disposed) return
      if (!JitsiMeetExternalAPI || !containerRef.current) {
        setIsLoading(false)
        setMeetingError(ee("meetingLive.text010"))
        return
      }

      try {
        const { normalizeJitsiDomain } = await import("./meeting-utils")
        meetingAPI = new JitsiMeetExternalAPI(normalizeJitsiDomain(domain), {
          roomName,
          parentNode: containerRef.current,
          jwt,
				onload: () => {
					if (!disposed) {
						clearMeetingLoadTimeout()
						setIsLoading(false)
					}
          },
          userInfo: {
            displayName,
            avatarUrl: avatar,
          },
          configOverwrite: {
            // Mobile customers need an audio track for Jigasi transcription as soon as they join.
            // Desktop rooms keep the existing muted-by-default behavior.
            startWithAudioMuted: !mobileLayout,
            startWithVideoMuted: true,
			disableInitialGUM: !mobileLayout,
            disableDeepLinking: true,
            disableSimulcast: false,
            enableWelcomePage: false,
            enableClosePage: false,
			prejoinPageEnabled: false,
			prejoinConfig: { enabled: false },
            resolution: 720,
            constraints: {
              video: {
                height: { ideal: 720, max: 720, min: 240 },
              },
            },
            toolbarButtons: [],
          },
          interfaceConfigOverwrite: {
            TOOLBAR_ALWAYS_VISIBLE: false,
            FILM_STRIP_MAX_HEIGHT: 120,
            SHOW_JITSI_WATERMARK: false,
            SHOW_WATERMARK_FOR_GUESTS: false,
            SHOW_BRAND_WATERMARK: false,
            SHOW_POWERED_BY: false,
            DISABLE_JOIN_LEAVE_NOTIFICATIONS: false,
            DISABLE_TRANSCRIPTION_SUBTITLES: false,
            DISABLE_RECORDING_BUTTON: true,
            DISABLE_LIVESTREAMING_BUTTON: true,
            HIDE_INVITE_MORE_HEADER: true,
          },
        })
			const api = meetingAPI!

			apiRef.current = api
		setARCaptureSupported(arEnabled && typeof api.captureLargeVideoScreenshot === "function")
			meetingLoadTimeoutRef.current = setTimeout(() => {
				if (!disposed) {
					setIsLoading(false)
						setMeetingError(ee("meetingLive.text011"))
				}
				meetingLoadTimeoutRef.current = null
			}, 20_000)
			api.addListener("videoConferenceJoined", markConferenceJoined)
			api.addListener("videoConferenceLeft", handleConferenceLeft)
			api.addListener("prejoinScreenLoaded", handlePrejoinScreenLoaded)
			api.addListener("endpointTextMessageReceived", handleEndpointTextMessage)
			api.addListener("incomingMessage", (payload: unknown) => {
				const record = isRecord(payload) ? payload : null
				const message = record && typeof record.message === "string" ? record.message.trim() : ""
				if (!message) return
				const providerEventId = record && typeof record.id === "string" && record.id.trim()
					? `jitsi-chat-${record.id.trim()}`
					: `jitsi-chat-${Date.now()}-${message.slice(0, 24)}`
				const now = Date.now()
				const chatEvent: LiveTranscriptEvent = {
					provider: "jitsi_chat",
					providerEventId,
					participantId: record && typeof record.from === "string" ? record.from : "unknown",
					speakerName: record && typeof record.from === "string" ? record.from : ee("meetingLive.text031"),
					language: "zh-CN",
					text: message,
					isFinal: true,
					startedAtMs: now,
					endedAtMs: now,
					confidence: 1,
				}
				setChatMessages((current) => [...current.slice(-19), {
					id: `${Date.now()}-${current.length}`,
					sender: chatEvent.speakerName,
					text: message,
				}])
				const archiveHandler = onTranscriptFinalRef.current
				if (!archiveHandler || archivedTranscriptIDsRef.current.has(providerEventId) || archivingTranscriptIDsRef.current.has(providerEventId)) return
				archivingTranscriptIDsRef.current.add(providerEventId)
				void archiveTranscriptWithRetry(archiveHandler, chatEvent, () => disposed).then((archived) => {
					if (!archived || disposed) return
					archivedTranscriptIDsRef.current.add(providerEventId)
					setCaptions((current) => mergeArchivedTranscripts(current, [archived]))
				}).catch((error) => {
					if (!disposed) console.warn("Failed to archive meeting chat message:", error)
				}).finally(() => archivingTranscriptIDsRef.current.delete(providerEventId))
			})
			api.addListener("deviceListChanged", handleDeviceListChanged)

        api.addListener("readyToClose", handleReadyToClose)
        api.addListener("transcribingStatusChanged", handleTranscribingStatusChanged)
        api.addListener("transcriptionChunkReceived", handleTranscriptionChunkReceived)

        api.addListener("audioMuteStatusChanged", (payload: unknown) => {
          const muted =
            typeof payload === "object" && payload !== null && "muted" in payload
              ? Boolean((payload as { muted?: boolean }).muted)
              : false
          setIsAudioMuted(muted)
        })

        api.addListener("videoMuteStatusChanged", (payload: unknown) => {
          const muted =
            typeof payload === "object" && payload !== null && "muted" in payload
              ? Boolean((payload as { muted?: boolean }).muted)
              : false
          setIsVideoMuted(muted)
        })

        api.addListener("screenSharingStatusChanged", (payload: unknown) => {
          const on =
            typeof payload === "object" && payload !== null && "on" in payload
              ? Boolean((payload as { on?: boolean }).on)
              : false
          setIsScreenSharing(on)
        })

			api.addListener("connectionQualityChanged", (payload: unknown) => {
          const quality = typeof payload === "object" && payload !== null && "connectionQuality" in payload
            ? Number((payload as { connectionQuality?: number }).connectionQuality)
            : Number(payload)
          if (!Number.isFinite(quality)) return
          if (quality <= 1) setConnectionQuality("very-poor")
          else if (quality <= 2) setConnectionQuality("poor")
          else setConnectionQuality("good")
			})
      } catch (err) {
        console.error("Failed to initialize Jitsi meeting:", err)
        if (!disposed) {
          setIsLoading(false)
          setMeetingError(ee("meetingLive.text012"))
        }
      }
    }

    initMeeting()

    return () => {
      disposed = true
		const activeAPI = meetingAPI
		if (activeAPI && apiRef.current === activeAPI) {
			activeAPI.removeListener("readyToClose", handleReadyToClose)
			activeAPI.removeListener("videoConferenceJoined", markConferenceJoined)
			activeAPI.removeListener("videoConferenceLeft", handleConferenceLeft)
			activeAPI.removeListener("prejoinScreenLoaded", handlePrejoinScreenLoaded)
			activeAPI.removeListener("endpointTextMessageReceived", handleEndpointTextMessage)
			activeAPI.removeListener("incomingMessage", () => undefined)
			activeAPI.removeListener("deviceListChanged", handleDeviceListChanged)
			activeAPI.removeListener("transcribingStatusChanged", handleTranscribingStatusChanged)
			activeAPI.removeListener("transcriptionChunkReceived", handleTranscriptionChunkReceived)
			if (joinedThisAPI) reportConferenceLeft()
			activeAPI.executeCommand("hangup")
			apiRef.current = null
			window.setTimeout(() => activeAPI.dispose(), 250)
		} else if (activeAPI) {
			// An older async initialization must never hang up a newer conference instance.
			activeAPI.dispose()
		}
			if (transcriptionTimeoutRef.current) {
        clearTimeout(transcriptionTimeoutRef.current)
        transcriptionTimeoutRef.current = null
			}
			clearMeetingLoadTimeout()
		}
		}, [avatar, displayName, domain, exitRoom, joinRequested, jwt, loadJitsiApi, meetingId, reportConferenceLeft, roomName, arEnabled, mobileLayout])

  useEffect(() => {
    if (!meetingId || !fetchMeetingStatusRef.current) return
    let disposed = false
    let checking = false
    const checkStatus = async () => {
      if (checking || disposed) return
      checking = true
      try {
        const status = await fetchMeetingStatusRef.current?.()
        if (!disposed && status?.status === "ended") {
          setActionError(ee("meetingLive.text013"))
          apiRef.current?.executeCommand("hangup")
          reportConferenceLeft()
          exitRoom()
        }
      } catch (error) {
        console.warn("Failed to refresh meeting status:", error)
      } finally {
        checking = false
      }
    }
    const timer = window.setInterval(() => void checkStatus(), 3_000)
    void checkStatus()
    return () => {
      disposed = true
      window.clearInterval(timer)
    }
  }, [exitRoom, meetingId, reportConferenceLeft])

  useEffect(() => {
    if (!meetingId || typeof BroadcastChannel === "undefined") return
    const channel = new BroadcastChannel(`remotehelpdesk:meeting-ar:${meetingId}`)
    channel.onmessage = (event: MessageEvent<MeetingAROverlayMessage>) => {
      const message = event.data
      if (message?.type === "rhd.ar_overlay" && message.meetingId === meetingId) {
        setAROverlay(message.annotations)
      }
    }
    return () => channel.close()
  }, [meetingId])

  // 切换音频
  const toggleAudio = useCallback(() => {
    apiRef.current?.executeCommand("toggleAudio")
  }, [])

  const selectAudioInput = useCallback((deviceId: string) => {
    const api = apiRef.current
    const device = audioInputDevices.find((item) => item.deviceId === deviceId)
    if (!api || !device) return
    setActionError("")
    void api.setAudioInputDevice(device.label, device.deviceId)
      .then(() => setSelectedAudioInputId(device.deviceId))
      .catch((error) => {
        console.warn("Failed to select Jitsi audio input:", error)
        setActionError(ee("meetingLive.text014"))
      })
  }, [audioInputDevices])

  // 切换视频
  const toggleVideo = useCallback(() => {
    apiRef.current?.executeCommand("toggleVideo")
  }, [])

  // 切换屏幕共享
  const toggleScreenShare = useCallback(() => {
    apiRef.current?.executeCommand("toggleShareScreen")
  }, [])

  const toggleParticipants = useCallback(() => {
    apiRef.current?.executeCommand("toggleParticipantsPane")
  }, [])

  const toggleChat = useCallback(() => {
	    if (mobileLayout) {
	      setShowMobileChat((current) => !current)
	      return
	    }
	    apiRef.current?.executeCommand("toggleChat")
	  }, [mobileLayout])

	const sendMobileChat = useCallback(() => {
		const message = chatDraft.trim()
		if (!message || !apiRef.current) return
		apiRef.current.executeCommand("sendChatMessage", message)
		setChatMessages((current) => [...current.slice(-19), { id: `${Date.now()}-self`, sender: displayName, text: message }])
		setChatDraft("")
		setShowMobileChat(true)
	}, [chatDraft, displayName])

  const toggleTileView = useCallback(() => {
    apiRef.current?.executeCommand("toggleTileView")
  }, [])

  // 挂断
  const hangup = useCallback(() => {
    apiRef.current?.executeCommand("hangup")
    reportConferenceLeft()
    exitRoom()
  }, [exitRoom, reportConferenceLeft])

  const copyCustomerInvite = useCallback(async () => {
    if (!meetingId) return
    try {
      await navigator.clipboard.writeText(
        `${window.location.origin}/customer/meeting?meetingId=${encodeURIComponent(meetingId)}`,
      )
      setInviteCopied(true)
		toast.success(ee("meetingLive.text015"))
      window.setTimeout(() => setInviteCopied(false), 2000)
    } catch {
      toast.error(ee("meetingLive.text016"))
    }
  }, [meetingId])

  const endForEveryone = useCallback(async () => {
    if (!meetingId || endingMeeting) return
		setEndingMeeting(true)
		setActionError("")
    try {
      const result = await endMeeting(meetingId)
      if (!result.success) {
        throw new Error(result.error?.message || ee("meetingLive.text017"))
      }
      apiRef.current?.executeCommand("endConference")
		publishMeetingSync({ type: "meeting-ended", meetingId, status: "ended" })
      setEndConfirmOpen(false)
      reportConferenceLeft()
      exitRoom()
    } catch (error) {
			setActionError(error instanceof Error ? error.message : ee("meetingLive.text017"))
    } finally {
      setEndingMeeting(false)
    }
  }, [endingMeeting, exitRoom, meetingId, reportConferenceLeft])

	  const toggleTranscription = useCallback(() => {
	    if (!meetingId || !apiRef.current) return
	    if (transcriptionState === "connecting" || transcriptionState === "stopping") return
    if (transcriptionTimeoutRef.current) {
      clearTimeout(transcriptionTimeoutRef.current)
      transcriptionTimeoutRef.current = null
    }
    const enable = transcriptionState !== "active"
    setTranscriptionError("")
	    apiRef.current.executeCommand("setSubtitles", enable, false, transcriptionLanguage)
    if (!enable) {
      setTranscriptionState("stopping")
      transcriptionTimeoutRef.current = setTimeout(() => {
        setTranscriptionState("active")
        setTranscriptionError(ee("meetingLive.text018"))
        transcriptionTimeoutRef.current = null
      }, 15_000)
      return
    }
    setTranscriptionState("connecting")
    setShowTranscriptPanel(true)
    transcriptionTimeoutRef.current = setTimeout(() => {
      setTranscriptionState("idle")
      setTranscriptionError(ee("meetingLive.text019"))
      transcriptionTimeoutRef.current = null
    }, 15_000)
		  }, [meetingId, transcriptionLanguage, transcriptionState])

		useEffect(() => {
			if (
				!meetingId ||
				!transcriptionEnabled ||
				!transcriptionReady ||
				!isConferenceJoined ||
				transcriptionState !== "idle" ||
				autoTranscriptionRequestedRef.current ||
				!apiRef.current
			) {
				return
			}
			autoTranscriptionRequestedRef.current = true
			if (transcriptionTimeoutRef.current) {
				clearTimeout(transcriptionTimeoutRef.current)
				transcriptionTimeoutRef.current = null
			}
			setTranscriptionError("")
			apiRef.current.executeCommand("setSubtitles", true, false, transcriptionLanguage)
			setTranscriptionState("connecting")
			setShowTranscriptPanel(true)
			transcriptionTimeoutRef.current = setTimeout(() => {
				setTranscriptionState("idle")
				setTranscriptionError(ee("meetingLive.text019"))
				transcriptionTimeoutRef.current = null
			}, 15_000)
		}, [isConferenceJoined, meetingId, transcriptionEnabled, transcriptionLanguage, transcriptionReady, transcriptionState])

	const selectTranscriptionLanguage = useCallback((language: string) => {
		if (language !== "zh-CN" && language !== "en-US") return
		setTranscriptionLanguage(language)
		window.localStorage.setItem("remotehelpdesk:meeting-transcription-language", language)
	}, [])

	const captureARFrame = useCallback(async () => {
			if (!meetingId || capturingFrame) return
			setCapturingFrame(true)
			setActionError("")
    try {
		  const screenshot = await apiRef.current?.captureLargeVideoScreenshot?.()
		  const frameDataURL = screenshot?.dataURL || ""
		  if (!frameDataURL) throw new Error(ee("meetingLive.text020"))
	      const blob = await fetch(frameDataURL).then((response) => response.blob())
	      const uploaded = await uploadMeetingFrame(meetingId, blob)
	      if (!uploaded.success || !uploaded.data) throw new Error(uploaded.error?.message || ee("meetingLive.text021"))
	      setARFrame({ dataUrl: frameDataURL, assetId: uploaded.data.assetId })
    } catch (error) {
			setActionError(error instanceof Error ? error.message : ee("meetingLive.text022"))
    } finally {
      setCapturingFrame(false)
    }
		}, [capturingFrame, meetingId])

	const applyAROverlay = useCallback((annotations: MeetingARAnnotation[]) => {
		if (!meetingId || annotations.length === 0) return
		const message: MeetingAROverlayMessage = {
			type: "rhd.ar_overlay",
			meetingId,
			annotations,
			timestamp: Date.now(),
		}
		setAROverlay(annotations)
		apiRef.current?.executeCommand("sendEndpointTextMessage", "", JSON.stringify(message))
		if (typeof BroadcastChannel !== "undefined") {
			const channel = new BroadcastChannel(`remotehelpdesk:meeting-ar:${meetingId}`)
			channel.postMessage(message)
			channel.close()
		}
		setARFrame(null)
		toast.success(ee("meetingLive.text023"))
	}, [meetingId])

	const controlsDisabled = !isConferenceJoined || Boolean(meetingError)
	const useCompactControls = mobileLayout || compactControlsLayout

  return (
    <div
      ref={roomRootRef}
      className={cn("relative h-full min-h-[400px] w-full overflow-hidden bg-black", mobileLayout && "min-h-0", className)}
      style={{ "--meeting-transcript-width": "clamp(16rem, 28%, 22rem)" } as CSSProperties}
    >
      {/* Keep the embedded conference inside the space reserved for our chrome. */}
      <div
        className={cn(
          "absolute inset-0 min-h-0",
          useCompactControls
            ? "top-12 bottom-[calc(4.75rem+env(safe-area-inset-bottom))]"
            : "top-12 bottom-16",
          !mobileLayout && showTranscriptPanel && !compactPanelLayout && "right-[calc(var(--meeting-transcript-width)+0.75rem)]",
          !mobileLayout && showTranscriptPanel && compactPanelLayout && "right-0 bottom-[calc(min(38%,14rem)+4.5rem+env(safe-area-inset-bottom))]",
        )}
      >
        <div ref={containerRef} className="h-full w-full min-w-0" />
      </div>

		{!joinRequested ? (
			<div className="absolute inset-0 z-20 flex items-center justify-center bg-zinc-950 px-5 text-white">
				<div className="w-full max-w-lg text-center">
					<div className="mx-auto flex size-16 items-center justify-center rounded-full bg-white/10">
						<VideoIcon className="size-7" />
					</div>
						<h2 className="mt-5 text-xl font-semibold">{ee("meetingLive.text024")}</h2>
					<p className="mt-2 truncate text-sm text-white/65">{subject || roomName}</p>
					<div className="mt-6 grid grid-cols-2 border-y border-white/15 py-4 text-left text-sm">
						<div className="border-r border-white/15 px-4">
								<div className="text-white/50">{ee("meetingLive.text025")}</div>
							<div className="mt-1 truncate font-medium">{displayName}</div>
						</div>
						<div className="px-4">
								<div className="text-white/50">{ee("meetingLive.text026")}</div>
							<div className="mt-1 font-medium">{ee("meetingLive.text027")}</div>
						</div>
					</div>
					<div className="mt-6 flex flex-wrap justify-center gap-2">
						<RailopsButton onClick={exitRoom}>
							<ArrowLeftIcon className="size-4" />
							{returnLabel}
						</RailopsButton>
						<RailopsButton onClick={() => {
							setIsLoading(true)
							setJoinRequested(true)
						}}>
							<VideoIcon className="size-4" />{ee("meetingLive.text028")}</RailopsButton>
					</div>
				</div>
			</div>
		) : null}

      {/* 加载状态 */}
      {isLoading && (
        <div className="absolute inset-0 flex items-center justify-center bg-black/50">
          <div className="flex flex-col items-center gap-2 text-white">
            <Loader2Icon className="w-8 h-8 animate-spin" />
	            <span className="text-sm">{ee("meetingLive.text029")}</span>
          </div>
        </div>
      )}

      {meetingError ? (
              <div className="absolute inset-0 flex items-center justify-center bg-zinc-950 px-6 text-white">
          <div className="max-w-sm text-center">
            <p className="text-sm text-white/80">{meetingError}</p>
            <div className="mt-4 flex flex-wrap justify-center gap-2">
              <RailopsButton className="border-white/20 bg-transparent text-white hover:bg-white/10 hover:text-white" onClick={exitRoom}>
                <ArrowLeftIcon className="size-4" />
                {returnLabel}
              </RailopsButton>
              <RailopsButton onClick={() => window.location.reload()}>{ee("meetingLive.text030")}</RailopsButton>
            </div>
          </div>
        </div>
      ) : null}

      <div className="absolute inset-x-0 top-0 flex min-h-12 items-center justify-between gap-3 bg-black/65 px-3 text-white backdrop-blur-sm sm:px-4">
        <div className="flex min-w-0 items-center gap-2">
			<span className={cn(
				"size-2 shrink-0 rounded-full",
				meetingError ? "bg-destructive" : isConferenceJoined ? "bg-primary" : "bg-amber-400",
			)} />
          <span className="truncate text-sm font-medium">{subject || ee("meetingLive.text031")}</span>
          <span className="hidden truncate text-xs text-white/55 md:inline">{roomName}</span>
        </div>
        <div className="flex shrink-0 items-center gap-1.5 text-xs text-white/80">
          {connectionQuality === "good" || !connectionQuality ? (
            <WifiIcon className="size-3.5" />
          ) : (
            <WifiOffIcon className="size-3.5" />
          )}
          <span className="hidden sm:inline">{connectionLabel(connectionQuality, isConferenceJoined)}</span>
        </div>
      </div>

		{arOverlay.length > 0 && isConferenceJoined ? (
			<div className={cn(
				"pointer-events-none absolute bottom-20 left-0 right-0 top-12",
				!mobileLayout && showTranscriptPanel && !compactPanelLayout && "right-[calc(var(--meeting-transcript-width)+0.75rem)]",
				!mobileLayout && showTranscriptPanel && compactPanelLayout && "right-0 bottom-[calc(min(38%,14rem)+4.5rem+env(safe-area-inset-bottom))]",
			)}>
				{arOverlay.map((annotation) => (
					<LiveAnnotationBox key={annotation.id} annotation={annotation} />
				))}
			</div>
		) : null}

      {showTranscriptPanel ? (
        <aside className={cn(
          "absolute bottom-16 right-3 top-14 z-10 flex w-[var(--meeting-transcript-width)] flex-col overflow-hidden border border-white/15 bg-zinc-950/90 text-white shadow-xl backdrop-blur-sm",
          !mobileLayout && compactPanelLayout && "bottom-[calc(4.5rem+env(safe-area-inset-bottom))] left-3 right-3 top-auto h-[min(38%,14rem)] w-auto",
          mobileLayout && "hidden",
        )}>
          <div className="flex h-11 shrink-0 items-center justify-between border-b border-white/10 px-3">
            <div className="flex items-center gap-2">
              <CaptionsIcon className="size-4 text-blue-300" />
              <span className="text-sm font-medium">{ee("meetingLive.text032")}</span>
              <span className="text-xs text-white/50">
				{transcriptionProviderLabel(transcriptionProvider)} · {transcriptionLanguageLabel(transcriptionLanguage)}
              </span>
            </div>
            <IconButton
              variant="text"
              icon={<XIcon className="size-4" />}
              className="text-white/70 hover:bg-white/10 hover:text-white"
              onClick={() => setShowTranscriptPanel(false)}
              tooltip={ee("meetingLive.text033")}
              aria-label={ee("meetingLive.text033")}
            />
	          </div>
				{transcriptionNeedsAudio ? (
					<div className="border-b border-amber-300/20 bg-amber-400/10 px-3 py-2 text-xs leading-5 text-amber-100">{ee("meetingLive.text034")}</div>
				) : null}
		          <div ref={transcriptScrollRef} className="min-h-0 flex-1 overflow-y-auto p-3">
			{hasOlderTranscripts && fetchTranscriptPage ? (
				<div className="mb-3 flex justify-center">
					<RailopsButton
						variant="text"
						size="small"
						className="text-white/70 hover:bg-white/10 hover:text-white"
						onClick={() => void loadOlderTranscripts()}
						disabled={loadingOlderTranscripts}
					>
						{loadingOlderTranscripts ? <Loader2Icon className="animate-spin" /> : <ScrollTextIcon />}{ee("meetingLive.text035")}</RailopsButton>
				</div>
			) : null}
            {transcriptionState === "connecting" || transcriptionState === "stopping" ? (
              <div className="flex h-full flex-col items-center justify-center gap-2 text-white/60">
                <Loader2Icon className="size-5 animate-spin" />
                <span className="text-xs">
                  {transcriptionState === "stopping" ? ee("meetingLive.text036") : ee("meetingLive.text037")}
                </span>
              </div>
	            ) : captions.length === 0 ? (
	              <div className="flex h-full items-center justify-center text-xs text-white/50">
					{transcriptionState === "active" ? ee("meetingLive.text038") : ee("meetingLive.text039")}
				  </div>
            ) : (
              <div className="space-y-3">
                {captions.map((item) => (
                  <div key={item.providerEventId} className="text-sm leading-6">
                    {item.speakerName ? <div className="mb-0.5 text-xs font-medium text-blue-300">{item.speakerName}</div> : null}
                    <p className="text-white/90">{item.text}</p>
                    {item.translatedText ? <p className="mt-0.5 text-blue-200">{item.translatedText}</p> : null}
                  </div>
                ))}
              </div>
            )}
          </div>
        </aside>
      ) : null}

		{mobileLayout && isConferenceJoined && (transcriptionEnabled || captions.length > 0 || showMobileChat) ? (
			<div className="pointer-events-none absolute bottom-[calc(5.75rem+env(safe-area-inset-bottom))] left-3 right-3 z-10 flex max-h-[42vh] flex-col justify-end gap-2">
				{(transcriptionEnabled || showTranscriptPanel) ? captions.slice(-4).map((item) => (
					<div key={`caption-${item.providerEventId}`} className="w-fit max-w-[88%] rounded-xl bg-black/65 px-3 py-2 text-sm leading-5 text-white shadow-lg backdrop-blur-sm">
						{item.speakerName ? <span className="mr-1 text-xs font-medium text-blue-200">{item.speakerName}</span> : null}
						<span>{item.text}</span>
						{item.translatedText ? <span className="mt-0.5 block text-xs text-blue-100">{item.translatedText}</span> : null}
					</div>
				)) : null}
				{showMobileChat ? chatMessages.slice(-4).map((item) => (
					<div key={item.id} className="w-fit max-w-[88%] rounded-xl bg-white/90 px-3 py-2 text-sm leading-5 text-zinc-900 shadow-lg">
						<span className="mr-1 text-xs font-medium text-blue-700">{item.sender}</span><span>{item.text}</span>
					</div>
				)) : null}
			</div>
		) : null}

			{actionError || transcriptionError ? (
			<div className="absolute left-3 top-14 flex max-w-sm items-start gap-2 bg-destructive/90 px-3 py-2 text-xs text-white" role="alert">
				<span className="min-w-0 flex-1">{actionError || transcriptionError}</span>
				<IconButton
					variant="text"
					icon={<XIcon className="size-4" />}
					className="-mr-1 -mt-1 shrink-0 text-white hover:bg-white/15 hover:text-white"
					onClick={() => {
						setActionError("")
						setTranscriptionError("")
					}}
					tooltip={ee("meetingLive.text040")}
					aria-label={ee("meetingLive.text040")}
				/>
			</div>
		) : null}

      {/* 底部控制栏 */}
		{isConferenceJoined && !useCompactControls ? <div
        className={cn("absolute bottom-0 left-0 right-0 flex justify-start gap-2 overflow-x-auto bg-gradient-to-t from-black/60 to-transparent p-4 sm:justify-center", mobileLayout && "hidden")}
      >
		<IconButton
          danger={isAudioMuted}
          icon={isAudioMuted ? <MicOffIcon className="w-5 h-5" /> : <MicIcon className="w-5 h-5" />}
			onClick={toggleAudio}
			disabled={controlsDisabled}
          className="rounded-full"
			tooltip={isAudioMuted ? ee("meetingLive.text041") : ee("meetingLive.text042")}
          aria-label={isAudioMuted ? ee("meetingLive.text041") : ee("meetingLive.text042")}
        />
		{audioInputDevices.length > 0 ? (
			<DropdownMenu>
				<DropdownMenuTrigger
					render={<RailopsButton size="small" className="rounded-full" />}
					disabled={controlsDisabled}
					aria-label={ee("meetingLive.text043")}
					title={ee("meetingLive.text043")}
				>
					<Settings2Icon className="size-5" />
				</DropdownMenuTrigger>
				<DropdownMenuContent side="top" align="start" className="w-72 min-w-72">
					<DropdownMenuGroup>
						<DropdownMenuLabel>{ee("meetingLive.text044")}</DropdownMenuLabel>
						<DropdownMenuRadioGroup value={selectedAudioInputId} onValueChange={selectAudioInput}>
							{audioInputDevices.map((device) => (
								<DropdownMenuRadioItem key={device.deviceId} value={device.deviceId}>
									<span className="truncate">{device.label || ee("meetingLive.text045")}</span>
								</DropdownMenuRadioItem>
							))}
						</DropdownMenuRadioGroup>
					</DropdownMenuGroup>
				</DropdownMenuContent>
			</DropdownMenu>
		) : null}
		<IconButton
          danger={isVideoMuted}
          icon={isVideoMuted ? <CameraOffIcon className="w-5 h-5" /> : <CameraIcon className="w-5 h-5" />}
			onClick={toggleVideo}
			disabled={controlsDisabled}
          className="rounded-full"
			tooltip={isVideoMuted ? ee("meetingLive.text046") : ee("meetingLive.text047")}
          aria-label={isVideoMuted ? ee("meetingLive.text046") : ee("meetingLive.text047")}
        />
		<IconButton
          variant={isScreenSharing ? "primary" : undefined}
          icon={<MonitorIcon className="w-5 h-5" />}
			onClick={toggleScreenShare}
			disabled={controlsDisabled}
          className="rounded-full"
			tooltip={isScreenSharing ? ee("meetingLive.text048") : ee("meetingLive.text049")}
          aria-label={isScreenSharing ? ee("meetingLive.text048") : ee("meetingLive.text049")}
        />
		<IconButton
          icon={<UsersIcon className="size-5" />}
			onClick={toggleParticipants}
			disabled={controlsDisabled}
          className="rounded-full"
			tooltip={ee("meetingLive.text050")}
          aria-label={ee("meetingLive.text050")}
        />
		<IconButton
          icon={<MessageSquareIcon className="size-5" />}
			onClick={toggleChat}
			disabled={controlsDisabled}
          className="rounded-full"
			tooltip={ee("meetingLive.text051")}
          aria-label={ee("meetingLive.text051")}
        />
		<IconButton
          icon={<Grid2X2Icon className="size-5" />}
			onClick={toggleTileView}
			disabled={controlsDisabled}
          className="rounded-full"
			tooltip={ee("meetingLive.text052")}
          aria-label={ee("meetingLive.text052")}
        />
		{meetingId && transcriptionEnabled ? (
			<DropdownMenu>
				<DropdownMenuTrigger
					render={<RailopsButton size="small" className="rounded-full" />}
					disabled={transcriptionState !== "idle"}
					aria-label={ee("meetingLive.text053", { value0: transcriptionLanguageLabel(transcriptionLanguage) })}
					title={ee("meetingLive.text053", { value0: transcriptionLanguageLabel(transcriptionLanguage) })}
				>
					<LanguagesIcon className="size-5" />
				</DropdownMenuTrigger>
				<DropdownMenuContent side="top" align="center" className="w-44 min-w-44">
					<DropdownMenuLabel>{ee("meetingLive.text054")}</DropdownMenuLabel>
					<DropdownMenuRadioGroup value={transcriptionLanguage} onValueChange={selectTranscriptionLanguage}>
						<DropdownMenuRadioItem value="zh-CN">{ee("meetingLive.text004")}</DropdownMenuRadioItem>
						<DropdownMenuRadioItem value="en-US">English</DropdownMenuRadioItem>
					</DropdownMenuRadioGroup>
				</DropdownMenuContent>
			</DropdownMenu>
		) : null}
		        {meetingId ? (
			  <IconButton
				variant={showTranscriptPanel ? "primary" : undefined}
				icon={<ScrollTextIcon className="size-5" />}
				onClick={() => setShowTranscriptPanel((current) => !current)}
				disabled={captions.length === 0 && !fetchTranscripts && !fetchTranscriptPage}
				tooltip={showTranscriptPanel ? ee("meetingLive.text033") : ee("meetingLive.text055")}
				aria-label={showTranscriptPanel ? ee("meetingLive.text033") : ee("meetingLive.text055")}
			  />
			) : null}
		        {meetingId ? (
	          <IconButton
            variant={transcriptionState === "active" || transcriptionState === "stopping" ? "primary" : undefined}
            icon={transcriptionState === "connecting" || transcriptionState === "stopping" ? <Loader2Icon className="size-5 animate-spin" /> : <CaptionsIcon className="size-5" />}
            onClick={toggleTranscription}
            disabled={!transcriptionEnabled || !transcriptionReady || !isConferenceJoined || transcriptionState === "connecting" || transcriptionState === "stopping"}
			tooltip={
              !transcriptionEnabled
                ? ee("meetingLive.text056")
                : !transcriptionReady
                  ? transcriptionUnavailableReason || ee("meetingLive.text057")
		                : transcriptionState === "active"
		                  ? ee("meetingLive.text058")
	                  : isAudioMuted
	                    ? ee("meetingLive.text059")
	                    : ee("meetingLive.text060")
	            }
            aria-label={transcriptionState === "active" ? ee("meetingLive.text058") : ee("meetingLive.text060")}
          />
        ) : null}
	        {meetingId && arEnabled ? (
          <IconButton
            icon={capturingFrame ? <Loader2Icon className="size-5 animate-spin" /> : <ScanLineIcon className="size-5" />}
            onClick={() => void captureARFrame()}
			disabled={controlsDisabled || capturingFrame || !arCaptureSupported}
			tooltip={ee("meetingLive.text061")}
			aria-label={ee("meetingLive.text061")}
          />
        ) : null}
	        {meetingId && customerInviteEnabled ? (
          <IconButton
            icon={inviteCopied ? <CheckIcon className="size-5" /> : <UserPlusIcon className="size-5" />}
            onClick={() => void copyCustomerInvite()}
			tooltip={inviteCopied ? ee("meetingLive.text015") : ee("meetingLive.text062")}
			aria-label={inviteCopied ? ee("meetingLive.text015") : ee("meetingLive.text062")}
          />
        ) : null}
        <IconButton
          icon={<LogOutIcon className="w-5 h-5" />}
          onClick={hangup}
          className="rounded-full"
		  tooltip={ee("meetingLive.text063")}
          aria-label={ee("meetingLive.text063")}
        />
        {canEndMeeting && meetingId ? (
          <IconButton
            danger
            icon={<PhoneOffIcon className="w-5 h-5" />}
            onClick={() => setEndConfirmOpen(true)}
            className="rounded-full"
			tooltip={ee("meetingLive.text064")}
            aria-label={ee("meetingLive.text064")}
          />
        ) : null}
	      </div> : null}

		{useCompactControls && isConferenceJoined ? (
			<>
				{canEndMeeting ? (
					<div className={cn("absolute right-3 z-20", actionError || transcriptionError ? "top-28" : "top-14")}>
						<IconButton danger icon={<PhoneOffIcon className="size-5" />} onClick={() => setEndConfirmOpen(true)} className="size-11 rounded-full bg-red-600 text-white hover:bg-red-500" tooltip={ee("meetingLive.text064")} aria-label={ee("meetingLive.text064")} />
					</div>
				) : (
					<div className={cn("absolute right-3 z-20", actionError || transcriptionError ? "top-28" : "top-14")}>
						<IconButton icon={<LogOutIcon className="size-5" />} onClick={hangup} className="size-11 rounded-full bg-white/15 text-white hover:bg-white/25" tooltip={ee("meetingLive.text063")} aria-label={ee("meetingLive.text063")} />
					</div>
				)}
				{mobileLayout && showMobileChat ? (
					<form className="absolute bottom-[calc(4.3rem+env(safe-area-inset-bottom))] left-3 right-3 z-20 flex gap-2" onSubmit={(event) => { event.preventDefault(); sendMobileChat() }}>
						<input value={chatDraft} onChange={(event) => setChatDraft(event.target.value)} placeholder={ee("meetingLive.text051")} className="min-w-0 flex-1 rounded-full border border-white/20 bg-black/80 px-4 py-2.5 text-sm text-white outline-none placeholder:text-white/50 focus:border-blue-300" aria-label={ee("meetingLive.text051")} />
						<button type="submit" className="size-11 shrink-0 rounded-full bg-blue-600 text-sm font-semibold text-white disabled:opacity-50" disabled={!chatDraft.trim()} aria-label={ee("meetingLive.text051")}><SendIcon className="mx-auto size-4" /></button>
					</form>
				) : null}
				<div className="absolute bottom-0 left-0 right-0 z-20 flex items-center justify-center gap-3 bg-black/75 px-4 py-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))]">
					<IconButton danger={isAudioMuted} icon={isAudioMuted ? <MicOffIcon className="size-6" /> : <MicIcon className="size-6" />} onClick={toggleAudio} disabled={controlsDisabled} className="size-12 rounded-full" tooltip={isAudioMuted ? ee("meetingLive.text041") : ee("meetingLive.text042")} aria-label={isAudioMuted ? ee("meetingLive.text041") : ee("meetingLive.text042")} />
					<IconButton danger={isVideoMuted} icon={isVideoMuted ? <CameraOffIcon className="size-6" /> : <CameraIcon className="size-6" />} onClick={toggleVideo} disabled={controlsDisabled} className="size-12 rounded-full" tooltip={isVideoMuted ? ee("meetingLive.text046") : ee("meetingLive.text047")} aria-label={isVideoMuted ? ee("meetingLive.text046") : ee("meetingLive.text047")} />
					<IconButton variant={isScreenSharing ? "primary" : undefined} icon={<MonitorIcon className="size-6" />} onClick={toggleScreenShare} disabled={controlsDisabled} className="size-12 rounded-full" tooltip={ee("meetingLive.text049")} aria-label={ee("meetingLive.text049")} />
					<IconButton icon={<UsersIcon className="size-6" />} onClick={toggleParticipants} disabled={controlsDisabled} className="size-12 rounded-full" tooltip={ee("meetingLive.text050")} aria-label={ee("meetingLive.text050")} />
					<DropdownMenu>
						<DropdownMenuTrigger render={<RailopsButton size="small" className="size-12 rounded-full" />} aria-label={ee("meetingLive.text052")} title={ee("meetingLive.text052")}><Settings2Icon className="size-6" /></DropdownMenuTrigger>
						<DropdownMenuContent side="top" align="end" className="w-52 min-w-52">
							<DropdownMenuGroup>
								<DropdownMenuLabel>{ee("meetingLive.text052")}</DropdownMenuLabel>
								<button type="button" className="flex w-full items-center gap-2 px-2 py-2 text-left text-sm hover:bg-muted" onClick={toggleChat}><MessageSquareIcon className="size-4" />{ee("meetingLive.text051")}</button>
								<button type="button" className="flex w-full items-center gap-2 px-2 py-2 text-left text-sm hover:bg-muted" onClick={() => setShowTranscriptPanel((current) => !current)}><CaptionsIcon className="size-4" />{ee("meetingLive.text032")}</button>
								<button type="button" className="flex w-full items-center gap-2 px-2 py-2 text-left text-sm hover:bg-muted" onClick={toggleTileView}><Grid2X2Icon className="size-4" />{ee("meetingLive.text052")}</button>
							</DropdownMenuGroup>
						</DropdownMenuContent>
					</DropdownMenu>
				</div>
			</>
		) : null}
		      {arFrame && meetingId && arEnabled ? (
        <MeetingARWorkspace
          meetingId={meetingId}
          frameAssetId={arFrame.assetId}
          frameDataUrl={arFrame.dataUrl}
          onApply={applyAROverlay}
          onClose={() => setARFrame(null)}
        />
      ) : null}
      <StandardModal
        open={endConfirmOpen}
        onCancel={() => setEndConfirmOpen(false)}
        title={ee("meetingLive.text064")}
        footer={
          <>
            <RailopsButton onClick={() => setEndConfirmOpen(false)} disabled={endingMeeting}>{ee("meetingLive.text065")}</RailopsButton>
            <RailopsButton danger onClick={() => void endForEveryone()} disabled={endingMeeting}>
              {endingMeeting ? <Loader2Icon className="size-4 animate-spin" /> : <PhoneOffIcon className="size-4" />}{ee("meetingLive.text066")}</RailopsButton>
          </>
        }
      />
    </div>
  )
}

function connectionLabel(quality: "good" | "poor" | "very-poor" | null, joined: boolean) {
  if (quality === "good") return ee("meetingLive.text067")
  if (quality === "poor") return ee("meetingLive.text068")
  if (quality === "very-poor") return ee("meetingLive.text069")
  if (joined) return ee("meetingLive.text070")
  return ee("meetingLive.text071")
}

function parseAROverlayMessage(payload: unknown): MeetingAROverlayMessage | null {
  const record = isRecord(payload) ? payload : null
  const data = record && isRecord(record.data) ? record.data : null
  const eventData = data && isRecord(data.eventData) ? data.eventData : null
  const candidates: unknown[] = [payload, record?.text, record?.data, data?.text, eventData?.text]
  for (const candidate of candidates) {
    let parsed = candidate
    if (typeof candidate === "string") {
      try {
        parsed = JSON.parse(candidate)
      } catch {
        continue
      }
    }
    if (!isRecord(parsed) || parsed.type !== "rhd.ar_overlay" || !Array.isArray(parsed.annotations)) continue
    return {
      type: "rhd.ar_overlay",
      meetingId: typeof parsed.meetingId === "string" ? parsed.meetingId : "",
      annotations: parsed.annotations as MeetingARAnnotation[],
      timestamp: typeof parsed.timestamp === "number" ? parsed.timestamp : Date.now(),
    }
  }
  return null
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null
}

function LiveAnnotationBox({ annotation }: { annotation: MeetingARAnnotation }) {
  const { bounds } = annotation
  return (
    <div
      className="absolute border-2"
      style={{
        left: `${bounds.x * 100}%`,
        top: `${bounds.y * 100}%`,
        width: `${bounds.width * 100}%`,
        height: `${bounds.height * 100}%`,
        borderColor: annotation.color,
      }}
    >
      <span
        className="absolute left-0 top-0 max-w-48 -translate-y-full truncate px-1.5 py-1 text-xs font-medium text-white shadow"
        style={{ backgroundColor: annotation.color }}
      >
        {annotation.label}{annotation.partCode ? ` · ${annotation.partCode}` : ""}
        <span className="ml-1 opacity-80">{Math.round(annotation.confidence * 100)}%</span>
      </span>
    </div>
  )
}
