package services

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/emersion/go-imap/v2/imapserver/imapmemserver"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/constants"
	"remotehelpdesk/internal/pkg/dto"
	"remotehelpdesk/internal/pkg/dto/request"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/secretstore"
)

func setupEmailMVP(t *testing.T) (*gorm.DB, *dto.AuthPrincipal) {
	t.Helper()
	previous := config.CurrentOrDefault()
	t.Cleanup(func() { config.SetCurrent(&previous) })
	config.SetCurrent(&config.Config{EncryptionKey: "email-mvp-synthetic-key-0123456789"})
	t.Setenv("RHD_PROJECT_CONFIG_FILE", "")
	t.Setenv("RHD_PROJECT_CONFIG_REQUIRED", "")
	t.Setenv("RHD_PROJECT_ENVIRONMENT", "development")
	db := setupSLATenantTestDB(t)
	db.Logger = logger.Default.LogMode(logger.Silent)
	require.NoError(t, db.AutoMigrate(models.Models...))
	require.NoError(t, db.Create(&models.Tenant{ID: 1, Name: "Synthetic mail tenant", ServiceScene: "knowledge_support", Status: enums.StatusOk}).Error)
	require.NoError(t, db.Create(&models.Tenant{ID: 2, Name: "Other mail tenant", ServiceScene: "knowledge_support", Status: enums.StatusOk}).Error)
	return db, &dto.AuthPrincipal{TenantID: 1, UserID: 7, DomainType: models.DomainTypeEnterprise, Roles: []string{EnterpriseRoleAdmin}, Permissions: []string{constants.PermissionTicketView.Code, constants.PermissionTicketProgress.Code}}
}
func sampleInbound(id string) InboundEmailInput {
	return InboundEmailInput{TenantID: 1, Mailbox: "support@example.test", FromAddress: "customer@example.test", MessageID: id, Subject: "无法登录", TextBody: "登录页面报错，请协助处理"}
}
func emailCount(t *testing.T, db *gorm.DB, model any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(model).Count(&n).Error)
	return n
}

func TestEmailMVPAtomicIngestAndThreadIsolation(t *testing.T) {
	db, op := setupEmailMVP(t)
	input := sampleInbound("original@example.test")
	// A downstream failure must roll the receipt back and allow an exact retry.
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("fail-email-ticket", func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "Ticket" {
			tx.AddError(errors.New("fixture write failure"))
		}
	}))
	_, err := IngestInboundEmail(input)
	require.Error(t, err)
	require.EqualValues(t, 0, emailCount(t, db, &models.InboundEmail{}))
	require.NoError(t, db.Callback().Create().Remove("fail-email-ticket"))
	created, err := IngestInboundEmail(input)
	require.NoError(t, err)
	require.Equal(t, InboundEmailCreated, created.Action)
	require.Less(t, len(created.SourceRecordID), 100)
	duplicate, err := IngestInboundEmail(input)
	require.NoError(t, err)
	require.Equal(t, InboundEmailDuplicate, duplicate.Action)
	require.EqualValues(t, 1, emailCount(t, db, &models.Ticket{}))
	input.TextBody = "same message ID, different body"
	_, err = IngestInboundEmail(input)
	require.Error(t, err)
	reply := sampleInbound("reply@example.test")
	reply.InReplyTo = "<original@example.test>"
	appended, err := IngestInboundEmail(reply)
	require.NoError(t, err)
	require.Equal(t, created.TicketID, appended.TicketID)
	require.Equal(t, InboundEmailAppended, appended.Action)
	require.EqualValues(t, 2, emailCount(t, db, &models.TicketProgress{}))
	for _, tc := range []struct {
		name   string
		mutate func(*InboundEmailInput)
	}{
		{"sender", func(i *InboundEmailInput) { i.FromAddress = "other@example.test" }},
		{"tenant", func(i *InboundEmailInput) { i.TenantID = 2 }},
		{"mailbox", func(i *InboundEmailInput) { i.Mailbox = "another@example.test" }},
		{"unknown", func(i *InboundEmailInput) { i.InReplyTo = "unknown@example.test" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			i := reply
			i.MessageID = tc.name + "@example.test"
			tc.mutate(&i)
			r, e := IngestInboundEmail(i)
			require.NoError(t, e)
			require.Equal(t, InboundEmailPendingManual, r.Action)
			require.Zero(t, r.TicketID)
		})
	}
	marker := sampleInbound("marker@example.test")
	marker.Subject = "Re: [Ticket #" + created.TicketNo + "]"
	r, err := IngestInboundEmail(marker)
	require.NoError(t, err)
	require.Equal(t, created.TicketID, r.TicketID)
	plain := sampleInbound("plain@example.test")
	plain.Subject = "Invoice 123 or " + created.TicketNo
	r, err = IngestInboundEmail(plain)
	require.NoError(t, err)
	require.Equal(t, InboundEmailCreated, r.Action)
	require.NotEqual(t, created.TicketID, r.TicketID)
	conflict := reply
	conflict.MessageID = "conflict@example.test"
	conflict.Subject = "[Ticket #" + r.TicketNo + "]"
	r, err = IngestInboundEmail(conflict)
	require.NoError(t, err)
	require.Equal(t, InboundEmailPendingManual, r.Action)
	history, err := GetTicketEmailHistory(created.TicketID, op)
	require.NoError(t, err)
	require.Len(t, history.Incoming, 3)
	foreign := *op
	foreign.TenantID = 2
	_, err = GetTicketEmailHistory(created.TicketID, &foreign)
	require.Error(t, err)
	_, err = QueueTicketEmailReply(created.TicketID, "回复", "wrong-tenant", &foreign)
	require.Error(t, err)
	denied := *op
	denied.Permissions = nil
	_, err = QueueTicketEmailReply(created.TicketID, "回复", "denied", &denied)
	require.Error(t, err)
}

