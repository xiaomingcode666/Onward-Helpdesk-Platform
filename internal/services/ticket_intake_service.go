package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/repositories"
)

var intakeChannels = map[string]bool{"phone": true, "email": true, "monitoring_alert": true, "api": true, "webhook": true, "whatsapp": true, "chatbot_handoff": true}
var intakeRequiredFields = map[string]bool{"caller_name": true, "caller_phone": true, "customer_id": true, "product_id": true, "device_id": true, "service_region": true}

func ValidateTicketIntakePolicy(policy dto.TicketIntakePolicy) error {
	if len(policy.Rules) > 200 {
		return errorsx.InvalidParam("at most 200 intake rules are allowed")
	}
	seen := map[string]bool{}
	for _, rule := range policy.Rules {
		if strings.TrimSpace(rule.ProjectKey) == "" || rule.ProjectKey != strings.TrimSpace(rule.ProjectKey) || utf8.RuneCountInString(rule.ProjectKey) > 64 || strings.TrimSpace(rule.TicketType) == "" || rule.TicketType != strings.TrimSpace(rule.TicketType) || utf8.RuneCountInString(rule.TicketType) > 64 || !intakeChannels[rule.Channel] {
			return errorsx.InvalidParam("each intake rule requires a valid project_key, channel and ticket_type")
		}
		key, _ := json.Marshal([]string{rule.ProjectKey, rule.Channel, rule.TicketType})
		if seen[string(key)] {
			return errorsx.InvalidParam("duplicate project/channel/ticket-type rule")
		}
		seen[string(key)] = true
		fields := map[string]bool{}
		for _, field := range rule.RequiredFields {
			if !intakeRequiredFields[field] || fields[field] {
				return errorsx.InvalidParam("unknown or duplicate required context field: " + field)
			}
			fields[field] = true
		}
	}
	return nil
}

func ticketIntakePolicyDB(db *gorm.DB, tenantID int64) (*dto.TicketIntakePolicy, error) {
	var tenant models.Tenant
	if err := db.First(&tenant, tenantID).Error; err != nil {
		return nil, err
	}
	policy := &dto.TicketIntakePolicy{Rules: []dto.TicketIntakeRule{}}
	if value := strings.TrimSpace(tenant.TicketIntakePolicyJSON); value != "" {
		if err := json.Unmarshal([]byte(value), policy); err != nil {
			return nil, fmt.Errorf("invalid stored intake policy: %w", err)
		}
	}
	if err := ValidateTicketIntakePolicy(*policy); err != nil {
		return nil, err
	}
	return policy, nil
}

func GetTicketIntakePolicy(tenantID int64) (*dto.TicketIntakePolicy, error) {
	if tenantID <= 0 {
		return nil, errorsx.InvalidParam("tenant is required")
	}
	return ticketIntakePolicyDB(sqls.DB(), tenantID)
}

func UpdateTicketIntakePolicy(tenantID int64, policy dto.TicketIntakePolicy, operator *dto.AuthPrincipal) error {
	if operator == nil || operator.TenantID != tenantID || !operator.HasPermission(constants.PermissionTicketUpdate.Code) {
		return errorsx.ForbiddenI18n("error.e0225")
	}
	if err := ValidateTicketIntakePolicy(policy); err != nil {
		return err
	}
	if _, err := GetTicketIntakePolicy(tenantID); err != nil {
		return err
	}
	encoded, err := json.Marshal(policy)
	if err != nil {
		return err
	}
	return repositories.TenantRepository.Updates(sqls.DB(), tenantID, map[string]any{"ticket_intake_policy_json": string(encoded), "updated_at": time.Now(), "update_user_id": operator.UserID, "update_user_name": operator.Username})
}

// EvaluateTicketIntake uses only recorded values and explicitly reports an absent rule.
func EvaluateTicketIntake(ticket *models.Ticket, policy dto.TicketIntakePolicy) []string {
	missing := []string{}
	if ticket.ProjectKey == "" {
		missing = append(missing, "project_key")
	}
	if ticket.TicketType == "" {
		missing = append(missing, "ticket_type")
	}
	present := map[string]bool{
		"caller_name": strings.TrimSpace(ticket.CallerName) != "", "caller_phone": strings.TrimSpace(ticket.CallerPhone) != "",
		"customer_id": ticket.CustomerID > 0, "product_id": ticket.ProductID > 0, "device_id": ticket.DeviceID > 0, "service_region": strings.TrimSpace(ticket.ServiceRegion) != "",
	}
	matched := false
	for _, rule := range policy.Rules {
		if rule.ProjectKey != ticket.ProjectKey || rule.Channel != ticket.Channel || rule.TicketType != ticket.TicketType {
			continue
		}
		matched = true
		for _, field := range rule.RequiredFields {
			if !present[field] {
				missing = append(missing, field)
			}
		}
		break
	}
	if !matched {
		missing = append(missing, "projectPolicy")
	}
	return missing
}

