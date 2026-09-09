package providers

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"remotehelpdesk/internal/pkg/config"

	"github.com/gorilla/websocket"
)

const (
	defaultXfyunRTASREndpoint = "wss://rtasr.xfyun.cn/v1/ws"
	xfyunAudioWriteInterval   = 40 * time.Millisecond
	xfyunWriteTimeout         = 10 * time.Second
	xfyunStartedTimeout       = 10 * time.Second
	xfyunMaxResponseBytes     = 1 << 20
)

type XfyunRTASRClient struct {
	appID    string
	apiKey   string
	endpoint string
	domain   string
	dialer   *websocket.Dialer
}

func NewXfyunRTASRClient(cfg *config.SpeechConfig) *XfyunRTASRClient {
	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second
	client := &XfyunRTASRClient{endpoint: defaultXfyunRTASREndpoint, dialer: &dialer}
	if cfg == nil || !strings.EqualFold(strings.TrimSpace(cfg.Provider), "xfyun") {
		return client
	}
	client.appID = strings.TrimSpace(cfg.Xfyun.AppID)
	client.apiKey = strings.TrimSpace(cfg.Xfyun.APIKey)
	client.domain = strings.TrimSpace(cfg.Xfyun.Domain)
	if endpoint := strings.TrimSpace(cfg.Xfyun.Endpoint); endpoint != "" {
		client.endpoint = endpoint
	}
	return client
}

func (c *XfyunRTASRClient) Name() string { return "xfyun_rtasr" }

func (c *XfyunRTASRClient) Configured() bool {
	return c != nil && c.appID != "" && c.apiKey != ""
}

func (c *XfyunRTASRClient) authURL(now time.Time, options SpeechTranscriptionOptions) (string, error) {
	if !c.Configured() {
		return "", errors.New("xfyun rtasr is not configured")
	}
	parsed, err := url.Parse(c.endpoint)
	if err != nil {
		return "", fmt.Errorf("parse xfyun rtasr endpoint: %w", err)
	}
	ts := strconv.FormatInt(now.Unix(), 10)
	digest := md5.Sum([]byte(c.appID + ts))
	md5Text := hex.EncodeToString(digest[:])
	mac := hmac.New(sha1.New, []byte(c.apiKey))
	_, _ = mac.Write([]byte(md5Text))
	signa := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	query := parsed.Query()
	query.Set("appid", c.appID)
	query.Set("ts", ts)
	query.Set("signa", signa)
	if language := normalizeXfyunLanguage(options.Language); language != "" {
		query.Set("lang", language)
	}
	domain := strings.TrimSpace(options.Domain)
	if domain == "" {
		domain = c.domain
	}
	if domain != "" {
		query.Set("pd", domain)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *XfyunRTASRClient) Open(ctx context.Context, options SpeechTranscriptionOptions) (SpeechTranscriptionStream, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateXfyunAudioFormat(options.AudioFormat); err != nil {
		return nil, err
	}
	authURL, err := c.authURL(time.Now(), options)
	if err != nil {
		return nil, err
	}
	conn, response, err := c.dialer.DialContext(ctx, authURL, http.Header{})
	if err != nil {
		if response != nil {
			_ = response.Body.Close()
			return nil, fmt.Errorf("open xfyun rtasr stream: HTTP %d: %w", response.StatusCode, err)
		}
		return nil, fmt.Errorf("open xfyun rtasr stream: %w", err)
	}
	conn.SetReadLimit(xfyunMaxResponseBytes)
	stream := &xfyunRTASRStream{ctx: ctx, conn: conn}
	stream.stopContextClose = context.AfterFunc(ctx, func() { _ = stream.closeConnection() })
	if err := waitForXfyunRTASRStarted(conn); err != nil {
		_ = stream.Close(context.Background())
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, err
	}
	return stream, nil
}

func validateXfyunAudioFormat(audioFormat SpeechAudioFormat) error {
	audioFormat = audioFormat.Normalized()
	if err := audioFormat.Validate(); err != nil {
		return err
	}
	if audioFormat.Encoding != SpeechAudioEncodingPCM16LE ||
		audioFormat.SampleRateHz != 16_000 ||
		audioFormat.Channels != 1 ||
		audioFormat.BitsPerSample != 16 {
		return errors.New("xfyun rtasr requires pcm_s16le 16000 Hz mono 16-bit audio")
	}
	return nil
}

func waitForXfyunRTASRStarted(conn *websocket.Conn) error {
	if err := conn.SetReadDeadline(time.Now().Add(xfyunStartedTimeout)); err != nil {
		return err
	}
	defer func() { _ = conn.SetReadDeadline(time.Time{}) }()
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("wait for xfyun rtasr started event: %w", err)
		}
		var envelope xfyunEnvelope
		if err := json.Unmarshal(payload, &envelope); err != nil {
			return fmt.Errorf("decode xfyun rtasr started event: %w", err)
		}
		code := normalizeXfyunCode(envelope.Code)
		if code != "" && code != "0" {
			return &XfyunRTASRError{
				Code: code, Description: strings.TrimSpace(envelope.Desc), SID: strings.TrimSpace(envelope.SID),
			}
		}
		switch strings.ToLower(strings.TrimSpace(envelope.Action)) {
		case "started":
			return nil
		case "error":
			return &XfyunRTASRError{
				Code: code, Description: strings.TrimSpace(envelope.Desc), SID: strings.TrimSpace(envelope.SID),
			}
		}
	}
}