func TestEmailMVPMIMEAndAttachmentBoundary(t *testing.T) {
	raw := "From: Customer <customer@example.test>\r\nMessage-ID: <mime@example.test>\r\nSubject: =?UTF-8?B?5peg5rOV55m75b2V?=\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=test\r\n\r\n--test\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: base64\r\n\r\n5rWL6K+V\r\n--test\r\nContent-Type: application/octet-stream\r\nContent-Disposition: attachment; filename=unsafe.exe\r\n\r\nDO-NOT-IMPORT\r\n--test--\r\n"
	in := parseInboundMIME([]byte(raw), "fallback")
	require.Equal(t, "无法登录", in.Subject)
	require.Equal(t, "测试", in.TextBody)
	require.Equal(t, 1, in.AttachmentCount)
	require.Empty(t, in.HoldReason)
	rich := parseInboundMIME([]byte("From: customer@example.test\r\nContent-Type: text/html\r\n\r\n<p>Help &amp; support</p><script>secret()</script>"), "fallback")
	require.Contains(t, rich.TextBody, "Help & support")
	require.NotContains(t, rich.TextBody, "secret")
	require.Equal(t, "message_too_large", parseInboundMIME([]byte(strings.Repeat("x", maxEmailBytes+1)), "fallback").HoldReason)
	require.Equal(t, "invalid_sender", parseInboundMIME([]byte("From: not-an-email\r\n\r\nhelp"), "fallback").HoldReason)
	require.Equal(t, "automated_email", parseInboundMIME([]byte("From: customer@example.test\r\nAuto-Submitted: auto-replied\r\n\r\nhelp"), "fallback").HoldReason)
}

func TestEmailMVPSettingsPersistAndSecrets(t *testing.T) {
	db, _ := setupEmailMVP(t)
	req := request.UpdateMailSettingRequest{FromAddress: "support@example.test", SMTPHost: "smtp.example.test", SMTPPort: 465, Password: "smtp-fixture", UseTLS: true, IMAPHost: "imap.example.test", IMAPPort: 993, IMAPUsername: "support@example.test", IMAPPassword: "imap-fixture", IMAPUseTLS: true, IMAPEnabled: true}
	got, err := MailSettingService.Save(1, req)
	require.NoError(t, err)
	require.True(t, got.HasIMAPPassword)
	var stored models.TenantMailSetting
	require.NoError(t, db.Where("tenant_id = 1").First(&stored).Error)
	require.NotEqual(t, "imap-fixture", stored.IMAPPassword)
	req.IMAPHost = "new.example.test"
	req.IMAPPassword = ""
	req.Password = ""
	_, err = MailSettingService.Save(1, req)
	require.NoError(t, err)
	cfg, err := resolveInboundMailbox(1)
	require.NoError(t, err)
	require.Equal(t, "new.example.test", cfg.Host)
	require.Equal(t, "imap-fixture", cfg.Password)
	req.IMAPUseTLS = false
	_, err = MailSettingService.Save(1, req)
	require.Error(t, err)
	req.IMAPEnabled = false
	_, err = MailSettingService.Save(1, req)
	require.NoError(t, err)
	cfg, err = resolveInboundMailbox(1)
	require.NoError(t, err)
	require.Nil(t, cfg)
	_, err = PollEnabledInboundMail()
	require.NoError(t, err, "unconfigured tenants must be skipped")
}

