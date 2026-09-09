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

func GetAIAgentReviewStatusLabel(status AIAgentReviewStatus) string {
	switch status {
	case AIAgentReviewStatusPending:
		return "待审核"
	case AIAgentReviewStatusApproved:
		return "审核通过"
	case AIAgentReviewStatusRejected:
		return "已退回"
	default:
		return "未审核"
	}
}
