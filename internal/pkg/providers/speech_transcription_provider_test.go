package providers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"remotehelpdesk/internal/pkg/config"
)

type testSpeechProvider struct {
	name string
}

func (p testSpeechProvider) Name() string     { return p.name }
func (p testSpeechProvider) Configured() bool { return true }
func (p testSpeechProvider) Open(context.Context, SpeechTranscriptionOptions) (SpeechTranscriptionStream, error) {
	return newTestSpeechStream(), nil
}

type testSpeechStream struct {
	closed chan struct{}
	once   sync.Once
}

func newTestSpeechStream() *testSpeechStream {
	return &testSpeechStream{closed: make(chan struct{})}
}

func (s *testSpeechStream) WriteAudio(context.Context, []byte) error { return nil }
func (s *testSpeechStream) EndAudio(context.Context) error           { return nil }
func (s *testSpeechStream) ReadEvent(context.Context) (*SpeechTranscriptionEvent, error) {
	<-s.closed
	return nil, io.EOF
}
func (s *testSpeechStream) Close(context.Context) error {
	s.once.Do(func() { close(s.closed) })
	return nil
}

func TestSpeechProviderSelection(t *testing.T) {
	disabled, err := buildSpeechTranscriptionProvider(&config.SpeechConfig{Provider: "disabled"})
	if err != nil {
		t.Fatalf("build disabled provider: %v", err)
	}
	if disabled.Name() != "disabled" || disabled.Configured() {
		t.Fatalf("disabled provider = %s configured=%v", disabled.Name(), disabled.Configured())
	}

	xfyun, err := buildSpeechTranscriptionProvider(&config.SpeechConfig{
		Provider: "xfyun",
		Xfyun: config.XfyunSpeechConfig{
			AppID:  "test-app",
			APIKey: "test-key",
		},
	})
	if err != nil {
		t.Fatalf("build xfyun provider: %v", err)
	}
	if xfyun.Name() != "xfyun_rtasr" || !xfyun.Configured() {
		t.Fatalf("xfyun provider = %s configured=%v", xfyun.Name(), xfyun.Configured())
	}

	aliyun, err := buildSpeechTranscriptionProvider(&config.SpeechConfig{
		Provider: "aliyun",
		Aliyun:   config.AliyunSpeechConfig{APIKey: "test-key"},
	})
	if err != nil {
		t.Fatalf("build aliyun provider: %v", err)
	}
	if aliyun.Name() != "aliyun_qwen_asr" || !aliyun.Configured() {
		t.Fatalf("aliyun provider = %s configured=%v", aliyun.Name(), aliyun.Configured())
	}

	mock, err := buildSpeechTranscriptionProvider(&config.SpeechConfig{Provider: "mock"})
	if err != nil {
		t.Fatalf("build mock provider: %v", err)
	}
	if mock.Name() != "mock_speech" || !mock.Configured() {
		t.Fatalf("mock provider = %s configured=%v", mock.Name(), mock.Configured())
	}
}

func TestSpeechProviderRegistryBuildsCustomProvider(t *testing.T) {
	const providerName = "test_custom_speech"
	registeredOptions := false
	err := RegisterSpeechTranscriptionProvider(providerName, func(cfg *config.SpeechConfig) (SpeechTranscriptionProvider, error) {
		registeredOptions = cfg.ProviderOptions["region"] == "cn-east"
		return testSpeechProvider{name: providerName}, nil
	})
	if err != nil {
		t.Fatalf("register custom provider: %v", err)
	}
	t.Cleanup(func() {
		speechProviderFactoryMu.Lock()
		delete(speechProviderFactories, providerName)
		speechProviderFactoryMu.Unlock()
	})

	provider, err := buildSpeechTranscriptionProvider(&config.SpeechConfig{
		Provider:        providerName,
		ProviderOptions: map[string]any{"region": "cn-east"},
	})
	if err != nil {
		t.Fatalf("build custom provider: %v", err)
	}
	if provider.Name() != providerName || !registeredOptions {
		t.Fatalf("custom provider = %q optionsPassed=%v", provider.Name(), registeredOptions)
	}
	if _, err := buildSpeechTranscriptionProvider(&config.SpeechConfig{Provider: "not_registered"}); err == nil {
		t.Fatal("unregistered provider was accepted")
	}
}

func TestDefaultSpeechAudioFormat(t *testing.T) {
	format := DefaultSpeechAudioFormat()
	bytesPerSecond, err := format.BytesPerSecond()
	if err != nil {
		t.Fatalf("bytes per second: %v", err)
	}
	frameBytes, err := format.FrameBytes()
	if err != nil {
		t.Fatalf("frame bytes: %v", err)
	}
	if bytesPerSecond != 32_000 || frameBytes != 1_280 {
		t.Fatalf("default audio format = %d bytes/s, %d bytes/frame", bytesPerSecond, frameBytes)
	}
}

func TestInitSpeechKeepsCurrentProviderWhenReplacementFails(t *testing.T) {
	previous := currentSpeechTranscriptionProvider.Load()
	t.Cleanup(func() { currentSpeechTranscriptionProvider.Store(previous) })

	const providerName = "test_replacement_speech"
	if err := RegisterSpeechTranscriptionProvider(providerName, func(*config.SpeechConfig) (SpeechTranscriptionProvider, error) {
		return testSpeechProvider{name: providerName}, nil
	}); err != nil {
		t.Fatalf("register provider: %v", err)
	}
	t.Cleanup(func() {
		speechProviderFactoryMu.Lock()
		delete(speechProviderFactories, providerName)
		speechProviderFactoryMu.Unlock()
	})
	if err := InitSpeech(&config.SpeechConfig{Provider: providerName}); err != nil {
		t.Fatalf("initialize provider: %v", err)
	}
	if got := CurrentSpeechTranscriptionProvider().Name(); got != providerName {
		t.Fatalf("current provider = %q", got)
	}
	if err := InitSpeech(&config.SpeechConfig{Provider: "not_registered"}); err == nil {
		t.Fatal("unregistered replacement was accepted")
	}
	if got := CurrentSpeechTranscriptionProvider().Name(); got != providerName {
		t.Fatalf("failed replacement changed current provider to %q", got)
	}
}

