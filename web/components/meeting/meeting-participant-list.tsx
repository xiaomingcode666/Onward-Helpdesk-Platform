"use client"

import { translateCurrentMessage } from "@/i18n/messages"
import { CrownIcon, UserRoundIcon, UsersRoundIcon } from "lucide-react"

import { StatusTag } from "@railops/ui"
import { useAppLocale } from "@/i18n/provider"
import type { MeetingParticipant } from "@/lib/api/types"
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


function participantIdentityLabel(identityType: string | undefined, userType: string) {
  switch (identityType || userType) {
    case "platform_admin":
    case "platform_staff":
    case "platform_operations":
    case "super_admin":
    case "admin":
      return ee("meetingParticipants.text017")
    case "tenant_admin":
    case "tenant_owner":
    case "tenant_admin_seed":
      return ee("meetingParticipants.text018")
    case "authorized_support":
      return ee("meetingParticipants.text019")
    case "repair_engineer":
      return ee("meetingParticipants.text020")
    case "enterprise":
      return ee("meetingParticipants.text001")
    case "customer":
      return ee("meetingParticipants.text002")
    case "partner":
    case "supplier":
    case "external":
      return ee("meetingParticipants.text003")
    default:
      return ee("meetingParticipants.text004")
  }
}

function participantRoleLabel(role: string) {
  switch (role) {
    case "moderator":
    case "host":
      return ee("meetingParticipants.text005")
    default:
      return ee("meetingParticipants.text006")
  }
}

function participantTime(value: string | undefined, locale: string) {
  if (!value) return "—"
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return value
  return new Intl.DateTimeFormat(locale, {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(parsed)
}

function participantDuration(seconds?: number) {
  const value = Math.max(0, Number(seconds || 0))
  if (value < 1) return ee("meetingParticipants.text007")
  if (value < 60) return ee("meetingParticipants.text008", { value0: Math.floor(value) })
  const minutes = Math.floor(value / 60)
  if (minutes < 60) return ee("meetingParticipants.text009", { value0: minutes })
  return ee("meetingParticipants.text010", { value0: Math.floor(minutes / 60), value1: minutes % 60 })
}

export function MeetingParticipantList({
  participants,
  meetingStatus,
}: {
  participants: MeetingParticipant[]
  meetingStatus: string
}) {
  const { locale } = useAppLocale()

  if (participants.length === 0) {
    return (
      <div className="border-y border-border px-4 py-8 text-center">
        <UsersRoundIcon className="mx-auto size-6 text-muted-foreground" />
        <p className="mt-3 text-sm font-medium text-foreground">{ee("meetingParticipants.text011")}</p>
        <p className="mt-1 text-xs text-muted-foreground">{ee("meetingParticipants.text012")}</p>
      </div>
    )
  }

  return (
    <div className="divide-y divide-border border-y border-border" data-testid="meeting-participant-list">
      {participants.map((participant) => {
        const identity = participantIdentityLabel(participant.identity_type, participant.user_type)
        const role = participantRoleLabel(participant.role)
        const online = Boolean(participant.joined_at && !participant.left_at && meetingStatus !== "finished" && meetingStatus !== "ended")
        return (
          <article key={participant.id} className="flex items-start gap-3 px-4 py-3.5">
            <span className="flex size-9 shrink-0 items-center justify-center rounded-md border border-border bg-muted text-muted-foreground">
              {role === ee("meetingParticipants.text005") ? <CrownIcon className="size-4" /> : <UserRoundIcon className="size-4" />}
            </span>
            <div className="min-w-0 flex-1">
              <div className="flex flex-wrap items-center gap-2">
                <strong className="text-sm text-foreground">{participant.name || identity}</strong>
                <StatusTag tone="neutral" data-participant-role={identity}>{identity}</StatusTag>
                <StatusTag tone="neutral" data-meeting-role={role}>{role}</StatusTag>
                {online ? <span className="text-xs font-medium text-primary">{ee("meetingParticipants.text013")}</span> : null}
              </div>
              <div className="mt-1.5 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                <span>{ee("meetingParticipants.text014")}{participantTime(participant.joined_at, locale)}</span>
                {participant.left_at ? <span>{ee("meetingParticipants.text015")}{participantTime(participant.left_at, locale)}</span> : null}
                <span>{ee("meetingParticipants.text016")}{participantDuration(participant.duration_seconds)}</span>
              </div>
            </div>
          </article>
        )
      })}
    </div>
  )
}
