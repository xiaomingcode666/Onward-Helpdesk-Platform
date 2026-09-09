package providers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"remotehelpdesk/internal/pkg/config"
)

type SpeechAudioEncoding string

const SpeechAudioEncodingPCM16LE SpeechAudioEncoding = "pcm_s16le"

type SpeechAudioFormat struct {
	Encoding      SpeechAudioEncoding
	SampleRateHz  int
	Channels      int
	BitsPerSample int
	FrameDuration time.Duration
}

func DefaultSpeechAudioFormat() SpeechAudioFormat {
	return SpeechAudioFormat{
		Encoding:      SpeechAudioEncodingPCM16LE,
		SampleRateHz:  16_000,
		Channels:      1,
		BitsPerSample: 16,
		FrameDuration: 40 * time.Millisecond,
	}
}

func (f SpeechAudioFormat) Normalized() SpeechAudioFormat {
	defaults := DefaultSpeechAudioFormat()
	if f.Encoding == "" {
		f.Encoding = defaults.Encoding
	}
	if f.SampleRateHz == 0 {
		f.SampleRateHz = defaults.SampleRateHz
	}
	if f.Channels == 0 {
		f.Channels = defaults.Channels
	}
	if f.BitsPerSample == 0 {
		f.BitsPerSample = defaults.BitsPerSample
	}
	if f.FrameDuration == 0 {
		f.FrameDuration = defaults.FrameDuration
	}
	return f
}

func (f SpeechAudioFormat) Validate() error {
	f = f.Normalized()
	if strings.TrimSpace(string(f.Encoding)) == "" {
		return errors.New("speech audio encoding is required")
	}
	if f.SampleRateHz < 8_000 || f.SampleRateHz > 192_000 {
		return errors.New("speech audio sample rate must be between 8000 and 192000 Hz")
	}
	if f.Channels < 1 || f.Channels > 8 {
		return errors.New("speech audio channels must be between 1 and 8")
	}
	if f.BitsPerSample < 8 || f.BitsPerSample > 32 || f.BitsPerSample%8 != 0 {
		return errors.New("speech audio bits per sample must be 8, 16, 24, or 32")
	}
	if f.FrameDuration < 5*time.Millisecond || f.FrameDuration > time.Second {
		return errors.New("speech audio frame duration must be between 5ms and 1s")
	}
	return nil
}

func (f SpeechAudioFormat) BytesPerSecond() (int, error) {
	f = f.Normalized()
	if err := f.Validate(); err != nil {
		return 0, err
	}
	return f.SampleRateHz * f.Channels * f.BitsPerSample / 8, nil
}

func (f SpeechAudioFormat) FrameBytes() (int, error) {
	f = f.Normalized()
	bytesPerSecond, err := f.BytesPerSecond()
	if err != nil {
		return 0, err
	}
	frameBytes := int(int64(bytesPerSecond) * int64(f.FrameDuration) / int64(time.Second))
	if frameBytes <= 0 {
		return 0, errors.New("speech audio frame size must be positive")
	}
	return frameBytes, nil
}

type SpeechTranscriptionEvent struct {
	ProviderEventID string  `json:"providerEventId"`
	Text            string  `json:"text"`
	IsFinal         bool    `json:"isFinal"`
	StartedAtMS     int64   `json:"startedAtMs"`
	EndedAtMS       int64   `json:"endedAtMs"`
	Confidence      float64 `json:"confidence,omitempty"`
	RawJSON         string  `json:"-"`
}

type SpeechTranscriptionStream interface {
	WriteAudio(ctx context.Context, data []byte) error
	EndAudio(ctx context.Context) error
	ReadEvent(ctx context.Context) (*SpeechTranscriptionEvent, error)
	Close(ctx context.Context) error
}

type SpeechTranscriptionOptions struct {
	Language    string
	Domain      string
	AudioFormat SpeechAudioFormat
}

type SpeechTranscriptionProvider interface {
	Name() string
	Configured() bool
	Open(ctx context.Context, options SpeechTranscriptionOptions) (SpeechTranscriptionStream, error)
}

type SpeechTranscriptionProviderFactory func(cfg *config.SpeechConfig) (SpeechTranscriptionProvider, error)

var (
	speechProviderFactoryMu  sync.RWMutex
	processSpeechStreamQuota = newSpeechStreamQuota(32)
	speechProviderFactories  = map[string]SpeechTranscriptionProviderFactory{
		"mock": func(cfg *config.SpeechConfig) (SpeechTranscriptionProvider, error) {
			return NewMockSpeechTranscriptionProvider(cfg.Mock), nil
		},
		"xfyun": func(cfg *config.SpeechConfig) (SpeechTranscriptionProvider, error) {
			return NewXfyunRTASRClient(cfg), nil
		},
		"aliyun": func(cfg *config.SpeechConfig) (SpeechTranscriptionProvider, error) {
			return NewAliyunASRClient(cfg), nil
		},
	}
	currentSpeechTranscriptionProvider atomic.Pointer[speechTranscriptionProviderSnapshot]
)

