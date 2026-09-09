"use client"

import { ArrowRightIcon, VideoIcon } from "lucide-react"
import { usePathname } from "next/navigation"
import { useEffect, useState, type ReactNode } from "react"

import { RailopsButton, StatusTag } from "@railops/ui"

import { DomainShell } from "@/components/layout/domain-shell"
import { Skeleton } from "@/components/ui/skeleton"
import { useCustomerPortalPresenceHeartbeat } from "@/hooks/use-customer-portal-presence-heartbeat"
import { useI18n } from "@/i18n/provider"
import { fetchCustomerHome } from "@/lib/api/customer-home"
import { fetchCustomerMeetingsPage } from "@/lib/api/customer-meetings"
import type { CustomerPortalHome } from "@/lib/api/customer-portal-types"
import { getCustomerNavigationSections } from "@/lib/navigation-customer"

function CustomerVideoReminder() {
  const t = useI18n()
  const [home, setHome] = useState<CustomerPortalHome | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let active = true
    const loadHome = async (initial = false) => {
      try {
        let nextHome = await fetchCustomerHome()
        if (!nextHome.upcoming_meeting) {
          try {
            const meetingPage = await fetchCustomerMeetingsPage({ filter: "upcoming", page: 1, limit: 20 })
            const fallbackMeeting = meetingPage.results.find((meeting) => meeting.status === "active")
              ?? meetingPage.results.find((meeting) => meeting.status === "waiting" || meeting.status === "scheduled")
            if (fallbackMeeting) {
              nextHome = { ...nextHome, upcoming_meeting: fallbackMeeting }
            }
          } catch {
            // Keep the home response usable when the reminder fallback is temporarily unavailable.
          }
        }
        if (active) setHome(nextHome)
      } catch {
        if (active && initial) setHome(null)
      } finally {
        if (active && initial) setLoading(false)
      }
    }
    void loadHome(true)
    const timer = window.setInterval(() => void loadHome(), 30_000)
    return () => {
      active = false
      window.clearInterval(timer)
    }
  }, [])

  const upcomingMeeting = home?.upcoming_meeting
  const meetingStatusLabels: Record<string, string> = {
    active: t("remoteSidebar.customerReminderActive"),
    scheduled: t("remoteSidebar.customerReminderScheduled"),
    waiting: t("remoteSidebar.customerReminderWaiting"),
  }
  const joinableMeeting = upcomingMeeting
    && (upcomingMeeting.status === "active" || upcomingMeeting.status === "waiting" || upcomingMeeting.status === "scheduled")
  const reminderTitle = joinableMeeting
    ? meetingStatusLabels[upcomingMeeting.status]
    : t("remoteSidebar.customerReminderNone")
  const reminderSummary = joinableMeeting
    ? upcomingMeeting.ticket_no
    : t("remoteSidebar.customerReminderHistoryHint")
  const ReminderActionIcon = joinableMeeting ? VideoIcon : ArrowRightIcon

  return (
    <section className="rhd-customer-sidebar-reminder" aria-label={t("remoteSidebar.customerReminder")}>
      <div className="rhd-customer-sidebar-reminder-heading">
        <span className="rhd-customer-sidebar-reminder-icon" aria-hidden="true">
          <VideoIcon />
        </span>
        <span className="rhd-customer-sidebar-reminder-label">
          {t("remoteSidebar.customerReminder")}
        </span>
        {!loading ? (
          <StatusTag tone={joinableMeeting ? "blue" : "neutral"} className="shrink-0">
            {joinableMeeting ? t("remoteSidebar.customerReminderJoinSoon") : t("remoteSidebar.customerReminderIdle")}
          </StatusTag>
        ) : null}
      </div>
      {loading ? (
        <div
          className="rhd-customer-sidebar-reminder-loading"
          role="status"
          aria-label={t("remoteSidebar.customerReminderLoading")}
          aria-busy="true"
        >
          <Skeleton className="h-4 w-20" />
          <Skeleton className="h-3 w-full" />
          <Skeleton className="h-7 w-full rounded-md" />
        </div>
      ) : (
        <div className="rhd-customer-sidebar-reminder-content">
          <div className="rhd-customer-sidebar-reminder-title">{reminderTitle}</div>
          <div className="rhd-customer-sidebar-reminder-summary" title={reminderSummary || undefined}>
            {reminderSummary}
          </div>
          {joinableMeeting && upcomingMeeting.created_by ? (
            <div className="rhd-customer-sidebar-reminder-meta" title={upcomingMeeting.created_by}>
              {upcomingMeeting.created_by}
            </div>
          ) : null}
          <RailopsButton
            block
            size="small"
            variant={joinableMeeting ? "primary" : "text"}
            href={joinableMeeting ? `/customer/meeting?meetingId=${upcomingMeeting.id}` : "/customer/meeting"}
            icon={<ReminderActionIcon className="size-3.5" />}
            className="rhd-customer-sidebar-reminder-action"
          >
            {joinableMeeting ? t("remoteSidebar.customerReminderJoin") : t("remoteSidebar.customerReminderViewMeetings")}
          </RailopsButton>
        </div>
      )}
    </section>
  )
}

export function CustomerShell({ children }: { children: ReactNode }) {
  const pathname = usePathname()
  const isLoginRoute = pathname?.startsWith("/customer/login") ?? false
  useCustomerPortalPresenceHeartbeat(!isLoginRoute)

  if (isLoginRoute) {
    return <>{children}</>
  }

  return (
    <DomainShell
      domain="customer"
      sections={getCustomerNavigationSections()}
      sidebarFooter={<CustomerVideoReminder />}
      sidebarTitleKey="remoteSidebar.service"
    >
      {children}
    </DomainShell>
  )
}
