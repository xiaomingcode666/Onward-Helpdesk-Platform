package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	jigasiWhisperHeaderBytes   = 60
	jigasiClientWriteTimeout   = 10 * time.Second
	jigasiClientIdleTimeout    = 2 * time.Minute
	jigasiReaderShutdownWindow = 5 * time.Second
)

// JigasiWhisperGateway implements the websocket protocol used by Jigasi's
// built-in WhisperTranscriptionService and routes each participant stream to
// the configured speech provider.
type JigasiWhisperGateway struct {
	provider     SpeechTranscriptionProvider
	maxStreams   int
	audioFormat  SpeechAudioFormat
	frameBytes   int
	connectionID string
}

func NewJigasiWhisperGateway(provider SpeechTranscriptionProvider, maxStreams int) *JigasiWhisperGateway {
	if maxStreams <= 0 {
		maxStreams = 8
	}
	audioFormat := DefaultSpeechAudioFormat()
	frameBytes, _ := audioFormat.FrameBytes()
	return &JigasiWhisperGateway{
		provider: provider, maxStreams: maxStreams,
		audioFormat: audioFormat, frameBytes: frameBytes,
	}
}

func (g *JigasiWhisperGateway) WithConnectionID(connectionID string) *JigasiWhisperGateway {
	if g != nil {
		g.connectionID = strings.TrimSpace(connectionID)
	}
	return g
}

type jigasiWhisperAudioPacket struct {
	ParticipantID string
	Language      string
	Audio         []byte
}

type jigasiWhisperParticipantSession struct {
	participantID string
	language      string
	stream        SpeechTranscriptionStream
	pending       []byte
	frameBytes    int
	ended         atomic.Bool
	observedAudio bool
}

type jigasiWhisperResult struct {
	Type          string          `json:"type"`
	ParticipantID string          `json:"participant_id"`
	Text          string          `json:"text"`
	Variance      json.RawMessage `json:"variance"`
}

