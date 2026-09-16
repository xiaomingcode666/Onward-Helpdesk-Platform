package services

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	_ "github.com/emersion/go-message/charset"
	mailmime "github.com/emersion/go-message/mail"
	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"
	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/projectconfig"
	"remotehelpdesk/internal/pkg/secretstore"
)

type inboundMailbox struct {
	TenantID                                   int64
	Host                                       string
	Port                                       int
	Username, Password, ProjectKey, TicketType string
}

func (m inboundMailbox) key() string {
	return emailHash(fmt.Sprintf("%s:%d/%s", strings.ToLower(m.Host), m.Port, strings.ToLower(m.Username)))
}

func resolveInboundMailbox(tenantID int64) (*inboundMailbox, error) {
	var tenant models.Tenant
	if err := sqls.DB().First(&tenant, tenantID).Error; err != nil {
		return nil, err
	}
	if tenant.Status != enums.StatusOk {
		return nil, nil
	}
	r, _, err := projectRuntimeDB(sqls.DB(), tenantID, 0)
	if err != nil {
		return nil, err
	}
	if r != nil {
		i := r.Mail.IMAP
		if i == nil || !i.Enabled {
			return nil, nil
		}
		enabled := false
		for _, ch := range r.Channels {
			if ch.Name == "email" {
				enabled = ch.Enabled
			}
		}
		if !enabled {
			return nil, nil
		}
		secret, err := projectconfig.ReadSecret(os.Getenv("RHD_PROJECT_SECRET_DIR"), tenantID, projectconfig.Environment(), i.PasswordRef)
		if err != nil {
			return nil, errors.New("IMAP 密钥不可读取")
		}
		return &inboundMailbox{tenantID, i.Host, i.Port, i.Username, strings.TrimSpace(string(secret)), i.ProjectKey, i.TicketType}, nil
	}
	var s models.TenantMailSetting
	err = sqls.DB().Where("tenant_id = ?", tenantID).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !s.IMAPEnabled || s.Status != int(enums.StatusOk) {
		return nil, nil
	}
	if !s.IMAPUseTLS {
		return nil, errors.New("IMAP 必须启用 TLS")
	}
	password, err := secretstore.Decrypt(s.IMAPPassword)
	if err != nil {
		return nil, errors.New("IMAP 密钥不可读取")
	}
	return &inboundMailbox{TenantID: tenantID, Host: s.IMAPHost, Port: s.IMAPPort, Username: s.IMAPUsername, Password: password}, nil
}

// PollEnabledInboundMail uses the durable cursor, never Seen flags. Each tenant
// failure is isolated. Raw server errors/credentials are never logged.
func PollEnabledInboundMail() (int, error) {
	var tenants []models.Tenant
	if err := sqls.DB().Where("status = ?", enums.StatusOk).Find(&tenants).Error; err != nil {
		return 0, errors.New("邮箱租户查询失败")
	}
	total, failed := 0, 0
	for _, tenant := range tenants {
		setting, err := resolveInboundMailbox(tenant.ID)
		if err != nil {
			failed++
			continue
		}
		if setting == nil {
			continue
		}
		n, err := pollMailbox(context.Background(), *setting, dialInboundIMAP)
		total += n
		if err != nil {
			failed++
		}
	}
	if failed > 0 {
		return total, fmt.Errorf("%d 个邮箱收件失败，请在接入管理查看收件状态", failed)
	}
	return total, nil
}
func PollTenantInboundMail(ctx context.Context, tenantID int64) (int, error) {
	setting, err := resolveInboundMailbox(tenantID)
	if err != nil {
		return 0, err
	}
	if setting == nil {
		return 0, errors.New("尚未启用邮件收件")
	}
	return pollMailbox(ctx, *setting, dialInboundIMAP)
}

type imapDialer func(context.Context, inboundMailbox) (*imapclient.Client, error)

func dialInboundIMAP(ctx context.Context, s inboundMailbox) (*imapclient.Client, error) {
	conn, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(s.Host, fmt.Sprint(s.Port)))
	if err != nil {
		return nil, err
	}
	secured := tls.Client(conn, &tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12})
	if err := secured.HandshakeContext(ctx); err != nil {
		conn.Close()
		return nil, err
	}
	return imapclient.New(secured, nil), nil
}

