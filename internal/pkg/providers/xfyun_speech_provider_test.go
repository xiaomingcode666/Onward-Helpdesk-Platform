package providers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/pkg/config"

	"github.com/gorilla/websocket"
)

func TestXfyunRTASRAuthURL(t *testing.T) {
	client := NewXfyunRTASRClient(&config.SpeechConfig{
		Provider: "xfyun",
		Xfyun: config.XfyunSpeechConfig{
			AppID:    "595f23df",
			APIKey:   "d9f4aa7ea6d94faca62cd88a28fd5234",
			Endpoint: "wss://rtasr.xfyun.cn/v1/ws?lang=cn",
		},
	})
	signed, err := client.authURL(time.Unix(1_512_041_814, 0), SpeechTranscriptionOptions{
		Language: "zh-CN",
		Domain:   "tech",
	})
	if err != nil {
		t.Fatalf("authURL() error = %v", err)
	}
	parsed, err := url.Parse(signed)
	if err != nil {
		t.Fatalf("url.Parse() error = %v", err)
	}
	if parsed.Query().Get("appid") != "595f23df" || parsed.Query().Get("ts") != "1512041814" {
		t.Fatalf("auth query = %q", parsed.RawQuery)
	}
	if parsed.Query().Get("signa") != "IrrzsJeOFk1NGfJHW6SkHUoN9CU=" {
		t.Fatalf("signa = %q", parsed.Query().Get("signa"))
	}
	if parsed.Query().Get("lang") != "cn" {
		t.Fatalf("normalized language = %q", parsed.RawQuery)
	}
	if parsed.Query().Get("pd") != "tech" {
		t.Fatalf("domain = %q", parsed.RawQuery)
	}
}