func (g *JigasiWhisperGateway) Serve(ctx context.Context, client *websocket.Conn) error {
	if g == nil || g.provider == nil || !g.provider.Configured() {
		return errors.New("speech transcription provider is not configured")
	}
	ctx, cancel := context.WithCancel(ctx)
	if err := client.SetReadDeadline(time.Now().Add(jigasiClientIdleTimeout)); err != nil {
		cancel()
		return err
	}
	client.SetPongHandler(func(string) error {
		return client.SetReadDeadline(time.Now().Add(jigasiClientIdleTimeout))
	})
	var closeClientOnce sync.Once
	closeClient := func(code int, reason string) {
		closeClientOnce.Do(func() {
			_ = client.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(code, reason),
				time.Now().Add(jigasiClientWriteTimeout),
			)
			_ = client.Close()
		})
	}
	stopContextClose := context.AfterFunc(ctx, func() {
		closeClient(websocket.CloseGoingAway, "transcription context closed")
	})
	defer func() {
		stopContextClose()
		cancel()
		closeClient(websocket.CloseNormalClosure, "")
	}()

	sessions := make(map[string]*jigasiWhisperParticipantSession)
	var readers sync.WaitGroup
	var writeMu sync.Mutex
	var ending atomic.Bool
	startedAt := time.Now()
	var nonBinaryMessages int64
	var audioPackets int64
	var audioBytes int64
	var audioDetected bool
	closeSessions := func(graceful bool) error {
		ending.Store(true)
		shutdownCtx, stopShutdown := context.WithTimeout(context.WithoutCancel(ctx), jigasiReaderShutdownWindow)
		defer stopShutdown()
		var failures []error
		for _, session := range sessions {
			if graceful {
				if err := session.flushAudio(shutdownCtx); err != nil {
					failures = append(failures, err)
				}
				if err := session.stream.EndAudio(shutdownCtx); err != nil {
					failures = append(failures, err)
				}
			} else {
				if err := session.stream.Close(shutdownCtx); err != nil {
					failures = append(failures, err)
				}
			}
		}
		return errors.Join(failures...)
	}
	pruneEndedSessions := func() {
		pruneEndedJigasiSessions(ctx, sessions)
	}
	defer func() {
		fields := []any{
			"connection_id", g.connectionID,
			"provider", g.provider.Name(),
			"duration_ms", time.Since(startedAt).Milliseconds(),
			"participant_streams", len(sessions),
			"audio_packets", audioPackets,
			"audio_bytes", audioBytes,
			"non_binary_messages", nonBinaryMessages,
			"audio_detected", audioDetected,
		}
		if audioPackets == 0 {
			slog.Warn("jigasi transcription websocket closed without participant audio packets", fields...)
		} else if !audioDetected {
			slog.Warn("jigasi transcription websocket closed without audible participant audio", fields...)
		} else {
			slog.Info("jigasi transcription websocket finished", fields...)
		}
	}()
	defer func() {
		_ = closeSessions(false)
		_ = waitForJigasiReaders(&readers, sessions, jigasiReaderShutdownWindow)
	}()

	startReader := func(session *jigasiWhisperParticipantSession) {
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				event, err := session.stream.ReadEvent(ctx)
				if err != nil {
					session.ended.Store(true)
					if !ending.Load() && !errors.Is(err, io.EOF) {
						if isSpeechProviderIdleTimeout(err) {
							slog.Info("jigasi participant transcription stream idle; waiting for audio to reopen",
								"participant_id", session.participantID,
								"provider", g.provider.Name(),
							)
						} else {
							slog.Warn("jigasi participant transcription stream failed; waiting for audio to reopen",
								"participant_id", session.participantID,
								"provider", g.provider.Name(),
								"error", err,
							)
						}
					}
					return
				}
				if event == nil || strings.TrimSpace(event.Text) == "" {
					continue
				}
				payload, err := marshalJigasiWhisperResult(session.participantID, event)
				if err != nil {
					continue
				}
				writeMu.Lock()
				err = client.SetWriteDeadline(time.Now().Add(jigasiClientWriteTimeout))
				if err == nil {
					err = client.WriteMessage(websocket.TextMessage, payload)
				}
				writeMu.Unlock()
				if err != nil {
					closeClient(websocket.CloseInternalServerErr, "transcription client write failed")
					cancel()
					return
				}
			}
		}()
	}

	for {
		messageType, payload, err := client.ReadMessage()
		if err != nil {
			return err
		}
		if err := client.SetReadDeadline(time.Now().Add(jigasiClientIdleTimeout)); err != nil {
			return err
		}
		pruneEndedSessions()
		if messageType != websocket.BinaryMessage {
			nonBinaryMessages++
			continue
		}
		if len(payload) == 1 {
			closeErr := closeSessions(true)
			readerErr := waitForJigasiReaders(&readers, sessions, jigasiReaderShutdownWindow)
			return errors.Join(closeErr, readerErr)
		}

		packet, err := parseJigasiWhisperAudioPacket(payload)
		if err != nil {
			return err
		}
		audioPackets++
		audioBytes += int64(len(packet.Audio))
		session := sessions[packet.ParticipantID]
		if session == nil {
			if len(sessions) >= g.maxStreams {
				return fmt.Errorf("jigasi participant stream limit exceeded: %d", g.maxStreams)
			}
			stream, err := g.provider.Open(ctx, SpeechTranscriptionOptions{
				Language: packet.Language, AudioFormat: g.audioFormat,
			})
			if err != nil {
				return fmt.Errorf("open %s stream for participant %s: %w", g.provider.Name(), packet.ParticipantID, err)
			}
			session = &jigasiWhisperParticipantSession{
				participantID: packet.ParticipantID,
				language:      packet.Language,
				stream:        stream,
				frameBytes:    g.frameBytes,
			}
			sessions[packet.ParticipantID] = session
			slog.Info("jigasi participant transcription stream opened",
				"participant_id", packet.ParticipantID,
				"language", packet.Language,
				"provider", g.provider.Name(),
			)
			startReader(session)
		}
		if !session.observedAudio {
			if peak := pcm16LEPeak(packet.Audio); peak > 256 {
				session.observedAudio = true
				audioDetected = true
				slog.Info("jigasi participant audio detected",
					"participant_id", packet.ParticipantID,
					"peak", peak,
				)
			}
		}
		if err := session.writeAudio(ctx, packet.Audio); err != nil {
			return fmt.Errorf("write %s audio for participant %s: %w", g.provider.Name(), packet.ParticipantID, err)
		}
	}
}

