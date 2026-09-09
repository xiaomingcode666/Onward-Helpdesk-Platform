"use client"

import { useCallback, useEffect, useRef, useState } from "react"
import { useRouter, useSearchParams } from "next/navigation"

import {
  buildCustomerEntryState,
  confirmCustomerPrivacyConsent,
  fetchEntrySessionContext,
  resolveServiceCode,
  type CustomerEntryContext,
  type CustomerEntryChat,
  type CustomerEntrySessionResponse,
  type CustomerEntryState,
} from "@/lib/api/customer-entry"
import type { CookieConsent } from "@/app/c/[serviceCode]/components/privacy-consent"
import {
  MobileServiceShell,
  type MobileServiceTab,
} from "@/components/remote-helpdesk/mobile-service-shell"
import { CustomerEntryPresenceBridge } from "@/components/remote-helpdesk/customer-entry-presence-bridge"
import { selectCustomerRealtimeConversationId } from "@/lib/customer-entry-realtime"
import { useAuth } from "@/components/auth-provider"
import { bindCustomerDevice, fetchCustomerDeviceAccess } from "@/lib/api/customer-devices"
import { createOrMatchImConversation } from "@/lib/api/im"
import { MobileCustomerLogin } from "@/components/remote-helpdesk/mobile-customer-login"
import { useI18n } from "@/i18n/provider"
import { AUTH_SESSION_EXPIRED_EVENT } from "@/lib/auth"

const CUSTOMER_ENTRY_SESSION_KEY_PREFIX = "remote_helpdesk_customer_entry_session:"
const CUSTOMER_ENTRY_PENDING_TOKEN_KEY_PREFIX = "remote_helpdesk_customer_entry_pending_token:"

// Mobile state machine:
//   resolving -> device_info -> chat -> diagnosis -> ticket -> video -> history
// Supports branching from chat to any sub-state, and returning to chat.
type MobileState =
  | "resolving"        // Resolving service code
  | "device_info"      // Device info display
  | "login"            // Login / identity
  | "privacyConsent"   // Privacy consent
  | "chat"             // Chat after service-code resolution
  | "voice"            // Voice support
  | "diagnosis"        // Diagnosis
  | "guide"            // Step-by-step guide
  | "tickets"          // Ticket list
  | "ticketDetail"     // Ticket detail
  | "ticketVideo"      // Video linked to ticket
  | "video"            // Video meeting (alias for ticketVideo in the machine)
  | "meeting"          // Meeting panel
  | "devices"          // Device info
  | "manual"           // Operation manual
  | "history"          // Repair history
  | "my"               // My account

const MOBILE_STATE_VALUES: MobileState[] = [
  "resolving",
  "device_info",
  "login",
  "privacyConsent",
  "chat",
  "voice",
  "diagnosis",
  "guide",
  "tickets",
  "ticketDetail",
  "ticketVideo",
  "video",
  "meeting",
  "devices",
  "manual",
  "history",
  "my",
]
const MOBILE_STATE_SET = new Set<MobileState>(MOBILE_STATE_VALUES)

function normalizeMobileState(value?: string | null): MobileState {
  return MOBILE_STATE_SET.has(value as MobileState) ? value as MobileState : "resolving"
}

// Bottom tabs shown when guestSessionReady
const BOTTOM_TABS: MobileState[] = ["chat", "tickets", "video", "devices", "my"]
const BOTTOM_TAB_TARGETS: MobileState[] = [
  "chat",
  "tickets",
  "ticketVideo",
  "devices",
  "my",
]
const LIVE_CONTEXT_STATES = new Set<MobileState>([
  "chat",
  "tickets",
  "ticketDetail",
  "ticketVideo",
])
const PRIVACY_PROTECTED_STATES = new Set<MobileState>([
  "chat",
  "voice",
  "diagnosis",
  "guide",
  "tickets",
  "ticketDetail",
  "ticketVideo",
  "video",
  "meeting",
  "devices",
  "manual",
  "history",
  "my",
])
const CONTEXT_STATES = new Set<MobileState>([
  ...PRIVACY_PROTECTED_STATES,
])

