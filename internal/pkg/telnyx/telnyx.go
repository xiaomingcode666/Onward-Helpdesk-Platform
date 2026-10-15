// Package telnyx authenticates Telnyx Voice API webhooks and sends Call
// Control commands. See work/AI-PHONE-SDD-SPEC.md section 15.
package telnyx

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultBaseURL        = "https://api.telnyx.com/v2"
	signatureTolerance    = 5 * time.Minute
	minWebhookTokenLength = 32
	callEndedErrorCode    = "90018"
)

var (
	ErrWebhookRejected = errors.New("telnyx webhook rejected")
	// ErrCallEnded means Telnyx refused a command because the call is gone.
	ErrCallEnded = errors.New("telnyx call has already ended")
)

// WebhookAuthenticator verifies the Ed25519 signature over
// "{timestamp}|{raw body}". Accounts that cannot read the Telnyx public key
// may instead use a URL token of at least 32 characters; a configured public
// key always takes precedence. With neither, every webhook is rejected.
type WebhookAuthenticator struct {
	publicKey ed25519.PublicKey
	token     string
	now       func() time.Time
}

func NewWebhookAuthenticator(publicKey, token string) *WebhookAuthenticator {
	token = strings.TrimSpace(token)
	if len(token) < minWebhookTokenLength {
		token = ""
	}
	return &WebhookAuthenticator{publicKey: parsePublicKey(publicKey), token: token, now: time.Now}
}

func parsePublicKey(value string) ed25519.PublicKey {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil || len(raw) == 0 {
		return nil
	}
	if len(raw) == ed25519.PublicKeySize {
		return ed25519.PublicKey(raw)
	}
	parsed, err := x509.ParsePKIXPublicKey(raw)
	if err != nil {
		return nil
	}
	key, _ := parsed.(ed25519.PublicKey)
	return key
}

func (a *WebhookAuthenticator) Authenticate(body []byte, headers http.Header, queryToken string) error {
	if a.publicKey != nil || a.token == "" {
		return a.verifySignature(body, headers)
	}
	if queryToken == "" || subtle.ConstantTimeCompare([]byte(queryToken), []byte(a.token)) != 1 {
		return fmt.Errorf("%w: invalid token", ErrWebhookRejected)
	}
	return nil
}

func (a *WebhookAuthenticator) verifySignature(body []byte, headers http.Header) error {
	if a.publicKey == nil {
		return fmt.Errorf("%w: no public key or token configured", ErrWebhookRejected)
	}
	timestamp := strings.TrimSpace(headers.Get("Telnyx-Timestamp"))
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(headers.Get("Telnyx-Signature-Ed25519")))
	if timestamp == "" || err != nil || len(signature) == 0 {
		return fmt.Errorf("%w: missing or malformed signature", ErrWebhookRejected)
	}
	sentAt, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("%w: malformed timestamp", ErrWebhookRejected)
	}
	if age := a.now().Sub(time.Unix(sentAt, 0)); age > signatureTolerance || age < -signatureTolerance {
		return fmt.Errorf("%w: timestamp outside tolerance", ErrWebhookRejected)
	}
	if !ed25519.Verify(a.publicKey, []byte(timestamp+"|"+string(body)), signature) {
		return fmt.Errorf("%w: invalid signature", ErrWebhookRejected)
	}
	return nil
}

// Client sends Call Control commands: POST /calls/{call_control_id}/actions/{action}.
type Client struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

func (c *Client) Command(ctx context.Context, callControlID, action string, body map[string]any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	baseURL := strings.TrimRight(c.BaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	endpoint := baseURL + "/calls/" + url.PathEscape(callControlID) + "/actions/" + url.PathEscape(action)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("telnyx %s: %w", action, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var parsed struct {
		Errors []struct {
			Code  string `json:"code"`
			Title string `json:"title"`
		} `json:"errors"`
	}
	_ = json.Unmarshal(raw, &parsed)
	for _, item := range parsed.Errors {
		if item.Code == callEndedErrorCode {
			return ErrCallEnded
		}
	}
	return fmt.Errorf("telnyx %s failed with HTTP %d: %s", action, resp.StatusCode, strings.TrimSpace(string(raw)))
}