type emailLiteral struct {
	*strings.Reader
	n int64
}

func (l emailLiteral) Size() int64 { return l.n }
func startEmailIMAP(t *testing.T) (*imapmemserver.User, imapDialer) {
	t.Helper()
	memory := imapmemserver.New()
	user := imapmemserver.NewUser("support@example.test", "imap-fixture")
	require.NoError(t, user.Create("INBOX", nil))
	memory.AddUser(user)
	server := imapserver.New(&imapserver.Options{InsecureAuth: true, NewSession: func(*imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
		return memory.NewSession(), nil, nil
	}})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = server.Serve(ln) }()
	t.Cleanup(func() { _ = server.Close() })
	return user, func(ctx context.Context, _ inboundMailbox) (*imapclient.Client, error) {
		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", ln.Addr().String())
		if err != nil {
			return nil, err
		}
		return imapclient.New(conn, nil), nil
	}
}
func appendEmailFixture(t *testing.T, user *imapmemserver.User, id, thread string) {
	t.Helper()
	raw := "From: customer@example.test\r\nTo: support@example.test\r\nMessage-ID: <" + id + ">\r\nSubject: Help\r\n"
	if thread != "" {
		raw += "In-Reply-To: <" + thread + ">\r\n"
	}
	raw += "\r\nPlease help with login"
	_, err := user.Append("INBOX", emailLiteral{strings.NewReader(raw), int64(len(raw))}, &imap.AppendOptions{})
	require.NoError(t, err)
}
func seedEmailMailbox(t *testing.T, db *gorm.DB) inboundMailbox {
	t.Helper()
	password, err := secretstore.Encrypt("imap-fixture")
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.TenantMailSetting{TenantID: 1, IMAPHost: "fixture.test", IMAPPort: 993, IMAPUsername: "support@example.test", IMAPPassword: password, IMAPUseTLS: true, IMAPEnabled: true, Status: int(enums.StatusOk)}).Error)
	s, err := resolveInboundMailbox(1)
	require.NoError(t, err)
	return *s
}
func TestEmailMVPIMAPCursorBatchFailureAndReadFlags(t *testing.T) {
	db, _ := setupEmailMVP(t)
	s := seedEmailMailbox(t, db)
	user, dial := startEmailIMAP(t)
	appendEmailFixture(t, user, "old@example.test", "")
	n, err := pollMailbox(context.Background(), s, dial)
	require.NoError(t, err)
	require.Zero(t, n)
	for i := 0; i < 23; i++ {
		appendEmailFixture(t, user, fmt.Sprintf("new-%d@example.test", i), "")
	}
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("fail-poll", func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Name == "Ticket" {
			tx.AddError(errors.New("fixture failure"))
		}
	}))
	_, err = pollMailbox(context.Background(), s, dial)
	require.Error(t, err)
	var cursor models.MailboxSyncState
	require.NoError(t, db.First(&cursor).Error)
	require.EqualValues(t, 1, cursor.LastUID)
	require.NoError(t, db.Callback().Create().Remove("fail-poll"))
	n, err = pollMailbox(context.Background(), s, dial)
	require.NoError(t, err)
	require.Equal(t, 20, n)
	n, err = pollMailbox(context.Background(), s, dial)
	require.NoError(t, err)
	require.Equal(t, 3, n)
	n, err = pollMailbox(context.Background(), s, dial)
	require.NoError(t, err)
	require.Zero(t, n)
	require.EqualValues(t, 23, emailCount(t, db, &models.Ticket{}))
	client, err := dial(context.Background(), s)
	require.NoError(t, err)
	defer client.Close()
	require.NoError(t, client.Login(s.Username, s.Password).Wait())
	_, err = client.Select("INBOX", nil).Wait()
	require.NoError(t, err)
	found, err := client.Search(&imap.SearchCriteria{Flag: []imap.Flag{imap.FlagSeen}}, nil).Wait()
	require.NoError(t, err)
	require.Empty(t, found.AllSeqNums(), "polling must never mark messages read")
	// A UIDValidity reset starts a fresh baseline rather than interpreting old UIDs.
	require.NoError(t, db.Model(&models.MailboxSyncState{}).Where("tenant_id = 1").Update("uid_validity", 999).Error)
	n, err = pollMailbox(context.Background(), s, dial)
	require.NoError(t, err)
	require.Zero(t, n)
}