// State machine transition map:
// Each state can transition to the listed target states.
const STATE_TRANSITIONS: Record<MobileState, MobileState[]> = {
  device_info:    ["privacyConsent"],
  chat:           ["diagnosis", "tickets", "ticketDetail", "ticketVideo", "video", "history", "voice", "guide", "manual", "device_info"],
  diagnosis:      ["chat", "tickets", "guide", "history"],
  guide:          ["chat", "diagnosis", "tickets", "history"],
  tickets:        ["ticketDetail", "chat", "diagnosis", "history"],
  ticketDetail:   ["tickets", "chat", "ticketVideo"],
  ticketVideo:    ["ticketDetail", "chat", "meeting"],
  video:          ["chat", "tickets", "history"],
  meeting:        ["chat", "tickets"],
  history:        ["chat", "tickets", "diagnosis"],
  manual:         ["chat", "devices"],
  devices:        ["chat", "manual", "my"],
  my:             ["chat", "history", "devices"],
  voice:          ["chat", "diagnosis"],
  login:          ["chat"],
  privacyConsent: ["chat", "login"],
  resolving:      ["chat", "login"],
}

function storeEntrySession(serviceCode: string, session: CustomerEntrySessionResponse) {
  if (typeof window === "undefined") {
    return
  }
  window.localStorage.setItem(
    `${CUSTOMER_ENTRY_SESSION_KEY_PREFIX}${serviceCode}`,
    JSON.stringify(session)
  )
  window.sessionStorage.removeItem(
    `${CUSTOMER_ENTRY_SESSION_KEY_PREFIX}${serviceCode}`
  )
  window.localStorage.removeItem(
    `${CUSTOMER_ENTRY_PENDING_TOKEN_KEY_PREFIX}${serviceCode}`
  )
  window.sessionStorage.removeItem(
    `${CUSTOMER_ENTRY_PENDING_TOKEN_KEY_PREFIX}${serviceCode}`
  )
}

function clearStoredEntrySession(serviceCode: string) {
  if (typeof window === "undefined") {
    return
  }
  const key = `${CUSTOMER_ENTRY_SESSION_KEY_PREFIX}${serviceCode}`
  window.localStorage.removeItem(key)
  window.sessionStorage.removeItem(key)
}

function toErrorMessage(error: unknown, fallback: string) {
  if (error instanceof Error && error.message.trim()) {
    return error.message
  }
  return fallback
}

function getEntrySessionIdFromState(state: CustomerEntryState | null) {
  if (!state || state.kind !== "guestSessionReady") {
    return 0
  }
  const value = state.session?.entrySessionId ?? state.session?.id
  if (typeof value === "number") {
    return value
  }
  if (typeof value === "string") {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? parsed : 0
  }
  return 0
}

function getEntryChatFromState(state: CustomerEntryState | null) {
  if (!state || state.kind !== "guestSessionReady") {
    return null
  }
  return (state.session as { chat?: CustomerEntryChat } | undefined)?.chat ?? null
}

function buildDeviceAccessErrorState(
  state: Extract<CustomerEntryState, { kind: "needRegister" }>,
  reason: string,
): Extract<CustomerEntryState, { kind: "accessError" }> {
  return {
    kind: "accessError",
    serviceCode: state.serviceCode,
    reason,
    tenant: state.tenant,
    product: state.product,
    device: state.device,
  }
}

type MobileServiceClientProps = {
  serviceCode: string
  initialState?: string
  initialStateId?: string
  /** App builds use a static host route because service codes are resolved client-side. */
  hostRoute?: "serviceCode" | "mobile"
}

