package bootstrap

import (
	"context"
	"log/slog"

	"remotehelpdesk/internal/ai/rag/vectordb"
	"remotehelpdesk/internal/oidcclient"
	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/eventbus"
	"remotehelpdesk/internal/pkg/i18nx"
	"remotehelpdesk/internal/pkg/logx"
	"remotehelpdesk/internal/pkg/providers"
	"remotehelpdesk/internal/services"
	"remotehelpdesk/internal/services/cronx"
	"remotehelpdesk/internal/wxwork"

	"github.com/mlogclub/simple/sqls"

	_ "remotehelpdesk/internal/services/event_handlers"
)

func Init(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("init config failed", "error", err)
		return err
	}
	config.SetCurrent(cfg)
	providers.InitJitsi(&cfg.Jitsi)
	if err := providers.InitSpeech(&cfg.Speech); err != nil {
		slog.Error("init speech transcription provider failed", "error", err)
		return err
	}
	if err := providers.InitMeetingARDetection(cfg.MeetingAR); err != nil {
		slog.Error("init meeting AR detection provider failed", "error", err)
		return err
	}
	i18nx.SetDefaultLocale(cfg.LanguageOrDefault())

	logx.Init(logx.Config{
		Level:     cfg.Logger.Level,
		Format:    cfg.Logger.Format,
		AddSource: cfg.Logger.AddSource,
	})

	if _, err := InitDB(cfg.DB); err != nil {
		slog.Error("init db failed", "error", err)
		return err
	}
	if err := InitMigrations(); err != nil {
		slog.Error("init migrations failed", "error", err)
		return err
	}
	if err := services.PlatformIntegrationConfigService.LoadPersisted(); err != nil {
		slog.Error("restore persisted platform integrations failed; using file/environment config", "error", err)
	}
	if err := services.SpeechRuntimeService.LoadPersisted(); err != nil {
		slog.Error("restore persisted speech runtime failed; using file/environment config", "error", err)
	}
	services.StartMQTTConnectorWorkers(context.Background())
	services.StartAIReplyJobWorker(context.Background())
	services.WsService.StartBroker(context.Background())
	if err := services.SyncDefaultIAMRolePoliciesDB(sqls.DB()); err != nil {
		slog.Error("sync default IAM role policies failed", "error", err)
		return err
	}
	if err := services.NotificationQueueService.Start(context.Background()); err != nil {
		slog.Error("start notification queue consumer failed", "error", err)
		return err
	}
	eventbus.StartDefaultOutboxPublisher(context.Background())
	if err := vectordb.Init(&cfg.VectorDB); err != nil {
		slog.Error("init vector db failed", "error", err)
		return err
	}
	if err := services.KnowledgeIndexGenerationService.EnsureActiveCollectionSchema(context.Background()); err != nil {
		slog.Error("init knowledge collection schema failed", "error", err)
		return err
	}

	// 启动任务调度器
	cronx.Init()

	wxwork.Init()
	if err := oidcclient.Init(context.Background()); err != nil {
		slog.Error("init oidc failed", "error", err)
		return err
	}
	return nil
}
