package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"remotehelpdesk/internal/models"
	"remotehelpdesk/internal/pkg/aistream"
	"remotehelpdesk/internal/pkg/enums"
)

// Exercise the actual model adapter and Eino runner against a local SSE server.
// The server cannot finish until the caller has received its first snapshot.
func TestWorkflowNodeStreamsBeforeHTTPResponseCompletes(t *testing.T) {
	t.Setenv("LLM_FALLBACK_MODEL", "")
	t.Setenv("LLM_FALLBACK_BASE_URL", "")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	release := make(chan struct{})
	var once sync.Once
	finish := func() { once.Do(func() { close(release) }) }
	defer finish()
	requests := make(chan bool, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Stream bool `json:"stream"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		requests <- body.Stream
		if !body.Stream {
			http.Error(w, "stream is required", 400)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"id\":\"reply\",\"object\":\"chat.completion.chunk\",\"model\":\"fixture\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Hello\"}}]}\n\n")
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
			return
		case <-ctx.Done():
			return
		}
		fmt.Fprint(w, "data: {\"id\":\"reply\",\"object\":\"chat.completion.chunk\",\"model\":\"fixture\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\" world\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	observed := make(chan string, 8)
	ctx = aistream.WithHandler(ctx, func(text string) { observed <- text })
	type outcome struct {
		result *RunResult
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		result, err := NewService().ExecuteWorkflowNode(ctx, WorkflowNodeInput{
			AIAgent: models.AIAgent{Name: "fixture_agent"}, UserPrompt: "Say hello",
			AIConfig: models.AIConfig{Provider: enums.AIProviderOpenAI, ModelName: "fixture", APIKey: "fixture-key", BaseURL: server.URL + "/v1"},
		})
		done <- outcome{result, err}
	}()
	select {
	case text := <-observed:
		if text != "Hello" {
			t.Fatalf("first snapshot = %q", text)
		}
	case result := <-done:
		t.Fatalf("execution finished before first snapshot: %+v, %v", result.result, result.err)
	case <-ctx.Done():
		t.Fatal("first snapshot was buffered until completion")
	}
	if !<-requests {
		t.Fatal("model request did not enable streaming")
	}
	finish()
	select {
	case result := <-done:
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.result.ReplyText != "Hello world" {
			t.Fatalf("final reply = %q", result.result.ReplyText)
		}
	case <-ctx.Done():
		t.Fatal("stream did not finish")
	}
}
