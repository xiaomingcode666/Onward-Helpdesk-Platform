package dto

type EngineerWorkStatusDTO struct {
	Status            string `json:"status"`
	Note              string `json:"note"`
	AvailableAt       string `json:"availableAt,omitempty"`
	ConfirmedAt       string `json:"confirmedAt,omitempty"`
	StatusChangedAt   string `json:"statusChangedAt,omitempty"`
	NeedsConfirmation bool   `json:"needsConfirmation"`
}

type EngineerTeamBriefDTO struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	ProductID int64  `json:"productId"`
}

type EngineerBriefingDTO struct {
	IsEngineer        bool                          `json:"isEngineer"`
	WorkStatus        EngineerWorkStatusDTO         `json:"workStatus"`
	Teams             []EngineerTeamBriefDTO        `json:"teams"`
	UnassignedCount   int64                         `json:"unassignedTicketCount"`
	MyOpenCount       int64                         `json:"myOpenTicketCount"`
	UnassignedTickets []EnterpriseTicketListItemDTO `json:"unassignedTickets"`
	MyOpenTickets     []EnterpriseTicketListItemDTO `json:"myOpenTickets"`
}
