package providers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"remotehelpdesk/internal/pkg/config"
)

const (
	defaultXfyunTranslationEndpoint = "https://itrans.xfyun.cn/v2/its"
	xfyunTranslationMaxEncodedBytes = 4096
)

type XfyunTextTranslationProvider struct {
	appID      string
	apiKey     string
	apiSecret  string
	endpoint   string
	targetLang string
	httpClient *http.Client
}

func NewXfyunTextTranslationProvider(cfg config.XfyunSpeechConfig) *XfyunTextTranslationProvider {
	endpoint := strings.TrimSpace(cfg.TranslationEndpoint)
	if endpoint == "" {
		endpoint = defaultXfyunTranslationEndpoint
	}
	targetLang := normalizeXfyunTranslationLanguage(cfg.TranslationTargetLanguage)
	if targetLang == "" {
		targetLang = "en"
	}
	return &XfyunTextTranslationProvider{
		appID: strings.TrimSpace(cfg.AppID), apiKey: strings.TrimSpace(cfg.APIKey),
		apiSecret: strings.TrimSpace(cfg.TranslationAPISecret), endpoint: endpoint,
		targetLang: targetLang, httpClient: &http.Client{Timeout: 8 * time.Second},
	}
}

func (p *XfyunTextTranslationProvider) Name() string { return "xfyun_translation" }

func (p *XfyunTextTranslationProvider) Configured() bool {
	return p != nil && p.appID != "" && p.apiKey != "" && p.apiSecret != "" && p.endpoint != ""
}

func (p *XfyunTextTranslationProvider) Translate(ctx context.Context, input TextTranslationRequest) (*TextTranslationResult, error) {
	if !p.Configured() {
		return nil, errors.New("xfyun translation is not configured")
	}
	text := strings.TrimSpace(input.Text)
	if text == "" {
		return nil, errors.New("translation text is required")
	}
	sourceLanguage := normalizeXfyunTranslationLanguage(input.SourceLanguage)
	if sourceLanguage == "" {
		sourceLanguage = "cn"
	}
	targetLanguage := normalizeXfyunTranslationLanguage(input.TargetLanguage)
	if targetLanguage == "" {
		targetLanguage = p.targetLang
	}
	encodedText := base64.StdEncoding.EncodeToString([]byte(text))
	if len(encodedText) > xfyunTranslationMaxEncodedBytes {
		return nil, fmt.Errorf("xfyun translation text exceeds %d encoded bytes", xfyunTranslationMaxEncodedBytes)
	}
	body, err := json.Marshal(map[string]any{
		"common":   map[string]string{"app_id": p.appID},
		"business": map[string]string{"from": sourceLanguage, "to": targetLanguage},
		"data":     map[string]string{"text": encodedText},
	})
	if err != nil {
		return nil, err
	}
	endpoint, err := url.Parse(p.endpoint)
	if err != nil || endpoint.Host == "" {
		return nil, errors.New("invalid xfyun translation endpoint")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	date := time.Now().UTC().Format(http.TimeFormat)
	digestBytes := sha256.Sum256(body)
	digest := "SHA-256=" + base64.StdEncoding.EncodeToString(digestBytes[:])
	requestURI := endpoint.EscapedPath()
	if requestURI == "" {
		requestURI = "/"
	}
	if endpoint.RawQuery != "" {
		requestURI += "?" + endpoint.RawQuery
	}
	signatureSource := fmt.Sprintf("host: %s\ndate: %s\nPOST %s HTTP/1.1\ndigest: %s", endpoint.Host, date, requestURI, digest)
	mac := hmac.New(sha256.New, []byte(p.apiSecret))
	_, _ = mac.Write([]byte(signatureSource))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json,version=1.0")
	req.Header.Set("Date", date)
	req.Header.Set("Digest", digest)
	req.Header.Set("Authorization", fmt.Sprintf(
		`api_key="%s", algorithm="hmac-sha256", headers="host date request-line digest", signature="%s"`,
		p.apiKey, signature,
	))
	response, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call xfyun translation: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read xfyun translation response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("xfyun translation returned HTTP %d", response.StatusCode)
	}
	var payload struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		SID     string `json:"sid"`
		Data    struct {
			Result struct {
				From        string `json:"from"`
				To          string `json:"to"`
				TransResult struct {
					Dst string `json:"dst"`
				} `json:"trans_result"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("decode xfyun translation response: %w", err)
	}
	if payload.Code != 0 {
		return nil, fmt.Errorf("xfyun translation error code=%d message=%s", payload.Code, strings.TrimSpace(payload.Message))
	}
	translatedText := strings.TrimSpace(payload.Data.Result.TransResult.Dst)
	if translatedText == "" {
		return nil, errors.New("xfyun translation returned empty text")
	}
	return &TextTranslationResult{
		Text: translatedText, SourceLanguage: payload.Data.Result.From,
		TargetLanguage: payload.Data.Result.To, Provider: p.Name(), ProviderEventID: strings.TrimSpace(payload.SID),
	}, nil
}
