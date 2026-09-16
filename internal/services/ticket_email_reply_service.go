package services

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/errorsx"
)

type TicketEmailHistory struct {
	Incoming []TicketEmailIncoming     `json:"incoming"`
	Replies  []models.TicketEmailReply `json:"replies"`
}
type TicketEmailIncoming struct {
	ID              int64     `json:"id"`
	From            string    `json:"from"`
	Subject         string    `json:"subject"`
	Body            string    `json:"body"`
	AttachmentCount int       `json:"attachment_count"`
	ReceivedAt      time.Time `json:"received_at"`
}

func GetTicketEmailHistory(ticketID int64, operator *dto.AuthPrincipal) (*TicketEmailHistory, error) {
	if operator == nil || !operator.HasPermission(constants.PermissionTicketView.Code) {
		return nil, errorsx.Forbidden("无工单查看权限")
	}
	if _, err := EnterpriseTicketService.GetAggregateForOperator(operator.TenantID, ticketID, operator); err != nil {
		return nil, err
	}
	history := &TicketEmailHistory{Incoming: []TicketEmailIncoming{}, Replies: []models.TicketEmailReply{}}
	var rows []models.InboundEmail
	if err := sqls.DB().Where("tenant_id = ? AND ticket_id = ?", operator.TenantID, ticketID).Order("id DESC").Limit(50).Find(&rows).Error; err != nil {
		return nil, errors.New("邮件记录查询失败")
	}
	for _, row := range rows {
		history.Incoming = append(history.Incoming, TicketEmailIncoming{row.ID, row.FromAddress, row.Subject, row.TextBody, row.AttachmentCount, row.ReceivedAt})
	}
	if err := sqls.DB().Where("tenant_id = ? AND ticket_id = ?", operator.TenantID, ticketID).Order("id DESC").Limit(50).Find(&history.Replies).Error; err != nil {
		return nil, errors.New("回信记录查询失败")
	}
	return history, nil
}

func QueueTicketEmailReply(ticketID int64, body, key string, operator *dto.AuthPrincipal) (*models.TicketEmailReply, error) {
	if operator == nil || !operator.HasPermission(constants.PermissionTicketProgress.Code) {
		return nil, errorsx.Forbidden("无回复工单权限")
	}
	if _, err := EnterpriseTicketService.GetAggregateForOperator(operator.TenantID, ticketID, operator); err != nil {
		return nil, err
	}
	ticket := TicketService.Get(ticketID)
	if err := requireTicketMutationAccess(ticket, operator); err != nil {
		return nil, err
	}
	body = strings.TrimSpace(body)
	key = strings.TrimSpace(key)
	if body == "" || len(body) > 64000 || key == "" || len(key) > 80 {
		return nil, errorsx.InvalidParam("回复内容或请求编号无效")
	}
	var reply models.TicketEmailReply
	err := sqls.DB().Transaction(func(db *gorm.DB) error {
		hash := emailHash(fmt.Sprintf("%d:%s", operator.UserID, body))
		var origin models.InboundEmail
		if err := db.Where("tenant_id = ? AND ticket_id = ? AND status = ?", operator.TenantID, ticketID, InboundEmailCreated).Order("id").First(&origin).Error; err != nil {
			return errorsx.InvalidParam("此工单没有可回复的邮件来源")
		}
		var last models.InboundEmail
		if err := db.Where("tenant_id = ? AND ticket_id = ? AND from_address = ?", operator.TenantID, ticketID, origin.FromAddress).Order("id DESC").First(&last).Error; err != nil {
			return err
		}
		recipient, err := emailAddress(origin.FromAddress)
		if err != nil {
			return err
		}
		mailbox, err := emailAddress(origin.Mailbox)
		if err != nil {
			return err
		}
		domain := strings.Split(mailbox, "@")[1]
		now := time.Now().UTC()
		reply = models.TicketEmailReply{TenantID: operator.TenantID, TicketID: ticketID, RequestKey: key, PayloadHash: hash, MessageID: uuid.NewString() + "@" + domain, InReplyTo: last.MessageID, Recipient: recipient, Mailbox: mailbox, Subject: "[Ticket #" + ticket.TicketNo + "] " + ticket.Title, Body: body, Status: "queued", AuthorID: operator.UserID, CreatedAt: now, UpdatedAt: now}
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&reply).Error; err != nil {
			return err
		}
		if err := db.Where("tenant_id = ? AND ticket_id = ? AND request_key = ?", operator.TenantID, ticketID, key).First(&reply).Error; err != nil {
			return err
		}
		if reply.PayloadHash != hash {
			return errorsx.InvalidParam("同一发送请求的内容发生变化，请刷新后重试")
		}
		return nil
	})
	return &reply, err
}

