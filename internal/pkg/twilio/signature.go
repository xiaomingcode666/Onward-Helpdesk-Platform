package twilio

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"sort"
	"strings"
)

// Sign implements Twilio's X-Twilio-Signature algorithm for form callbacks.
func Sign(authToken, callbackURL string, params map[string]string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var builder strings.Builder
	builder.WriteString(callbackURL)
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteString(params[key])
	}
	mac := hmac.New(sha1.New, []byte(authToken))
	_, _ = mac.Write([]byte(builder.String()))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func ValidateSignature(authToken, callbackURL string, params map[string]string, provided string) bool {
	authToken = strings.TrimSpace(authToken)
	provided = strings.TrimSpace(provided)
	if authToken == "" || callbackURL == "" || provided == "" {
		return false
	}
	expected := Sign(authToken, callbackURL, params)
	if len(expected) != len(provided) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) == 1
}

func NormalizeWhatsAppAddress(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= len("whatsapp:") && strings.EqualFold(value[:len("whatsapp:")], "whatsapp:") {
		value = strings.TrimSpace(value[len("whatsapp:"):])
	}
	return value
}