func startEmailSMTP(t *testing.T, uncertain bool) (int, <-chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	received := make(chan string, 2)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
		r := bufio.NewReader(conn)
		fmt.Fprint(conn, "220 localhost fixture\r\n")
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO"):
				fmt.Fprint(conn, "250 localhost\r\n")
			case strings.HasPrefix(line, "DATA"):
				fmt.Fprint(conn, "354 send data\r\n")
				var b strings.Builder
				for {
					s, e := r.ReadString('\n')
					if e != nil {
						return
					}
					if s == ".\r\n" {
						break
					}
					b.WriteString(s)
				}
				received <- b.String()
				if uncertain {
					return
				}
				fmt.Fprint(conn, "250 accepted\r\n")
			default:
				fmt.Fprint(conn, "250 OK\r\n")
			}
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, received
}
func TestEmailMVPEndToEndIMAPSMTPReplyToSameTicket(t *testing.T) {
	db, op := setupEmailMVP(t)
	s := seedEmailMailbox(t, db)
	user, dial := startEmailIMAP(t)
	_, err := pollMailbox(context.Background(), s, dial)
	require.NoError(t, err)
	appendEmailFixture(t, user, "first@example.test", "")
	n, err := pollMailbox(context.Background(), s, dial)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	var ticket models.Ticket
	require.NoError(t, db.First(&ticket).Error)
	require.Equal(t, "email", ticket.Channel)
	queued, err := QueueTicketEmailReply(ticket.ID, "您好，请尝试重置密码。", "send-once", op)
	require.NoError(t, err)
	again, err := QueueTicketEmailReply(ticket.ID, queued.Body, "send-once", op)
	require.NoError(t, err)
	require.Equal(t, queued.ID, again.ID)
	_, err = QueueTicketEmailReply(ticket.ID, "changed", "send-once", op)
	require.Error(t, err)
	port, received := startEmailSMTP(t, false)
	require.NoError(t, db.Model(&models.TenantMailSetting{}).Where("tenant_id = 1").Updates(map[string]any{"smtp_host": "127.0.0.1", "smtp_port": port, "from_address": s.Username, "use_tls": false}).Error)
	require.NoError(t, DispatchTicketEmailReply(context.Background(), queued.ID))
	require.NoError(t, db.First(queued, queued.ID).Error)
	require.Equal(t, "sent", queued.Status)
	select {
	case raw := <-received:
		require.Contains(t, raw, "In-Reply-To: <first@example.test>")
		require.Contains(t, raw, "Reply-To: support@example.test")
		parsed := parseInboundMIME([]byte(raw), "")
		require.Equal(t, queued.MessageID, parsed.MessageID)
		require.Equal(t, queued.Body, parsed.TextBody)
	case <-time.After(time.Second):
		t.Fatal("SMTP received no message")
	}
	require.NoError(t, DispatchTicketEmailReply(context.Background(), queued.ID)) // no second network send
	appendEmailFixture(t, user, "customer-response@example.test", queued.MessageID)
	n, err = pollMailbox(context.Background(), s, dial)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.EqualValues(t, 1, emailCount(t, db, &models.Ticket{}))
	history, err := GetTicketEmailHistory(ticket.ID, op)
	require.NoError(t, err)
	require.Len(t, history.Incoming, 2)
	require.Len(t, history.Replies, 1)
}
func TestEmailMVPUnknownSMTPOutcomeDoesNotResend(t *testing.T) {
	db, op := setupEmailMVP(t)
	s := seedEmailMailbox(t, db)
	in, err := IngestInboundEmail(sampleInbound("unknown-send@example.test"))
	require.NoError(t, err)
	queued, err := QueueTicketEmailReply(in.TicketID, "test response", "uncertain", op)
	require.NoError(t, err)
	port, received := startEmailSMTP(t, true)
	require.NoError(t, db.Model(&models.TenantMailSetting{}).Where("tenant_id = 1").Updates(map[string]any{"smtp_host": "127.0.0.1", "smtp_port": port, "from_address": s.Username, "use_tls": false}).Error)
	require.NoError(t, DispatchTicketEmailReply(context.Background(), queued.ID))
	<-received
	require.NoError(t, db.First(queued, queued.ID).Error)
	require.Equal(t, "unknown", queued.Status)
	ProcessTicketEmailReplies()
	require.NoError(t, db.First(queued, queued.ID).Error)
	require.Equal(t, "unknown", queued.Status)
}
