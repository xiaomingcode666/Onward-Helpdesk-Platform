package runtime

import (
	"context"
	"sync"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/aistream"
	"remotehelpdesk/internal/pkg/enums"
	svc "remotehelpdesk/internal/services"
)

func TestReplyStreamFlushesPendingTextAndClosesOnCancellation(t *testing.T) {
	conversation := models.Conversation{ID: 42, TenantID: 7, Status: enums.IMConversationStatusAIServing}
	frames := make(chan svc.AIMessageStreamPayload, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx, closeStream := newReplyStream(ctx, conversation, svc.AIMessageStreamPayload{ConversationID: 42, SourceMessageID: 10, StreamID: "test-stream", ClientMsgID: "ai_reply_10"},
		func() *models.Conversation { return &conversation },
		func(_ *models.Conversation, frame svc.AIMessageStreamPayload) { frames <- frame })
	defer closeStream()
	aistream.Emit(ctx, "Hello")
	first := readReplyStreamFrame(t, frames)
	if first.Content != "Hello" || first.Done {
		t.Fatalf("first frame: %+v", first)
	}
	aistream.Emit(ctx, "Hello world")
	second := readReplyStreamFrame(t, frames)
	if second.Content != "Hello world" || second.Sequence <= first.Sequence {
		t.Fatalf("pending text not flushed: %+v", second)
	}
	cancel()
	closeStream()
	final := readReplyStreamFrame(t, frames)
	if !final.Done || final.Content != "" || final.Sequence <= second.Sequence {
		t.Fatalf("cancellation left a draft: %+v", final)
	}
	aistream.Emit(ctx, "must not appear")
	closeStream()
	select {
	case frame := <-frames:
		t.Fatalf("closed stream emitted again: %+v", frame)
	default:
	}
}

func TestReplyStreamStopsAtHumanTakeover(t *testing.T) {
	var mu sync.Mutex
	conversation := models.Conversation{ID: 42, TenantID: 7, Status: enums.IMConversationStatusAIServing}
	current := conversation
	frames := make(chan svc.AIMessageStreamPayload, 8)
	ctx, closeStream := newReplyStream(context.Background(), conversation, svc.AIMessageStreamPayload{ConversationID: 42, SourceMessageID: 10, StreamID: "test-stream"},
		func() *models.Conversation { mu.Lock(); defer mu.Unlock(); snapshot := current; return &snapshot },
		func(_ *models.Conversation, frame svc.AIMessageStreamPayload) { frames <- frame })
	defer closeStream()
	aistream.Emit(ctx, "Hello")
	_ = readReplyStreamFrame(t, frames)
	mu.Lock()
	current.CurrentAssigneeID = 99
	mu.Unlock()
	aistream.Emit(ctx, "must not appear after takeover")
	final := readReplyStreamFrame(t, frames)
	if !final.Done || final.Content != "" {
		t.Fatalf("reply exposed after takeover: %+v", final)
	}
	closeStream()
	select {
	case frame := <-frames:
		t.Fatalf("takeover sent duplicate terminal frame: %+v", frame)
	default:
	}
}

func TestReplyStreamClosedBeforeFirstChunkCannotRestart(t *testing.T) {
	conversation := models.Conversation{ID: 42, TenantID: 7}
	ctx, closeStream := newReplyStream(context.Background(), conversation, svc.AIMessageStreamPayload{},
		func() *models.Conversation { t.Fatal("closed stream accessed conversation"); return nil },
		func(_ *models.Conversation, _ svc.AIMessageStreamPayload) {
			t.Fatal("closed stream published late text")
		})
	closeStream()
	aistream.Emit(ctx, "late model output")
}

func readReplyStreamFrame(t *testing.T, frames <-chan svc.AIMessageStreamPayload) svc.AIMessageStreamPayload {
	t.Helper()
	select {
	case frame := <-frames:
		return frame
	case <-time.After(2 * time.Second):
		t.Fatal("stream frame not received")
		return svc.AIMessageStreamPayload{}
	}
}
