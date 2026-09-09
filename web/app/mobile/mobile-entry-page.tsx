"use client"

import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import { ArrowLeftIcon, CameraIcon, KeyboardIcon, Loader2Icon, ScanLineIcon, XIcon } from "lucide-react"

import { MobileServiceClient } from "../c/[serviceCode]/mobile-service-client"
import { useAuth } from "@/components/auth-provider"
import { useI18n } from "@/i18n/provider"
import { MobileCustomerLogin } from "@/components/remote-helpdesk/mobile-customer-login"
import { MobileCustomerPortal } from "@/components/remote-helpdesk/mobile-customer-portal"
import { MobileLanguageSwitch } from "@/components/remote-helpdesk/mobile-language-switch"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { MOBILE_PUSH_ACTION_EVENT } from "@/lib/mobile/push-registration"

const mobileStates = new Set(["chat", "tickets", "video", "devices", "my", "login", "register", "scan"])
const mobileStateParams: Partial<Record<string, string[]>> = {
  chat: ["conversationId", "deviceId"],
  devices: ["deviceId"],
  tickets: ["ticketNo"],
  video: ["id"],
  register: ["invite"],
}

function serviceCodeFromUrl(value: string) {
  const raw = value.trim()
  if (!raw) return ""
  try {
    const url = new URL(raw, "https://remotehelpdesk.local")
    const pathCode = url.pathname.match(/\/c\/([^/]+)/)?.[1]
    const customSchemeCode = url.protocol === "remotehelpdesk:" && url.hostname === "c"
      ? url.pathname.replace(/^\/+/, "")
      : ""
    const queryCode = url.pathname === "/mobile" ? url.searchParams.get("serviceCode") : ""
    return decodeURIComponent(pathCode || customSchemeCode || queryCode || "").trim() || raw
  } catch {
    return raw
  }
}

function mobilePathFromUrl(value: string) {
  try {
    const url = new URL(value)
    if (url.protocol !== "remotehelpdesk:" || url.hostname !== "mobile") return ""

    const params = new URLSearchParams()
    const state = url.searchParams.get("state")?.trim() || ""
    if (mobileStates.has(state)) params.set("state", state)
    for (const key of mobileStateParams[state] ?? []) {
      const parameter = url.searchParams.get(key)?.trim() || ""
      if (parameter && parameter.length <= 128) params.set(key, parameter)
    }
    return `/mobile${params.size ? `?${params.toString()}` : ""}`
  } catch {
    return ""
  }
}

function mobilePathFromPushData(value: unknown) {
  if (!value || typeof value !== "object") return ""
  const data = value as Record<string, unknown>
  for (const key of ["deepLink", "url", "actionUrl"]) {
    const candidate = typeof data[key] === "string" ? data[key].trim() : ""
    if (!candidate) continue
    const customSchemePath = mobilePathFromUrl(candidate)
    if (customSchemePath) return customSchemePath
    try {
      const parsed = new URL(candidate, "https://mobile.remotehelpdesk.invalid")
      if (parsed.origin !== "https://mobile.remotehelpdesk.invalid" || parsed.pathname !== "/mobile") continue
      return mobilePathFromUrl(`remotehelpdesk://mobile?${parsed.searchParams.toString()}`)
    } catch {
      // Ignore malformed notification links.
    }
  }

  const state = typeof data.state === "string" ? data.state.trim() : ""
  if (!mobileStates.has(state)) return ""
  const params = new URLSearchParams({ state })
  for (const key of mobileStateParams[state] ?? []) {
    const parameter = typeof data[key] === "string" || typeof data[key] === "number"
      ? String(data[key]).trim()
      : ""
    if (parameter && parameter.length <= 128) params.set(key, parameter)
  }
  return `/mobile?${params.toString()}`
}

type H5ScannerControls = {
  stop: () => void
}

