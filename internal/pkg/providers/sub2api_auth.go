package providers

import (
	"net/http"
	"strings"
	"time"
)

const sub2APIKeyHeader = "X-Api-Key"

type sub2APIAuthTransport struct {
	base   http.RoundTripper
	apiKey string
}

func (t *sub2APIAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	cloned.Header = req.Header.Clone()
	cloned.Header.Set(sub2APIKeyHeader, t.apiKey)
	return t.base.RoundTrip(cloned)
}

// NewSub2APIHTTPClient adapts OpenAI-compatible clients to Sub2API's X-Api-Key authentication.
func NewSub2APIHTTPClient(apiKey string, timeout time.Duration) *http.Client {
	transport := http.DefaultTransport
	return &http.Client{
		Timeout: timeout,
		Transport: &sub2APIAuthTransport{
			base:   transport,
			apiKey: strings.TrimSpace(apiKey),
		},
	}
}
