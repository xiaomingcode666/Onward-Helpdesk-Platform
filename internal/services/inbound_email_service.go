package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
)

const (
	InboundEmailCreated       = "ticket_created"
	InboundEmailAppended      = "ticket_appended"
	InboundEmailPendingManual = "pending_manual_link"
	InboundEmailDuplicate     = "duplicate"
	maxEmailBytes             = 2 * 1024 * 1024
)

type InboundEmailInput struct {
	TenantID                       int64
	Mailbox, MessageID, InReplyTo  string
	References                     []string
	FromAddress, Subject, TextBody string
	ReceivedAt                     time.Time
	ProjectKey, TicketType         string
	AttachmentCount                int
	// HoldReason is a safe parser outcome, never raw server data.
	HoldReason string
}
type InboundEmailResult struct {
	Action         string `json:"action"`
	InboundEmailID int64  `json:"inbound_email_id"`
	TicketID       int64  `json:"ticket_id"`
	TicketNo       string `json:"ticket_no,omitempty"`
	SourceRecordID string `json:"source_record_id"`
	MatchedBy      string `json:"matched_by,omitempty"`
}

func ListPendingInboundEmailLinks(tenantID int64) ([]models.InboundEmail, error) {
	if tenantID <= 0 {
		return nil, errors.New("tenant is required")
	}
	var rows []models.InboundEmail
	if err := sqls.DB().Where("tenant_id = ? AND status = ?", tenantID, InboundEmailPendingManual).Order("received_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func emailHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func normalizeMailID(value string) string { return strings.Trim(strings.TrimSpace(value), "<>") }
func emailAddress(value string) (string, error) {
	if strings.ContainsAny(value, "\r\n") {
		return "", errors.New("invalid email address")
	}
	a, err := mail.ParseAddress(value)
	if err != nil || len(a.Address) > 255 {
		return "", errors.New("invalid email address")
	}
	return strings.ToLower(a.Address), nil
}

// Receipt + ticket + progress + event commit together. Failed attempts roll
// back, so retry never mistakes a partially handled receipt for success.
func IngestInboundEmail(input InboundEmailInput) (*InboundEmailResult, error) {
	var err error
	input.Mailbox, err = emailAddress(input.Mailbox)
	if err != nil {
		return nil, err
	}
	input.FromAddress, err = emailAddress(input.FromAddress)
	if err != nil && input.HoldReason == "" {
		return nil, err
	}
	input.MessageID = normalizeMailID(input.MessageID)
	input.InReplyTo = normalizeMailID(input.InReplyTo)
	if input.TenantID <= 0 || input.MessageID == "" || len(input.MessageID) > 500 || strings.ContainsAny(input.MessageID+input.InReplyTo, "\r\n") || len(input.InReplyTo) > 500 || len(input.TextBody) > maxEmailBytes || len(input.References) > 100 {
		return nil, errors.New("invalid inbound email")
	}
	for i := range input.References {
		input.References[i] = normalizeMailID(input.References[i])
		if len(input.References[i]) > 500 || strings.ContainsAny(input.References[i], "\r\n") {
			return nil, errors.New("invalid references")
		}
	}
	if input.ReceivedAt.IsZero() {
		input.ReceivedAt = time.Now().UTC()
	}
	if len([]rune(input.Subject)) > 250 {
		input.Subject = string([]rune(input.Subject)[:250])
	}
	if strings.TrimSpace(input.Subject) == "" {
		input.Subject = "（无主题邮件）"
	}
	if strings.TrimSpace(input.TextBody) == "" && input.HoldReason == "" {
		input.HoldReason = "empty_body"
	}
	refs, _ := json.Marshal(input.References)
	payload, _ := json.Marshal([]any{input.FromAddress, input.Subject, input.TextBody, input.InReplyTo, input.References, input.AttachmentCount, input.HoldReason})
	result := &InboundEmailResult{}
	err = sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		db := ctx.Tx
		now := time.Now().UTC()
		row := models.InboundEmail{TenantID: input.TenantID, Mailbox: input.Mailbox, MessageID: input.MessageID, InReplyTo: input.InReplyTo, ReferencesJSON: string(refs), FromAddress: input.FromAddress, Subject: input.Subject, TextBody: input.TextBody, PayloadHash: emailHash(string(payload)), AttachmentCount: input.AttachmentCount, SourceRecordID: "email:" + emailHash(input.Mailbox+"\x00"+input.MessageID), Status: "received", ReceivedAt: input.ReceivedAt, CreatedAt: now, UpdatedAt: now}
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
			return err
		}
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND mailbox = ? AND message_id = ?", input.TenantID, input.Mailbox, input.MessageID).First(&row).Error; err != nil {
			return err
		}
		result.InboundEmailID, row.UpdatedAt = row.ID, now
		result.SourceRecordID, result.TicketID = row.SourceRecordID, row.TicketID
		if row.PayloadHash != "" && row.PayloadHash != emailHash(string(payload)) {
			return errors.New("email message-id payload conflict")
		}
		if row.ProcessedAt != nil {
			result.Action = InboundEmailDuplicate
			return nil
		}
		ticketID, matchedBy, hold, err := matchInboundTicket(db, input)
		if err != nil {
			return err
		}
		if input.HoldReason != "" {
			hold = input.HoldReason
		}
		if hold != "" {
			row.Status = InboundEmailPendingManual
			row.FailureReason = hold
		} else if ticketID > 0 {
			var ticket models.Ticket
			if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ?", ticketID, input.TenantID).First(&ticket).Error; err != nil {
				return err
			}
			meta, _ := json.Marshal(map[string]any{"channel": "email", "direction": "inbound", "inbound_email_id": row.ID, "attachment_count": row.AttachmentCount})
			progress := models.TicketProgress{TenantID: input.TenantID, TicketID: ticketID, EventType: enums.TicketProgressEventProgress, Content: input.TextBody, VisibleToCustomer: true, MetadataJSON: string(meta), CreatedAt: now}
			if err := db.Create(&progress).Error; err != nil {
				return err
			}
			if err := db.Model(&ticket).Update("updated_at", now).Error; err != nil {
				return err
			}
			row.TicketID, row.Status, row.MatchedBy = ticketID, InboundEmailAppended, matchedBy
			result.TicketNo = ticket.TicketNo
		} else {
			operator := &dto.AuthPrincipal{TenantID: input.TenantID, DomainType: "service_account", SubjectType: "service_account", Username: "email-ingest", Nickname: "邮件收件"}
			received := now
			req := request.CreateTicketRequest{Title: input.Subject, Description: input.TextBody, Source: string(enums.TicketSourceEmail), Channel: "email", IdempotencyKey: row.SourceRecordID, TenantID: input.TenantID, TicketIntakeInput: dto.TicketIntakeInput{SourceRecordID: row.SourceRecordID, ProjectKey: input.ProjectKey, TicketType: input.TicketType, ReceivedAt: &received}}
			prepared, existing, err := TicketService.prepareTicketCreate(db, req, operator)
			if err != nil {
				return err
			}
			if existing == nil {
				if err := TicketService.createTicketPreparedTx(ctx, prepared, operator); err != nil {
					return err
				}
				existing = prepared.ticket
			}
			row.TicketID, row.Status = existing.ID, InboundEmailCreated
			result.TicketNo = existing.TicketNo
		}
		row.ProcessedAt = &now
		if err := db.Save(&row).Error; err != nil {
			return err
		}
		result.Action, result.TicketID, result.MatchedBy = row.Status, row.TicketID, row.MatchedBy
		return nil
	})
	return result, err
}

