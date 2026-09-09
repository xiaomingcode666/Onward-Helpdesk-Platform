package cronx

import (
	"context"
	"log/slog"

	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/services"

	"github.com/robfig/cron/v3"
)

func Init() {
	c := cron.New()

	addFunc(c, "15 0 * * *", func() {
		if err := services.MeteringService.AggregateDailyUsage(); err != nil {
			slog.Error("daily AI usage aggregation failed", "error", err)
		}
	})

	addFunc(c, "@every 1h", func() {
		if err := services.DSARService.CheckDSARDeadlines(); err != nil {
			slog.Error("DSAR deadline scan failed", "error", err)
		}
	})

	addFunc(c, "0 3 * * *", func() {
		if err := services.DataRetentionService.ScheduledRetentionCleanup(); err != nil {
			slog.Error("tenant data retention cleanup failed", "error", err)
		}
	})
	addNonOverlappingFunc(c, "30 3 * * *", func() {
		if err := services.AuditRetentionService.ScheduledCleanup(); err != nil {
			slog.Error("audit retention cleanup failed", "error", err)
		}
	})

	addFunc(c, "@every 30s", func() {
		if _, err := services.ConversationDispatchService.DispatchPendingConversations(0); err != nil {
			slog.Warn("dispatch pending conversations loop failed", "error", err)
		}
		if _, err := services.TicketDispatchService.DispatchPendingTickets(0); err != nil {
			slog.Warn("dispatch pending tickets loop failed", "error", err)
		}
	})

	addFunc(c, "@every 1m", func() {
		if warnings, err := services.SLAService.CheckSLAWarnings(); err != nil {
			slog.Warn("sla warning scan failed", "error", err)
		} else if warnings > 0 {
			slog.Info("sla warnings scheduled", "count", warnings)
		}
		violations, err := services.SLAService.CheckSLAViolations()
		if err != nil {
			slog.Warn("sla violation scan failed", "error", err)
		} else if len(violations) > 0 {
			slog.Warn("sla violations detected", "count", len(violations))
		}
		escalations, err := services.EscalationService.CheckAndEscalate()
		if err != nil {
			slog.Warn("ticket escalation scan failed", "error", err)
		} else if len(escalations) > 0 {
			slog.Warn("tickets escalated", "count", len(escalations))
		}
		if recovered, err := services.TicketDispatchService.RecoverExpiredTicketAcceptances(0); err != nil {
			slog.Warn("ticket accept deadline recovery failed", "error", err)
		} else if recovered > 0 {
			slog.Info("expired ticket acceptances recovered", "count", recovered)
		}
		if recovered, err := services.TicketDispatchService.RecoverIneligibleTicketAcceptances(0); err != nil {
			slog.Warn("ticket ineligible assignee recovery failed", "error", err)
		} else if recovered > 0 {
			slog.Info("ineligible ticket acceptances recovered", "count", recovered)
		}
		if recovered, err := services.ConversationDispatchService.RecoverIneligibleAssignments(0); err != nil {
			slog.Warn("conversation ineligible assignee recovery failed", "error", err)
		} else if recovered > 0 {
			slog.Info("ineligible conversation assignments recovered", "count", recovered)
		}
		if escalated, err := services.TicketSupplierCollaborationService.EscalateUnresponsiveInvitations(0, 100); err != nil {
			slog.Warn("supplier collaboration timeout escalation failed", "error", err)
		} else if escalated > 0 {
			slog.Info("supplier collaboration invitations escalated after timeout", "count", escalated)
		}
	})
	addNonOverlappingFunc(c, "@every 10s", func() {
		if expired, err := services.TicketSupplierCollaborationService.ExpireAuthorizations(100); err != nil {
			slog.Warn("supplier collaboration authorization expiry failed", "error", err)
		} else if expired > 0 {
			slog.Info("supplier collaboration authorizations expired", "count", expired)
		}
	})

	if recovered := services.RecoverTimedOutAIReplyJobs(); recovered > 0 {
		slog.Info("recovered timed out ai reply jobs", "count", recovered)
	}
	addFunc(c, "@every 10s", func() {
		if recovered := services.RecoverTimedOutAIReplyJobs(); recovered > 0 {
			slog.Info("recovered timed out ai reply jobs", "count", recovered)
		}
	})
	addFunc(c, "@every 2s", func() {
		if count := services.ProcessDueAIReplyJobs(context.Background(), 20); count > 0 {
			slog.Info("ai reply jobs processed", "count", count)
		}
	})
	addNonOverlappingFunc(c, "@every 2s", func() {
		if count := services.MeetingIntelligenceService.ProcessPendingTranscriptTranslations(context.Background(), 20); count > 0 {
			slog.Info("meeting transcript translations processed", "count", count)
		}
	})

	addFunc(c, "@every 5m", func() {
		if count := services.TicketAutoCloseService.CloseDueTickets(100); count > 0 {
			slog.Info("auto closed resolved tickets", "count", count)
		}
	})

	if rebuilt, err := services.ServiceOutcomeFactService.RebuildMissingBatch(context.Background(), 100); err != nil {
		slog.Warn("initial service outcome backfill failed", "rebuilt", rebuilt, "error", err)
	} else if rebuilt > 0 {
		slog.Info("service outcome facts backfilled", "count", rebuilt)
	}
	addNonOverlappingFunc(c, "@every 1m", func() {
		if rebuilt, err := services.ServiceOutcomeFactService.RebuildMissingBatch(context.Background(), 100); err != nil {
			slog.Warn("service outcome backfill failed", "rebuilt", rebuilt, "error", err)
		} else if rebuilt > 0 {
			slog.Info("service outcome facts backfilled", "count", rebuilt)
		}
	})

	if recovered := services.TicketLifecycleService.RecoverClosedTicketSideEffects(100); recovered > 0 {
		slog.Info("recovered closed ticket side effects", "count", recovered)
	}
	addFunc(c, "@every 1m", func() {
		if recovered := services.TicketLifecycleService.RecoverClosedTicketSideEffects(100); recovered > 0 {
			slog.Info("recovered closed ticket side effects", "count", recovered)
		}
	})

	if expired := services.MeetingService.ExpireStaleMeetings(100); expired > 0 {
		slog.Info("expired stale video meetings", "count", expired)
	}
	if recovered, err := services.JitsiWebhookService.RecoverPending(context.Background(), 100); err != nil {
		slog.Warn("recover jitsi webhook inbox failed", "error", err)
	} else if recovered > 0 {
		slog.Info("recovered jitsi webhook events", "count", recovered)
	}
	addFunc(c, "@every 10s", func() {
		if recovered, err := services.JitsiWebhookService.RecoverPending(context.Background(), 100); err != nil {
			slog.Warn("recover jitsi webhook inbox failed", "error", err)
		} else if recovered > 0 {
			slog.Info("recovered jitsi webhook events", "count", recovered)
		}
	})
	addFunc(c, "@every 5m", func() {
		if expired := services.MeetingService.ExpireStaleMeetings(100); expired > 0 {
			slog.Info("expired stale video meetings", "count", expired)
		}
	})

	addFunc(c, "@every 5s", func() {
		count := services.WxWorkKFOutboundService.DispatchPendingOutbox()
		if count > 0 {
			slog.Info("wxwork kf outbox dispatched", "count", count)
		}
	})

	addNonOverlappingFunc(c, "@every 15s", func() {
		if count := services.NotificationDeliveryService.ProcessDue(context.Background(), 20); count > 0 {
			slog.Info("notification deliveries processed", "count", count)
		}
	})

	if recovered := services.ProductResourceService.RecoverTimedOutProvisioningJobs(); recovered > 0 {
		slog.Info("recovered timed out product resource provisioning jobs", "count", recovered)
	}
	addFunc(c, "@every 10s", func() {
		if recovered := services.ProductResourceService.RecoverTimedOutProvisioningJobs(); recovered > 0 {
			slog.Info("recovered timed out product resource provisioning jobs", "count", recovered)
		}
		if count := services.ProductResourceService.ProcessDueProvisioningJobs(context.Background(), 20); count > 0 {
			slog.Info("product resource provisioning jobs processed", "count", count)
		}
	})

	ragCfg := config.CurrentOrDefault().RAG.Normalized()
	if ragCfg.Enabled && ragCfg.IngestionEnabled {
		if recovered := services.KnowledgeIndexSyncService.ReconcileEntryTasks(ragCfg.TaskBatchSize); recovered > 0 {
			slog.Info("reconciled knowledge index tasks", "count", recovered)
		}
		if recovered := services.KnowledgeIndexSyncService.RecoverTimedOutTasks(); recovered > 0 {
			slog.Info("recovered timed out knowledge index tasks", "count", recovered)
		}
		addFunc(c, "@every "+ragCfg.TaskInterval.String(), func() {
			services.KnowledgeIndexSyncService.ReconcileEntryTasks(ragCfg.TaskBatchSize)
			if count := services.KnowledgeIndexSyncService.ProcessDueTasks(context.Background(), ragCfg.TaskBatchSize); count > 0 {
				slog.Info("knowledge index tasks processed", "count", count)
			}
		})
	}

	// 初始化事件总线消费者
	InitEventConsumers()

	c.Start()
}

