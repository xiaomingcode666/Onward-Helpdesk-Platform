package services

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"log/slog"
	"net/smtp"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/enums"
	"remotehelpdesk/internal/pkg/secretstore"
	"remotehelpdesk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var EmailNotificationService = newEmailNotificationService()

func newEmailNotificationService() *emailNotificationService {
	return &emailNotificationService{}
}

type emailNotificationService struct{}

// EmailTemplate 邮件模板定义
type emailTemplate struct {
	Subject string
	Body    string
}

// emailTemplates 内置邮件模板
var emailTemplates = map[string]*emailTemplate{
	"ticket_created": {
		Subject: "[RemoteHelpDesk] 新工单 #{{.TicketNo}} - {{.Title}}",
		Body: `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; padding: 20px; color: #333;">
<h2>新工单已创建</h2>
<p>工单编号: <strong>{{.TicketNo}}</strong></p>
<p>标题: {{.Title}}</p>
<p>客户: {{.CustomerName}}</p>
<p>产品: {{.ProductName}}</p>
<p>优先级: {{.Priority}}</p>
<p>创建时间: {{.CreatedAt}}</p>
<p style="margin-top: 20px;"><a href="{{.ActionURL}}" style="display: inline-block; padding: 10px 20px; background: #1890ff; color: #fff; text-decoration: none; border-radius: 4px;">查看工单</a></p>
</body>
</html>`,
	},
	"ticket_assigned": {
		Subject: "[RemoteHelpDesk] 工单已分配 - #{{.TicketNo}}",
		Body: `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; padding: 20px; color: #333;">
<h2>工单已分配给您</h2>
<p>工单编号: <strong>{{.TicketNo}}</strong></p>
<p>标题: {{.Title}}</p>
<p>客户: {{.CustomerName}}</p>
<p>SLA 截止: {{.SLADueAt}}</p>
<p style="margin-top: 20px;"><a href="{{.ActionURL}}" style="display: inline-block; padding: 10px 20px; background: #1890ff; color: #fff; text-decoration: none; border-radius: 4px;">查看工单</a></p>
</body>
</html>`,
	},
	"ticket_closed": {
		Subject: "[RemoteHelpDesk] 工单已关闭 - #{{.TicketNo}}",
		Body: `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; padding: 20px; color: #333;">
<h2>工单已关闭</h2>
<p>工单编号: <strong>{{.TicketNo}}</strong></p>
<p>标题: {{.Title}}</p>
<p>解决方案: {{.Solution}}</p>
<p>关闭时间: {{.ClosedAt}}</p>
<p style="margin-top: 20px;"><a href="{{.ActionURL}}" style="display: inline-block; padding: 10px 20px; background: #1890ff; color: #fff; text-decoration: none; border-radius: 4px;">查看详情</a></p>
</body>
</html>`,
	},
	"meeting_invitation": {
		Subject: "[RemoteHelpDesk] 远程会议邀请 - 工单 #{{.TicketNo}}",
		Body: `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; padding: 20px; color: #333;">
<h2>远程会议邀请</h2>
<p>工单编号: <strong>{{.TicketNo}}</strong></p>
<p>会议主题: {{.MeetingTitle}}</p>
<p>发起人: {{.Initiator}}</p>
<p style="margin-top: 20px;"><a href="{{.JoinURL}}" style="display: inline-block; padding: 10px 20px; background: #52c41a; color: #fff; text-decoration: none; border-radius: 4px;">加入会议</a></p>
</body>
</html>`,
	},
	"sla_warning": {
		Subject: "[RemoteHelpDesk] SLA 预警 - 工单 #{{.TicketNo}}",
		Body: `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; padding: 20px; color: #333;">
<h2 style="color: #faad14;">SLA 即将超时</h2>
<p>工单编号: <strong>{{.TicketNo}}</strong></p>
<p>标题: {{.Title}}</p>
<p>当前负责人: {{.Assignee}}</p>
<p>SLA 截止时间: <strong style="color: #ff4d4f;">{{.SLADueAt}}</strong></p>
<p>剩余时间: {{.RemainingTime}}</p>
<p style="margin-top: 20px;"><a href="{{.ActionURL}}" style="display: inline-block; padding: 10px 20px; background: #ff4d4f; color: #fff; text-decoration: none; border-radius: 4px;">立即处理</a></p>
</body>
</html>`,
	},
	"mail_test": {
		Subject: "[RemoteHelpDesk] 邮箱发送配置测试邮件",
		Body: `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; padding: 20px; color: #333;">
<h2 style="color: #52c41a;">测试邮件发送成功</h2>
<p>这是一封来自「设备售后平台」邮箱发送配置的测试邮件。</p>
<p>发信账号: {{.FromAddress}}</p>
<p>SMTP 服务: {{.SMTPServer}}</p>
<p>发送时间: {{.SentAt}}</p>
<p>如果您收到这封邮件，说明当前租户的邮箱发送配置可以正常工作。</p>
</body>
</html>`,
	},
	"marketing_demo_request": {
		Subject: "[RemoteHelpDesk] 新 Demo 申请 - {{.Company}}",
		Body: `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; padding: 20px; color: #1e293b;">
<h2>新的 RemoteHelpDesk Demo 申请</h2>
<table style="border-collapse: collapse; width: 100%; max-width: 620px;">
<tr><td style="padding: 8px; color: #64748b; width: 120px;">企业名称</td><td style="padding: 8px;"><strong>{{.Company}}</strong></td></tr>
<tr><td style="padding: 8px; color: #64748b;">联系人</td><td style="padding: 8px;">{{.ContactName}}</td></tr>
<tr><td style="padding: 8px; color: #64748b;">邮箱</td><td style="padding: 8px;"><a href="mailto:{{.Email}}">{{.Email}}</a></td></tr>
<tr><td style="padding: 8px; color: #64748b;">电话</td><td style="padding: 8px;">{{.Mobile}}</td></tr>
<tr><td style="padding: 8px; color: #64748b;">国家/地区</td><td style="padding: 8px;">{{.CountryRegion}}</td></tr>
<tr><td style="padding: 8px; color: #64748b; vertical-align: top;">需求说明</td><td style="padding: 8px; white-space: pre-wrap;">{{.Requirements}}</td></tr>
<tr><td style="padding: 8px; color: #64748b;">提交时间</td><td style="padding: 8px;">{{.SubmittedAt}}</td></tr>
</table>
</body>
</html>`,
	},
	"notification_generic": {
		Subject: "[RemoteHelpDesk] {{.Title}}",
		Body: `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; padding: 20px; color: #333;">
<h2>{{.Title}}</h2>
<div style="white-space: pre-line; line-height: 1.6;">{{.Content}}</div>
{{if .ActionURL}}<p style="margin-top: 20px;"><a href="{{.ActionURL}}" style="display: inline-block; padding: 10px 20px; background: #1890ff; color: #fff; text-decoration: none; border-radius: 4px;">查看详情</a></p>{{end}}
</body>
</html>`,
	},
	"portal_invitation": {
		Subject: "[RemoteHelpDesk] {{.PortalName}}邀请 - 验证码 {{.InviteCode}}",
		Body: `<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: Arial, sans-serif; padding: 20px; color: #333;">
<h2>{{.PortalName}}邀请</h2>
<p>{{.TenantName}} 邀请 {{.TargetName}} 绑定 RemoteHelpDesk {{.PortalName}}账号。</p>
{{if .OrgName}}<p>关联组织: <strong>{{.OrgName}}</strong></p>{{end}}
<p>邀请码: <strong style="font-size: 20px; letter-spacing: 2px;">{{.InviteCode}}</strong></p>
<p>有效期至: {{.ExpiresAt}}</p>
<p style="margin-top: 20px;"><a href="{{.ActionURL}}" style="display: inline-block; padding: 10px 20px; background: #1890ff; color: #fff; text-decoration: none; border-radius: 4px;">打开绑定链接</a></p>
<p style="margin-top: 16px; color: #666;">如果按钮无法打开，请复制以下链接到浏览器：<br>{{.ActionURL}}</p>
</body>
</html>`,
	},
}