func refreshTicketIntakeDB(db *gorm.DB, ticket *models.Ticket) error {
	if ticket.SourceRecordID == "" {
		return nil
	}
	policy, err := ticketIntakePolicyDB(db, ticket.TenantID)
	if err != nil {
		return err
	}
	missing := EvaluateTicketIntake(ticket, *policy)
	encoded, err := json.Marshal(missing)
	if err != nil {
		return err
	}
	ticket.MissingContextJSON = string(encoded)
	ticket.ContextStatus = "complete"
	if len(missing) > 0 {
		ticket.ContextStatus = "context_incomplete"
	}
	return nil
}

func validateIntakeText(input dto.TicketIntakeInput) error {
	for _, field := range []struct {
		name, value string
		limit       int
	}{
		{"source_record_id", input.SourceRecordID, 160}, {"project_key", input.ProjectKey, 64}, {"ticket_type", input.TicketType, 64}, {"caller_name", input.CallerName, 120}, {"caller_phone", input.CallerPhone, 64},
	} {
		if utf8.RuneCountInString(field.value) > field.limit || strings.ContainsAny(field.value, "\x00\r\n") {
			return errorsx.InvalidParam("invalid " + field.name)
		}
	}
	return nil
}

func prepareTicketIntakeDB(db *gorm.DB, ticket *models.Ticket, input dto.TicketIntakeInput) error {
	if ticket.Source == enums.TicketSourceManual && !intakeChannels[ticket.Channel] {
		switch ticket.Channel {
		case "", "enterprise", "dashboard", "web", "im", "widget":
		default:
			return errorsx.InvalidParam("unsupported manual intake channel")
		}
	}
	if ticket.Channel != "phone" && input == (dto.TicketIntakeInput{}) {
		// Existing non-intake channels keep their original behavior.
		return nil
	}
	if !intakeChannels[ticket.Channel] {
		return errorsx.InvalidParam("unsupported intake channel")
	}
	if ticket.Channel == "phone" && ticket.Source != enums.TicketSourceManual {
		return errorsx.InvalidParam("phone intake requires manual source")
	}
	if ticket.TenantID <= 0 {
		return errorsx.InvalidParam("phone intake requires tenant identity")
	}
	if err := validateIntakeText(input); err != nil {
		return err
	}
	ticket.SourceRecordID = strings.TrimSpace(input.SourceRecordID)
	if ticket.SourceRecordID == "" {
		if ticket.Channel != "phone" {
			return errorsx.InvalidParam("source_record_id is required for external intake")
		}
		ticket.SourceRecordID = "MP-" + uuid.NewString()
	}
	sum := sha256.Sum256([]byte(ticket.Channel + "\x00" + ticket.SourceRecordID))
	key := hex.EncodeToString(sum[:])
	ticket.SourceRecordKey = &key
	var count int64
	if err := db.Model(&models.Ticket{}).Where("tenant_id = ? AND source_record_key = ?", ticket.TenantID, key).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errorsx.InvalidParam("source record already has a ticket in this tenant and channel")
	}
	ticket.ProjectKey = strings.TrimSpace(input.ProjectKey)
	ticket.TicketType = strings.TrimSpace(input.TicketType)
	ticket.CallerName = strings.TrimSpace(input.CallerName)
	ticket.CallerPhone = strings.TrimSpace(input.CallerPhone)
	ticket.ReceivedAt = input.ReceivedAt
	if ticket.ReceivedAt == nil {
		now := time.Now()
		ticket.ReceivedAt = &now
	}
	if ticket.ReceivedAt.IsZero() || ticket.ReceivedAt.After(time.Now().Add(time.Minute)) {
		return errorsx.InvalidParam("received_at must be a valid, non-future timestamp")
	}
	return refreshTicketIntakeDB(db, ticket)
}

func BuildTicketIntakeDTO(ticket *models.Ticket) dto.TicketIntakeDTO {
	missing := []string{}
	_ = json.Unmarshal([]byte(ticket.MissingContextJSON), &missing)
	status := ticket.ContextStatus
	if status == "" {
		status = "not_evaluated"
	}
	return dto.TicketIntakeDTO{TicketIntakeInput: dto.TicketIntakeInput{SourceRecordID: ticket.SourceRecordID, ProjectKey: ticket.ProjectKey, TicketType: ticket.TicketType, CallerName: ticket.CallerName, CallerPhone: ticket.CallerPhone, ReceivedAt: ticket.ReceivedAt}, ContextStatus: status, MissingContext: missing}
}

