package providers

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"remotehelpdesk/internal/pkg/config"

	"github.com/golang-jwt/jwt/v5"
	oauthjwt "golang.org/x/oauth2/jwt"
)

const (
	firebaseMessagingScope = "https://www.googleapis.com/auth/firebase.messaging"
	firebaseMessagingURL   = "https://fcm.googleapis.com/v1/projects/%s/messages:send"
	apnsProductionURL      = "https://api.push.apple.com"
	apnsSandboxURL         = "https://api.sandbox.push.apple.com"
)

type MobilePushMessage struct {
	Title string
	Body  string
	Data  map[string]string
}

type MobilePushProviderError struct {
	Platform     string
	StatusCode   int
	Reason       string
	InvalidToken bool
	Permanent    bool
}

func (e *MobilePushProviderError) Error() string {
	if e == nil {
		return "mobile push delivery failed"
	}
	reason := strings.TrimSpace(e.Reason)
	if reason == "" {
		reason = "provider rejected the notification"
	}
	return fmt.Sprintf("%s push failed (%d): %s", e.Platform, e.StatusCode, reason)
}

func IsInvalidMobilePushToken(err error) bool {
	var providerErr *MobilePushProviderError
	return errors.As(err, &providerErr) && providerErr.InvalidToken
}

func IsPermanentMobilePushError(err error) bool {
	var providerErr *MobilePushProviderError
	return errors.As(err, &providerErr) && providerErr.Permanent
}

type MobilePushProvider struct {
	httpClient  *http.Client
	fcmBaseURL  string
	apnsBaseURL string
	now         func() time.Time
}

var DefaultMobilePushProvider = NewMobilePushProvider()

func NewMobilePushProvider() *MobilePushProvider {
	return &MobilePushProvider{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		now:        time.Now,
	}
}

func (p *MobilePushProvider) Configured(platform string) bool {
	cfg := config.CurrentOrDefault().MobilePush
	if !cfg.Enabled {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "android":
		return strings.TrimSpace(cfg.FCM.ProjectID) != "" && strings.TrimSpace(cfg.FCM.CredentialsJSON) != ""
	case "ios":
		return strings.TrimSpace(cfg.APNS.TeamID) != "" && strings.TrimSpace(cfg.APNS.KeyID) != "" &&
			strings.TrimSpace(cfg.APNS.PrivateKey) != "" && strings.TrimSpace(cfg.APNS.BundleID) != ""
	default:
		return false
	}
}

