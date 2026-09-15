package enums

// Case states describe the customer case, independently of technical work steps.
var TicketCaseStatuses = []string{"new", "acknowledged", "in_triage", "assigned", "waiting", "restored", "resolved", "closure_pending", "closed", "cancelled"}

func IsValidTicketCaseStatus(status string) bool {
	for _, value := range TicketCaseStatuses {
		if value == status {
			return true
		}
	}
	return false
}
