"use client"

import { useCallback, useEffect, useRef } from "react"
import { useRouter, useSearchParams } from "next/navigation"
import {
  ClipboardListIcon,
  LogOutIcon,
  MessageCircleIcon,
  UserCircleIcon,
  VideoIcon,
  WrenchIcon,
} from "lucide-react"

import { useAuth } from "@/components/auth-provider"
import { useI18n } from "@/i18n/provider"
import { useCustomerPortalPresenceHeartbeat } from "@/hooks/use-customer-portal-presence-heartbeat"



import { MobileLanguageSwitch } from "@/components/remote-helpdesk/mobile-language-switch"
import {
  MobileCustomerChatPage,
  MobileCustomerDevicesPage,
  MobileCustomerMeetingsPage,
  MobileCustomerProfilePage,
  MobileCustomerTicketsPage,
  type MobileCustomerNavigate,
} from "@/components/remote-helpdesk/mobile-customer-pages"
import { Button } from "@/components/ui/button"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { getCustomerThemeClassName, normalizeCustomerTheme } from "@/lib/customer-theme"
import { cn } from "@/lib/utils"
import {
  registerNativeMobilePushNotifications,
  unregisterNativeMobilePushNotifications,
} from "@/lib/mobile/push-registration"

export type MobileCustomerTab = "chat" | "tickets" | "video" | "devices" | "my"

const mobileCustomerPortalI18nPrefix = "opsComponentsExtract.mobileCustomerPortal."
type MobileCustomerPortalT = ReturnType<typeof useI18n>
const ml = (t: MobileCustomerPortalT, key: string, values?: Record<string, string | number>) =>
  t(`${mobileCustomerPortalI18nPrefix}${key}`, values)

const tabValues = ["chat", "tickets", "video", "devices", "my"] satisfies readonly MobileCustomerTab[]

function normalizeTab(value: string | null): MobileCustomerTab {
  return tabValues.some((tab) => tab === value) ? value as MobileCustomerTab : "chat"
}

function MobileCustomerPage({ tab, navigate }: { tab: MobileCustomerTab; navigate: MobileCustomerNavigate }) {
  if (tab === "tickets") return <MobileCustomerTicketsPage navigate={navigate} />
  if (tab === "video") return <MobileCustomerMeetingsPage />
  if (tab === "devices") return <MobileCustomerDevicesPage navigate={navigate} />
  if (tab === "my") return <MobileCustomerProfilePage />
  return <MobileCustomerChatPage navigate={navigate} />
}

