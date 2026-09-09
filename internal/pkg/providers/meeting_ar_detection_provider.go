package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"remotehelpdesk/internal/pkg/config"
)

type MeetingARDetectionInput struct {
	TenantID     int64
	MeetingID    string
	TicketID     string
	FrameAssetID int64
	Filename     string
	ContentType  string
	Image        io.Reader
}

type MeetingARDetection struct {
	ExternalID   string
	Label        string
	PartCode     string
	Confidence   float64
	X            float64
	Y            float64
	Width        float64
	Height       float64
	Color        string
	Note         string
	MetadataJSON string
}

type MeetingARDetectionProvider interface {
	Name() string
	Configured() bool
	Detect(ctx context.Context, input MeetingARDetectionInput) ([]MeetingARDetection, error)
}

type MeetingARDetectionHealthChecker interface {
	CheckHealth(ctx context.Context) error
}

type MeetingARDetectionProviderFactory func(cfg config.MeetingARConfig) (MeetingARDetectionProvider, error)

var (
	meetingARProviderFactoriesMu sync.RWMutex
	meetingARProviderFactories   = map[string]MeetingARDetectionProviderFactory{}
	currentMeetingARProvider     atomic.Pointer[meetingARDetectionProviderSnapshot]
)

type meetingARDetectionProviderSnapshot struct {
	provider MeetingARDetectionProvider
}

func init() {
	meetingARProviderFactories["http"] = newHTTPMeetingARDetectionProvider
	currentMeetingARProvider.Store(&meetingARDetectionProviderSnapshot{provider: disabledMeetingARDetectionProvider{}})
}

func RegisterMeetingARDetectionProvider(name string, factory MeetingARDetectionProviderFactory) error {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" || name == "disabled" || factory == nil {
		return errors.New("valid AR detection provider name and factory are required")
	}
	meetingARProviderFactoriesMu.Lock()
	defer meetingARProviderFactoriesMu.Unlock()
	if _, exists := meetingARProviderFactories[name]; exists {
		return fmt.Errorf("AR detection provider %q is already registered", name)
	}
	meetingARProviderFactories[name] = factory
	return nil
}

func InitMeetingARDetection(cfg config.MeetingARConfig) error {
	provider, err := NewMeetingARDetectionProvider(cfg)
	if err != nil {
		return err
	}
	currentMeetingARProvider.Store(&meetingARDetectionProviderSnapshot{provider: provider})
	return nil
}

// NewMeetingARDetectionProvider builds an isolated provider so draft settings
// can be tested before they replace the live meeting runtime.
func NewMeetingARDetectionProvider(cfg config.MeetingARConfig) (MeetingARDetectionProvider, error) {
	name := cfg.ProviderName()
	if name == "disabled" {
		return disabledMeetingARDetectionProvider{}, nil
	}
	meetingARProviderFactoriesMu.RLock()
	factory := meetingARProviderFactories[name]
	meetingARProviderFactoriesMu.RUnlock()
	if factory == nil {
		return nil, fmt.Errorf("AR detection provider %q is not registered", name)
	}
	provider, err := factory(cfg)
	if err != nil {
		return nil, fmt.Errorf("initialize AR detection provider %q: %w", name, err)
	}
	if provider == nil || !provider.Configured() {
		return nil, fmt.Errorf("AR detection provider %q is not configured", name)
	}
	return provider, nil
}

func CurrentMeetingARDetectionProvider() MeetingARDetectionProvider {
	snapshot := currentMeetingARProvider.Load()
	if snapshot == nil || snapshot.provider == nil {
		return disabledMeetingARDetectionProvider{}
	}
	return snapshot.provider
}

type disabledMeetingARDetectionProvider struct{}

func (disabledMeetingARDetectionProvider) Name() string     { return "disabled" }
func (disabledMeetingARDetectionProvider) Configured() bool { return false }
func (disabledMeetingARDetectionProvider) Detect(context.Context, MeetingARDetectionInput) ([]MeetingARDetection, error) {
	return nil, errors.New("AR detection provider is not configured")
}

type httpMeetingARDetectionProvider struct {
	endpoint   string
	apiKey     string
	healthPath string
	client     *http.Client
}

func newHTTPMeetingARDetectionProvider(cfg config.MeetingARConfig) (MeetingARDetectionProvider, error) {
	endpoint := meetingAROption(cfg.ProviderOptions, "endpoint")
	if endpoint == "" {
		return nil, errors.New("meeting AR HTTP endpoint is required")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, errors.New("meeting AR endpoint must be an absolute HTTP(S) URL")
	}
	return &httpMeetingARDetectionProvider{
		endpoint: endpoint, apiKey: meetingAROption(cfg.ProviderOptions, "apiKey"),
		healthPath: meetingAROption(cfg.ProviderOptions, "healthPath"),
		client:     &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func meetingAROption(options map[string]any, key string) string {
	if options == nil {
		return ""
	}
	value, ok := options[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func (p *httpMeetingARDetectionProvider) Name() string { return "http" }

func (p *httpMeetingARDetectionProvider) Configured() bool {
	return p != nil && strings.TrimSpace(p.endpoint) != ""
}

func (p *httpMeetingARDetectionProvider) CheckHealth(ctx context.Context) error {
	if !p.Configured() {
		return errors.New("meeting AR HTTP provider is not configured")
	}
	healthURL, err := p.resolveHealthURL()
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return err
	}
	p.authorize(req)
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("check meeting AR provider: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("meeting AR provider returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func (p *httpMeetingARDetectionProvider) Detect(ctx context.Context, input MeetingARDetectionInput) ([]MeetingARDetection, error) {
	if !p.Configured() {
		return nil, errors.New("meeting AR HTTP provider is not configured")
	}
	if input.Image == nil {
		return nil, errors.New("meeting AR image is required")
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image", strings.TrimSpace(input.Filename))
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, io.LimitReader(input.Image, 16<<20)); err != nil {
		return nil, err
	}
	fields := map[string]string{
		"tenantId": fmt.Sprint(input.TenantID), "meetingId": input.MeetingID,
		"ticketId": input.TicketID, "frameAssetId": fmt.Sprint(input.FrameAssetID),
		"contentType": input.ContentType,
	}
	for key, value := range fields {
		if strings.TrimSpace(value) != "" && value != "0" {
			_ = writer.WriteField(key, value)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	p.authorize(req)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call meeting AR provider: %w", err)
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("meeting AR provider returned HTTP %d", resp.StatusCode)
	}
	var payload struct {
		Detections []MeetingARDetection `json:"detections"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, fmt.Errorf("decode meeting AR response: %w", err)
	}
	return payload.Detections, nil
}

func (p *httpMeetingARDetectionProvider) authorize(req *http.Request) {
	if value := strings.TrimSpace(p.apiKey); value != "" {
		req.Header.Set("Authorization", "Bearer "+value)
	}
}

func (p *httpMeetingARDetectionProvider) resolveHealthURL() (string, error) {
	if healthPath := strings.TrimSpace(p.healthPath); healthPath != "" {
		if parsed, err := url.Parse(healthPath); err == nil && parsed.IsAbs() {
			return parsed.String(), nil
		}
		base, err := url.Parse(p.endpoint)
		if err != nil {
			return "", err
		}
		return base.ResolveReference(&url.URL{Path: healthPath}).String(), nil
	}
	base, err := url.Parse(p.endpoint)
	if err != nil {
		return "", err
	}
	base.Path = "/health"
	base.RawQuery = ""
	base.Fragment = ""
	return base.String(), nil
}