func isSpeechProviderIdleTimeout(err error) bool {
	if err == nil {
		return false
	}
	var xfyunErr *XfyunRTASRError
	if errors.As(err, &xfyunErr) && strings.Contains(strings.ToLower(xfyunErr.Description), "idle timeout") {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "idle timeout")
}

func pcm16LEPeak(audio []byte) int {
	peak := 0
	for offset := 0; offset+1 < len(audio); offset += 2 {
		sample := int(int16(uint16(audio[offset]) | uint16(audio[offset+1])<<8))
		if sample < 0 {
			sample = -sample
		}
		if sample > peak {
			peak = sample
		}
	}
	return peak
}

func pruneEndedJigasiSessions(ctx context.Context, sessions map[string]*jigasiWhisperParticipantSession) {
	for participantID, session := range sessions {
		if session == nil || !session.ended.Load() {
			continue
		}
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), jigasiReaderShutdownWindow)
		_ = session.stream.Close(closeCtx)
		cancel()
		delete(sessions, participantID)
	}
}

func waitForJigasiReaders(readers *sync.WaitGroup, sessions map[string]*jigasiWhisperParticipantSession, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = jigasiReaderShutdownWindow
	}
	done := make(chan struct{})
	go func() {
		readers.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		timeoutErr := fmt.Errorf("timed out waiting %s for jigasi speech readers", timeout)
		for _, session := range sessions {
			closeCtx, cancel := context.WithTimeout(context.Background(), timeout)
			_ = session.stream.Close(closeCtx)
			cancel()
		}
		select {
		case <-done:
			return timeoutErr
		case <-time.After(timeout):
			return fmt.Errorf("%w; readers did not stop after streams were closed", timeoutErr)
		}
	}
}

func (s *jigasiWhisperParticipantSession) writeAudio(ctx context.Context, audio []byte) error {
	if len(audio) == 0 {
		return nil
	}
	if s.frameBytes <= 0 {
		return errors.New("speech audio frame size is not configured")
	}
	s.pending = append(s.pending, audio...)
	for len(s.pending) >= s.frameBytes {
		if err := s.stream.WriteAudio(ctx, s.pending[:s.frameBytes]); err != nil {
			return err
		}
		s.pending = s.pending[s.frameBytes:]
	}
	return nil
}

func (s *jigasiWhisperParticipantSession) flushAudio(ctx context.Context) error {
	if len(s.pending) == 0 {
		return nil
	}
	err := s.stream.WriteAudio(ctx, s.pending)
	s.pending = nil
	return err
}

func parseJigasiWhisperAudioPacket(payload []byte) (*jigasiWhisperAudioPacket, error) {
	if len(payload) <= jigasiWhisperHeaderBytes {
		return nil, errors.New("invalid jigasi audio packet")
	}
	header := string(bytes.TrimRight(payload[:jigasiWhisperHeaderBytes], "\x00"))
	participantID, language, found := strings.Cut(header, "|")
	participantID = strings.TrimSpace(participantID)
	language = strings.TrimSpace(language)
	if !found || participantID == "" {
		return nil, errors.New("jigasi audio packet is missing participant metadata")
	}
	if language == "" {
		language = "zh-CN"
	}
	audio := make([]byte, len(payload)-jigasiWhisperHeaderBytes)
	copy(audio, payload[jigasiWhisperHeaderBytes:])
	return &jigasiWhisperAudioPacket{ParticipantID: participantID, Language: language, Audio: audio}, nil
}

func marshalJigasiWhisperResult(participantID string, event *SpeechTranscriptionEvent) ([]byte, error) {
	resultType := "partial"
	if event.IsFinal {
		resultType = "final"
	}
	stability := math.Max(0, math.Min(1, event.Confidence))
	return json.Marshal(jigasiWhisperResult{
		Type:          resultType,
		ParticipantID: participantID,
		Text:          event.Text,
		Variance:      json.RawMessage(strconv.FormatFloat(stability, 'f', 6, 64)),
	})
}
