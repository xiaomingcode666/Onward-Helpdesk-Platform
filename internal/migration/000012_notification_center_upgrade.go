package migration

import (
	"errors"
	"strings"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"github.com/mlogclub/simple/sqls"
	"gorm.io/gorm"
)

func init() {
	register(12, "notification center upgrade: backfill category/level, seed demo notifications and default mail setting", func() error {
		return sqls.WithTransaction(func(ctx *sqls.TxContext) error {
			if err := backfillNotificationFields(ctx.Tx); err != nil {
				return err
			}
			var tenant models.Tenant
			if err := ctx.Tx.Order("id asc").First(&tenant).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return nil // 尚无租户,跳过种子数据
				}
				return err
			}
			if err := seedDemoNotifications(ctx.Tx, tenant.ID); err != nil {
				return err
			}
			return seedDefaultMailSetting(ctx.Tx, tenant.ID)
		})
	})
}

// backfillNotificationFields 根据既有 notification_type / biz_type 回填 category / level / channels / recipient_name。
// AutoMigrate 已为新列补上默认值(level=info, category=system, channels=in_app),此处按业务语义修正存量行。
func backfillNotificationFields(tx *gorm.DB) error {
	var list []models.Notification
	if err := tx.Find(&list).Error; err != nil {
		return err
	}
	for _, item := range list {
		category, level := deriveNotificationCategoryLevel(item.NotificationType, item.BizType)
		channels := "in_app"
		if item.ExternalChannelStatus == "email_sent" || strings.Contains(item.Channels, "email") {
			channels = "in_app,email"
		}
		recipientName := item.RecipientName
		if recipientName == "" {
			recipientName = deriveRecipientName(category)
		}
		if err := tx.Model(&models.Notification{}).Where("id = ?", item.ID).Updates(map[string]any{
			"category":       category,
			"level":          level,
			"channels":       channels,
			"recipient_name": recipientName,
		}).Error; err != nil {
			return err
		}
	}
	return nil
}

func deriveNotificationCategoryLevel(notificationType, bizType string) (string, string) {
	nt := strings.ToLower(strings.TrimSpace(notificationType))
	bt := strings.ToLower(strings.TrimSpace(bizType))
	switch {
	case strings.Contains(nt, "sla"), strings.Contains(nt, "escalat"):
		return "sla", "urgent"
	case strings.HasPrefix(nt, "data_breach_"):
		if strings.Contains(nt, "urgent") {
			return "system", "urgent"
		}
		return "system", "warning"
	case strings.HasPrefix(nt, "dsar_"):
		return "system", "warning"
	case strings.HasPrefix(nt, "ticket"), bt == "ticket":
		return "ticket", "info"
	case strings.HasPrefix(nt, "conversation"), bt == "conversation":
		return "ticket", "info"
	case strings.Contains(nt, "meeting"), bt == "meeting":
		return "video", "info"
	case strings.Contains(nt, "knowledge"):
		return "knowledge", "info"
	case strings.Contains(nt, "approval"):
		return "approval", "warning"
	case strings.Contains(nt, "quota"):
		return "quota", "warning"
	default:
		return "system", "info"
	}
}

func deriveRecipientName(category string) string {
	switch category {
	case "ticket", "sla":
		return "工单负责人"
	case "video":
		return "工单参与人"
	case "approval":
		return "权限管理员"
	case "quota":
		return "企业管理员 / 财务"
	case "knowledge":
		return "知识运营组"
	default:
		return "系统管理员"
	}
}

// seedDemoNotifications 为默认租户插入原型设计中的 8 条示例消息(仅当该租户没有任何通知时)。
func seedDemoNotifications(tx *gorm.DB, tenantID int64) error {
	var count int64
	if err := tx.Model(&models.Notification{}).Where("tenant_id = ?", tenantID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	now := time.Now()
	read := func() *time.Time { t := now; return &t }()
	type demo struct {
		category     string
		level        string
		title        string
		content      string
		recipient    string
		channels     string
		emailSent    bool
		delivery     string
		readAt       *time.Time
		ageOffsetMin int
		actionURL    string
	}
	demos := []demo{
		{"ticket", "urgent", "工单 TK-24071805 已升级为紧急", "HP-300 无法启动，急停回路异常", "服务运营部 / 李主管", "in_app,email", true, "sent", nil, 2, "/enterprise/ticket-workbench"},
		{"sla", "urgent", "SLA 预警：工单 TK-24071793 已超时", "已超时 15 分钟，等待主管处理", "客服主管组", "in_app,email", true, "sent", nil, 8, "/enterprise/ticket-workbench"},
		{"approval", "warning", "审批待办：张工申请 EU 区域权限", "海外工程师角色变更，需主管审批", "权限管理员", "in_app", false, "pending", nil, 60, "/enterprise/iam-audit"},
		{"quota", "warning", "额度告警：AI Token 用量已达 80%", "本月已用 2.84M / 总额 3M，建议关注", "企业管理员 / 财务", "in_app,email", false, "read", read, 180, "/enterprise/usage"},
		{"ticket", "info", "工单 TK-24071642 已被张工受理", "CX-8 控制器报警 E-42", "客户用户 Martin Vogel", "in_app", false, "read", read, 1440, "/enterprise/ticket-workbench"},
		{"knowledge", "info", "知识条目「HP-300 维修手册」已通过审核", "v3.6 已发布，覆盖中英双语", "知识运营组", "in_app", false, "read", read, 1440, "/enterprise/knowledge"},
		{"video", "info", "视频会议 MT-240720 纪要已归档", "字幕总结和截图已回写工单 TK-240720", "工单参与人", "in_app", false, "read", read, 1440, "/enterprise/video"},
		{"system", "info", "供应商「EuroHydraulic」合同将在 30 天内到期", "合同截止 2026-12-31，建议提前续约", "商务服务组", "in_app,email", true, "read", read, 2880, "/enterprise/partners"},
	}
	for _, d := range demos {
		external := ""
		if d.emailSent {
			external = "email_sent"
		}
		item := models.Notification{
			TenantID:              tenantID,
			RecipientUserID:       0, // 租户广播消息,企业端所有成员可见
			Title:                 d.title,
			Content:               d.content,
			NotificationType:      d.category,
			BizType:               d.category,
			ActionURL:             d.actionURL,
			DeliveryStatus:        d.delivery,
			ExternalChannelStatus: external,
			Level:                 d.level,
			Category:              d.category,
			RecipientName:         d.recipient,
			Channels:              d.channels,
			ReadAt:                d.readAt,
			Status:                int(enums.StatusOk),
			CreatedAt:             now.Add(-time.Duration(d.ageOffsetMin) * time.Minute),
		}
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
	}
	return nil
}

// seedDefaultMailSetting 为默认租户插入原型默认邮箱配置(仅当尚未配置时)。
func seedDefaultMailSetting(tx *gorm.DB, tenantID int64) error {
	var count int64
	if err := tx.Model(&models.TenantMailSetting{}).Where("tenant_id = ?", tenantID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	now := time.Now()
	return tx.Create(&models.TenantMailSetting{
		TenantID:    tenantID,
		SMTPPort:    465,
		RetryPolicy: "retry_3_10m",
		UseTLS:      true,
		Status:      int(enums.StatusDisabled),
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error
}