func (p *MobilePushProvider) SendNotificationPush(ctx context.Context, platform, token string, message MobilePushMessage) (string, error) {
	platform = strings.ToLower(strings.TrimSpace(platform))
	token = strings.TrimSpace(token)
	if token == "" {
		return "", &MobilePushProviderError{Platform: platform, Reason: "device token is empty", InvalidToken: true, Permanent: true}
	}
	if !p.Configured(platform) {
		return "", &MobilePushProviderError{Platform: platform, Reason: "provider is not configured"}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	switch platform {
	case "android":
		return p.sendFCM(ctx, token, message)
	case "ios":
		return p.sendAPNS(ctx, token, message)
	default:
		return "", &MobilePushProviderError{Platform: platform, Reason: "unsupported platform", Permanent: true}
	}
}

func (p *MobilePushProvider) sendFCM(ctx context.Context, deviceToken string, message MobilePushMessage) (string, error) {
	cfg := config.CurrentOrDefault().MobilePush.FCM
	credentials, err := decodeConfiguredSecret(cfg.CredentialsJSON)
	if err != nil {
		return "", fmt.Errorf("decode FCM credentials: %w", err)
	}
	var serviceAccount struct {
		ClientEmail  string `json:"client_email"`
		PrivateKey   string `json:"private_key"`
		PrivateKeyID string `json:"private_key_id"`
		TokenURI     string `json:"token_uri"`
	}
	if err := json.Unmarshal(credentials, &serviceAccount); err != nil {
		return "", fmt.Errorf("parse FCM credentials: %w", err)
	}
	if strings.TrimSpace(serviceAccount.ClientEmail) == "" || strings.TrimSpace(serviceAccount.PrivateKey) == "" {
		return "", errors.New("FCM credentials require client_email and private_key")
	}
	tokenURL := strings.TrimSpace(serviceAccount.TokenURI)
	if tokenURL == "" {
		tokenURL = "https://oauth2.googleapis.com/token"
	}
	jwtConfig := &oauthjwt.Config{
		Email:        strings.TrimSpace(serviceAccount.ClientEmail),
		PrivateKey:   []byte(strings.ReplaceAll(serviceAccount.PrivateKey, `\n`, "\n")),
		PrivateKeyID: strings.TrimSpace(serviceAccount.PrivateKeyID),
		Scopes:       []string{firebaseMessagingScope},
		TokenURL:     tokenURL,
	}
	oauthToken, err := jwtConfig.TokenSource(ctx).Token()
	if err != nil {
		return "", fmt.Errorf("authorize FCM request: %w", err)
	}
	payload := map[string]any{
		"message": map[string]any{
			"token": deviceToken,
			"notification": map[string]string{
				"title": strings.TrimSpace(message.Title),
				"body":  strings.TrimSpace(message.Body),
			},
			"data": clonePushData(message.Data),
			"android": map[string]any{
				"priority": "HIGH",
				"notification": map[string]string{
					"channel_id": "conversation_updates",
					"icon":       "ic_stat_remotehelpdesk",
					"sound":      "default",
				},
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	endpoint := strings.TrimSpace(p.fcmBaseURL)
	if endpoint == "" {
		endpoint = fmt.Sprintf(firebaseMessagingURL, url.PathEscape(strings.TrimSpace(cfg.ProjectID)))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+oauthToken.AccessToken)
	req.Header.Set("Content-Type", "application/json")
	response, err := p.client().Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		var result struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(responseBody, &result)
		return strings.TrimSpace(result.Name), nil
	}
	return "", parseFCMError(response.StatusCode, responseBody)
}

func (p *MobilePushProvider) sendAPNS(ctx context.Context, deviceToken string, message MobilePushMessage) (string, error) {
	cfg := config.CurrentOrDefault().MobilePush.APNS
	privateKey, err := parseAPNSPrivateKey(cfg.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("parse APNs private key: %w", err)
	}
	now := time.Now()
	if p.now != nil {
		now = p.now()
	}
	providerToken := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": strings.TrimSpace(cfg.TeamID),
		"iat": now.Unix(),
	})
	providerToken.Header["kid"] = strings.TrimSpace(cfg.KeyID)
	signedToken, err := providerToken.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("sign APNs provider token: %w", err)
	}
	payload := map[string]any{
		"aps": map[string]any{
			"alert": map[string]string{
				"title": strings.TrimSpace(message.Title),
				"body":  strings.TrimSpace(message.Body),
			},
			"sound": "default",
		},
	}
	for key, value := range clonePushData(message.Data) {
		payload[key] = value
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	baseURL := strings.TrimRight(strings.TrimSpace(p.apnsBaseURL), "/")
	if baseURL == "" {
		baseURL = apnsProductionURL
		if !cfg.Production {
			baseURL = apnsSandboxURL
		}
	}
	endpoint := baseURL + "/3/device/" + url.PathEscape(deviceToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "bearer "+signedToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apns-topic", strings.TrimSpace(cfg.BundleID))
	req.Header.Set("apns-push-type", "alert")
	req.Header.Set("apns-priority", "10")
	response, err := p.client().Do(req)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if response.StatusCode == http.StatusOK {
		return strings.TrimSpace(response.Header.Get("apns-id")), nil
	}
	return "", parseAPNSError(response.StatusCode, responseBody)
}

func (p *MobilePushProvider) client() *http.Client {
	if p.httpClient != nil {
		return p.httpClient
	}
	return http.DefaultClient
}

func clonePushData(data map[string]string) map[string]string {
	result := make(map[string]string, len(data))
	for key, value := range data {
		key = strings.TrimSpace(key)
		if key != "" {
			result[key] = value
		}
	}
	return result
}

func decodeConfiguredSecret(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New("secret is empty")
	}
	if strings.HasPrefix(value, "{") {
		return []byte(value), nil
	}
	if strings.Contains(value, "-----BEGIN") {
		return []byte(strings.ReplaceAll(value, `\n`, "\n")), nil
	}
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding} {
		decoded, err := encoding.DecodeString(value)
		if err == nil {
			return decoded, nil
		}
	}
	return nil, errors.New("secret must be raw JSON/PEM or base64 encoded")
}

func parseAPNSPrivateKey(value string) (*ecdsa.PrivateKey, error) {
	decoded, err := decodeConfiguredSecret(value)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(decoded)
	if block == nil {
		return nil, errors.New("PEM block is missing")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if ecKey, ok := key.(*ecdsa.PrivateKey); ok {
			return ecKey, nil
		}
		return nil, errors.New("private key is not ECDSA")
	}
	return x509.ParseECPrivateKey(block.Bytes)
}

func parseFCMError(statusCode int, body []byte) error {
	var payload struct {
		Error struct {
			Status  string `json:"status"`
			Message string `json:"message"`
			Details []struct {
				ErrorCode string `json:"errorCode"`
			} `json:"details"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &payload)
	reason := strings.TrimSpace(payload.Error.Status)
	for _, detail := range payload.Error.Details {
		if value := strings.TrimSpace(detail.ErrorCode); value != "" {
			reason = value
			break
		}
	}
	if reason == "" {
		reason = strings.TrimSpace(payload.Error.Message)
	}
	invalid := reason == "UNREGISTERED" || reason == "SENDER_ID_MISMATCH"
	permanent := invalid || statusCode == http.StatusBadRequest || statusCode == http.StatusForbidden || statusCode == http.StatusNotFound
	return &MobilePushProviderError{Platform: "android", StatusCode: statusCode, Reason: reason, InvalidToken: invalid, Permanent: permanent}
}

func parseAPNSError(statusCode int, body []byte) error {
	var payload struct {
		Reason string `json:"reason"`
	}
	_ = json.Unmarshal(body, &payload)
	reason := strings.TrimSpace(payload.Reason)
	invalid := reason == "BadDeviceToken" || reason == "DeviceTokenNotForTopic" || reason == "Unregistered"
	permanent := invalid || statusCode == http.StatusBadRequest || statusCode == http.StatusForbidden || statusCode == http.StatusGone
	return &MobilePushProviderError{Platform: "ios", StatusCode: statusCode, Reason: reason, InvalidToken: invalid, Permanent: permanent}
}