type speechTranscriptionProviderSnapshot struct {
	provider SpeechTranscriptionProvider
	config   config.SpeechConfig
}

func init() {
	currentSpeechTranscriptionProvider.Store(&speechTranscriptionProviderSnapshot{
		provider: disabledSpeechTranscriptionProvider{},
	})
}

func RegisterSpeechTranscriptionProvider(name string, factory SpeechTranscriptionProviderFactory) error {
	name = normalizeSpeechProviderName(name)
	if name == "" || name == "disabled" {
		return errors.New("speech provider name must not be empty or reserved")
	}
	if factory == nil {
		return errors.New("speech provider factory is required")
	}
	speechProviderFactoryMu.Lock()
	defer speechProviderFactoryMu.Unlock()
	if _, exists := speechProviderFactories[name]; exists {
		return fmt.Errorf("speech provider %q is already registered", name)
	}
	speechProviderFactories[name] = factory
	return nil
}

func InitSpeech(cfg *config.SpeechConfig) error {
	if cfg == nil {
		cfg = &config.SpeechConfig{}
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	provider, err := buildSpeechTranscriptionProvider(cfg)
	if err != nil {
		return err
	}
	currentSpeechTranscriptionProvider.Store(&speechTranscriptionProviderSnapshot{
		provider: provider,
		config:   *cfg,
	})
	initTextTranslation(cfg)
	slog.Info("speech transcription provider initialized",
		"provider", provider.Name(),
		"configured", provider.Configured(),
	)
	return nil
}

// NewSpeechTranscriptionProvider builds an isolated provider for configuration
// validation and connection tests without replacing the live runtime.
func NewSpeechTranscriptionProvider(cfg *config.SpeechConfig) (SpeechTranscriptionProvider, error) {
	if cfg == nil {
		cfg = &config.SpeechConfig{}
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.ProviderName() == "disabled" {
		return disabledSpeechTranscriptionProvider{}, nil
	}
	name := cfg.ProviderName()
	speechProviderFactoryMu.RLock()
	factory := speechProviderFactories[name]
	speechProviderFactoryMu.RUnlock()
	if factory == nil {
		return nil, fmt.Errorf("speech transcription provider %q is not registered", name)
	}
	provider, err := factory(cfg)
	if err != nil {
		return nil, fmt.Errorf("initialize speech transcription provider %q: %w", name, err)
	}
	if provider == nil || !provider.Configured() {
		return nil, fmt.Errorf("speech transcription provider %q is not configured", name)
	}
	return NewLimitedSpeechTranscriptionProvider(provider, cfg.Jigasi.MaxConcurrent()), nil
}

func CurrentSpeechTranscriptionProvider() SpeechTranscriptionProvider {
	_, provider := CurrentSpeechRuntime()
	return provider
}

func CurrentSpeechConfig() config.SpeechConfig {
	cfg, _ := CurrentSpeechRuntime()
	return cfg
}

// CurrentSpeechRuntime returns one consistent provider/config snapshot so a
// live provider replacement cannot mix new credentials with old Jigasi limits.
func CurrentSpeechRuntime() (config.SpeechConfig, SpeechTranscriptionProvider) {
	snapshot := currentSpeechTranscriptionProvider.Load()
	if snapshot == nil || snapshot.provider == nil {
		return config.SpeechConfig{}, disabledSpeechTranscriptionProvider{}
	}
	return snapshot.config, snapshot.provider
}

func buildSpeechTranscriptionProvider(cfg *config.SpeechConfig) (SpeechTranscriptionProvider, error) {
	if cfg == nil || cfg.ProviderName() == "disabled" {
		return disabledSpeechTranscriptionProvider{}, nil
	}
	name := cfg.ProviderName()
	speechProviderFactoryMu.RLock()
	factory := speechProviderFactories[name]
	speechProviderFactoryMu.RUnlock()
	if factory == nil {
		return nil, fmt.Errorf("speech transcription provider %q is not registered", name)
	}
	provider, err := factory(cfg)
	if err != nil {
		return nil, fmt.Errorf("initialize speech transcription provider %q: %w", name, err)
	}
	if provider == nil || !provider.Configured() {
		return nil, fmt.Errorf("speech transcription provider %q is not configured", name)
	}
	processSpeechStreamQuota.setLimit(cfg.Jigasi.MaxConcurrent())
	return newLimitedSpeechTranscriptionProvider(provider, processSpeechStreamQuota), nil
}

func normalizeSpeechProviderName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

type disabledSpeechTranscriptionProvider struct{}

func (disabledSpeechTranscriptionProvider) Name() string     { return "disabled" }
func (disabledSpeechTranscriptionProvider) Configured() bool { return false }
func (disabledSpeechTranscriptionProvider) Open(context.Context, SpeechTranscriptionOptions) (SpeechTranscriptionStream, error) {
	return nil, errors.New("speech transcription is disabled")
}
