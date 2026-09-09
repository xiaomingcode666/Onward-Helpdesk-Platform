package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"remotehelpdesk/internal/pkg/config"
)

var defaultMockTranscriptPhrases = []string{
	"设备运行状态正常。",
	"正在检查故障代码和传感器数据。",
	"请确认电源、网络和急停开关状态。",
}

type MockSpeechTranscriptionProvider struct {
	segmentDuration time.Duration
	phrases         []string
}

func NewMockSpeechTranscriptionProvider(cfg config.MockSpeechConfig) *MockSpeechTranscriptionProvider {
	return &MockSpeechTranscriptionProvider{
		segmentDuration: cfg.SegmentDuration(),
		phrases:         append([]string(nil), defaultMockTranscriptPhrases...),
	}
}

func (p *MockSpeechTranscriptionProvider) Name() string { return "mock_speech" }

func (p *MockSpeechTranscriptionProvider) Configured() bool {
	return p != nil && p.segmentDuration > 0 && len(p.phrases) > 0
}

func (p *MockSpeechTranscriptionProvider) Open(ctx context.Context, options SpeechTranscriptionOptions) (SpeechTranscriptionStream, error) {
	if !p.Configured() {
		return nil, errors.New("mock speech provider is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	audioFormat := options.AudioFormat.Normalized()
	bytesPerSecond, err := audioFormat.BytesPerSecond()
	if err != nil {
		cancel()
		return nil, err
	}
	segmentBytes := int64(p.segmentDuration) * int64(bytesPerSecond) / int64(time.Second)
	if segmentBytes <= 0 {
		cancel()
		return nil, errors.New("mock speech segment size must be positive")
	}
	language := strings.TrimSpace(options.Language)
	if language == "" {
		language = "zh-CN"
	}
	return &mockSpeechTranscriptionStream{
		ctx:            ctx,
		language:       language,
		segmentBytes:   segmentBytes,
		bytesPerSecond: int64(bytesPerSecond),
		phrases:        append([]string(nil), p.phrases...),
		events:         make(chan *SpeechTranscriptionEvent, 8),
		cancel:         cancel,
	}, nil
}

type mockSpeechTranscriptionStream struct {
	ctx            context.Context
	language       string
	segmentBytes   int64
	bytesPerSecond int64
	phrases        []string
	events         chan *SpeechTranscriptionEvent
	cancel         context.CancelFunc

	mu               sync.Mutex
	closed           bool
	segmentIndex     int64
	segmentAudioSize int64
	partialEmitted   bool
}

func (s *mockSpeechTranscriptionStream) WriteAudio(ctx context.Context, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	s.segmentAudioSize += int64(len(data))
	if !s.partialEmitted && s.segmentAudioSize >= s.segmentBytes/2 {
		if err := s.emitLocked(ctx, false); err != nil {
			return err
		}
		s.partialEmitted = true
	}
	for s.segmentAudioSize >= s.segmentBytes {
		if err := s.emitLocked(ctx, true); err != nil {
			return err
		}
		s.segmentAudioSize -= s.segmentBytes
		s.segmentIndex++
		s.partialEmitted = false
		if s.segmentAudioSize >= s.segmentBytes/2 {
			if err := s.emitLocked(ctx, false); err != nil {
				return err
			}
			s.partialEmitted = true
		}
	}
	return nil
}

func (s *mockSpeechTranscriptionStream) EndAudio(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	if s.segmentAudioSize > 0 {
		if err := s.emitLocked(ctx, true); err != nil {
			return err
		}
	}
	s.closed = true
	close(s.events)
	return nil
}

func (s *mockSpeechTranscriptionStream) ReadEvent(ctx context.Context) (*SpeechTranscriptionEvent, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	case event, ok := <-s.events:
		if !ok {
			return nil, io.EOF
		}
		return event, nil
	}
}

func (s *mockSpeechTranscriptionStream) Close(_ context.Context) error {
	s.cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.events)
	}
	return nil
}

func (s *mockSpeechTranscriptionStream) emitLocked(ctx context.Context, final bool) error {
	phrase := s.phrases[int(s.segmentIndex)%len(s.phrases)]
	text := phrase
	if !final {
		text = strings.TrimRight(phrase, "。！？.!?") + "..."
	}
	startedAtMS := s.segmentIndex * s.segmentBytes * 1000 / s.bytesPerSecond
	endedBytes := s.segmentIndex*s.segmentBytes + min(s.segmentAudioSize, s.segmentBytes)
	endedAtMS := endedBytes * 1000 / s.bytesPerSecond
	rawJSON, _ := json.Marshal(map[string]any{"mock": true, "language": s.language})
	event := &SpeechTranscriptionEvent{
		ProviderEventID: fmt.Sprintf("mock-%06d", s.segmentIndex+1),
		Text:            text,
		IsFinal:         final,
		StartedAtMS:     startedAtMS,
		EndedAtMS:       endedAtMS,
		Confidence:      0.99,
		RawJSON:         string(rawJSON),
	}
	if !final {
		select {
		case s.events <- event:
		default:
		}
		return nil
	}
	select {
	case s.events <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
}