func TestCurrentSpeechRuntimeReturnsConsistentAtomicSnapshot(t *testing.T) {
	previous := currentSpeechTranscriptionProvider.Load()
	t.Cleanup(func() { currentSpeechTranscriptionProvider.Store(previous) })

	providerNames := []string{"snapshot_a", "snapshot_b"}
	for _, name := range providerNames {
		providerName := name
		if err := RegisterSpeechTranscriptionProvider(providerName, func(*config.SpeechConfig) (SpeechTranscriptionProvider, error) {
			return testSpeechProvider{name: providerName}, nil
		}); err != nil {
			t.Fatalf("register %s: %v", providerName, err)
		}
	}
	t.Cleanup(func() {
		speechProviderFactoryMu.Lock()
		for _, name := range providerNames {
			delete(speechProviderFactories, name)
		}
		speechProviderFactoryMu.Unlock()
	})
	if err := InitSpeech(&config.SpeechConfig{Provider: providerNames[0]}); err != nil {
		t.Fatalf("initialize snapshot provider: %v", err)
	}

	errorsCh := make(chan error, 1)
	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		for i := 0; i < 200; i++ {
			if err := InitSpeech(&config.SpeechConfig{Provider: providerNames[i%len(providerNames)]}); err != nil {
				select {
				case errorsCh <- err:
				default:
				}
				return
			}
		}
	}()
	go func() {
		defer wait.Done()
		for i := 0; i < 10_000; i++ {
			cfg, provider := CurrentSpeechRuntime()
			if provider == nil || cfg.ProviderName() != provider.Name() {
				select {
				case errorsCh <- fmt.Errorf("mixed runtime snapshot: config=%q provider=%v", cfg.ProviderName(), provider):
				default:
				}
				return
			}
		}
	}()
	wait.Wait()
	select {
	case err := <-errorsCh:
		t.Fatal(err)
	default:
	}
}

func TestSpeechAudioFormatRejectsInvalidFrameDuration(t *testing.T) {
	format := DefaultSpeechAudioFormat()
	format.FrameDuration = time.Microsecond
	if _, err := format.FrameBytes(); err == nil || !strings.Contains(err.Error(), "frame duration") {
		t.Fatalf("FrameBytes() error = %v", err)
	}
}

func TestSpeechStreamLimiterReleasesSlotOnClose(t *testing.T) {
	provider := NewLimitedSpeechTranscriptionProvider(testSpeechProvider{name: "test_limited"}, 1)
	first, err := provider.Open(context.Background(), SpeechTranscriptionOptions{})
	if err != nil {
		t.Fatalf("open first stream: %v", err)
	}
	if _, err := provider.Open(context.Background(), SpeechTranscriptionOptions{}); !errors.Is(err, ErrSpeechStreamLimitExceeded) {
		t.Fatalf("open over limit error = %v", err)
	}
	if err := first.Close(context.Background()); err != nil {
		t.Fatalf("close first stream: %v", err)
	}
	second, err := provider.Open(context.Background(), SpeechTranscriptionOptions{})
	if err != nil {
		t.Fatalf("open after release: %v", err)
	}
	_ = second.Close(context.Background())
}

func TestSpeechProviderReplacementsShareProcessQuota(t *testing.T) {
	previousLimit := processSpeechStreamQuota.limit.Load()
	t.Cleanup(func() { processSpeechStreamQuota.setLimit(int(previousLimit)) })
	const providerName = "test_quota_speech"
	if err := RegisterSpeechTranscriptionProvider(providerName, func(*config.SpeechConfig) (SpeechTranscriptionProvider, error) {
		return testSpeechProvider{name: providerName}, nil
	}); err != nil {
		t.Fatalf("register provider: %v", err)
	}
	t.Cleanup(func() {
		speechProviderFactoryMu.Lock()
		delete(speechProviderFactories, providerName)
		speechProviderFactoryMu.Unlock()
	})
	configWithSingleSlot := &config.SpeechConfig{
		Provider: providerName,
		Jigasi:   config.JigasiSpeechConfig{MaxConcurrentStreams: 1},
	}
	firstProvider, err := buildSpeechTranscriptionProvider(configWithSingleSlot)
	if err != nil {
		t.Fatalf("build first provider: %v", err)
	}
	first, err := firstProvider.Open(context.Background(), SpeechTranscriptionOptions{})
	if err != nil {
		t.Fatalf("open first stream: %v", err)
	}
	defer first.Close(context.Background())

	replacement, err := buildSpeechTranscriptionProvider(configWithSingleSlot)
	if err != nil {
		t.Fatalf("build replacement provider: %v", err)
	}
	if _, err := replacement.Open(context.Background(), SpeechTranscriptionOptions{}); !errors.Is(err, ErrSpeechStreamLimitExceeded) {
		t.Fatalf("replacement open error = %v", err)
	}
	if err := first.Close(context.Background()); err != nil {
		t.Fatalf("close first stream: %v", err)
	}
	stream, err := replacement.Open(context.Background(), SpeechTranscriptionOptions{})
	if err != nil {
		t.Fatalf("open replacement after release: %v", err)
	}
	_ = stream.Close(context.Background())
}