export function MobileCustomerPortal({ serviceCode = "" }: { serviceCode?: string }) {
  const router = useRouter()
  const searchParams = useSearchParams()
  const t = useI18n()
  const { session, signOut } = useAuth()
  const contentRef = useRef<HTMLElement | null>(null)
  const hasDeviceConcept = session?.featureFlags?.device !== false
  const requestedTab = normalizeTab(searchParams.get("state"))
  const activeTab = requestedTab === "devices" && !hasDeviceConcept ? "chat" : requestedTab
  const conversationDetailOpen = activeTab === "chat" && Number(searchParams.get("conversationId") || "0") > 0
  const displayName = session?.user?.nickname || session?.user?.username || ml(t, "portalName")
  const customerTheme = normalizeCustomerTheme(session?.tenantBranding?.customerTheme)
  const branding = session?.tenantBranding
  const tabs: Array<{ value: MobileCustomerTab; label: string; title: string; icon: typeof MessageCircleIcon }> = [
    { value: "chat", label: ml(t, "tab.chat.label"), title: ml(t, "tab.chat.title"), icon: MessageCircleIcon },
    { value: "tickets", label: ml(t, "tab.tickets.label"), title: ml(t, "tab.tickets.title"), icon: ClipboardListIcon },
    { value: "video", label: ml(t, "tab.video.label"), title: ml(t, "tab.video.title"), icon: VideoIcon },
    ...(hasDeviceConcept ? [{ value: "devices" as const, label: ml(t, "tab.devices.label"), title: ml(t, "tab.devices.title"), icon: WrenchIcon }] : []),
    { value: "my", label: ml(t, "tab.my.label"), title: ml(t, "tab.my.title"), icon: UserCircleIcon },
  ]
  const activeTabMeta = tabs.find((tab) => tab.value === activeTab) ?? tabs[0]
  const pushRegistrationKey = session
    ? `${session.domainType || "customer"}:${session.tenantId}:${session.user.id}`
    : ""
  useCustomerPortalPresenceHeartbeat(session?.domainType === "customer")

  useEffect(() => {
    if (!pushRegistrationKey || session?.domainType !== "customer") return
    void registerNativeMobilePushNotifications(pushRegistrationKey)
  }, [pushRegistrationKey, session?.domainType])

  useEffect(() => {
    window.scrollTo({ top: 0, left: 0 })
    contentRef.current?.scrollTo({ top: 0, left: 0 })
  }, [activeTab])

  const openTab = useCallback<MobileCustomerNavigate>((tab, extraParams) => {
    if (tab === "scan") {
      router.replace("/mobile?state=scan", { scroll: false })
      return
    }
    const params = new URLSearchParams()
    if (serviceCode) params.set("serviceCode", serviceCode)
    params.set("state", tab)
    Object.entries(extraParams ?? {}).forEach(([key, value]) => params.set(key, value))
    router.replace(`/mobile?${params.toString()}`, { scroll: false })
  }, [router, serviceCode])

  const handleSignOut = useCallback(async () => {
    if (pushRegistrationKey) {
      await unregisterNativeMobilePushNotifications(pushRegistrationKey)
    }
    await signOut("/mobile?state=login")
  }, [pushRegistrationKey, signOut])

  return (
    <main
      className={cn(
        "rhd-mobile-touch-surface rhd-customer-portal h-dvh overflow-hidden bg-[#e9edf2] text-[#172033]",
        getCustomerThemeClassName(customerTheme),
      )}
      data-customer-theme={customerTheme}
    >
      <div className={cn(
        "rhd-customer-portal-shell relative mx-auto grid h-dvh w-full max-w-lg overflow-hidden bg-[#f4f6f8] shadow-[0_0_48px_rgba(15,23,42,0.08)]",
        conversationDetailOpen ? "grid-rows-[minmax(0,1fr)]" : "grid-rows-[auto_minmax(0,1fr)]",
      )}>
        {!conversationDetailOpen ? <header className="rhd-customer-portal-header z-30 flex h-[calc(64px+env(safe-area-inset-top))] shrink-0 items-end justify-between overflow-hidden border-b border-[#e5e9ef] bg-white/96 px-4 pb-3 pt-[env(safe-area-inset-top)] backdrop-blur-xl">
          <div className="rhd-customer-portal-brand relative z-10 min-w-0">
            {branding?.logoUrl ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img className="rhd-customer-portal-logo max-h-5 max-w-44 object-contain object-left" src={branding.logoUrl} alt={branding.brandName || "Tenant logo"} />
            ) : (
              <p className="text-rhd-2xs font-medium text-[#8390a3]">{branding?.brandName || "RemoteHelpDesk"}</p>
            )}
            <h1 className="mt-0.5 truncate text-xl font-semibold leading-6 text-[#111827]">{activeTabMeta.title}</h1>
          </div>
          <div className="relative z-10 flex items-center gap-2">
            <MobileLanguageSwitch />
            {activeTab === "my" ? (
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger render={
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="size-11 rounded-full text-[#637083] hover:bg-[#eef1f5] hover:text-[#172033]"
                      aria-label={ml(t, "signOut")}
                      onClick={() => void handleSignOut()}
                    />
                  }>
                    <LogOutIcon className="size-4" />
                  </TooltipTrigger>
                  <TooltipContent>{ml(t, "signOut")}</TooltipContent>
                </Tooltip>
              </TooltipProvider>
            ) : (
              <span className="rhd-customer-portal-avatar flex size-9 items-center justify-center rounded-full bg-[#172033] text-sm font-semibold text-white" aria-label={displayName}>
                {displayName.trim().slice(0, 1).toUpperCase() || ml(t, "avatarFallback")}
              </span>
            )}
          </div>
        </header> : null}

        <section
          ref={contentRef}
          className={cn(
            "min-h-0 min-w-0 flex-1 overscroll-contain",
            activeTab === "chat" ? "overflow-hidden" : "overflow-y-auto",
            !conversationDetailOpen && "pb-[calc(76px+env(safe-area-inset-bottom))]",
          )}
        >
          <MobileCustomerPage key={activeTab} tab={activeTab} navigate={openTab} />
        </section>

        {!conversationDetailOpen ? <nav className="rhd-customer-portal-nav fixed inset-x-0 bottom-0 z-50 mx-auto w-full max-w-lg shrink-0 border-t border-[#e2e7ed] bg-white/96 px-2 pb-[max(5px,env(safe-area-inset-bottom))] pt-1 shadow-[0_-8px_24px_rgba(15,23,42,0.08)] backdrop-blur-xl" aria-label={ml(t, "navAria")}>
          <div className="grid gap-1" style={{ gridTemplateColumns: `repeat(${tabs.length}, minmax(0, 1fr))` }}>
            {tabs.map((tab) => {
              const Icon = tab.icon
              const selected = tab.value === activeTab
              return (
                <button
                  key={tab.value}
                  type="button"
                  className={cn(
                    "relative flex h-[54px] min-w-0 flex-col items-center justify-center gap-0.5 text-rhd-2xs font-medium text-[#7b8494] transition-colors",
                    selected ? "text-[#1769e0]" : "active:text-[#172033]",
                  )}
                  aria-current={selected ? "page" : undefined}
                  onClick={() => openTab(tab.value)}
                >
                  <span className={cn("absolute top-0 h-0.5 w-5 rounded-full bg-[#1769e0] opacity-0", selected && "opacity-100")} />
                  <span className={cn("flex size-7 items-center justify-center rounded-md", selected && "bg-[#edf4ff]")}>
                    <Icon className="size-5" strokeWidth={selected ? 2.4 : 2} />
                  </span>
                  <span>{tab.label}</span>
                </button>
              )
            })}
          </div>
        </nav> : null}
      </div>
    </main>
  )
}