// ResolveInboundEmailManualLink attaches a pending email reply only after an
// operator explicitly selects the target ticket. A pending receipt is never
// guessed into a ticket by this method, and a second resolution cannot move it
// to another ticket silently.
func ResolveInboundEmailManualLink(inboundEmailID, ticketID int64, operator *dto.AuthPrincipal) error {
	if operator == nil || operator.TenantID <= 0 || inboundEmailID <= 0 || ticketID <= 0 {
		return errors.New("manual email link requires operator, inbound email and ticket")
	}
	return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
		now := time.Now().UTC()
		var row models.InboundEmail
		if err := ctx.Tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ?", inboundEmailID, operator.TenantID).First(&row).Error; err != nil {
			return err
		}
		if row.TicketID > 0 {
			if row.TicketID == ticketID && row.Status == InboundEmailAppended {
				return nil
			}
			return errors.New("inbound email is already linked to another ticket")
		}
		if row.Status != InboundEmailPendingManual {
			return errors.New("inbound email is not waiting for manual link")
		}
		var ticket models.Ticket
		if err := ctx.Tx.Where("id = ? AND tenant_id = ?", ticketID, operator.TenantID).First(&ticket).Error; err != nil {
			return err
		}
		metadata, _ := json.Marshal(map[string]any{
			"channel": "email", "direction": "inbound", "inbound_email_id": row.ID, "matched_by": "manual",
		})
		if err := ctx.Tx.Create(&models.TicketProgress{
			TenantID:          operator.TenantID,
			TicketID:          ticketID,
			EventType:         enums.TicketProgressEventProgress,
			Content:           row.TextBody,
			VisibleToCustomer: true,
			MetadataJSON:      string(metadata),
			CreatedAt:         now,
		}).Error; err != nil {
			return err
		}
		if err := ctx.Tx.Model(&ticket).Updates(map[string]any{"updated_at": now}).Error; err != nil {
			return err
		}
		return ctx.Tx.Model(&row).Updates(map[string]any{
			"ticket_id":      ticketID,
			"status":         InboundEmailAppended,
			"matched_by":     "manual",
			"failure_reason": "",
			"updated_at":     now,
		}).Error
	})
}

