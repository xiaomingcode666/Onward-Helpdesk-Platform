package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	defaultModel       = "qwen-audio-3.0-asr-flash-streaming"
	defaultSampleRate  = 16_000
	chunkDuration      = 40 * time.Millisecond
	probeTimeout       = 30 * time.Second
	websocketReadLimit = 1 << 20
)

type eventHeader struct {
	Action  string `json:"action"`
	Event   string `json:"event"`
	TaskID  string `json:"task_id"`
	Message string `json:"error_message"`
	Code    string `json:"error_code"`
}

type serverEvent struct {
	Header  eventHeader `json:"header"`
	Payload struct {
		Output struct {
			Sentence struct {
				Text        string `json:"text"`
				SentenceEnd bool   `json:"sentence_end"`
				BeginTime   int64  `json:"begin_time"`
				EndTime     int64  `json:"end_time"`
			} `json:"sentence"`
		} `json:"output"`
	} `json:"payload"`
}

type clientEvent struct {
	Header  map[string]string `json:"header"`
	Payload map[string]any    `json:"payload"`
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()

	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")), "/")
	model := strings.TrimSpace(os.Getenv("ASR_MODEL"))
	if model == "" {
		model = defaultModel
	}
	sampleRate := defaultSampleRate
	if rawSampleRate := strings.TrimSpace(os.Getenv("ASR_SAMPLE_RATE")); rawSampleRate != "" {
		parsedSampleRate, parseErr := strconv.Atoi(rawSampleRate)
		if parseErr != nil || parsedSampleRate <= 0 {
			fatal("ASR_SAMPLE_RATE must be a positive integer")
		}
		sampleRate = parsedSampleRate
	}
	chunkBytes := sampleRate * 2 * int(chunkDuration) / int(time.Second)
	if apiKey == "" || baseURL == "" {
		fatal("OPENAI_API_KEY and OPENAI_BASE_URL are required")
	}

	audioPath := ""
	if len(os.Args) > 1 {
		audioPath = os.Args[1]
	}
	if audioPath == "" {
		fatal("usage: ASR_SAMPLE_RATE=16000 go run ./scripts/dev/probe-aliyun-asr.go /path/to/mono-s16le.pcm")
	}
	audio, err := os.ReadFile(filepath.Clean(audioPath))
	if err != nil {
		fatal("read PCM: %v", err)
	}
	if len(audio) == 0 {
		fatal("PCM file is empty")
	}

	endpoint, err := asrEndpoint(baseURL, model)
	if err != nil {
		fatal("build ASR endpoint: %v", err)
	}
	requestHeader := http.Header{}
	requestHeader.Set("Authorization", "Bearer "+apiKey)
	conn, response, err := (&websocket.Dialer{HandshakeTimeout: 10 * time.Second}).DialContext(ctx, endpoint, requestHeader)
	if err != nil {
		if response != nil {
			_ = response.Body.Close()
			fatal("ASR WebSocket handshake failed: HTTP %d", response.StatusCode)
		}
		fatal("ASR WebSocket handshake failed: %v", err)
	}
	defer conn.Close()
	conn.SetReadLimit(websocketReadLimit)

	taskID := randomID()
	if err := conn.WriteJSON(clientEvent{
		Header: map[string]string{"action": "run-task", "task_id": taskID, "streaming": "duplex"},
		Payload: map[string]any{
			"task_group": "audio",
			"task":       "asr",
			"function":   "recognition",
			"model":      model,
			"parameters": map[string]any{
				"format":                       "pcm",
				"sample_rate":                  sampleRate,
				"semantic_punctuation_enabled": true,
				"heartbeat":                    true,
			},
			"input": map[string]any{},
		},
	}); err != nil {
		fatal("send run-task: %v", err)
	}

	if err := waitForEvent(conn, "task-started"); err != nil {
		fatal("wait for task-started: %v", err)
	}
	fmt.Printf("ASR_TASK_STARTED model=%s audio_bytes=%d\n", model, len(audio))

	for offset := 0; offset < len(audio); offset += chunkBytes {
		end := offset + chunkBytes
		if end > len(audio) {
			end = len(audio)
		}
		if err := conn.WriteMessage(websocket.BinaryMessage, audio[offset:end]); err != nil {
			fatal("send audio at byte %d: %v", offset, err)
		}
		select {
		case <-ctx.Done():
			fatal("send audio: %v", ctx.Err())
		case <-time.After(chunkDuration):
		}
	}

	if err := conn.WriteJSON(clientEvent{
		Header:  map[string]string{"action": "finish-task", "task_id": taskID, "streaming": "duplex"},
		Payload: map[string]any{"input": map[string]any{}},
	}); err != nil {
		fatal("send finish-task: %v", err)
	}
	if err := drainUntilFinished(conn); err != nil {
		fatal("wait for final ASR result: %v", err)
	}
	fmt.Println("ASR_PROBE_OK")
}

func asrEndpoint(baseURL, model string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Host == "" {
		return "", errors.New("OPENAI_BASE_URL must be an absolute URL")
	}
	parsed.Scheme = "wss"
	parsed.Path = "/api-ws/v1/inference"
	parsed.RawQuery = ""
	query := parsed.Query()
	query.Set("model", model)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func waitForEvent(conn *websocket.Conn, expected string) error {
	for {
		event, err := readEvent(conn)
		if err != nil {
			return err
		}
		switch event.Header.Event {
		case expected:
			return nil
		case "task-failed":
			return fmt.Errorf("%s: %s", event.Header.Code, event.Header.Message)
		}
	}
}

func drainUntilFinished(conn *websocket.Conn) error {
	for {
		event, err := readEvent(conn)
		if err != nil {
			return err
		}
		text := strings.TrimSpace(event.Payload.Output.Sentence.Text)
		if text != "" {
			kind := "partial"
			if event.Payload.Output.Sentence.SentenceEnd {
				kind = "final"
			}
			fmt.Printf("ASR_RESULT type=%s begin_ms=%d end_ms=%d text=%s\n",
				kind,
				event.Payload.Output.Sentence.BeginTime,
				event.Payload.Output.Sentence.EndTime,
				text,
			)
		}
		if event.Header.Event == "task-finished" {
			return nil
		}
		if event.Header.Event == "task-failed" {
			return fmt.Errorf("%s: %s", event.Header.Code, event.Header.Message)
		}
	}
}

func readEvent(conn *websocket.Conn) (serverEvent, error) {
	_, payload, err := conn.ReadMessage()
	if err != nil {
		return serverEvent{}, err
	}
	var event serverEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return serverEvent{}, fmt.Errorf("decode server event: %w", err)
	}
	return event, nil
}

func randomID() string {
	var data [16]byte
	if _, err := io.ReadFull(rand.Reader, data[:]); err != nil {
		fatal("generate task id: %v", err)
	}
	data[6] = (data[6] & 0x0f) | 0x40
	data[8] = (data[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(data[0:4]),
		hex.EncodeToString(data[4:6]),
		hex.EncodeToString(data[6:8]),
		hex.EncodeToString(data[8:10]),
		hex.EncodeToString(data[10:16]),
	)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ASR_PROBE_FAILED: "+format+"\n", args...)
	os.Exit(1)
}
