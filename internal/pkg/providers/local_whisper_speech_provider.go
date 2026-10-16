package providers

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"remotehelpdesk/internal/pkg/config"
)

// ErrLocalSpeechUnavailable marks failures to reach the local ASR service
// (not running, timed out, or returned a server error).
var ErrLocalSpeechUnavailable = errors.New("local speech service unavailable")

const maxLocalWhisperResponseBytes = 1 << 20

// LocalWhisperSpeechProvider sends buffered audio to services/local_asr, which
// runs a local Whisper model. It is a batch engine behind the streaming
// interface: audio is collected until EndAudio and transcribed in one request.
type LocalWhisperSpeechProvider struct {
	cfg    config.LocalWhisperSpeechConfig
	client *http.Client
}

func NewLocalWhisperSpeechProvider(cfg config.LocalWhisperSpeechConfig) SpeechTranscriptionProvider {
	return &LocalWhisperSpeechProvider{cfg: cfg, client: &http.Client{Timeout: cfg.Timeout()}}
}

func (p *LocalWhisperSpeechProvider) Name() string { return "local_whisper" }

func (p *LocalWhisperSpeechProvider) Configured() bool {
	return p != nil && strings.TrimSpace(p.cfg.Endpoint) != ""
}

func (p *LocalWhisperSpeechProvider) Open(_ context.Context, options SpeechTranscriptionOptions) (SpeechTranscriptionStream, error) {
	if !p.Configured() {
		return nil, errors.New("local whisper speech provider is not configured")
	}
	format := options.AudioFormat.Normalized()
	if err := format.Validate(); err != nil {
		return nil, err
	}
	if format.Encoding != SpeechAudioEncodingPCM16LE {
		return nil, fmt.Errorf("local whisper requires %s audio", SpeechAudioEncodingPCM16LE)
	}
	language := strings.TrimSpace(options.Language)
	if language == "" {
		language = "zh-CN"
	}
	return &localWhisperStream{provider: p, format: format, language: language}, nil
}

type localWhisperStream struct {
	provider *LocalWhisperSpeechProvider
	format   SpeechAudioFormat
	language string

	mu     sync.Mutex
	pcm    bytes.Buffer
	ended  bool
	closed bool
	event  *SpeechTranscriptionEvent
}

func (s *localWhisperStream) WriteAudio(_ context.Context, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.ended {
		return errors.New("local whisper stream is no longer accepting audio")
	}
	s.pcm.Write(data)
	return nil
}

func (s *localWhisperStream) EndAudio(ctx context.Context) error {
	s.mu.Lock()
	if s.closed || s.ended {
		s.mu.Unlock()
		return nil
	}
	s.ended = true
	pcm := append([]byte(nil), s.pcm.Bytes()...)
	s.mu.Unlock()

	text, err := s.provider.transcribe(ctx, encodePCM16WAV(pcm, s.format), s.language)
	if err != nil {
		return err
	}
	bytesPerSecond, err := s.format.BytesPerSecond()
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.event = &SpeechTranscriptionEvent{
		ProviderEventID: "local-whisper-000001",
		Text:            text,
		IsFinal:         true,
		EndedAtMS:       int64(len(pcm)) * 1000 / int64(bytesPerSecond),
	}
	s.mu.Unlock()
	return nil
}

func (s *localWhisperStream) ReadEvent(_ context.Context) (*SpeechTranscriptionEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.event == nil {
		return nil, io.EOF
	}
	event := s.event
	s.event = nil
	return event, nil
}

func (s *localWhisperStream) Close(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	s.pcm.Reset()
	return nil
}

func (p *LocalWhisperSpeechProvider) transcribe(ctx context.Context, wav []byte, language string) (string, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(p.cfg.Endpoint), "/") + "/v1/asr/transcribe?language=" + url.QueryEscape(language)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(wav))
	if err != nil {
		return "", fmt.Errorf("build local whisper request: %w", err)
	}
	req.Header.Set("Content-Type", "audio/wav")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call local whisper: %w: %w", ErrLocalSpeechUnavailable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxLocalWhisperResponseBytes))
	if err != nil {
		return "", fmt.Errorf("read local whisper response: %w: %w", ErrLocalSpeechUnavailable, err)
	}
	if resp.StatusCode >= http.StatusInternalServerError {
		return "", fmt.Errorf("local whisper returned HTTP %d: %w", resp.StatusCode, ErrLocalSpeechUnavailable)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("local whisper rejected audio: HTTP %d", resp.StatusCode)
	}
	var result struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("decode local whisper response: %w", err)
	}
	return strings.TrimSpace(result.Text), nil
}

func encodePCM16WAV(pcm []byte, format SpeechAudioFormat) []byte {
	var buffer bytes.Buffer
	blockAlign := format.Channels * format.BitsPerSample / 8
	buffer.WriteString("RIFF")
	_ = binary.Write(&buffer, binary.LittleEndian, uint32(36+len(pcm)))
	buffer.WriteString("WAVEfmt ")
	_ = binary.Write(&buffer, binary.LittleEndian, uint32(16))
	_ = binary.Write(&buffer, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buffer, binary.LittleEndian, uint16(format.Channels))
	_ = binary.Write(&buffer, binary.LittleEndian, uint32(format.SampleRateHz))
	_ = binary.Write(&buffer, binary.LittleEndian, uint32(format.SampleRateHz*blockAlign))
	_ = binary.Write(&buffer, binary.LittleEndian, uint16(blockAlign))
	_ = binary.Write(&buffer, binary.LittleEndian, uint16(format.BitsPerSample))
	buffer.WriteString("data")
	_ = binary.Write(&buffer, binary.LittleEndian, uint32(len(pcm)))
	buffer.Write(pcm)
	return buffer.Bytes()
}