func normalizeXfyunLanguage(language string) string {
	language = strings.ToLower(strings.TrimSpace(language))
	if language == "" {
		return ""
	}
	switch {
	case language == "cn", strings.HasPrefix(language, "zh"):
		return "cn"
	case language == "en", strings.HasPrefix(language, "en-"):
		return "en"
	default:
		if base, _, found := strings.Cut(language, "-"); found {
			return base
		}
		return language
	}
}

type xfyunRTASRStream struct {
	ctx              context.Context
	conn             *websocket.Conn
	writeMu          sync.Mutex
	lastAudioWrite   time.Time
	closeOnce        sync.Once
	closeErr         error
	stopContextClose func() bool
}

func (s *xfyunRTASRStream) WriteAudio(ctx context.Context, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if !s.lastAudioWrite.IsZero() {
		wait := time.Until(s.lastAudioWrite.Add(xfyunAudioWriteInterval))
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
	if err := s.conn.SetWriteDeadline(speechOperationDeadline(ctx, xfyunWriteTimeout)); err != nil {
		return err
	}
	if err := s.conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		return err
	}
	s.lastAudioWrite = time.Now()
	return nil
}

func (s *xfyunRTASRStream) EndAudio(ctx context.Context) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := s.conn.SetWriteDeadline(speechOperationDeadline(ctx, xfyunWriteTimeout)); err != nil {
		return err
	}
	return s.conn.WriteMessage(websocket.BinaryMessage, []byte(`{"end": true}`))
}

func (s *xfyunRTASRStream) Close(_ context.Context) error {
	if s.stopContextClose != nil {
		s.stopContextClose()
	}
	return s.closeConnection()
}

func (s *xfyunRTASRStream) closeConnection() error {
	s.closeOnce.Do(func() { s.closeErr = s.conn.Close() })
	return s.closeErr
}

func (s *xfyunRTASRStream) ReadEvent(ctx context.Context) (*SpeechTranscriptionEvent, error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if deadline, ok := ctx.Deadline(); ok {
			if err := s.conn.SetReadDeadline(deadline); err != nil {
				return nil, err
			}
		}
		_, payload, err := s.conn.ReadMessage()
		if err != nil {
			return nil, err
		}
		event, err := parseXfyunRTASREvent(payload)
		if err != nil {
			return nil, err
		}
		if event != nil {
			return event, nil
		}
	}
}

func speechOperationDeadline(ctx context.Context, fallback time.Duration) time.Time {
	deadline := time.Now().Add(fallback)
	if ctx != nil {
		if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
			return contextDeadline
		}
	}
	return deadline
}

type xfyunEnvelope struct {
	Action string          `json:"action"`
	Code   json.RawMessage `json:"code"`
	Desc   string          `json:"desc"`
	Data   string          `json:"data"`
	SID    string          `json:"sid"`
}

