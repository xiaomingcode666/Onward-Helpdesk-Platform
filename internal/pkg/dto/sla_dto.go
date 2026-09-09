package dto

import "time"

// EnterpriseSLAPolicyDTO is the stable enterprise API contract for SLA policies.
type EnterpriseSLAPolicyDTO struct {
	ID                string    `json:"id"`
	TenantID          string    `json:"tenant_id"`
	Name              string    `json:"name"`
	Priority          string    `json:"priority"`
	FRTMinutes        int       `json:"frt_minutes"`
	AssignmentMinutes int       `json:"assignment_minutes"`
	ResolutionMinutes int       `json:"resolution_minutes"`
	CalendarID        string    `json:"calendar_id"`
	Status            string    `json:"status"`
	Active            bool      `json:"active"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
