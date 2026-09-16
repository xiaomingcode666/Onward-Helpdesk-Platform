package services

import (
	"errors"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/enums"

	"gorm.io/gorm"
)

// notificationTemplateSeed 平台推荐模板，只作为草稿写入租户，需要管理员批准后才会用于发送。
type notificationTemplateSeed struct {
	Code     string
	Name     string
	Channel  string
	Language string
	Title    string
	Body     string
}

// EnsurePlatformDefaultsDB 幂等写入平台基线模板，并直接标记为「已批准」。
// 作用：租户没有自定义模板时，通知仍然由已批准的模板生成，不会退化成直接发原文。
func (s *notificationTemplateService) EnsurePlatformDefaultsDB(db *gorm.DB) (int, error) {
	if db == nil || !db.Migrator().HasTable(&models.NotificationTemplate{}) {
		return 0, nil
	}
	created := 0
	now := time.Now()
	for _, seed := range defaultNotificationTemplateSeeds() {
		var existing models.NotificationTemplate
		err := db.Where("tenant_id = ? AND code = ? AND channel = ? AND language = ?",
			platformNotificationTemplateTenant, seed.Code, seed.Channel, seed.Language).Take(&existing).Error
		if err == nil {
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return created, err
		}
		approvedAt := now
		variables, err := encodeNotificationTemplateVariables(nil, seed.Title, seed.Body)
		if err != nil {
			return created, err
		}
		item := &models.NotificationTemplate{
			TenantID:        platformNotificationTemplateTenant,
			Code:            seed.Code,
			Name:            seed.Name + "（平台基线）",
			Channel:         seed.Channel,
			Language:        seed.Language,
			TitleTemplate:   seed.Title,
			ContentTemplate: seed.Body,
			VariablesJSON:   variables,
			ApprovalStatus:  NotificationTemplateStatusApproved,
			ApprovedAt:      &approvedAt,
			Status:          int(enums.StatusOk),
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := db.Create(item).Error; err != nil {
			return created, err
		}
		created++
	}
	return created, nil
}

// defaultNotificationTemplateSeeds 返回标准通知场景的多语言推荐模板。
// 可用变量：{{Title}} {{Content}} {{RecipientName}} {{ActionURL}} {{TicketNo}} {{TicketTitle}} {{Reason}}。
func defaultNotificationTemplateSeeds() []notificationTemplateSeed {
	type variant [2]string
	catalog := []struct {
		Code  string
		Name  string
		InApp map[string]variant
		Email map[string]variant
	}{
		{
			Code: "ticket_created",
			Name: "工单创建提醒",
			InApp: map[string]variant{
				"zh-CN": {"新工单 {{TicketNo}} 待处理", "{{TicketTitle}}\n请在处理工作台确认并派工。"},
				"en-US": {"New ticket {{TicketNo}} pending", "{{TicketTitle}}\nReview and assign it in the ticket workbench."},
				"es-ES": {"Nuevo ticket {{TicketNo}} pendiente", "{{TicketTitle}}\nRevise y asigne el ticket en el panel de trabajo."},
			},
			Email: map[string]variant{
				"zh-CN": {"[工单] {{TicketNo}} 待处理", "{{TicketTitle}}\n\n请在处理工作台确认并派工。\n{{ActionURL}}"},
				"en-US": {"[Ticket] {{TicketNo}} pending", "{{TicketTitle}}\n\nReview and assign it in the ticket workbench.\n{{ActionURL}}"},
				"es-ES": {"[Ticket] {{TicketNo}} pendiente", "{{TicketTitle}}\n\nRevise y asigne el ticket en el panel de trabajo.\n{{ActionURL}}"},
			},
		},
		{
			Code: "ticket_assigned",
			Name: "工单分配提醒",
			InApp: map[string]variant{
				"zh-CN": {"工单 {{TicketNo}} 已分配给你", "{{TicketTitle}}\n{{Reason}}"},
				"en-US": {"Ticket {{TicketNo}} assigned to you", "{{TicketTitle}}\n{{Reason}}"},
				"es-ES": {"Ticket {{TicketNo}} asignado a usted", "{{TicketTitle}}\n{{Reason}}"},
			},
			Email: map[string]variant{
				"zh-CN": {"[工单] {{TicketNo}} 已分配给你", "{{TicketTitle}}\n\n{{Reason}}\n{{ActionURL}}"},
				"en-US": {"[Ticket] {{TicketNo}} assigned to you", "{{TicketTitle}}\n\n{{Reason}}\n{{ActionURL}}"},
				"es-ES": {"[Ticket] {{TicketNo}} asignado a usted", "{{TicketTitle}}\n\n{{Reason}}\n{{ActionURL}}"},
			},
		},
		{
			Code: "ticket_closed",
			Name: "工单关闭提醒",
			InApp: map[string]variant{
				"zh-CN": {"工单 {{TicketNo}} 已关闭", "{{Content}}"},
				"en-US": {"Ticket {{TicketNo}} closed", "{{Content}}"},
				"es-ES": {"Ticket {{TicketNo}} cerrado", "{{Content}}"},
			},
			Email: map[string]variant{
				"zh-CN": {"[工单] {{TicketNo}} 已关闭", "{{Content}}\n\n{{ActionURL}}"},
				"en-US": {"[Ticket] {{TicketNo}} closed", "{{Content}}\n\n{{ActionURL}}"},
				"es-ES": {"[Ticket] {{TicketNo}} cerrado", "{{Content}}\n\n{{ActionURL}}"},
			},
		},
		{
			Code: "sla_warning",
			Name: "SLA 预警提醒",
			InApp: map[string]variant{
				"zh-CN": {"工单 {{TicketNo}} 即将超时", "{{Content}}\n请尽快处理，避免违反响应或解决时限。"},
				"en-US": {"Ticket {{TicketNo}} is approaching its SLA", "{{Content}}\nPlease handle it before the response or resolution target."},
				"es-ES": {"El ticket {{TicketNo}} está por vencer", "{{Content}}\nAtiéndalo antes del objetivo de respuesta o resolución."},
			},
			Email: map[string]variant{
				"zh-CN": {"[SLA] {{TicketNo}} 即将超时", "{{Content}}\n\n请尽快处理，避免违反响应或解决时限。\n{{ActionURL}}"},
				"en-US": {"[SLA] {{TicketNo}} approaching target", "{{Content}}\n\nPlease handle it before the response or resolution target.\n{{ActionURL}}"},
				"es-ES": {"[SLA] {{TicketNo}} por vencer", "{{Content}}\n\nAtiéndalo antes del objetivo de respuesta o resolución.\n{{ActionURL}}"},
			},
		},
		{
			Code: "ticket_created_assigned",
			Name: "工单已分配（原待处理提醒更新）",
			InApp: map[string]variant{
				"zh-CN": {"工单 {{TicketNo}} 已分配", "{{TicketTitle}}\n已分配给 {{Assignee}}，请按当前负责人继续处理。"},
				"en-US": {"Ticket {{TicketNo}} assigned", "{{TicketTitle}}\nAssigned to {{Assignee}}; the ticket now follows the new owner."},
				"es-ES": {"Ticket {{TicketNo}} asignado", "{{TicketTitle}}\nAsignado a {{Assignee}}; el ticket sigue al nuevo responsable."},
			},
		},
		{
			Code: "ticket_assigned_transferred",
			Name: "工单已转派",
			InApp: map[string]variant{
				"zh-CN": {"工单 {{TicketNo}} 已转派", "{{TicketTitle}}\n已转派给 {{Assignee}}，原负责人无需继续处理。"},
				"en-US": {"Ticket {{TicketNo}} reassigned", "{{TicketTitle}}\nReassigned to {{Assignee}}; no further action is needed from you."},
				"es-ES": {"Ticket {{TicketNo}} reasignado", "{{TicketTitle}}\nReasignado a {{Assignee}}; no se requiere mas accion de su parte."},
			},
		},
		{
			Code: "ticket_assigned_accepted",
			Name: "工单已接单",
			InApp: map[string]variant{
				"zh-CN": {"工单 {{TicketNo}} 已接单", "{{TicketTitle}}\n已确认接单，请在处理中工单继续跟进。"},
				"en-US": {"Ticket {{TicketNo}} accepted", "{{TicketTitle}}\nAssignment accepted; continue in your in-progress tickets."},
				"es-ES": {"Ticket {{TicketNo}} aceptado", "{{TicketTitle}}\nAsignacion aceptada; continue en los tickets en curso."},
			},
		},
		{
			Code: "ticket_assigned_cancelled",
			Name: "工单已取消（原派单关闭）",
			InApp: map[string]variant{
				"zh-CN": {"工单 {{TicketNo}} 已取消", "{{TicketTitle}}\n工单已取消，原派单无需继续处理。"},
				"en-US": {"Ticket {{TicketNo}} cancelled", "{{TicketTitle}}\nThe ticket was cancelled; no further action is needed."},
				"es-ES": {"Ticket {{TicketNo}} cancelado", "{{TicketTitle}}\nEl ticket fue cancelado; no se requiere mas accion."},
			},
		},
		{
			Code: "ticket_assigned_recovered",
			Name: "工单已回收",
			InApp: map[string]variant{
				"zh-CN": {"工单 {{TicketNo}} 已回收", "{{TicketTitle}}\n{{Reason}}"},
				"en-US": {"Ticket {{TicketNo}} returned to the pool", "{{TicketTitle}}\n{{Reason}}"},
				"es-ES": {"Ticket {{TicketNo}} devuelto al grupo", "{{TicketTitle}}\n{{Reason}}"},
			},
		},
		{
			Code: "notification_generic",
			Name: "通用通知",
			InApp: map[string]variant{
				"zh-CN": {"{{Title}}", "{{Content}}"},
				"en-US": {"{{Title}}", "{{Content}}"},
				"es-ES": {"{{Title}}", "{{Content}}"},
			},
			Email: map[string]variant{
				"zh-CN": {"{{Title}}", "{{Content}}\n\n{{ActionURL}}"},
				"en-US": {"{{Title}}", "{{Content}}\n\n{{ActionURL}}"},
				"es-ES": {"{{Title}}", "{{Content}}\n\n{{ActionURL}}"},
			},
		},
	}
	ret := make([]notificationTemplateSeed, 0, len(catalog)*len(notificationTemplateSupportedLanguages)*2)
	for _, item := range catalog {
		for _, language := range notificationTemplateSupportedLanguages {
			if value, ok := item.InApp[language]; ok {
				ret = append(ret, notificationTemplateSeed{
					Code: item.Code, Name: item.Name + "（站内信）", Channel: NotificationTemplateChannelInApp,
					Language: language, Title: value[0], Body: value[1],
				})
			}
			if value, ok := item.Email[language]; ok {
				ret = append(ret, notificationTemplateSeed{
					Code: item.Code, Name: item.Name + "（邮件）", Channel: NotificationTemplateChannelEmail,
					Language: language, Title: value[0], Body: value[1],
				})
			}
		}
	}
	return ret
}
