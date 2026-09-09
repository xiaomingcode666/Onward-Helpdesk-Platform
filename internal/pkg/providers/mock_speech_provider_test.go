package providers

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"remotehelpdesk/internal/pkg/config"
)

func TestMockSpeechProviderEmitsPartialAndFinalSegments(t *testing.T) {
	provider := NewMockSpeechTranscriptionProvider(config.MockSpeechConfig{SegmentDurationMS: 500})
	stream, err := provider.Open(context.Background(), SpeechTranscriptionOptions{Language: "zh-CN"})
	if err != nil {
		t.Fatalf("open mock stream: %v", err)
	}

	bytesPerSecond, err := DefaultSpeechAudioFormat().BytesPerSecond()
	if err != nil {
		t.Fatalf("default speech format: %v", err)
	}
	halfSegment := make([]byte, bytesPerSecond/4)
	if err := stream.WriteAudio(context.Background(), halfSegment); err != nil {
		t.Fatalf("write first half: %v", err)
	}
	partial, err := stream.ReadEvent(context.Background())
	if err != nil {
		t.Fatalf("read partial: %v", err)
	}
	if partial.IsFinal || partial.ProviderEventID != "mock-000001" || !strings.HasSuffix(partial.Text, "...") {
		t.Fatalf("partial event = %#v", partial)
	}

	if err := stream.WriteAudio(context.Background(), halfSegment); err != nil {
		t.Fatalf("write second half: %v", err)
	}
	final, err := stream.ReadEvent(context.Background())
	if err != nil {
		t.Fatalf("read final: %v", err)
	}
	if !final.IsFinal || final.ProviderEventID != partial.ProviderEventID || final.EndedAtMS != 500 {
		t.Fatalf("final event = %#v", final)
	}
	if !strings.Contains(final.RawJSON, `"mock":true`) {
		t.Fatalf("raw json = %q", final.RawJSON)
	}

	if err := stream.EndAudio(context.Background()); err != nil {
		t.Fatalf("end mock audio: %v", err)
	}
	if _, err := stream.ReadEvent(context.Background()); err != io.EOF {
		t.Fatalf("read after end error = %v, want EOF", err)
	}
}

func TestMockSpeechProviderCloseUnblocksBackpressure(t *testing.T) {
	provider := NewMockSpeechTranscriptionProvider(config.MockSpeechConfig{SegmentDurationMS: 500})
	stream, err := provider.Open(context.Background(), SpeechTranscriptionOptions{})
	if err != nil {
		t.Fatalf("open mock stream: %v", err)
	}
	bytesPerSecond, err := DefaultSpeechAudioFormat().BytesPerSecond()
	if err != nil {
		t.Fatalf("default speech format: %v", err)
	}
	writeDone := make(chan error, 1)
	go func() {
		writeDone <- stream.WriteAudio(context.Background(), make([]byte, bytesPerSecond*20))
	}()

	select {
	case err := <-writeDone:
		t.Fatalf("large write unexpectedly completed before close: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	if err := stream.Close(context.Background()); err != nil {
		t.Fatalf("close mock stream: %v", err)
	}
	select {
	case err := <-writeDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("write error = %v, want context canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("mock write remained blocked after close")
	}
}