type xfyunResult struct {
	SegID xfyunInt64 `json:"seg_id"`
	CN    struct {
		ST struct {
			BG   xfyunInt64 `json:"bg"`
			ED   xfyunInt64 `json:"ed"`
			Type string     `json:"type"`
			RT   []struct {
				WS []struct {
					CW []struct {
						W  string       `json:"w"`
						SC xfyunFloat64 `json:"sc"`
					} `json:"cw"`
				} `json:"ws"`
			} `json:"rt"`
		} `json:"st"`
	} `json:"cn"`
}

type xfyunInt64 int64

func (v *xfyunInt64) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		*v = 0
		return nil
	}
	if strings.HasPrefix(raw, `"`) {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		raw = strings.TrimSpace(text)
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid integer %q: %w", raw, err)
	}
	*v = xfyunInt64(parsed)
	return nil
}

type xfyunFloat64 float64

func (v *xfyunFloat64) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		*v = 0
		return nil
	}
	if strings.HasPrefix(raw, `"`) {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		raw = strings.TrimSpace(text)
	}
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return fmt.Errorf("invalid number %q: %w", raw, err)
	}
	*v = xfyunFloat64(parsed)
	return nil
}

type XfyunRTASRError struct {
	Code        string
	Description string
	SID         string
}

func (e *XfyunRTASRError) Error() string {
	parts := []string{"xfyun rtasr error"}
	if e.Code != "" {
		parts = append(parts, "code="+e.Code)
	}
	if e.Description != "" {
		parts = append(parts, "desc="+e.Description)
	}
	if e.SID != "" {
		parts = append(parts, "sid="+e.SID)
	}
	return strings.Join(parts, " ")
}

func parseXfyunRTASREvent(payload []byte) (*SpeechTranscriptionEvent, error) {
	var envelope xfyunEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, fmt.Errorf("decode xfyun rtasr event: %w", err)
	}
	code := normalizeXfyunCode(envelope.Code)
	if code != "" && code != "0" {
		return nil, &XfyunRTASRError{
			Code: code, Description: strings.TrimSpace(envelope.Desc), SID: strings.TrimSpace(envelope.SID),
		}
	}
	switch strings.ToLower(envelope.Action) {
	case "started":
		return nil, nil
	case "error":
		return nil, &XfyunRTASRError{
			Code: code, Description: strings.TrimSpace(envelope.Desc), SID: strings.TrimSpace(envelope.SID),
		}
	case "result":
	default:
		return nil, nil
	}

	var result xfyunResult
	if err := json.Unmarshal([]byte(envelope.Data), &result); err != nil {
		return nil, fmt.Errorf("decode xfyun rtasr result: %w", err)
	}
	var textBuilder strings.Builder
	confidenceTotal := 0.0
	confidenceCount := 0
	for _, rt := range result.CN.ST.RT {
		for _, ws := range rt.WS {
			if len(ws.CW) == 0 {
				continue
			}
			textBuilder.WriteString(ws.CW[0].W)
			if ws.CW[0].SC > 0 {
				confidenceTotal += float64(ws.CW[0].SC)
				confidenceCount++
			}
		}
	}
	text := strings.TrimSpace(textBuilder.String())
	if text == "" {
		return nil, nil
	}
	confidence := 0.0
	if confidenceCount > 0 {
		confidence = confidenceTotal / float64(confidenceCount)
	}
	return &SpeechTranscriptionEvent{
		ProviderEventID: strconv.FormatInt(int64(result.SegID), 10),
		Text:            text,
		IsFinal:         result.CN.ST.Type == "0",
		StartedAtMS:     int64(result.CN.ST.BG),
		EndedAtMS:       int64(result.CN.ST.ED),
		Confidence:      confidence,
		RawJSON:         string(payload),
	}, nil
}

func normalizeXfyunCode(raw json.RawMessage) string {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		return ""
	}
	if strings.HasPrefix(value, `"`) {
		var text string
		if json.Unmarshal(raw, &text) == nil {
			return strings.TrimSpace(text)
		}
	}
	return value
}