func pollMailbox(parent context.Context, s inboundMailbox, dial imapDialer) (count int, err error) {
	ctx, cancel := context.WithTimeout(parent, 60*time.Second)
	defer cancel()
	db := sqls.DB()
	state := models.MailboxSyncState{TenantID: s.TenantID, MailboxKey: s.key()}
	if e := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&state).Error; e != nil {
		return 0, errors.New("收件进度初始化失败")
	}
	if e := db.Where("tenant_id = ? AND mailbox_key = ?", s.TenantID, s.key()).First(&state).Error; e != nil {
		return 0, errors.New("收件进度读取失败")
	}
	token := uuid.NewString()
	now := time.Now().UTC()
	locked := db.Model(&state).Where("lease_until IS NULL OR lease_until < ?", now).Updates(map[string]any{"lease_token": token, "lease_until": now.Add(2 * time.Minute)})
	if locked.Error != nil {
		return 0, errors.New("收件锁获取失败")
	}
	if locked.RowsAffected == 0 {
		return 0, nil
	}
	defer func() {
		updates := map[string]any{"lease_until": nil, "lease_token": "", "last_error": ""}
		if err != nil {
			updates["last_error"] = err.Error()
		} else {
			updates["last_success_at"] = time.Now().UTC()
		}
		if e := db.Model(&state).Where("lease_token = ?", token).Updates(updates).Error; e != nil && err == nil {
			err = errors.New("收件结果保存失败")
		}
	}()
	client, e := dial(ctx, s)
	if e != nil {
		return 0, errors.New("IMAP 连接失败，请检查服务器、端口和证书")
	}
	defer client.Close()
	stop := context.AfterFunc(ctx, func() { _ = client.Close() })
	defer stop()
	if e := client.Login(s.Username, s.Password).Wait(); e != nil {
		return 0, errors.New("IMAP 登录失败，请检查账号和授权码")
	}
	selected, e := client.Select("INBOX", &imap.SelectOptions{ReadOnly: true}).Wait()
	if e != nil {
		return 0, errors.New("INBOX 读取失败")
	}
	if selected.UIDValidity == 0 || selected.UIDNext == 0 {
		return 0, errors.New("邮箱未返回有效 UID 进度")
	}
	if !state.Initialized || state.UIDValidity != selected.UIDValidity {
		// First connection sets a baseline, avoiding accidental import of old mail.
		e := db.Model(&state).Where("lease_token = ?", token).Updates(map[string]any{"initialized": true, "uid_validity": selected.UIDValidity, "last_uid": uint32(selected.UIDNext) - 1}).Error
		if e != nil {
			return 0, errors.New("邮箱初始进度保存失败")
		}
		return 0, nil
	}
	if uint32(selected.UIDNext) <= state.LastUID+1 {
		return 0, nil
	}
	var uids imap.UIDSet
	uids.AddRange(imap.UID(state.LastUID+1), selected.UIDNext-1)
	found, e := client.UIDSearch(&imap.SearchCriteria{UID: []imap.UIDSet{uids}}, nil).Wait()
	if e != nil {
		return 0, errors.New("新邮件查询失败")
	}
	numbers := found.AllUIDs()
	sort.Slice(numbers, func(i, j int) bool { return numbers[i] < numbers[j] })
	if len(numbers) > 20 {
		numbers = numbers[:20]
	}
	for _, uid := range numbers {
		current, e := resolveInboundMailbox(s.TenantID)
		if e != nil || current == nil || current.key() != s.key() {
			return count, errors.New("邮箱配置已变化，暂停本批收件")
		}
		part := &imap.FetchItemBodySection{Peek: true, Partial: &imap.SectionPartial{Offset: 0, Size: maxEmailBytes + 1}}
		messages, e := client.Fetch(imap.UIDSetNum(uid), &imap.FetchOptions{UID: true, BodySection: []*imap.FetchItemBodySection{part}}).Collect()
		if e != nil {
			return count, errors.New("邮件下载失败，将在下次重试")
		}
		if len(messages) > 0 {
			raw := messages[0].FindBodySection(part)
			in := parseInboundMIME(raw, fmt.Sprintf("uid-%d-%d@%s", selected.UIDValidity, uid, s.key()))
			in.TenantID, in.Mailbox, in.ProjectKey, in.TicketType = s.TenantID, s.Username, s.ProjectKey, s.TicketType
			if _, e := IngestInboundEmail(in); e != nil {
				return count, errors.New("邮件入单失败，进度未推进，将在下次重试")
			}
			count++
		}
		saved := db.Model(&state).Where("lease_token = ?", token).Update("last_uid", uint32(uid))
		if saved.Error != nil || saved.RowsAffected != 1 {
			return count, errors.New("收件进度保存失败，将安全重试")
		}
	}
	return count, nil
}

func parseInboundMIME(raw []byte, fallback string) InboundEmailInput {
	in := InboundEmailInput{MessageID: fallback, ReceivedAt: time.Now().UTC()}
	if len(raw) > maxEmailBytes {
		in.HoldReason = "message_too_large"
		return in
	}
	reader, err := mailmime.CreateReader(bytes.NewReader(raw))
	if err != nil {
		in.HoldReason = "invalid_mime"
		return in
	}
	defer reader.Close()
	in.MessageID, err = reader.Header.MessageID()
	if err != nil || in.MessageID == "" {
		in.MessageID = fallback
	}
	in.Subject, _ = reader.Header.Subject()
	from, e := reader.Header.AddressList("From")
	if e != nil || len(from) != 1 {
		in.HoldReason = "invalid_sender"
	} else {
		in.FromAddress = from[0].Address
	}
	replies, _ := reader.Header.MsgIDList("In-Reply-To")
	if len(replies) > 0 {
		in.InReplyTo = replies[0]
	}
	in.References, _ = reader.Header.MsgIDList("References")
	auto := strings.ToLower(strings.TrimSpace(reader.Header.Get("Auto-Submitted")))
	if (auto != "" && auto != "no") || reader.Header.Get("List-Id") != "" {
		in.HoldReason = "automated_email"
	}
	var plain, rich []string
	for parts := 0; ; parts++ {
		if parts > 100 {
			in.HoldReason = "too_many_parts"
			break
		}
		p, e := reader.NextPart()
		if errors.Is(e, io.EOF) {
			break
		}
		if e != nil {
			in.HoldReason = "invalid_mime"
			break
		}
		if _, attachment := p.Header.(*mailmime.AttachmentHeader); attachment {
			in.AttachmentCount++
			continue
		}
		b, e := io.ReadAll(io.LimitReader(p.Body, maxEmailBytes+1))
		if e != nil || len(b) > maxEmailBytes {
			in.HoldReason = "invalid_body"
			break
		}
		if strings.HasPrefix(strings.ToLower(p.Header.Get("Content-Type")), "text/html") {
			rich = append(rich, html.UnescapeString(bluemonday.StrictPolicy().Sanitize(string(b))))
		} else {
			plain = append(plain, string(b))
		}
	}
	in.TextBody = strings.TrimSpace(strings.Join(plain, "\n"))
	if in.TextBody == "" {
		in.TextBody = strings.TrimSpace(strings.Join(rich, "\n"))
	}
	return in
}
