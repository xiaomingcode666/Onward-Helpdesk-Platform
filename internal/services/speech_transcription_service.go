package services

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"strings"
	"sync"
	"time"

	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/providers"
)

const (
	maxVoiceTranscriptionBytes = 25 << 20
	transcriptionTimeout       = 2 * time.Minute
)

var SpeechTranscriptionService = newSpeechTranscriptionService()

type speechTranscriptionService struct{}

type VoiceTranscriptionResult struct {
	Text            string `json:"text"`
	Language        string `json:"language"`
	Provider        string `json:"provider"`
	DurationSeconds int    `json:"durationSeconds"`
}

func newSpeechTranscriptionService() *speechTranscriptionService {
	return &speechTranscriptionService{}
}

// TranscribeCustomerAudio accepts a PCM WAV recording. Browsers encode their
// MediaRecorder output to this format before calling the endpoint, which keeps
// provider-specific audio decoding out of the server.
func (s *speechTranscriptionService) TranscribeCustomerAudio(file *multipart.FileHeader) (*VoiceTranscriptionResult, error) {
	return s.transcribeFile(file)
}

func (s *speechTranscriptionService) transcribeFile(file *multipart.FileHeader) (*VoiceTranscriptionResult, error) {
	if file == nil || file.Size <= 0 {
		return nil, errorsx.InvalidParam("audio file is required")
	}
	if file.Size > maxVoiceTranscriptionBytes {
		return nil, errorsx.InvalidParam("audio file is too large")
	}

	input, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("open audio file: %w", err)
	}
	defer func() { _ = input.Close() }()
	raw, err := io.ReadAll(io.LimitReader(input, maxVoiceTranscriptionBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read audio file: %w", err)
	}
	if len(raw) > maxVoiceTranscriptionBytes {
		return nil, errorsx.InvalidParam("audio file is too large")
	}
	pcm, durationSeconds, err := decodePCM16WAV(raw)
	if err != nil {
		return nil, errorsx.InvalidParam(err.Error())
	}

	_, provider := providers.CurrentSpeechRuntime()
	if provider == nil || !provider.Configured() {
		return nil, errors.New("speech transcription is disabled or not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), transcriptionTimeout)
	defer cancel()
	format := providers.DefaultSpeechAudioFormat()
	stream, err := provider.Open(ctx, providers.SpeechTranscriptionOptions{
		Language:    "zh-CN",
		AudioFormat: format,
	})
	if err != nil {
		return nil, fmt.Errorf("open speech transcription stream: %w", err)
	}
	defer func() { _ = stream.Close(context.Background()) }()

	var eventsMu sync.Mutex
	var events []*providers.SpeechTranscriptionEvent
	readErr := make(chan error, 1)
	go func() {
		for {
			event, readError := stream.ReadEvent(ctx)
			if errors.Is(readError, io.EOF) {
				readErr <- nil
				return
			}
			if readError != nil {
				readErr <- readError
				return
			}
			if event != nil {
				eventsMu.Lock()
				events = append(events, event)
				eventsMu.Unlock()
			}
		}
	}()

	frameBytes, err := format.FrameBytes()
	if err != nil {
		return nil, err
	}
	for offset := 0; offset < len(pcm); offset += frameBytes {
		end := offset + frameBytes
		if end > len(pcm) {
			end = len(pcm)
		}
		if err := stream.WriteAudio(ctx, pcm[offset:end]); err != nil {
			return nil, fmt.Errorf("write speech audio: %w", err)
		}
	}
	if err := stream.EndAudio(ctx); err != nil {
		return nil, speechTranscriptionError("finish speech audio", err)
	}
	if err := <-readErr; err != nil {
		return nil, speechTranscriptionError("read speech transcription", err)
	}

	var finalText []string
	var latestText string
	eventsMu.Lock()
	collectedEvents := append([]*providers.SpeechTranscriptionEvent(nil), events...)
	eventsMu.Unlock()
	for _, event := range collectedEvents {
		text := strings.TrimSpace(event.Text)
		if text == "" {
			continue
		}
		latestText = text
		if event.IsFinal {
			finalText = append(finalText, text)
		}
	}
	text := strings.TrimSpace(strings.Join(finalText, " "))
	if text == "" {
		text = latestText
	}
	if text == "" {
		return nil, errorsx.InvalidParamI18n("speech.noSpeech")
	}
	return &VoiceTranscriptionResult{
		Text:            text,
		Language:        "zh-CN",
		Provider:        provider.Name(),
		DurationSeconds: durationSeconds,
	}, nil
}

func speechTranscriptionError(step string, err error) error {
	if errors.Is(err, providers.ErrLocalSpeechUnavailable) {
		slog.Warn("local speech service unavailable", "step", step, "error", err)
		return errorsx.InvalidParamI18n("speech.localUnavailable")
	}
	return fmt.Errorf("%s: %w", step, err)
}

func decodePCM16WAV(raw []byte) ([]byte, int, error) {
	if len(raw) < 12 || string(raw[:4]) != "RIFF" || string(raw[8:12]) != "WAVE" {
		return nil, 0, errors.New("audio must be a PCM WAV file")
	}
	var channels, bitsPerSample, sampleRate int
	var pcm []byte
	for offset := 12; offset+8 <= len(raw); {
		chunkID := string(raw[offset : offset+4])
		chunkSize := int(binary.LittleEndian.Uint32(raw[offset+4 : offset+8]))
		offset += 8
		if chunkSize < 0 || offset+chunkSize > len(raw) {
			return nil, 0, errors.New("audio WAV chunk is invalid")
		}
		chunk := raw[offset : offset+chunkSize]
		switch chunkID {
		case "fmt ":
			if len(chunk) < 16 {
				return nil, 0, errors.New("audio WAV format chunk is invalid")
			}
			if binary.LittleEndian.Uint16(chunk[0:2]) != 1 {
				return nil, 0, errors.New("audio must use PCM encoding")
			}
			channels = int(binary.LittleEndian.Uint16(chunk[2:4]))
			sampleRate = int(binary.LittleEndian.Uint32(chunk[4:8]))
			bitsPerSample = int(binary.LittleEndian.Uint16(chunk[14:16]))
		case "data":
			pcm = append([]byte(nil), chunk...)
		}
		offset += chunkSize
		if chunkSize%2 == 1 {
			offset++
		}
	}
	if channels != 1 || sampleRate != 16_000 || bitsPerSample != 16 || len(pcm) == 0 {
		return nil, 0, errors.New("audio must be mono 16-bit PCM WAV at 16000 Hz")
	}
	return pcm, int((len(pcm) / 2) / sampleRate), nil
}