// resolveMailConfig 解析租户邮箱配置:优先使用 TenantMailSetting,
// 未配置的字段回退全局 config.yaml 的 Email 配置。
func (s *emailNotificationService) resolveMailConfig(tenantID int64) config.EmailConfig {
	cfg := config.CurrentOrDefault().Email
	if tenantID <= 0 {
		return cfg
	}
	setting := repositories.TenantMailSettingRepository.GetByTenantID(sqls.DB(), tenantID)
	if setting == nil || setting.Status != int(enums.StatusOk) {
		return cfg
	}
	if strings.TrimSpace(setting.FromAddress) != "" {
		cfg.FromAddress = setting.FromAddress
	}
	if strings.TrimSpace(setting.FromName) != "" {
		cfg.FromName = setting.FromName
	}
	if strings.TrimSpace(setting.SMTPHost) != "" {
		cfg.SMTPHost = setting.SMTPHost
	}
	if setting.SMTPPort > 0 {
		cfg.SMTPPort = setting.SMTPPort
	}
	if strings.TrimSpace(setting.Username) != "" {
		cfg.Username = setting.Username
	}
	if setting.Password != "" {
		if password, err := secretstore.Decrypt(setting.Password); err == nil {
			cfg.Password = password
		}
	}
	cfg.UseTLS = setting.UseTLS
	return cfg
}

