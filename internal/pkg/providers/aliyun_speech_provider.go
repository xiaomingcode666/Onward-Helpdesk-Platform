package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"remotehelpdesk/internal/pkg/config"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	defaultAliyunASREndpoint = "wss://dashscope.aliyuncs.com/api-ws/v1/inference"
	defaultAliyunASRModel    = "qwen-audio-3.0-asr-flash-streaming"
	aliyunAudioWriteInterval = 40 * time.Millisecond
	aliyunWriteTimeout       = 10 * time.Second
	aliyunStartedTimeout     = 10 * time.Second
	aliyunMaxResponseBytes   = 1 << 20
)

type AliyunASRClient struct {
	apiKey      string
	workspaceID string
	endpoint    string
	model       string
	dialer      *websocket.Dialer
}

func NewAliyunASRClient(cfg *config.SpeechConfig) *AliyunASRClient {
	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second
	client := &AliyunASRClient{
		endpoint: defaultAliyunASREndpoint,
		model:    defaultAliyunASRModel,
		dialer:   &dialer,
	}
	if cfg == nil || !strings.EqualFold(strings.TrimSpace(cfg.Provider), "aliyun") {
		return client
	}
	client.apiKey = strings.TrimSpace(cfg.Aliyun.APIKey)
	client.workspaceID = strings.TrimSpace(cfg.Aliyun.WorkspaceID)
	if endpoint := strings.TrimSpace(cfg.Aliyun.Endpoint); endpoint != "" {
		client.endpoint = endpoint
	}
	if model := strings.TrimSpace(cfg.Aliyun.Model); model != "" {
		client.model = model
	}
	return client
}

func (c *AliyunASRClient) Name() string { return "aliyun_qwen_asr" }

func (c *AliyunASRClient) Configured() bool {
	return c != nil && strings.TrimSpace(c.apiKey) != ""
}

func (c *AliyunASRClient) Open(ctx context.Context, options SpeechTranscriptionOptions) (SpeechTranscriptionStream, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateAliyunAudioFormat(options.AudioFormat); err != nil {
		return nil, err
	}
	if !c.Configured() {
		return nil, errors.New("aliyun asr is not configured")
	}
	endpoint, err := aliyunASREndpoint(c.endpoint, c.model)
	if err != nil {
		return nil, err
	}
	header := http.Header{}
	header.Set("Authorization", "Bearer "+c.apiKey)
	if c.workspaceID != "" {
		header.Set("X-DashScope-WorkSpace", c.workspaceID)
	}
	conn, response, err := c.dialer.DialContext(ctx, endpoint, header)
	if err != nil {
		if response != nil {
			_ = response.Body.Close()
			return nil, fmt.Errorf("open aliyun asr stream: HTTP %d: %w", response.StatusCode, err)
		}
		return nil, fmt.Errorf("open aliyun asr stream: %w", err)
	}
	conn.SetReadLimit(aliyunMaxResponseBytes)
	stream := &aliyunASRStream{
		ctx:    ctx,
		conn:   conn,
		taskID: uuid.NewString(),
	}
	stream.stopContextClose = context.AfterFunc(ctx, func() { _ = stream.closeConnection() })
	if err := stream.startTask(c.model); err != nil {
		_ = stream.Close(context.Background())
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, err
	}
	return stream, nil
}

func validateAliyunAudioFormat(audioFormat SpeechAudioFormat) error {
	audioFormat = audioFormat.Normalized()
	if err := audioFormat.Validate(); err != nil {
		return err
	}
	if audioFormat.Encoding != SpeechAudioEncodingPCM16LE ||
		audioFormat.SampleRateHz != 16_000 ||
		audioFormat.Channels != 1 ||
		audioFormat.BitsPerSample != 16 {
		return errors.New("aliyun asr requires pcm_s16le 16000 Hz mono 16-bit audio")
	}
	return nil
}

func aliyunASREndpoint(rawEndpoint, model string) (string, error) {
	endpoint := strings.TrimSpace(rawEndpoint)
	if endpoint == "" {
		endpoint = defaultAliyunASREndpoint
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parse aliyun asr endpoint: %w", err)
	}
	if parsed.Host == "" {
		return "", errors.New("parse aliyun asr endpoint: host is required")
	}
	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", errors.New("aliyun asr endpoint must use WS(S) or HTTP(S)")
	}
	if parsed.Path == "" || parsed.Path == "/" || strings.Contains(parsed.Path, "/compatible-mode/") {
		parsed.Path = "/api-ws/v1/inference"
	}
	query := parsed.Query()
	query.Set("model", strings.TrimSpace(model))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

type aliyunASRStream struct {
	ctx              context.Context
	conn             *websocket.Conn
	taskID           string
	writeMu          sync.Mutex
	lastAudioWrite   time.Time
	sequence         uint64
	finished         bool
	finishOnce       sync.Once
	finishErr        error
	closeOnce        sync.Once
	closeErr         error
	stopContextClose func() bool
}

func (s *aliyunASRStream) startTask(model string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := s.conn.SetWriteDeadline(time.Now().Add(aliyunWriteTimeout)); err != nil {
		return err
	}
	if err := s.conn.WriteJSON(aliyunClientEvent{
		Header: map[string]string{
			"action":    "run-task",
			"task_id":   s.taskID,
			"streaming": "duplex",
		},
		Payload: map[string]any{
			"task_group": "audio",
			"task":       "asr",
			"function":   "recognition",
			"model":      model,
			"parameters": map[string]any{
				"format":                       "pcm",
				"sample_rate":                  16_000,
				"semantic_punctuation_enabled": true,
				"heartbeat":                    true,
			},
			"input": map[string]any{},
		},
	}); err != nil {
		return fmt.Errorf("send aliyun asr run-task: %w", err)
	}
	if err := s.waitForStarted(); err != nil {
		return err
	}
	return nil
}

