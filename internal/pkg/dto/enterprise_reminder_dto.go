package dto

type EnterpriseReminderPollDTO struct {
	CheckedAt        string                         `json:"checked_at"`
	MeetingReminders []EnterpriseMeetingReminderDTO `json:"meeting_reminders"`
	TicketReminders  []EnterpriseTicketReminderDTO  `json:"ticket_reminders"`
}

type EnterpriseMeetingReminderDTO struct {
	ID              string `json:"id"`
	MeetingID       string `json:"meeting_id"`
	TicketID        int64  `json:"ticket_id"`
	TicketNo        string `json:"ticket_no"`
	Title           string `json:"title"`
	RoomName        string `json:"room_name"`
	Status          string `json:"status"`
	ScheduledAt     string `json:"scheduled_at"`
	StartsInMinutes int64  `json:"starts_in_minutes"`
	ActionURL       string `json:"action_url"`
}

type EnterpriseTicketReminderDTO struct {
	ID          string `json:"id"`
	TicketID    int64  `json:"ticket_id"`
	TicketNo    string `json:"ticket_no"`
	Title       string `json:"title"`
	ProductName string `json:"product_name"`
	TeamName    string `json:"team_name"`
	Priority    string `json:"priority"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	ActionURL   string `json:"action_url"`
}