function MobileQRCodeEntry({
  signedInCustomer,
  onBack,
  onOpenCode,
}: {
  signedInCustomer: boolean
  onBack: () => void
  onOpenCode: (value: string) => void
}) {
  const t = useI18n()
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const controlsRef = useRef<H5ScannerControls | null>(null)
  const detectedRef = useRef(false)
  const [manualCode, setManualCode] = useState("")
  const [detectedRaw, setDetectedRaw] = useState("")
  const [starting, setStarting] = useState(false)
  const [cameraActive, setCameraActive] = useState(false)
  const [scanError, setScanError] = useState("")

  const stopCamera = useCallback(() => {
    controlsRef.current?.stop()
    controlsRef.current = null
    if (videoRef.current) {
      videoRef.current.pause()
      videoRef.current.srcObject = null
      videoRef.current.removeAttribute("src")
      videoRef.current.load()
    }
    setCameraActive(false)
    setStarting(false)
  }, [])

  useEffect(() => stopCamera, [stopCamera])

  const openDetectedCode = useCallback((value: string) => {
    const code = serviceCodeFromUrl(value)
    if (!code) return
    stopCamera()
    onOpenCode(code)
  }, [onOpenCode, stopCamera])

  const startCamera = useCallback(async () => {
    setScanError("")
    setDetectedRaw("")
    if (!navigator.mediaDevices?.getUserMedia) {
      setScanError(t("portalExtract.mobileEntry.cameraUnsupported"))
      return
    }
    const video = videoRef.current
    if (!video) return
    detectedRef.current = false
    stopCamera()
    setStarting(true)
    try {
      const { BrowserMultiFormatReader } = await import("@zxing/browser")
      const codeReader = new BrowserMultiFormatReader(undefined, {
        delayBetweenScanAttempts: 180,
        delayBetweenScanSuccess: 500,
        tryPlayVideoTimeout: 5000,
      })
      const controls = await codeReader.decodeFromConstraints(
        {
          audio: false,
          video: { facingMode: { ideal: "environment" } },
        },
        video,
        (result, _error, controls) => {
          const rawValue = result?.getText().trim() || ""
          if (!rawValue || detectedRef.current) return
          detectedRef.current = true
          const code = serviceCodeFromUrl(rawValue)
          setDetectedRaw(rawValue)
          setManualCode(code)
          controls.stop()
          controlsRef.current = null
          setCameraActive(false)
        },
      )
      if (detectedRef.current) {
        controls.stop()
        controlsRef.current = null
        setCameraActive(false)
        setStarting(false)
        return
      }
      controlsRef.current = controls
      setCameraActive(true)
      setStarting(false)
    } catch {
      setScanError(t("portalExtract.mobileEntry.cameraPermissionFailed"))
      stopCamera()
    }
  }, [stopCamera, t])

  return (
    <main className="rhd-mobile-touch-surface min-h-svh bg-[#e9edf2] text-[#172033]">
      <div className="mx-auto grid min-h-svh w-full max-w-lg grid-rows-[auto_minmax(0,1fr)] bg-[#f4f6f8] shadow-[0_0_48px_rgba(15,23,42,0.08)]">
        <header className="flex h-[calc(64px+env(safe-area-inset-top))] items-end justify-between gap-3 border-b border-[#e5e9ef] bg-white/96 px-4 pb-3 pt-[env(safe-area-inset-top)]">
          <div className="flex min-w-0 items-center gap-3">
            <Button type="button" variant="ghost" size="icon" className="size-10 shrink-0 rounded-full text-[#637083]" onClick={onBack} aria-label={t("portalExtract.customerMobile.common.back")}>
              <ArrowLeftIcon className="size-4" />
            </Button>
            <div className="min-w-0">
              <p className="text-rhd-2xs font-medium text-[#8390a3]">RemoteHelpDesk</p>
              <h1 className="truncate text-xl font-semibold leading-6 text-[#111827]">{t("portalExtract.mobileEntry.scanTitle")}</h1>
            </div>
          </div>
          <MobileLanguageSwitch />
        </header>

        <section className="min-h-0 overflow-y-auto px-4 py-4">
          <div className="overflow-hidden rounded-xl border border-[#dce2ea] bg-[#101827]">
            <div className="relative h-[clamp(240px,40svh,340px)]">
              <video ref={videoRef} muted playsInline className="size-full object-cover" />
              {!cameraActive && !starting ? (
                <div className="absolute inset-0 flex flex-col items-center justify-center gap-2.5 bg-[#101827] px-6 text-center text-white">
                  <span className="grid size-16 place-items-center rounded-full bg-white/10">
                    <ScanLineIcon className="size-7" />
                  </span>
                  <strong className="text-base">{t("portalExtract.mobileEntry.scanPrompt")}</strong>
                  <p className="text-xs leading-5 text-white/70">{t("portalExtract.mobileEntry.scanBody")}</p>
                </div>
              ) : null}
              {starting ? (
                <div className="absolute inset-0 grid place-items-center bg-[#101827]/80 text-white">
                  <span className="flex items-center gap-2 text-sm">
                    <Loader2Icon className="size-4 animate-spin" />
                    {t("portalExtract.mobileEntry.cameraStarting")}
                  </span>
                </div>
              ) : null}
              {cameraActive ? <div className="pointer-events-none absolute inset-8 rounded-2xl border-2 border-white/90 shadow-[0_0_0_999px_rgba(15,23,42,0.28)]" /> : null}
            </div>
          </div>

          <div className="mt-3 grid grid-cols-2 gap-2">
            <Button type="button" className="h-11 bg-[#1769e0] shadow-none hover:bg-[#125fcf]" onClick={() => void startCamera()} disabled={starting || cameraActive}>
              {starting ? <Loader2Icon className="size-4 animate-spin" /> : <CameraIcon className="size-4" />}
              {t("portalExtract.mobileEntry.startScan")}
            </Button>
            <Button type="button" variant="outline" className="h-11 border-[#dce2ea] bg-white text-[#465266] shadow-none" onClick={stopCamera} disabled={!cameraActive && !starting}>
              <XIcon className="size-4" />
              {t("portalExtract.mobileEntry.stopScan")}
            </Button>
          </div>

          <div className="mt-3 rounded-xl border border-[#dce2ea] bg-white p-3">
            <label className="grid gap-2 text-sm font-medium text-[#303b4d]" htmlFor="mobile-entry-service-code">
              <span className="flex items-center gap-2">
                <KeyboardIcon className="size-4 text-[#657084]" />
                {t("portalExtract.mobileEntry.manualLabel")}
              </span>
              <Input
                id="mobile-entry-service-code"
                value={manualCode}
                onChange={(event) => {
                  setManualCode(event.target.value)
                  setDetectedRaw("")
                  setScanError("")
                }}
                className="h-10 font-mono uppercase"
                placeholder={t("portalExtract.mobileEntry.manualPlaceholder")}
                autoCapitalize="characters"
                autoCorrect="off"
              />
            </label>
            {detectedRaw ? (
              <p className="mt-2 rounded-md bg-[#edf4ff] px-3 py-2 text-xs text-[#1769e0]">
                {t("portalExtract.mobileEntry.detectedCode", { code: manualCode })}
              </p>
            ) : null}
            {scanError ? <p className="mt-2 rounded-md bg-red-50 px-3 py-2 text-xs text-red-600">{scanError}</p> : null}
            <Button type="button" className="mt-3 h-11 w-full bg-[#1769e0] shadow-none hover:bg-[#125fcf]" disabled={!manualCode.trim()} onClick={() => openDetectedCode(manualCode)}>
              <ScanLineIcon className="size-4" />
              {signedInCustomer ? t("portalExtract.mobileEntry.bindScannedDevice") : t("portalExtract.mobileEntry.openScannedDevice")}
            </Button>
          </div>
        </section>
      </div>
    </main>
  )
}