func TestXfyunRTASRLiveHandshake(t *testing.T) {
	if os.Getenv("XFYUN_RTASR_LIVE") != "1" {
		t.Skip("set XFYUN_RTASR_LIVE=1 to verify live credentials")
	}

	client := NewXfyunRTASRClient(&config.SpeechConfig{
		Provider: "xfyun",
		Xfyun: config.XfyunSpeechConfig{
			AppID:    os.Getenv("XFYUN_RTASR_APP_ID"),
			APIKey:   os.Getenv("XFYUN_RTASR_API_KEY"),
			Endpoint: os.Getenv("XFYUN_RTASR_ENDPOINT"),
			Domain:   os.Getenv("XFYUN_RTASR_DOMAIN"),
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	stream, err := client.Open(ctx, SpeechTranscriptionOptions{Language: "zh-CN"})
	if err != nil {
		t.Fatalf("open live stream: %v", err)
	}
	if err := stream.Close(context.Background()); err != nil {
		t.Fatalf("close live stream: %v", err)
	}
}

func TestXfyunRTASRLiveRecognition(t *testing.T) {
	if os.Getenv("XFYUN_RTASR_LIVE") != "1" {
		t.Skip("set XFYUN_RTASR_LIVE=1 to verify live recognition")
	}
	audioPath := strings.TrimSpace(os.Getenv("XFYUN_RTASR_AUDIO_FILE"))
	if audioPath == "" {
		t.Skip("set XFYUN_RTASR_AUDIO_FILE to a pcm_s16le 16000 Hz mono file")
	}
	audio, err := os.ReadFile(audioPath)
	if err != nil {
		t.Fatalf("read live audio: %v", err)
	}
	format := DefaultSpeechAudioFormat()
	frameBytes, err := format.FrameBytes()
	if err != nil {
		t.Fatalf("resolve frame size: %v", err)
	}

	client := NewXfyunRTASRClient(&config.SpeechConfig{
		Provider: "xfyun",
		Xfyun: config.XfyunSpeechConfig{
			AppID:    os.Getenv("XFYUN_RTASR_APP_ID"),
			APIKey:   os.Getenv("XFYUN_RTASR_API_KEY"),
			Endpoint: os.Getenv("XFYUN_RTASR_ENDPOINT"),
			Domain:   os.Getenv("XFYUN_RTASR_DOMAIN"),
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	stream, err := client.Open(ctx, SpeechTranscriptionOptions{Language: "zh-CN", AudioFormat: format})
	if err != nil {
		t.Fatalf("open live stream: %v", err)
	}
	defer stream.Close(context.Background())

	events := make(chan *SpeechTranscriptionEvent, 8)
	readErrors := make(chan error, 1)
	go func() {
		for {
			event, readErr := stream.ReadEvent(ctx)
			if readErr != nil {
				readErrors <- readErr
				return
			}
			if event != nil {
				events <- event
			}
		}
	}()

	for offset := 0; offset < len(audio); offset += frameBytes {
		end := min(offset+frameBytes, len(audio))
		if err := stream.WriteAudio(ctx, audio[offset:end]); err != nil {
			t.Fatalf("write live audio: %v", err)
		}
	}
	if err := stream.EndAudio(ctx); err != nil {
		t.Fatalf("end live audio: %v", err)
	}

	var transcript strings.Builder
	for {
		select {
		case event := <-events:
			transcript.WriteString(event.Text)
			if event.IsFinal {
				if strings.TrimSpace(transcript.String()) == "" {
					t.Fatal("live recognition returned an empty final transcript")
				}
				t.Logf("live transcript: %s", transcript.String())
				return
			}
		case readErr := <-readErrors:
			if strings.TrimSpace(transcript.String()) == "" {
				t.Fatalf("read live transcript: %v", readErr)
			}
			t.Logf("live transcript before close: %s", transcript.String())
			return
		case <-ctx.Done():
			t.Fatalf("wait for live transcript: %v", ctx.Err())
		}
	}
}

func TestNormalizeXfyunLanguage(t *testing.T) {
	tests := map[string]string{
		"zh-CN": "cn",
		"cn":    "cn",
		"en-US": "en",
		"fr-FR": "fr",
		"":      "",
	}
	for input, want := range tests {
		if got := normalizeXfyunLanguage(input); got != want {
			t.Fatalf("normalizeXfyunLanguage(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestParseXfyunRTASRResult(t *testing.T) {
	payload := []byte(`{
  "action": "result",
  "code": "0",
  "data": "{\"seg_id\":\"7\",\"cn\":{\"st\":{\"bg\":\"120\",\"ed\":\"680\",\"type\":\"0\",\"rt\":[{\"ws\":[{\"cw\":[{\"w\":\"设备\",\"sc\":\"0.9\"}]},{\"cw\":[{\"w\":\"已启动\",\"sc\":0.8}]}]}]}}}"
}`)
	event, err := parseXfyunRTASREvent(payload)
	if err != nil {
		t.Fatalf("parseXfyunRTASREvent() error = %v", err)
	}
	if event == nil || event.Text != "设备已启动" || !event.IsFinal {
		t.Fatalf("event = %#v", event)
	}
	if event.ProviderEventID != "7" || event.StartedAtMS != 120 || event.EndedAtMS != 680 {
		t.Fatalf("event timing/id = %#v", event)
	}
	if event.Confidence < 0.84 || event.Confidence > 0.86 {
		t.Fatalf("event confidence = %f", event.Confidence)
	}
}

func TestParseXfyunRTASRErrorKeepsCodeAndSID(t *testing.T) {
	_, err := parseXfyunRTASREvent([]byte(`{"action":"error","code":"10110","desc":"no license","sid":"rta-test"}`))
	var providerErr *XfyunRTASRError
	if !errors.As(err, &providerErr) {
		t.Fatalf("error = %v, want XfyunRTASRError", err)
	}
	if providerErr.Code != "10110" || providerErr.SID != "rta-test" {
		t.Fatalf("provider error = %#v", providerErr)
	}
}

func TestXfyunRTASREndAudioUsesBinaryFrame(t *testing.T) {
	received := make(chan struct {
		messageType int
		payload     string
	}, 1)
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		conn, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"action":"started","code":"0","sid":"test"}`)); err != nil {
			return
		}
		messageType, payload, err := conn.ReadMessage()
		if err == nil {
			received <- struct {
				messageType int
				payload     string
			}{messageType: messageType, payload: string(payload)}
		}
	}))
	defer server.Close()

	client := NewXfyunRTASRClient(&config.SpeechConfig{
		Provider: "xfyun",
		Xfyun: config.XfyunSpeechConfig{
			AppID: "test-app", APIKey: "test-key",
			Endpoint: "ws" + strings.TrimPrefix(server.URL, "http"),
		},
	})
	stream, err := client.Open(context.Background(), SpeechTranscriptionOptions{})
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	defer stream.Close(context.Background())
	if err := stream.EndAudio(context.Background()); err != nil {
		t.Fatalf("end audio: %v", err)
	}

	select {
	case message := <-received:
		if message.messageType != websocket.BinaryMessage || message.payload != `{"end": true}` {
			t.Fatalf("end frame = type %d payload %q", message.messageType, message.payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for end frame")
	}
}

func TestXfyunRTASROpenRejectsStartupError(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		conn, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"action":"error","code":"10110","desc":"no license","sid":"startup-test"}`))
	}))
	defer server.Close()

	client := NewXfyunRTASRClient(&config.SpeechConfig{
		Provider: "xfyun",
		Xfyun: config.XfyunSpeechConfig{
			AppID: "test-app", APIKey: "test-key",
			Endpoint: "ws" + strings.TrimPrefix(server.URL, "http"),
		},
	})
	_, err := client.Open(context.Background(), SpeechTranscriptionOptions{})
	var providerErr *XfyunRTASRError
	if !errors.As(err, &providerErr) || providerErr.Code != "10110" || providerErr.SID != "startup-test" {
		t.Fatalf("Open() error = %#v, want startup XfyunRTASRError", err)
	}
}

func TestXfyunRTASRDisabledWithoutServerCredentials(t *testing.T) {
	client := NewXfyunRTASRClient(&config.SpeechConfig{Provider: "disabled"})
	if client.Configured() {
		t.Fatal("Configured() = true without credentials")
	}
}

func TestXfyunRTASRRejectsUnsupportedAudioFormat(t *testing.T) {
	err := validateXfyunAudioFormat(SpeechAudioFormat{
		Encoding: SpeechAudioEncodingPCM16LE, SampleRateHz: 48_000,
		Channels: 1, BitsPerSample: 16, FrameDuration: 40 * time.Millisecond,
	})
	if err == nil || !strings.Contains(err.Error(), "16000 Hz") {
		t.Fatalf("validateXfyunAudioFormat() error = %v", err)
	}
}
