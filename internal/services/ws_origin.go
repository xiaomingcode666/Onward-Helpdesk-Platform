package services

import (
	"net/http"
	"net/url"
	"strings"

	"remotehelpdesk/internal/pkg/config"
)

func websocketOriginAllowed(request *http.Request) bool {
	if request == nil {
		return false
	}
	origin := strings.TrimRight(strings.TrimSpace(request.Header.Get("Origin")), "/")
	if origin == "" {
		// Native clients do not send Origin. Authentication and customer-session
		// validation still run before topic subscription.
		return true
	}
	originURL, err := url.Parse(origin)
	if err != nil || originURL.Host == "" {
		return false
	}

	// Native Capacitor WebViews use capacitor://localhost. That origin is
	// accepted only when explicitly configured, just like a browser origin.
	for _, allowed := range config.CurrentOrDefault().Server.CORS.AllowedOrigins {
		allowedURL, parseErr := url.Parse(strings.TrimRight(strings.TrimSpace(allowed), "/"))
		if parseErr != nil || allowedURL.Host == "" {
			continue
		}
		if strings.EqualFold(originURL.Scheme, allowedURL.Scheme) && strings.EqualFold(originURL.Host, allowedURL.Host) {
			return true
		}
	}
	if originURL.Scheme != "http" && originURL.Scheme != "https" {
		return false
	}

	requestScheme := "http"
	if request.TLS != nil {
		requestScheme = "https"
	} else if forwardedProto := strings.TrimSpace(strings.Split(request.Header.Get("X-Forwarded-Proto"), ",")[0]); forwardedProto == "http" || forwardedProto == "https" {
		requestScheme = forwardedProto
	}
	for _, requestHost := range websocketRequestHosts(request) {
		if strings.EqualFold(originURL.Scheme, requestScheme) && strings.EqualFold(originURL.Host, requestHost) {
			return true
		}
	}

	return false
}

func websocketRequestHosts(request *http.Request) []string {
	if request == nil {
		return nil
	}
	ret := make([]string, 0, 3)
	add := func(value string) {
		value = strings.TrimRight(strings.TrimSpace(strings.Split(value, ",")[0]), "/")
		if value == "" {
			return
		}
		for _, existing := range ret {
			if strings.EqualFold(existing, value) {
				return
			}
		}
		ret = append(ret, value)
	}
	add(request.Host)
	add(request.Header.Get("X-Forwarded-Host"))
	if forwardedHost := strings.TrimSpace(strings.Split(request.Header.Get("X-Forwarded-Host"), ",")[0]); forwardedHost != "" && !strings.Contains(forwardedHost, ":") {
		if forwardedPort := strings.TrimSpace(strings.Split(request.Header.Get("X-Forwarded-Port"), ",")[0]); forwardedPort != "" {
			add(forwardedHost + ":" + forwardedPort)
		}
	}
	return ret
}

// WebsocketOriginAllowed exposes the shared same-origin/CORS policy to feature
// WebSocket handlers without duplicating security rules.
func WebsocketOriginAllowed(request *http.Request) bool {
	return websocketOriginAllowed(request)
}