// DispatchTicketEmailReply sends once. SMTP acceptance is not a delivery receipt.
func DispatchTicketEmailReply(ctx context.Context, id int64) error {
	db := sqls.DB()
	now := time.Now().UTC()
	claim := db.Model(&models.TicketEmailReply{}).Where("id = ? AND status = ?", id, "queued").Updates(map[string]any{"status": "sending", "updated_at": now})
	if claim.Error != nil {
		return errors.New("发送任务读取失败")
	}
	if claim.RowsAffected == 0 {
		return nil
	}
	var row models.TicketEmailReply
	if err := db.First(&row, id).Error; err != nil {
		return errors.New("发送任务读取失败")
	}
	status, code := "failed", "smtp_configuration"
	cfg, err := resolveProjectMailConfig(row.TenantID)
	if err == nil && cfg == nil {
		c := EmailNotificationService.resolveMailConfig(row.TenantID)
		cfg = &c
	}
	if err == nil && cfg != nil {
		enabled, e := resolveInboundMailbox(row.TenantID)
		if e == nil && enabled != nil && strings.EqualFold(enabled.Username, row.Mailbox) {
			status, code = sendTicketMail(ctx, *cfg, row)
		} else {
			code = "mailbox_disabled_or_changed"
		}
	}
	if err := db.Model(&row).Updates(map[string]any{"status": status, "error_code": code, "updated_at": time.Now().UTC()}).Error; err != nil {
		return errors.New("发送结果保存失败，请勿重复发送")
	}
	return nil
}

func ProcessTicketEmailReplies() {
	db := sqls.DB()
	// A crash while sending cannot prove whether the SMTP server accepted DATA.
	db.Model(&models.TicketEmailReply{}).Where("status = ? AND updated_at < ?", "sending", time.Now().Add(-5*time.Minute)).Updates(map[string]any{"status": "unknown", "error_code": "interrupted_send"})
	var rows []models.TicketEmailReply
	if db.Where("status = ?", "queued").Order("id").Limit(20).Find(&rows).Error != nil {
		return
	}
	for _, row := range rows {
		_ = DispatchTicketEmailReply(context.Background(), row.ID)
	}
}

func buildTicketEmail(cfg config.EmailConfig, row models.TicketEmailReply) ([]byte, error) {
	from, err := emailAddress(cfg.FromAddress)
	if err != nil {
		return nil, err
	}
	for _, v := range []string{row.MessageID, row.InReplyTo, row.Subject, cfg.FromName} {
		if strings.ContainsAny(v, "\r\n") {
			return nil, errors.New("invalid mail header")
		}
	}
	to, err := emailAddress(row.Recipient)
	if err != nil {
		return nil, err
	}
	replyTo, err := emailAddress(row.Mailbox)
	if err != nil {
		return nil, err
	}
	subject := mime.QEncoding.Encode("utf-8", row.Subject)
	body := base64.StdEncoding.EncodeToString([]byte(row.Body))
	var wrapped strings.Builder
	for len(body) > 76 {
		wrapped.WriteString(body[:76] + "\r\n")
		body = body[76:]
	}
	wrapped.WriteString(body + "\r\n")
	headers := []string{"From: " + (&mail.Address{Name: cfg.FromName, Address: from}).String(), "To: " + to, "Reply-To: " + replyTo, "Subject: " + subject, "Date: " + time.Now().UTC().Format(time.RFC1123Z), "Message-ID: <" + row.MessageID + ">", "In-Reply-To: <" + row.InReplyTo + ">", "References: <" + row.InReplyTo + ">", "MIME-Version: 1.0", "Content-Type: text/plain; charset=UTF-8", "Content-Transfer-Encoding: base64", "Auto-Submitted: no"}
	return []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + wrapped.String()), nil
}

func sendTicketMail(parent context.Context, cfg config.EmailConfig, row models.TicketEmailReply) (string, string) {
	raw, err := buildTicketEmail(cfg, row)
	if err != nil || cfg.SMTPHost == "" {
		return "failed", "invalid_sender_configuration"
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(cfg.SMTPHost, fmt.Sprint(cfg.SMTPPort)))
	if err != nil {
		return "failed", "smtp_connect"
	}
	rawConn := conn
	defer rawConn.Close()
	_ = rawConn.SetDeadline(time.Now().Add(30 * time.Second))
	stop := context.AfterFunc(ctx, func() { _ = rawConn.Close() })
	defer stop()
	tlsConfig := &tls.Config{ServerName: cfg.SMTPHost, MinVersion: tls.VersionTLS12}
	if cfg.UseTLS {
		secured := tls.Client(conn, tlsConfig)
		if secured.HandshakeContext(ctx) != nil {
			return "failed", "smtp_tls"
		}
		conn = secured
	}
	client, err := smtp.NewClient(conn, cfg.SMTPHost)
	if err != nil {
		return "failed", "smtp_greeting"
	}
	defer client.Close()
	if !cfg.UseTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if client.StartTLS(tlsConfig) != nil {
				return "failed", "smtp_tls"
			}
		} else if ip := net.ParseIP(cfg.SMTPHost); ip == nil || !ip.IsLoopback() {
			return "failed", "smtp_tls_required"
		}
	}
	if cfg.Username != "" {
		if client.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)) != nil {
			return "failed", "smtp_auth"
		}
	}
	from, _ := emailAddress(cfg.FromAddress)
	if client.Mail(from) != nil || client.Rcpt(row.Recipient) != nil {
		return "failed", "smtp_recipient"
	}
	writer, err := client.Data()
	if err != nil {
		return "failed", "smtp_data_rejected"
	}
	if _, err = writer.Write(raw); err != nil {
		return "unknown", "smtp_outcome_unknown"
	}
	if writer.Close() != nil {
		return "unknown", "smtp_outcome_unknown"
	}
	return "sent", ""
}
