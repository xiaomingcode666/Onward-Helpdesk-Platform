package services

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/errorsx"
)

// sourceRecordChannels 是允许携带外部来源记录建单的渠道，用于来源去重和字段校验。
var sourceRecordChannels = map[string]bool{"phone": true, "email": true, "monitoring_alert": true, "api": true, "webhook": true, "whatsapp": true, "chatbot_handoff": true}

func validateTicketSourceText(input dto.TicketIntakeInput) error {
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

// prepareTicketSourceDB 登记工单来源：来源记录去重、来电/来信人资料和服务项目。
// 它只做来源登记，不再按受理规则评估资料是否齐全。
func prepareTicketSourceDB(db *gorm.DB, ticket *models.Ticket, input dto.TicketIntakeInput) error {
	if ticket.Source == enums.TicketSourceManual && (ticket.Channel == "manual" || ticket.Channel == "enterprise" || ticket.Channel == "dashboard" || ticket.Channel == "web" || ticket.Channel == "") {
		// Service category and type can scope SLA without claiming a phone call.
		if err := validateTicketSourceText(input); err != nil {
			return err
		}
		if input.SourceRecordID != "" || input.CallerPhone != "" || input.CallerName != "" || input.ReceivedAt != nil {
			return errorsx.InvalidParam("电话信息请使用人工电话渠道")
		}
		ticket.ProjectKey = strings.TrimSpace(input.ProjectKey)
		ticket.TicketType = strings.TrimSpace(input.TicketType)
		return nil
	}
	if ticket.Source == enums.TicketSourceManual && !sourceRecordChannels[ticket.Channel] {
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
	if !sourceRecordChannels[ticket.Channel] {
		return errorsx.InvalidParam("unsupported intake channel")
	}
	if ticket.Channel == "phone" && ticket.Source != enums.TicketSourceManual {
		return errorsx.InvalidParam("phone intake requires manual source")
	}
	if ticket.TenantID <= 0 {
		return errorsx.InvalidParam("phone intake requires tenant identity")
	}
	if err := validateTicketSourceText(input); err != nil {
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
	return nil
}

// BuildTicketIntakeDTO 输出工单记录的来源信息，供详情页展示。
func BuildTicketIntakeDTO(ticket *models.Ticket) dto.TicketIntakeDTO {
	if ticket == nil {
		return dto.TicketIntakeDTO{}
	}
	return dto.TicketIntakeDTO{
		TicketIntakeInput: dto.TicketIntakeInput{
			SourceRecordID: ticket.SourceRecordID,
			ProjectKey:     ticket.ProjectKey,
			TicketType:     ticket.TicketType,
			CallerName:     ticket.CallerName,
			CallerPhone:    ticket.CallerPhone,
			ReceivedAt:     ticket.ReceivedAt,
		},
		ProjectConfigVersionID: ticket.ProjectConfigVersionID,
	}
}