export function MobileServiceClient({
  serviceCode,
  initialState = "resolving",
  initialStateId,
  hostRoute = "serviceCode",
}: MobileServiceClientProps) {
  const router = useRouter()
  const searchParams = useSearchParams()
  const t = useI18n()
	const { ready: authReady, session: accountSession, refreshProfile } = useAuth()

  // State routing
  const currentState = normalizeMobileState(searchParams?.get("state") || initialState)
  const detailId = searchParams?.get("id") || initialStateId

  const [entryState, setEntryState] = useState<CustomerEntryState | null>(null)
  const [deviceNo, setDeviceNo] = useState("")
  const [loading, setLoading] = useState(true)
	const sessionStarting = false
  const [registering, setRegistering] = useState(false)
  const [error, setError] = useState("")
  const [subStateLoading, setSubStateLoading] = useState(false)
  const [subStateError, setSubStateError] = useState("")
  const [customerContext, setCustomerContext] =
    useState<CustomerEntryContext | null>(null)
  const [contextLoading, setContextLoading] = useState(false)
  const [contextError, setContextError] = useState("")
  const [privacyConsentError, setPrivacyConsentError] = useState("")

  // Track state machine transitions (for analytics / debugging)
  const stateHistory = useRef<string[]>([currentState])
  const currentStateRef = useRef(currentState)
  const privacyConsentAcceptedRef = useRef(false)
	const serviceCodeLoadRevisionRef = useRef(0)

  useEffect(() => {
    currentStateRef.current = currentState
  }, [currentState])

  // State routing helper
  const navigateTo = useCallback(
    (requestedState: string, extraParams?: Record<string, string>) => {
      const requestedMobileState = requestedState as MobileState
      const targetState =
        PRIVACY_PROTECTED_STATES.has(requestedMobileState) &&
        !privacyConsentAcceptedRef.current
          ? "privacyConsent"
          : requestedState
      // Validate transition
      const sourceState = currentStateRef.current
      if (sourceState === targetState && !extraParams) {
        return
      }
      const validTargets = STATE_TRANSITIONS[sourceState]
      if (
        validTargets &&
        !validTargets.includes(targetState as MobileState) &&
        !BOTTOM_TAB_TARGETS.includes(targetState as MobileState) &&
        targetState !== "privacyConsent" &&
        sourceState !== "resolving"
      ) {
        console.warn(
          `[MobileServiceClient] Invalid transition: ${sourceState} -> ${targetState}`
        )
        return
      }

      const params = new URLSearchParams()
      params.set("state", targetState)
      if (extraParams) {
        for (const [k, v] of Object.entries(extraParams)) {
          if (v) params.set(k, v)
        }
      }
      const path = hostRoute === "mobile" ? "/mobile" : `/c/${serviceCode}`
      const routeParams = new URLSearchParams()
      if (hostRoute === "mobile") {
        routeParams.set("serviceCode", serviceCode)
      }
      for (const [key, value] of params.entries()) {
        routeParams.set(key, value)
      }
      const routeQuery = routeParams.toString()
      router.push(`${path}${routeQuery ? `?${routeQuery}` : ""}`)
      stateHistory.current.push(targetState)
    },
    [hostRoute, router, serviceCode]
  )

  const isTabState = BOTTOM_TABS.includes(currentState)
  const activeTab: MobileServiceTab = isTabState
    ? (currentState as MobileServiceTab)
    : "chat"

  const handleTabChange = useCallback(
    (tab: MobileServiceTab) => {
      if (tab === currentState) return
      // Map bottom tab "video" -> internal "ticketVideo" state
      const targetState = tab === "video" ? "ticketVideo" : tab
      navigateTo(targetState)
    },
    [currentState, navigateTo]
  )

  useEffect(() => {
    setSubStateLoading(false)
    setSubStateError("")
  }, [currentState])

  // ---- Token refresh (auto-refresh access token) ----
  const [tokenRefreshed, setTokenRefreshed] = useState(false)
  useEffect(() => {
    if (!entryState || entryState.kind !== "guestSessionReady") return
    if (tokenRefreshed) return

    const handleAuthExpired = () => {
      navigateTo("login")
    }
    window.addEventListener(AUTH_SESSION_EXPIRED_EVENT, handleAuthExpired)
    setTokenRefreshed(true)

    return () => {
      window.removeEventListener(
        AUTH_SESSION_EXPIRED_EVENT,
        handleAuthExpired
      )
    }
  }, [entryState, navigateTo, tokenRefreshed])

  const openDeviceConversation = useCallback(async (deviceId: number, loadRevision?: number, forceNew = false) => {
    const conversation = await createOrMatchImConversation({ deviceId, forceNew })
    if (loadRevision !== undefined && loadRevision !== serviceCodeLoadRevisionRef.current) {
      return false
    }
    router.replace(`/customer/chat?conversationId=${conversation.id}`)
    return true
  }, [router])

	const loadServiceCode = useCallback(async () => {
		const loadRevision = ++serviceCodeLoadRevisionRef.current
		if (!authReady) {
			return
		}
    setLoading(true)
    setError("")
    setDeviceNo("")
    setCustomerContext(null)
    setContextError("")
    try {
      if (!serviceCode.trim()) {
        setEntryState(buildCustomerEntryState(null))
        return
      }

      const result = await resolveServiceCode(serviceCode)
      const nextState = buildCustomerEntryState(result)

			// Remove credentials created by the retired guest-entry flow. A service
			// code now only identifies a device; account binding is the auth boundary.
			clearStoredEntrySession(serviceCode)
			privacyConsentAcceptedRef.current = false

			const signedInCustomer = Boolean(
				accountSession?.accessToken && accountSession.domainType === "customer"
			)
			const resolvedDeviceId = Number(result.device?.id || 0)
			if (signedInCustomer && nextState.kind === "needRegister" && resolvedDeviceId > 0) {
				try {
					const access = await fetchCustomerDeviceAccess(resolvedDeviceId)
					if (loadRevision !== serviceCodeLoadRevisionRef.current) {
						return
					}
					if (access.accessible) {
						try {
							await openDeviceConversation(resolvedDeviceId, loadRevision)
							return
						} catch (value) {
							const reason = t("portalExtract.serviceCode.client.deviceBoundConversationUnavailable", {
								error: toErrorMessage(value, t("portalExtract.serviceCode.client.entryUnavailable")),
							})
							setError(reason)
							setEntryState(buildDeviceAccessErrorState(nextState, reason))
							return
						}
					}
				} catch (value) {
					if (loadRevision !== serviceCodeLoadRevisionRef.current) {
						return
					}
					const reason = t("portalExtract.serviceCode.client.deviceRecognizedAccessUnavailable", {
						error: toErrorMessage(value, t("portalExtract.serviceCode.client.entryUnavailable")),
					})
					setError(reason)
					setEntryState(buildDeviceAccessErrorState(nextState, reason))
					return
				}
			}
			if (loadRevision !== serviceCodeLoadRevisionRef.current) {
				return
			}
			setEntryState(nextState)
    } catch (resolveError) {
			if (loadRevision !== serviceCodeLoadRevisionRef.current) {
				return
			}
      setEntryState(
        buildCustomerEntryState({
          valid: false,
          serviceCode,
          reason: toErrorMessage(resolveError, t("portalExtract.serviceCode.client.entryUnavailable")),
        })
      )
      setError(toErrorMessage(resolveError, t("portalExtract.serviceCode.client.entryUnavailable")))
    } finally {
			if (loadRevision === serviceCodeLoadRevisionRef.current) {
				setLoading(false)
			}
    }
		}, [accountSession?.accessToken, accountSession?.domainType, authReady, hostRoute, openDeviceConversation, serviceCode, t])

  useEffect(() => {
    void loadServiceCode()
  }, [loadServiceCode])

  const currentEntrySessionId = getEntrySessionIdFromState(entryState)
  const currentEntryChat = getEntryChatFromState(entryState)
  const loadEntryContext = useCallback(async (options?: { silent?: boolean }) => {
    if (
      !currentEntrySessionId ||
      !currentEntryChat ||
      !privacyConsentAcceptedRef.current ||
      !CONTEXT_STATES.has(currentStateRef.current)
    ) {
      setCustomerContext(null)
      return
    }
    if (!options?.silent) {
      setContextLoading(true)
    }
    setContextError("")
    try {
      const context = await fetchEntrySessionContext(
        currentEntrySessionId,
        currentEntryChat.visitorId,
        currentEntryChat.visitorToken
      )
      setCustomerContext(context)
    } catch (contextErrorValue) {
      setContextError(toErrorMessage(contextErrorValue, t("portalExtract.serviceCode.client.entryUnavailable")))
    } finally {
      if (!options?.silent) {
        setContextLoading(false)
      }
    }
  }, [currentEntryChat, currentEntrySessionId, t])

  const retryEntryContext = useCallback(() => {
    void loadEntryContext()
  }, [loadEntryContext])

  useEffect(() => {
    void loadEntryContext()
  }, [currentState, loadEntryContext])

  useEffect(() => {
    if (!LIVE_CONTEXT_STATES.has(currentState) || !currentEntrySessionId) {
      return
    }
    const refreshVisibleContext = () => {
      if (document.visibilityState === "visible") {
        void loadEntryContext({ silent: true })
      }
    }
    const interval = window.setInterval(refreshVisibleContext, 5_000)
    window.addEventListener("focus", refreshVisibleContext)
    document.addEventListener("visibilitychange", refreshVisibleContext)
    return () => {
      window.clearInterval(interval)
      window.removeEventListener("focus", refreshVisibleContext)
      document.removeEventListener("visibilitychange", refreshVisibleContext)
    }
  }, [currentEntrySessionId, currentState, loadEntryContext])

	const handleRegisterDevice = useCallback(async () => {
		if (
		  entryState?.kind !== "needRegister" ||
		  (!entryState.device && !deviceNo.trim())
		) {
		  return
		}
			if (!authReady || !accountSession?.accessToken || accountSession.domainType !== "customer") {
			  navigateTo("login")
			  return
		}
		setRegistering(true)
		setError("")
		let binding: Awaited<ReturnType<typeof bindCustomerDevice>>
		try {
		  binding = await bindCustomerDevice({
			serviceCode,
			...(deviceNo.trim() ? { deviceNo: deviceNo.trim() } : {}),
		  })
		} catch (value) {
		  setError(toErrorMessage(value, t("portalExtract.serviceCode.client.entryUnavailable")))
		  setRegistering(false)
		  return
		}
		try {
			  await refreshProfile()
			  if (hostRoute === "mobile") {
				await openDeviceConversation(binding.deviceId, undefined, true)
			  } else {
				await openDeviceConversation(binding.deviceId)
			  }
		} catch (value) {
		  const reason = t("portalExtract.serviceCode.client.deviceBoundConversationUnavailable", {
        error: toErrorMessage(value, t("portalExtract.serviceCode.client.entryUnavailable")),
      })
		  setError(reason)
		  setEntryState(buildDeviceAccessErrorState(entryState, reason))
		} finally {
		  setRegistering(false)
		}
		}, [accountSession, authReady, deviceNo, entryState, hostRoute, navigateTo, openDeviceConversation, refreshProfile, serviceCode, t])

  const handlePrivacyConsentAgree = useCallback(async (consent: CookieConsent) => {
    if (
      entryState?.kind !== "guestSessionReady" ||
      !currentEntrySessionId ||
      !currentEntryChat
    ) {
      setPrivacyConsentError(t("portalExtract.serviceCode.client.sessionNotReady"))
      return
    }
    setPrivacyConsentError("")
    try {
      const currentSession = entryState.session as CustomerEntrySessionResponse
      const confirmed = await confirmCustomerPrivacyConsent(
        {
          entrySessionId: currentEntrySessionId,
          policyVersion: currentSession.privacyConsent.policyVersion,
          required: true,
          analytics: consent.analytics,
          marketing: consent.marketing,
        },
        currentEntryChat.visitorId,
        currentEntryChat.visitorToken
      )
      const updatedSession = { ...currentSession, privacyConsent: confirmed }
      privacyConsentAcceptedRef.current = confirmed.accepted
      storeEntrySession(serviceCode, updatedSession)
      setEntryState({ ...entryState, session: updatedSession })
      navigateTo("chat")
    } catch (value) {
      setPrivacyConsentError(toErrorMessage(value, t("portalExtract.serviceCode.client.entryUnavailable")))
    }
  }, [currentEntryChat, currentEntrySessionId, entryState, navigateTo, serviceCode, t])

  const realtimeConversationId =
    selectCustomerRealtimeConversationId(customerContext)

	const signedInCustomer = Boolean(
		authReady && accountSession?.accessToken && accountSession.domainType === "customer"
	)

	if (!signedInCustomer && currentState === "login") {
		return (
			<MobileCustomerLogin
				initialServiceCode={serviceCode}
				onBack={() => {
					const params = new URLSearchParams()
					if (hostRoute === "mobile") params.set("serviceCode", serviceCode)
					const path = hostRoute === "mobile" ? "/mobile" : `/c/${serviceCode}`
					router.replace(`${path}${params.size ? `?${params.toString()}` : ""}`)
				}}
			/>
		)
	}

  return (
    <>
      {currentEntryChat && realtimeConversationId > 0 ? (
        <CustomerEntryPresenceBridge
          chat={currentEntryChat}
          conversationId={realtimeConversationId}
        />
      ) : null}
      <MobileServiceShell
        serviceCode={serviceCode}
        state={entryState}
        activeTab={activeTab}
        loading={loading}
        sessionStarting={sessionStarting}
		registering={registering}
			authenticated={signedInCustomer}
        error={error}
        deviceNo={deviceNo}
        onDeviceNoChange={setDeviceNo}
        onRegisterDevice={handleRegisterDevice}
        onRetry={loadServiceCode}
        onTabChange={handleTabChange}
        currentMobileState={currentState}
        onNavigate={navigateTo}
        detailId={detailId}
        // Sub-state loading/error
        subStateLoading={subStateLoading}
        subStateError={subStateError}
        onSubStateRetry={() => {
          setSubStateError("")
          setSubStateLoading(false)
        }}
        customerContext={customerContext}
        contextLoading={contextLoading}
        contextError={contextError}
        onContextRetry={retryEntryContext}
        onPrivacyConsentAgree={handlePrivacyConsentAgree}
        privacyConsentError={privacyConsentError}
      />
    </>
  )
}

export type { MobileState }
