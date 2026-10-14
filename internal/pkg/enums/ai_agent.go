package enums

type AIAgentReviewStatus string

const (
	AIAgentReviewStatusUnreviewed AIAgentReviewStatus = "unreviewed"
	AIAgentReviewStatusPending    AIAgentReviewStatus = "pending"
	AIAgentReviewStatusApproved   AIAgentReviewStatus = "approved"
	AIAgentReviewStatusRejected   AIAgentReviewStatus = "rejected"
)

var AIAgentReviewStatusValues = []AIAgentReviewStatus{
	AIAgentReviewStatusUnreviewed,
	AIAgentReviewStatusPending,
	AIAgentReviewStatusApproved,
	AIAgentReviewStatusRejected,
}

func GetAIAgentReviewStatusLabel(status AIAgentReviewStatus, locale string) string {
	switch status {
	case AIAgentReviewStatusPending:
		return enumLabel(locale, "enum.AIAgentReviewStatusPending")
	case AIAgentReviewStatusApproved:
		return enumLabel(locale, "enum.AIAgentReviewStatusApproved")
	case AIAgentReviewStatusRejected:
		return enumLabel(locale, "enum.AIAgentReviewStatusRejected")
	default:
		return enumLabel(locale, "enum.AIAgentReviewStatusUnreviewed")
	}
}
