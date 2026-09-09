package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type recordingSpeechStream struct {
	writes [][]byte
}

func (s *recordingSpeechStream) WriteAudio(_ context.Context, data []byte) error {
	s.writes = append(s.writes, append([]byte(nil), data...))
	return nil
}

func (s *recordingSpeechStream) EndAudio(context.Context) error { return nil }
func (s *recordingSpeechStream) ReadEvent(context.Context) (*SpeechTranscriptionEvent, error) {
	return nil, nil
}
func (s *recordingSpeechStream) Close(context.Context) error { return nil }

func TestParseJigasiWhisperAudioPacket(t *testing.T) {
	header := make([]byte, jigasiWhisperHeaderBytes)
	copy(header, []byte("participant-7|zh-CN"))
	audio := bytes.Repeat([]byte{0x01, 0x02}, 320)
	packet, err := parseJigasiWhisperAudioPacket(append(header, audio...))
	if err != nil {
		t.Fatalf("parse packet: %v", err)
	}
	if packet.ParticipantID != "participant-7" || packet.Language != "zh-CN" {
		t.Fatalf("metadata = %#v", packet)
	}
	if !bytes.Equal(packet.Audio, audio) {
		t.Fatal("audio payload changed")
	}
}

func TestSpeechProviderIdleTimeoutClassification(t *testing.T) {
	if !isSpeechProviderIdleTimeout(&XfyunRTASRError{Code: "10700", Description: "engine error|37005:Client idle timeout"}) {
		t.Fatal("Xfyun idle timeout was not classified as recoverable")
	}
	if isSpeechProviderIdleTimeout(errors.New("authentication failed")) {
		t.Fatal("non-idle provider failure was classified as idle")
	}
}

func TestJigasiParticipantSessionBatchesFortyMilliseconds(t *testing.T) {
	stream := &recordingSpeechStream{}
	frameBytes, err := DefaultSpeechAudioFormat().FrameBytes()
	if err != nil {
		t.Fatalf("default frame bytes: %v", err)
	}
	session := &jigasiWhisperParticipantSession{stream: stream, frameBytes: frameBytes}
	packet := bytes.Repeat([]byte{0x11}, 640)
	if err := session.writeAudio(context.Background(), packet); err != nil {
		t.Fatalf("first packet: %v", err)
	}
	if len(stream.writes) != 0 {
		t.Fatalf("writes after 20ms = %d", len(stream.writes))
	}
	if err := session.writeAudio(context.Background(), packet); err != nil {
		t.Fatalf("second packet: %v", err)
	}
	if len(stream.writes) != 1 || len(stream.writes[0]) != frameBytes {
		t.Fatalf("writes = %#v", stream.writes)
	}
}

type blockingSpeechStream struct {
	closed chan struct{}
	once   sync.Once
}

func (s *blockingSpeechStream) WriteAudio(context.Context, []byte) error { return nil }
func (s *blockingSpeechStream) EndAudio(context.Context) error           { return nil }
func (s *blockingSpeechStream) ReadEvent(context.Context) (*SpeechTranscriptionEvent, error) {
	<-s.closed
	return nil, io.EOF
}
func (s *blockingSpeechStream) Close(context.Context) error {
	s.once.Do(func() { close(s.closed) })
	return nil
}

func TestWaitForJigasiReadersHasBoundedShutdown(t *testing.T) {
	stream := &blockingSpeechStream{closed: make(chan struct{})}
	sessions := map[string]*jigasiWhisperParticipantSession{
		"participant-7": {stream: stream},
	}
	var readers sync.WaitGroup
	readers.Add(1)
	go func() {
		defer readers.Done()
		_, _ = stream.ReadEvent(context.Background())
	}()

	startedAt := time.Now()
	err := waitForJigasiReaders(&readers, sessions, 10*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "timed out waiting") {
		t.Fatalf("waitForJigasiReaders() error = %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed > 200*time.Millisecond {
		t.Fatalf("bounded shutdown took %s", elapsed)
	}
	select {
	case <-stream.closed:
	default:
		t.Fatal("stream was not closed after reader timeout")
	}
}

func TestPruneEndedJigasiSessionsReleasesStream(t *testing.T) {
	stream := &blockingSpeechStream{closed: make(chan struct{})}
	session := &jigasiWhisperParticipantSession{stream: stream}
	session.ended.Store(true)
	sessions := map[string]*jigasiWhisperParticipantSession{"participant-7": session}

	pruneEndedJigasiSessions(context.Background(), sessions)
	if len(sessions) != 0 {
		t.Fatalf("sessions after prune = %d", len(sessions))
	}
	select {
	case <-stream.closed:
	default:
		t.Fatal("ended participant stream was not closed")
	}
}

type capturingSpeechProvider struct {
	provider SpeechTranscriptionProvider
	options  chan SpeechTranscriptionOptions
}

type finalSpeechProvider struct{}

func (finalSpeechProvider) Name() string     { return "test_final_speech" }
func (finalSpeechProvider) Configured() bool { return true }
func (finalSpeechProvider) Open(context.Context, SpeechTranscriptionOptions) (SpeechTranscriptionStream, error) {
	return &finalSpeechStream{events: make(chan *SpeechTranscriptionEvent, 1)}, nil
}

type finalSpeechStream struct {
	events    chan *SpeechTranscriptionEvent
	emitOnce  sync.Once
	closeOnce sync.Once
}

func (s *finalSpeechStream) WriteAudio(context.Context, []byte) error {
	s.emitOnce.Do(func() {
		s.events <- &SpeechTranscriptionEvent{
			ProviderEventID: "test-final-1",
			Text:            "测试语音转写结果",
			IsFinal:         true,
			EndedAtMS:       1_000,
		}
	})
	return nil
}

func (s *finalSpeechStream) EndAudio(context.Context) error {
	s.closeOnce.Do(func() { close(s.events) })
	return nil
}

func (s *finalSpeechStream) ReadEvent(ctx context.Context) (*SpeechTranscriptionEvent, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case event, ok := <-s.events:
		if !ok {
			return nil, io.EOF
		}
		return event, nil
	}
}

