package providers

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"

	"remotehelpdesk/internal/pkg/config"
)

type TextTranslationRequest struct {
	Text           string
	SourceLanguage string
	TargetLanguage string
}

type TextTranslationResult struct {
	Text            string
	SourceLanguage  string
	TargetLanguage  string
	Provider        string
	ProviderEventID string
}

type TextTranslationProvider interface {
	Name() string
	Configured() bool
	Translate(ctx context.Context, input TextTranslationRequest) (*TextTranslationResult, error)
}

type textTranslationProviderSnapshot struct {
	provider TextTranslationProvider
}

var currentTextTranslationProvider atomic.Pointer[textTranslationProviderSnapshot]

func init() {
	currentTextTranslationProvider.Store(&textTranslationProviderSnapshot{provider: disabledTextTranslationProvider{}})
}

func initTextTranslation(cfg *config.SpeechConfig) {
	var provider TextTranslationProvider = disabledTextTranslationProvider{}
	if cfg != nil && cfg.Xfyun.TranslationEnabled {
		provider = NewXfyunTextTranslationProvider(cfg.Xfyun)
	}
	currentTextTranslationProvider.Store(&textTranslationProviderSnapshot{provider: provider})
}

func CurrentTextTranslationProvider() TextTranslationProvider {
	snapshot := currentTextTranslationProvider.Load()
	if snapshot == nil || snapshot.provider == nil {
		return disabledTextTranslationProvider{}
	}
	return snapshot.provider
}

type disabledTextTranslationProvider struct{}

func (disabledTextTranslationProvider) Name() string     { return "disabled" }
func (disabledTextTranslationProvider) Configured() bool { return false }
func (disabledTextTranslationProvider) Translate(context.Context, TextTranslationRequest) (*TextTranslationResult, error) {
	return nil, errors.New("text translation is disabled")
}

func normalizeXfyunTranslationLanguage(language string) string {
	language = strings.ToLower(strings.TrimSpace(language))
	switch {
	case language == "cn", language == "zh", strings.HasPrefix(language, "zh-"):
		return "cn"
	case language == "en", strings.HasPrefix(language, "en-"):
		return "en"
	case language == "ja", strings.HasPrefix(language, "ja-"):
		return "ja"
	case language == "ko", strings.HasPrefix(language, "ko-"):
		return "ko"
	case language == "ru", strings.HasPrefix(language, "ru-"):
		return "ru"
	case language == "fr", strings.HasPrefix(language, "fr-"):
		return "fr"
	case language == "es", strings.HasPrefix(language, "es-"):
		return "es"
	case language == "vi", strings.HasPrefix(language, "vi-"):
		return "vi"
	default:
		return language
	}
}