// SendEmail 发送非通知任务邮件并记录独立投递日志。
func (s *emailNotificationService) SendEmail(tenantID int64, to, subject, templateName string, data map[string]interface{}) error {
	err := s.sendEmail(tenantID, to, subject, templateName, data)
	s.logDelivery(tenantID, to, templateName, err)
	return err
}

// SendNotificationEmail 由通知投递任务调用，任务自身负责持久化状态与重试。
func (s *emailNotificationService) SendNotificationEmail(tenantID int64, to, subject, templateName string, data map[string]interface{}) error {
	return s.sendEmail(tenantID, to, subject, templateName, data)
}

func (s *emailNotificationService) sendEmail(tenantID int64, to, subject, templateName string, data map[string]interface{}) error {
	cfg := s.resolveMailConfig(tenantID)
	if cfg.SMTPHost == "" {
		return fmt.Errorf("email SMTP not configured")
	}

	// 查找模板
	tmpl, ok := emailTemplates[templateName]
	if !ok {
		return fmt.Errorf("email template not found: %s", templateName)
	}

	// 渲染模板
	subjectText := tmpl.Subject
	if subject != "" {
		subjectText = subject
	}

	subjBuf := new(bytes.Buffer)
	subjTmpl, err := template.New("subject").Parse(subjectText)
	if err != nil {
		return fmt.Errorf("parse subject template error: %w", err)
	}
	if err := subjTmpl.Execute(subjBuf, data); err != nil {
		return fmt.Errorf("execute subject template error: %w", err)
	}

	bodyBuf := new(bytes.Buffer)
	bodyTmpl, err := template.New("body").Parse(tmpl.Body)
	if err != nil {
		return fmt.Errorf("parse body template error: %w", err)
	}
	if err := bodyTmpl.Execute(bodyBuf, data); err != nil {
		return fmt.Errorf("execute body template error: %w", err)
	}

	// 构建邮件
	fromName := cfg.FromName
	if fromName == "" {
		fromName = "RemoteHelpDesk"
	}
	from := fmt.Sprintf("%s <%s>", fromName, cfg.FromAddress)

	headers := make(map[string]string)
	headers["From"] = from
	headers["To"] = to
	headers["Subject"] = subjBuf.String()
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"

	msg := ""
	for k, v := range headers {
		msg += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	msg += "\r\n" + bodyBuf.String()

	// 发送
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)

	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
	}

	var sendErr error
	if cfg.UseTLS {
		sendErr = s.sendTLS(addr, auth, cfg.FromAddress, []string{to}, []byte(msg))
	} else {
		sendErr = smtp.SendMail(addr, auth, cfg.FromAddress, []string{to}, []byte(msg))
	}

	if sendErr != nil {
		slog.Error("send email failed", "to", to, "template", templateName, "error", sendErr)
	} else {
		slog.Info("email sent successfully", "to", to, "template", templateName)
	}

	return sendErr
}

// sendTLS 使用 TLS 发送邮件
func (s *emailNotificationService) sendTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte) error {
	tlsConfig := &tls.Config{
		ServerName: extractHost(addr),
	}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("tls dial error: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, extractHost(addr))
	if err != nil {
		return fmt.Errorf("smtp client error: %w", err)
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth error: %w", err)
		}
	}

	if err := client.Mail(from); err != nil {
		return err
	}
	for _, addr := range to {
		if err := client.Rcpt(addr); err != nil {
			return err
		}
	}

	w, err := client.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(msg)
	if err != nil {
		return err
	}
	return w.Close()
}

// logDelivery 记录邮件投递日志
func (s *emailNotificationService) logDelivery(tenantID int64, recipient, templateName string, sendErr error) {
	status := "sent"
	errMsg := ""
	if sendErr != nil {
		status = "failed"
		errMsg = sendErr.Error()
	}
	log := &models.DeliveryLog{
		TenantID:    tenantID,
		Channel:     "email",
		RecipientID: recipient,
		Status:      status,
		ErrorMsg:    errMsg,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := sqls.DB().Create(log).Error; err != nil {
		slog.Error("failed to save email delivery log", "error", err)
	}
}

// extractHost 从地址中提取主机名
func extractHost(addr string) string {
	for i := 0; i < len(addr); i++ {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}