func (s *finalSpeechStream) Close(context.Context) error {
	s.closeOnce.Do(func() { close(s.events) })
	return nil
}

func (p *capturingSpeechProvider) Name() string     { return p.provider.Name() }
func (p *capturingSpeechProvider) Configured() bool { return p.provider.Configured() }
func (p *capturingSpeechProvider) Open(ctx context.Context, options SpeechTranscriptionOptions) (SpeechTranscriptionStream, error) {
	p.options <- options
	return p.provider.Open(ctx, options)
}

func TestJigasiWhisperGatewayServeReturnsFinalAndNormalClose(t *testing.T) {
	provider := &capturingSpeechProvider{
		provider: finalSpeechProvider{},
		options:  make(chan SpeechTranscriptionOptions, 1),
	}
	gateway := NewJigasiWhisperGateway(provider, 2)
	serveErr := make(chan error, 1)
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		conn, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			serveErr <- err
			return
		}
		serveErr <- gateway.Serve(request.Context(), conn)
	}))
	defer server.Close()

	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial gateway: %v", err)
	}
	defer client.Close()
	if err := client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set client deadline: %v", err)
	}
	bytesPerSecond, err := DefaultSpeechAudioFormat().BytesPerSecond()
	if err != nil {
		t.Fatalf("default audio format: %v", err)
	}
	header := make([]byte, jigasiWhisperHeaderBytes)
	copy(header, "participant-7|zh-CN")
	packet := append(header, make([]byte, bytesPerSecond)...)
	if err := client.WriteMessage(websocket.BinaryMessage, packet); err != nil {
		t.Fatalf("write audio packet: %v", err)
	}

	finalSeen := false
	for !finalSeen {
		_, payload, err := client.ReadMessage()
		if err != nil {
			t.Fatalf("read transcription result: %v", err)
		}
		var result jigasiWhisperResult
		if err := json.Unmarshal(payload, &result); err != nil {
			t.Fatalf("decode transcription result: %v", err)
		}
		finalSeen = result.Type == "final" && strings.TrimSpace(result.Text) != ""
	}
	if err := client.WriteMessage(websocket.BinaryMessage, []byte{0}); err != nil {
		t.Fatalf("write end marker: %v", err)
	}
	for err == nil {
		_, _, err = client.ReadMessage()
	}
	var closeErr *websocket.CloseError
	if !errors.As(err, &closeErr) || closeErr.Code != websocket.CloseNormalClosure {
		t.Fatalf("gateway close error = %v", err)
	}

	select {
	case options := <-provider.options:
		if options.Language != "zh-CN" || options.AudioFormat.Normalized() != DefaultSpeechAudioFormat() {
			t.Fatalf("provider options = %#v", options)
		}
	case <-time.After(time.Second):
		t.Fatal("provider did not receive transcription options")
	}
	select {
	case err := <-serveErr:
		if err != nil {
			t.Fatalf("gateway serve error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("gateway did not stop after end marker")
	}
}

func TestMarshalJigasiWhisperResultUsesFloatingVariance(t *testing.T) {
	payload, err := marshalJigasiWhisperResult("participant-7", &SpeechTranscriptionEvent{
		Text: "设备已启动", IsFinal: true, Confidence: 0,
	})
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if !bytes.Contains(payload, []byte(`"variance":0.000000`)) {
		t.Fatalf("variance must decode as a Java Double: %s", payload)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if decoded["type"] != "final" || decoded["participant_id"] != "participant-7" {
		t.Fatalf("result = %#v", decoded)
	}
}
