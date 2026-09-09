import { apiGet } from "@/lib/api/client"
import type { ApiResponse } from "@/lib/api/types"

export type EnterpriseMeetingReminder = {
  id: string
  meeting_id: string
  ticket_id: number
  ticket_no: string
  title: string
  room_name: string
  status: string
  scheduled_at: string
  starts_in_minutes: number
  action_url: string
}

export type EnterpriseTicketReminder = {
  id: string
  ticket_id: number
  ticket_no: string
  title: string
  product_name: string
  team_name: string
  priority: string
  status: string
  created_at: string
  action_url: string
}

export type EnterpriseReminderPoll = {
  checked_at: string
  meeting_reminders: EnterpriseMeetingReminder[]
  ticket_reminders: EnterpriseTicketReminder[]
}

export async function fetchEnterpriseReminders(since?: string): Promise<ApiResponse<EnterpriseReminderPoll>> {
  return apiGet<EnterpriseReminderPoll>("/reminders/poll", { since })
}
