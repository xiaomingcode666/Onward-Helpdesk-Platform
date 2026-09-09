"use client"

import { useEffect, useRef } from "react"
import { usePathname } from "next/navigation"

import { GlobalSearch } from "@/components/global-search"
import { HeaderCrumbs } from "@/components/layout/header-crumbs"
import { PaletteToggle } from "@/components/palette-toggle"
import { RealtimeConnectionStatus } from "@/components/realtime-connection-status"
import { useI18n } from "@/i18n/provider"
import { getPageBreadcrumbKeys } from "@/lib/navigation"
import { useAgentConversationsStore } from "@/lib/stores/agent-conversations"
import { Separator } from "@/components/ui/separator"
import { SidebarTrigger, useSidebar } from "@/components/ui/sidebar"

const SIDEBAR_STORAGE_KEY = "dashboard_sidebar_open"

export function SiteHeader() {
  const t = useI18n()
  const pathname = usePathname()
  const { open, setOpen, isMobile } = useSidebar()
  const pageCrumbs = getPageBreadcrumbKeys(pathname).map((key) => t(key)).join(" / ")
  const realtimeStatus = useAgentConversationsStore((state) => state.realtimeStatus)
  const hasRestoredRef = useRef(false)
  const showConversationRealtime =
    pathname === "/conversations" || pathname.startsWith("/conversations/")

  useEffect(() => {
    if (hasRestoredRef.current || isMobile) {
      return
    }

    hasRestoredRef.current = true
    const storedValue = window.localStorage.getItem(SIDEBAR_STORAGE_KEY)
    if (storedValue === null) {
      return
    }

    setOpen(storedValue === "true")
  }, [isMobile, setOpen])

  useEffect(() => {
    if (!hasRestoredRef.current || isMobile) {
      return
    }

    window.localStorage.setItem(SIDEBAR_STORAGE_KEY, String(open))
  }, [isMobile, open])

  return (
    <header className="flex h-(--header-height) shrink-0 items-center gap-2 border-b border-border/70 bg-background/82 backdrop-blur supports-[backdrop-filter]:bg-background/72 transition-[width,height] ease-linear group-has-data-[collapsible=icon]/sidebar-wrapper:h-(--header-height)">
      <div className="flex w-full items-center justify-between gap-3 px-4 lg:px-6">
        <div className="flex min-w-0 items-center gap-2">
          <SidebarTrigger className="-ml-1" />
          <Separator
            orientation="vertical"
            className="mx-2 h-4 data-vertical:self-auto"
          />
          <div className="flex min-w-0 items-center gap-2">
            <HeaderCrumbs>
              {pageCrumbs}
            </HeaderCrumbs>
            {showConversationRealtime ? (
              <RealtimeConnectionStatus status={realtimeStatus} compact />
            ) : null}
          </div>
        </div>
        <div className="flex shrink-0 items-center justify-end gap-2">
          <div className="hidden sm:block">
            <GlobalSearch />
          </div>
          <PaletteToggle />
        </div>
      </div>
    </header>
  )
}
