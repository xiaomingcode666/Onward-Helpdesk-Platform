package services

import (
	"bytes"
	"encoding/binary"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"testing"

	"remotehelpdesk/internal/pkg/config"
	"remotehelpdesk/internal/pkg/errorsx"
	"remotehelpdesk/internal/pkg/providers"
)

func TestDecodePCM16WAV(t *testing.T) {
	raw := makePCM16WAV(16_000, 1)
	pcm, duration, err := decodePCM16WAV(raw)
	if err != nil {
		t.Fatalf("decodePCM16WAV() error = %v", err)
	}
	if len(pcm) != 32_000 || duration != 1 {
		t.Fatalf("decoded PCM = %d bytes, duration = %d; want 32000 bytes and 1 second", len(pcm), duration)
	}
}

func TestDecodePCM16WAVRejectsNonPCM(t *testing.T) {
	raw := makePCM16WAV(8_000, 2)
	if _, _, err := decodePCM16WAV(raw); err == nil {
		t.Fatal("decodePCM16WAV accepted an unsupported sample rate")
	}
}

func makePCM16WAV(sampleRate, seconds int) []byte {
	dataSize := sampleRate * seconds * 2
	raw := make([]byte, 44+dataSize)
	copy(raw[0:4], "RIFF")
	binary.LittleEndian.PutUint32(raw[4:8], uint32(36+dataSize))
	copy(raw[8:12], "WAVE")
	copy(raw[12:16], "fmt ")
	binary.LittleEndian.PutUint32(raw[16:20], 16)
	binary.LittleEndian.PutUint16(raw[20:22], 1)
	binary.LittleEndian.PutUint16(raw[22:24], 1)
	binary.LittleEndian.PutUint32(raw[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(raw[28:32], uint32(sampleRate*2))
	binary.LittleEndian.PutUint16(raw[32:34], 2)
	binary.LittleEndian.PutUint16(raw[34:36], 16)
	copy(raw[36:40], "data")
	binary.LittleEndian.PutUint32(raw[40:44], uint32(dataSize))
	return raw
}

func useLocalWhisperStub(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	if err := providers.InitSpeech(&config.SpeechConfig{
		Provider:     "local_whisper",
		LocalWhisper: config.LocalWhisperSpeechConfig{Endpoint: server.URL, TimeoutSeconds: 5},
	}); err != nil {
		t.Fatalf("InitSpeech() error = %v", err)
	}
	t.Cleanup(func() { _ = providers.InitSpeech(&config.SpeechConfig{}) })
}

func makeAudioFileHeader(t *testing.T, raw []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="file"; filename="voice.wav"`)
	header.Set("Content-Type", "audio/wav")
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(raw)
	_ = writer.Close()
	form, err := multipart.NewReader(&body, writer.Boundary()).ReadForm(32 << 20)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = form.RemoveAll() })
	return form.File["file"][0]
}

func requireI18nKey(t *testing.T, err error, key string) {
	t.Helper()
	var i18nErr *errorsx.I18nError
	if !errors.As(err, &i18nErr) || i18nErr.Key != key {
		t.Fatalf("error = %v, want i18n key %q", err, key)
	}
}

func TestTranscribeCustomerAudioReturnsText(t *testing.T) {
	useLocalWhisperStub(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"text":"设备无法开机"}`))
	})
	result, err := SpeechTranscriptionService.TranscribeCustomerAudio(makeAudioFileHeader(t, makePCM16WAV(16_000, 1)))
	if err != nil {
		t.Fatalf("TranscribeCustomerAudio() error = %v", err)
	}
	if result.Text != "设备无法开机" || result.Provider != "local_whisper" || result.DurationSeconds != 1 {
		t.Fatalf("result = %+v", result)
	}
}

func TestTranscribeCustomerAudioRejectsSilence(t *testing.T) {
	useLocalWhisperStub(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"text":"   "}`))
	})
	_, err := SpeechTranscriptionService.TranscribeCustomerAudio(makeAudioFileHeader(t, makePCM16WAV(16_000, 1)))
	requireI18nKey(t, err, "speech.noSpeech")
}

func TestTranscribeCustomerAudioMapsUnavailable(t *testing.T) {
	useLocalWhisperStub(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	})
	_, err := SpeechTranscriptionService.TranscribeCustomerAudio(makeAudioFileHeader(t, makePCM16WAV(16_000, 1)))
	requireI18nKey(t, err, "speech.localUnavailable")
}
