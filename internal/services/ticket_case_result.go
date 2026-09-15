package services

import (
	"strings"

	"github.com/mlogclub/simple/sqls"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/errorsx"
)

// TicketCaseCommandResult is a stable receipt for one successful command.
// Reading current state belongs to the aggregate GET endpoint, so a later
// transition cannot change the result returned for an earlier operation key.
type TicketCaseCommandResult struct {
	TicketID     int64  `json:"ticket_id"`
	OperationKey string `json:"operation_key"`
	Status       string `json:"status"`
	Revision     int64  `json:"revision"`
}

// GetTicketCaseCommandResult is called only after ExecuteTicketCaseCommand has
// authenticated the actor and verified the payload for the same operation key.
func GetTicketCaseCommandResult(tenantID, ticketID int64, operationKey string) (*TicketCaseCommandResult, error) {
	operationKey = strings.TrimSpace(operationKey)
	if tenantID <= 0 || ticketID <= 0 || operationKey == "" {
		return nil, errorsx.InvalidParam("工单操作回执参数无效")
	}
	var operation models.TicketCaseOperation
	if err := sqls.DB().Where("tenant_id = ? AND ticket_id = ? AND operation_key = ?", tenantID, ticketID, operationKey).First(&operation).Error; err != nil {
		return nil, err
	}
	return &TicketCaseCommandResult{
		TicketID: operation.TicketID, OperationKey: operation.OperationKey,
		Status: operation.ResultStatus, Revision: operation.ResultRevision,
	}, nil
}