func (s *aliyunASRStream) waitForStarted() error {
	if err := s.conn.SetReadDeadline(time.Now().Add(aliyunStartedTimeout)); err != nil {
		return err
	}
	defer func() { _ = s.conn.SetReadDeadline(time.Time{}) }()
	for {
		event, err := s.readServerEvent()
		if err != nil {
			return fmt.Errorf("wait for aliyun asr task-started event: %w", err)
		}
		if event.Header.Event == "task-started" {
			return nil
		}
		if event.failed() {
			return event.asError()
		}
	}
}

func (s *aliyunASRStream) WriteAudio(ctx context.Context, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if !s.lastAudioWrite.IsZero() {
		wait := time.Until(s.lastAudioWrite.Add(aliyunAudioWriteInterval))
		if wait > 0 {
			timer := time.NewTimer(wait)
			defer timer.Stop()
			select {
			case <-timer.C:
			case <-ctx.Done():
				return ctx.Err()
			case <-s.ctx.Done():
				return s.ctx.Err()
			}
		}
	}
	if err := s.conn.SetWriteDeadline(speechOperationDeadline(ctx, aliyunWriteTimeout)); err != nil {
		return err
	}
	if err := s.conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return err
	}
	s.lastAudioWrite = time.Now()
	return nil
}

func (s *aliyunASRStream) EndAudio(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	s.finishOnce.Do(func() {
		s.writeMu.Lock()
		defer s.writeMu.Unlock()
		if err := s.conn.SetWriteDeadline(speechOperationDeadline(ctx, aliyunWriteTimeout)); err != nil {
			s.finishErr = err
			return
		}
		s.finishErr = s.conn.WriteJSON(aliyunClientEvent{
			Header:  map[string]string{"action": "finish-task", "task_id": s.taskID, "streaming": "duplex"},
			Payload: map[string]any{"input": map[string]any{}},
		})
	})
	return s.finishErr
}

func (s *aliyunASRStream) ReadEvent(ctx context.Context) (*SpeechTranscriptionEvent, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if s.finished {
			return nil, io.EOF
		}
		if deadline, ok := ctx.Deadline(); ok {
			if err := s.conn.SetReadDeadline(deadline); err != nil {
				return nil, err
			}
		}
		event, err := s.readServerEvent()
		if err != nil {
			return nil, err
		}
		if event.failed() {
			return nil, event.asError()
		}
		if event.Header.Event == "task-finished" {
			s.finished = true
		}
		if sentence := event.Payload.Output.Sentence; strings.TrimSpace(sentence.Text) != "" {
			s.sequence++
			return &SpeechTranscriptionEvent{
				ProviderEventID: fmt.Sprintf("%s:%d", s.taskID, s.sequence),
				Text:            strings.TrimSpace(sentence.Text),
				IsFinal:         sentence.SentenceEnd || event.Header.Event == "task-finished",
				StartedAtMS:     sentence.BeginTime,
				EndedAtMS:       sentence.EndTime,
				RawJSON:         event.rawJSON,
			}, nil
		}
		if s.finished {
			return nil, io.EOF
		}
	}
}

func (s *aliyunASRStream) readServerEvent() (aliyunServerEvent, error) {
	_, payload, err := s.conn.ReadMessage()
	if err != nil {
		return aliyunServerEvent{}, err
	}
	var event aliyunServerEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return aliyunServerEvent{}, fmt.Errorf("decode aliyun asr event: %w", err)
	}
	event.rawJSON = string(payload)
	return event, nil
}

func (s *aliyunASRStream) Close(_ context.Context) error {
	if s.stopContextClose != nil {
		s.stopContextClose()
	}
	return s.closeConnection()
}

func (s *aliyunASRStream) closeConnection() error {
	s.closeOnce.Do(func() { s.closeErr = s.conn.Close() })
	return s.closeErr
}

type aliyunClientEvent struct {
	Header  map[string]string `json:"header"`
	Payload map[string]any    `json:"payload"`
}

type aliyunServerEvent struct {
	Header  aliyunEventHeader `json:"header"`
	Payload struct {
		Output struct {
			Sentence aliyunSentence `json:"sentence"`
		} `json:"output"`
	} `json:"payload"`
	rawJSON string
}

type aliyunEventHeader struct {
	Event     string `json:"event"`
	TaskID    string `json:"task_id"`
	ErrorCode string `json:"error_code"`
	Message   string `json:"error_message"`
}

type aliyunSentence struct {
	Text        string `json:"text"`
	SentenceEnd bool   `json:"sentence_end"`
	BeginTime   int64  `json:"begin_time"`
	EndTime     int64  `json:"end_time"`
}

type AliyunASRError struct {
	Event   string
	Code    string
	Message string
	TaskID  string
}

func (e *AliyunASRError) Error() string {
	parts := []string{"aliyun asr error"}
	if e.Code != "" {
		parts = append(parts, "code="+e.Code)
	}
	if e.Message != "" {
		parts = append(parts, "message="+e.Message)
	}
	if e.TaskID != "" {
		parts = append(parts, "task_id="+e.TaskID)
	}
	return strings.Join(parts, " ")
}

func (e aliyunServerEvent) asError() error {
	return &AliyunASRError{
		Event:   e.Header.Event,
		Code:    strings.TrimSpace(e.Header.ErrorCode),
		Message: strings.TrimSpace(e.Header.Message),
		TaskID:  strings.TrimSpace(e.Header.TaskID),
	}
}

func (e aliyunServerEvent) failed() bool {
	code := strings.TrimSpace(e.Header.ErrorCode)
	return e.Header.Event == "task-failed" || e.Header.Event == "error" || (code != "" && code != "0")
}