func addFunc(c *cron.Cron, spec string, cmd func()) {
	if _, err := c.AddFunc(spec, cmd); err != nil {
		slog.Error("add cron func error", slog.Any("err", err))
	}
}

func addNonOverlappingFunc(c *cron.Cron, spec string, cmd func()) {
	job := cron.NewChain(cron.SkipIfStillRunning(cron.DefaultLogger)).Then(cron.FuncJob(cmd))
	if _, err := c.AddJob(spec, job); err != nil {
		slog.Error("add non-overlapping cron func error", slog.Any("err", err))
	}
}

// InitEventConsumers 注册 EventBus（字符串类型）事件消费者
// 注意： typed eventbus 消费者已在 event_handlers 包中通过 init() 注册
func InitEventConsumers() {
	bus := services.GetEventBus()

	// ticket.closed -> 生成知识候选
	bus.Subscribe(services.EventTicketClosed, func(ctx context.Context, event services.Event) error {
		slog.Info("event bus consumer: ticket.closed",
			"eventId", event.ID,
			"tenantId", event.TenantID)
		return nil
	})

	// diagnosis.completed -> 判断转人工/更新会话
	bus.Subscribe(services.EventDiagnosisCompleted, func(ctx context.Context, event services.Event) error {
		slog.Info("event bus consumer: diagnosis.completed",
			"eventId", event.ID,
			"tenantId", event.TenantID)
		return nil
	})

	// meeting.ended -> 回写工单时间线
	bus.Subscribe(services.EventMeetingEnded, func(ctx context.Context, event services.Event) error {
		slog.Info("event bus consumer: meeting.ended",
			"eventId", event.ID,
			"tenantId", event.TenantID)
		return nil
	})

	// ticket.created -> 通知派单
	bus.Subscribe(services.EventTicketCreated, func(ctx context.Context, event services.Event) error {
		slog.Info("event bus consumer: ticket.created",
			"eventId", event.ID,
			"tenantId", event.TenantID)
		return nil
	})

	slog.Info("event bus consumers initialized")
}
