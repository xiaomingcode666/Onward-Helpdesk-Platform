"use client"

import { VideoIcon } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { useAppLocale } from "@/i18n/provider"
import type { CustomerEntryMeeting } from "@/lib/api/customer-entry"

type MeetingEntryProps = {
  meetings?: CustomerEntryMeeting[]
}

export function MeetingEntry({ meetings = [] }: MeetingEntryProps) {
  const { locale, t } = useAppLocale()
  const meetingStatusLabel = (status: string) => {
    if (status === "active") return t("portalExtract.serviceCode.meetingEntry.status.active")
    if (status === "scheduled") return t("portalExtract.serviceCode.meetingEntry.status.scheduled")
    if (status === "waiting") return t("portalExtract.serviceCode.meetingEntry.status.waiting")
    return t("portalExtract.serviceCode.meetingEntry.status.closed")
  }

  return (
    <div className="space-y-3 p-4">
      {meetings.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center py-8">
            <VideoIcon className="size-10 text-muted-foreground" />
            <p className="mt-2 text-sm text-muted-foreground">
              {t("portalExtract.serviceCode.meetingEntry.empty")}
            </p>
          </CardContent>
        </Card>
      ) : (
        meetings.map((meeting) => (
          <Card key={meeting.id}>
            <CardHeader className="pb-2">
              <div className="flex items-center gap-2">
                <VideoIcon className="size-5 text-primary" />
                <CardTitle className="flex-1 truncate text-sm font-medium">
                  {meeting.title}
                </CardTitle>
                <Badge
                  variant={
                    meeting.status === "active"
                      ? "default"
                      : meeting.status === "waiting" || meeting.status === "scheduled"
                        ? "secondary"
                        : "outline"
                  }
                >
                  {meetingStatusLabel(meeting.status)}
                </Badge>
              </div>
            </CardHeader>
            <CardContent>
              <p className="text-xs text-muted-foreground">
                {meeting.status === "waiting" || meeting.status === "scheduled"
                  ? t("portalExtract.serviceCode.meetingEntry.scheduledAt", {
                    time: new Date(meeting.scheduledAt).toLocaleString(locale),
                  })
                  : t("portalExtract.serviceCode.meetingEntry.updatedAt", {
                    time: new Date(meeting.startedAt || meeting.scheduledAt).toLocaleString(locale),
                  })}
                {meeting.initiator
                  ? t("portalExtract.serviceCode.meetingEntry.byInitiator", {
                    name: meeting.initiator,
                  })
                  : ""}
              </p>
              <Button
                type="button"
                className="mt-3 w-full"
                variant="outline"
                disabled
              >
                {meeting.status === "active"
                  ? t("portalExtract.serviceCode.meetingEntry.action.active")
                  : meeting.status === "scheduled"
                    ? t("portalExtract.serviceCode.meetingEntry.action.scheduled")
                    : meeting.status === "waiting"
                      ? t("portalExtract.serviceCode.meetingEntry.action.waiting")
                      : t("portalExtract.serviceCode.meetingEntry.action.closed")}
              </Button>
            </CardContent>
          </Card>
        ))
      )}
    </div>
  )
}
