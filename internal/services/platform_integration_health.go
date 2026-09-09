package services

import (
	"context"
	"time"

	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/providers"
)

const platformJitsiProbeTimeout = 3 * time.Second

type PlatformIntegrationAggregate struct {
	SpeechProvider        string
	SpeechConfigured      bool
	JigasiEnabled         bool
	TranslationProvider   string
	TranslationConfigured bool
	ARProvider            string
	ARConfigured          bool
	SMTPConfigured        bool
}

type PlatformOpsSpeechAggregate struct {
	GeneratedAt   time.Time
	Provider      string
	Configured    bool
	JigasiEnabled bool
}

type PlatformOpsTranslationAggregate struct {
	GeneratedAt time.Time
	Provider    string
	Configured  bool
}

type PlatformOpsARAggregate struct {
	GeneratedAt time.Time
	Provider    string
	Configured  bool
}

func probePlatformJitsiService() providers.JitsiServiceHealth {
	provider := providers.DefaultJitsiProvider
	if provider == nil {
		return providers.JitsiServiceHealth{Status: "unconfigured", CheckedAt: time.Now()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), platformJitsiProbeTimeout)
	defer cancel()
	return provider.CheckHealth(ctx)
}

func (s *platformConsoleService) GetOpsSpeech(now time.Time) *PlatformOpsSpeechAggregate {
	speechConfig, speechProvider := providers.CurrentSpeechRuntime()
	return &PlatformOpsSpeechAggregate{
		GeneratedAt: now, Provider: speechProvider.Name(),
		Configured: speechProvider.Configured(), JigasiEnabled: speechConfig.Jigasi.Enabled,
	}
}

func (s *platformConsoleService) GetOpsTranslation(now time.Time) *PlatformOpsTranslationAggregate {
	provider := providers.CurrentTextTranslationProvider()
	return &PlatformOpsTranslationAggregate{
		GeneratedAt: now, Provider: provider.Name(), Configured: provider.Configured(),
	}
}

func (s *platformConsoleService) GetOpsAR(now time.Time) *PlatformOpsARAggregate {
	provider := providers.CurrentMeetingARDetectionProvider()
	return &PlatformOpsARAggregate{
		GeneratedAt: now, Provider: provider.Name(), Configured: provider.Configured(),
	}
}

func buildPlatformIntegrationAggregate() *PlatformIntegrationAggregate {
	now := time.Now()
	speech := PlatformConsoleService.GetOpsSpeech(now)
	translation := PlatformConsoleService.GetOpsTranslation(now)
	ar := PlatformConsoleService.GetOpsAR(now)
	cfg := config.CurrentOrDefault()
	return &PlatformIntegrationAggregate{
		SpeechProvider: speech.Provider, SpeechConfigured: speech.Configured,
		JigasiEnabled:       speech.JigasiEnabled,
		TranslationProvider: translation.Provider, TranslationConfigured: translation.Configured,
		ARProvider: ar.Provider, ARConfigured: ar.Configured,
		SMTPConfigured: smtpRuntimeConfigured(cfg.Email),
	}
}
