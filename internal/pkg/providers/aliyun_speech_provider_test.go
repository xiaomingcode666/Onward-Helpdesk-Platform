package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"remotehelpdesk/internal/pkg/config"

	"github.com/gorilla/websocket"
)

func TestAliyunASRStreamRoundTrip(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		conn, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		messageType, payload, err := conn.ReadMessage()
		if err != nil || messageType != websocket.TextMessage {
			t.Errorf("read run-task = type %d err %v", messageType, err)
			return
		}
		var task aliyunClientEvent
		if err := json.Unmarshal(payload, &task); err != nil {
			t.Errorf("decode run-task: %v", err)
			return
		}
		if task.Header["action"] != "run-task" || task.Header["streaming"] != "duplex" || task.Payload["model"] != "test-model" {
			t.Errorf("run-task = %#v", task)
			return
		}
		if err := conn.WriteJSON(aliyunServerEvent{Header: aliyunEventHeader{Event: "task-started"}}); err != nil {
			return
		}

		messageType, payload, err = conn.ReadMessage()
		if err != nil || messageType != websocket.BinaryMessage || string(payload) != "pcm" {
			t.Errorf("read audio = type %d payload %q err %v", messageType, payload, err)
			return
		}
		messageType, payload, err = conn.ReadMessage()
		if err != nil || messageType != websocket.TextMessage {
			t.Errorf("read finish-task = type %d err %v", messageType, err)
			return
		}
		var finish aliyunClientEvent
		if err := json.Unmarshal(payload, &finish); err != nil {
			t.Errorf("decode finish-task: %v", err)
			return
		}
		if finish.Header["action"] != "finish-task" {
			t.Errorf("finish-task = %#v", finish)
			return
		}
		result := aliyunServerEvent{Header: aliyunEventHeader{Event: "result-generated"}}
		result.Payload.Output.Sentence = aliyunSentence{Text: "设备已恢复", SentenceEnd: true, BeginTime: 0, EndTime: 800}
		_ = conn.WriteJSON(result)
		_ = conn.WriteJSON(aliyunServerEvent{Header: aliyunEventHeader{Event: "task-finished"}})
	}))
	defer server.Close()

	client := NewAliyunASRClient(&config.SpeechConfig{
		Provider: "aliyun",
		Aliyun: config.AliyunSpeechConfig{
			APIKey:   "test-key",
			Endpoint: server.URL + "/asr",
			Model:    "test-model",
		},
	})
	stream, err := client.Open(context.Background(), SpeechTranscriptionOptions{AudioFormat: DefaultSpeechAudioFormat()})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer stream.Close(context.Background())
	if err := stream.WriteAudio(context.Background(), []byte("pcm")); err != nil {
		t.Fatalf("WriteAudio() error = %v", err)
	}
	if err := stream.EndAudio(context.Background()); err != nil {
		t.Fatalf("EndAudio() error = %v", err)
	}
	event, err := stream.ReadEvent(context.Background())
	if err != nil {
		t.Fatalf("ReadEvent() error = %v", err)
	}
	if event.Text != "设备已恢复" || !event.IsFinal || event.EndedAtMS != 800 {
		t.Fatalf("event = %#v", event)
	}
	if _, err := stream.ReadEvent(context.Background()); err != io.EOF {
		t.Fatalf("ReadEvent() after task-finished error = %v, want EOF", err)
	}
}

func TestAliyunASREndpointNormalizesCompatibleBaseURL(t *testing.T) {
	endpoint, err := aliyunASREndpoint("https://workspace.example.com/compatible-mode/v1", "test-model")
	if err != nil {
		t.Fatalf("aliyunASREndpoint() error = %v", err)
	}
	if endpoint != "wss://workspace.example.com/api-ws/v1/inference?model=test-model" {
		t.Fatalf("aliyunASREndpoint() = %q", endpoint)
	}
}