export function MobileEntryPage() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const t = useI18n()
  const { ready: authReady, session } = useAuth()
  const serviceCode = searchParams.get("serviceCode")?.trim() || ""
  const initialState = searchParams.get("state") || "resolving"
  const initialStateId = searchParams.get("id") || undefined

  useLayoutEffect(() => {
    if (window.location.pathname !== "/") return
    const normalizedUrl = `/mobile${window.location.search}${window.location.hash}`
    window.history.replaceState(window.history.state, "", normalizedUrl)
  }, [])

  const openServiceCode = useCallback(
    (value: string) => {
      const normalized = serviceCodeFromUrl(value)
      if (!normalized) return
      router.replace(`/mobile?serviceCode=${encodeURIComponent(normalized)}`)
    },
    [router]
  )

  const openNativeUrl = useCallback((value: string) => {
    const mobilePath = mobilePathFromUrl(value)
    if (mobilePath) {
      router.replace(mobilePath)
      return
    }
    openServiceCode(value)
  }, [openServiceCode, router])

  useEffect(() => {
    let listener: { remove: () => Promise<void> } | undefined
    let active = true

    void import("@capacitor/app")
      .then(async ({ App }) => {
        listener = await App.addListener("appUrlOpen", ({ url }) => {
          if (active) openNativeUrl(url)
        })

        const launchUrl = await App.getLaunchUrl()
        if (active && launchUrl?.url) openNativeUrl(launchUrl.url)
      })
      .catch(() => {
        // The web build does not have a native deep-link bridge.
      })

    return () => {
      active = false
      void listener?.remove()
    }
  }, [openNativeUrl])

  useEffect(() => {
    const openPushAction = (event: Event) => {
      const path = mobilePathFromPushData((event as CustomEvent<unknown>).detail)
      if (path) router.replace(path)
    }
    window.addEventListener(MOBILE_PUSH_ACTION_EVENT, openPushAction)
    return () => window.removeEventListener(MOBILE_PUSH_ACTION_EVENT, openPushAction)
  }, [router])

  const signedInCustomer = Boolean(session?.accessToken && session.domainType === "customer")

  if (!authReady) {
    return (
      <main className="flex min-h-svh items-center justify-center bg-slate-950 px-6 text-white">
        <p className="text-sm text-white/70">{t("portalExtract.mobileEntry.readingAccount")}</p>
      </main>
    )
  }

  if (!serviceCode && initialState === "scan") {
    if (signedInCustomer && session?.featureFlags?.device === false) {
      return <MobileCustomerPortal />
    }
    return (
      <MobileQRCodeEntry
        signedInCustomer={signedInCustomer}
        onBack={() => router.replace(signedInCustomer ? "/mobile" : "/mobile?state=login")}
        onOpenCode={openServiceCode}
      />
    )
  }

  if (signedInCustomer && !serviceCode) {
    return <MobileCustomerPortal />
  }

  if (!signedInCustomer && !serviceCode) {
    return (
      <MobileCustomerLogin
        initialMode={initialState === "register" ? "register" : "account"}
        initialInviteCode={searchParams.get("invite")?.trim() || ""}
      />
    )
  }

  if (serviceCode) {
    return (
      <MobileServiceClient
        serviceCode={serviceCode}
        initialState={initialState}
        initialStateId={initialStateId}
        hostRoute="mobile"
      />
    )
  }

  return <MobileCustomerLogin />
}