func CompleteTicketIntake(ticketID int64, input dto.CompleteTicketIntakeRequest, operator *dto.AuthPrincipal) error {
	if operator == nil || operator.TenantID <= 0 || !operator.HasPermission(constants.PermissionTicketUpdate.Code) {
		return errorsx.ForbiddenI18n("error.e0225")
	}
	if input.CustomerID < 0 || input.ProductID < 0 || input.DeviceID < 0 || utf8.RuneCountInString(input.ServiceRegion) > 64 {
		return errorsx.InvalidParam("invalid intake context")
	}
	if err := validateIntakeText(dto.TicketIntakeInput{ProjectKey: input.ProjectKey, TicketType: input.TicketType, CallerName: input.CallerName, CallerPhone: input.CallerPhone}); err != nil {
		return err
	}
	if err := requireTicketMutationAccess(TicketService.Get(ticketID), operator); err != nil {
		return err
	}
	scope := resolveEnterpriseProductAccessScope(operator.TenantID, operator)
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		var ticket models.Ticket
		if err := ctx.Tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ?", ticketID, operator.TenantID).First(&ticket).Error; err != nil {
			return errorsx.ForbiddenI18n("error.e0225")
		}
		if ticket.Channel != "phone" || ticket.SourceRecordID == "" {
			return errorsx.InvalidParam("ticket is not a recorded manual phone intake")
		}
		if !scope.canAccessTicket(&ticket) || (operator.HasRole(EnterpriseRoleEngineer) && !canManageTicketDispatch(operator) && ticket.CurrentAssigneeID != operator.UserID) {
			return errorsx.ForbiddenI18n("error.e0225")
		}
		before := BuildTicketIntakeDTO(&ticket)
		beforeContext := map[string]any{"customer_id": ticket.CustomerID, "product_id": ticket.ProductID, "device_id": ticket.DeviceID, "service_region": ticket.ServiceRegion}
		if input.CustomerID > 0 {
			if ticket.CustomerID > 0 && input.CustomerID != ticket.CustomerID {
				return errorsx.InvalidParam("existing customer identity cannot be replaced through intake completion")
			}
			if err := TicketService.validateTicketRefsDB(ctx.Tx, input.CustomerID, ticket.ConversationID, 0, ticket.TenantID); err != nil {
				return err
			}
			ticket.CustomerID = input.CustomerID
		}
		// Original caller snapshots may only be supplemented when absent.
		if ticket.CallerName != "" && strings.TrimSpace(input.CallerName) != ticket.CallerName || ticket.CallerPhone != "" && strings.TrimSpace(input.CallerPhone) != ticket.CallerPhone {
			return errorsx.InvalidParam("original caller snapshot cannot be overwritten")
		}
		ticket.ProjectKey, ticket.TicketType = strings.TrimSpace(input.ProjectKey), strings.TrimSpace(input.TicketType)
		ticket.CallerName, ticket.CallerPhone = strings.TrimSpace(input.CallerName), strings.TrimSpace(input.CallerPhone)
		if input.ProductID > 0 {
			if !scope.canAccessProduct(input.ProductID) {
				return errorsx.ForbiddenI18n("error.e0225")
			}
			ticket.ProductID = input.ProductID
		}
		if input.DeviceID > 0 {
			ticket.DeviceID = input.DeviceID
		}
		if !scope.canAccessTicket(&ticket) {
			return errorsx.ForbiddenI18n("error.e0225")
		}
		if strings.TrimSpace(input.ServiceRegion) != "" {
			ticket.ServiceRegion = strings.TrimSpace(input.ServiceRegion)
		}
		if err := TicketService.validateAfterSalesContextDB(ctx.Tx, ticket.TenantID, ticket.ProductID, ticket.ProductModelID, ticket.ProductModuleID, ticket.DeviceID, ticket.ServiceCodeID, ticket.CustomerEntrySessionID); err != nil {
			return err
		}
		if err := refreshTicketIntakeDB(ctx.Tx, &ticket); err != nil {
			return err
		}
		updates := map[string]any{"project_key": ticket.ProjectKey, "ticket_type": ticket.TicketType, "caller_name": ticket.CallerName, "caller_phone": ticket.CallerPhone, "product_id": ticket.ProductID, "device_id": ticket.DeviceID, "service_region": ticket.ServiceRegion, "context_status": ticket.ContextStatus, "missing_context_json": ticket.MissingContextJSON, "updated_at": time.Now(), "update_user_id": operator.UserID, "update_user_name": operator.Username}
		updates["customer_id"] = ticket.CustomerID
		if err := repositories.TicketRepository.Updates(ctx.Tx, ticket.ID, updates); err != nil {
			return err
		}
		metadata, err := json.Marshal(map[string]any{"before": before, "after": BuildTicketIntakeDTO(&ticket), "before_context": beforeContext, "customer_id": ticket.CustomerID, "product_id": ticket.ProductID, "device_id": ticket.DeviceID, "service_region": ticket.ServiceRegion})
		if err != nil {
			return err
		}
		return repositories.TicketProgressRepository.Create(ctx.Tx, &models.TicketProgress{TenantID: ticket.TenantID, TicketID: ticket.ID, EventType: enums.TicketProgressEventProgress, Content: "Phone intake context supplemented", MetadataJSON: string(metadata), AuthorID: operator.UserID, CreatedAt: time.Now()})
	})
}
