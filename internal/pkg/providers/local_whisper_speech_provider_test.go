package providers

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"remotehelpdesk/internal/pkg/config"
)

func openLocalWhisperStream(t *testing.T, endpoint string) SpeechTranscriptionStream {
	t.Helper()
	provider := NewLocalWhisperSpeechProvider(config.LocalWhisperSpeechConfig{Endpoint: endpoint, TimeoutSeconds: 5})
	require.True(t, provider.Configured())
	require.Equal(t, "local_whisper", provider.Name())
	stream, err := provider.Open(context.Background(), SpeechTranscriptionOptions{Language: "zh-CN", AudioFormat: DefaultSpeechAudioFormat()})
	require.NoError(t, err)
	t.Cleanup(func() { _ = stream.Close(context.Background()) })
	return stream
}

func TestLocalWhisperTranscribesBufferedAudio(t *testing.T) {
	pcm := bytes.Repeat([]byte{1, 0}, 16_000) // 1 second of 16 kHz mono PCM16
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/asr/transcribe", r.URL.Path)
		require.Equal(t, "zh-CN", r.URL.Query().Get("language"))
		require.Equal(t, "audio/wav", r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, "RIFF", string(body[0:4]))
		require.Equal(t, "WAVE", string(body[8:12]))
		require.Equal(t, uint16(1), binary.LittleEndian.Uint16(body[22:24]))
		require.Equal(t, uint32(16_000), binary.LittleEndian.Uint32(body[24:28]))
		require.Equal(t, uint16(16), binary.LittleEndian.Uint16(body[34:36]))
		require.Equal(t, uint32(len(pcm)), binary.LittleEndian.Uint32(body[40:44]))
		require.Equal(t, pcm, body[44:])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":" 你好 ","language":"zh-CN","durationSeconds":1}`))
	}))
	defer server.Close()

	stream := openLocalWhisperStream(t, server.URL)
	ctx := context.Background()
	require.NoError(t, stream.WriteAudio(ctx, pcm[:12_000]))
	require.NoError(t, stream.WriteAudio(ctx, pcm[12_000:]))
	require.NoError(t, stream.EndAudio(ctx))

	event, err := stream.ReadEvent(ctx)
	require.NoError(t, err)
	require.True(t, event.IsFinal)
	require.Equal(t, "你好", event.Text)
	require.Equal(t, int64(1000), event.EndedAtMS)

	_, err = stream.ReadEvent(ctx)
	require.ErrorIs(t, err, io.EOF)
}

func TestLocalWhisperUnavailable(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	endpoint := "http://" + listener.Addr().String()
	require.NoError(t, listener.Close())

	stream := openLocalWhisperStream(t, endpoint)
	ctx := context.Background()
	require.NoError(t, stream.WriteAudio(ctx, []byte{1, 0, 1, 0}))
	err = stream.EndAudio(ctx)
	require.True(t, errors.Is(err, ErrLocalSpeechUnavailable), "err = %v", err)
}

func TestLocalWhisperServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "faster-whisper missing", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	stream := openLocalWhisperStream(t, server.URL)
	ctx := context.Background()
	require.NoError(t, stream.WriteAudio(ctx, []byte{1, 0, 1, 0}))
	err := stream.EndAudio(ctx)
	require.True(t, errors.Is(err, ErrLocalSpeechUnavailable), "err = %v", err)
}