var emailTicketMarker = regexp.MustCompile(`(?i)\[Ticket #([A-Za-z0-9_-]{1,64})\]`)

// All hints must agree; subject numbers alone never authorize a sender.
func matchInboundTicket(db *gorm.DB, in InboundEmailInput) (int64, string, string, error) {
	ids := append([]string{in.InReplyTo}, in.References...)
	candidates := map[int64]bool{}
	hasThread := false
	for _, id := range ids {
		if id == "" {
			continue
		}
		hasThread = true
		var receipts []models.InboundEmail
		if err := db.Where("tenant_id = ? AND mailbox = ? AND message_id = ? AND ticket_id > 0", in.TenantID, in.Mailbox, id).Find(&receipts).Error; err != nil {
			return 0, "", "", err
		}
		for _, row := range receipts {
			candidates[row.TicketID] = true
		}
		var replies []models.TicketEmailReply
		if err := db.Where("tenant_id = ? AND mailbox = ? AND message_id = ?", in.TenantID, in.Mailbox, id).Find(&replies).Error; err != nil {
			return 0, "", "", err
		}
		for _, row := range replies {
			candidates[row.TicketID] = true
		}
	}
	matchedBy := "thread_header"
	for _, m := range emailTicketMarker.FindAllStringSubmatch(in.Subject, -1) {
		var ticket models.Ticket
		err := db.Where("tenant_id = ? AND ticket_no = ?", in.TenantID, m[1]).First(&ticket).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, "", "unknown_ticket_marker", nil
		}
		if err != nil {
			return 0, "", "", err
		}
		candidates[ticket.ID] = true
		if !hasThread {
			matchedBy = "subject_ticket_no"
		}
	}
	if len(candidates) > 1 {
		return 0, "", "conflicting_threads", nil
	}
	for ticketID := range candidates {
		var origin models.InboundEmail
		err := db.Where("tenant_id = ? AND mailbox = ? AND ticket_id = ? AND status = ?", in.TenantID, in.Mailbox, ticketID, InboundEmailCreated).Order("id").First(&origin).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, "", "unknown_email_participant", nil
		}
		if err != nil {
			return 0, "", "", err
		}
		if origin.FromAddress != in.FromAddress {
			return 0, "", "sender_mismatch", nil
		}
		return ticketID, matchedBy, "", nil
	}
	if hasThread {
		return 0, "", "unmatched_reply", nil
	}
	return 0, "", "", nil
}
