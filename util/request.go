package util

import (
	"net/http"
	"strings"
)

// RequestOrigin returns the request's origin (scheme + host) for redirect URLs.
// Prefers X-Forwarded-Proto and X-Forwarded-Host when behind a proxy.
func RequestOrigin(r *http.Request) string {
	scheme := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(r.Host)
	}
	if host == "" {
		return ""
	}
	if scheme != "https" && scheme != "http" {
		scheme = "https"
	}
	return scheme + "://" + host
}

// AcceptsHTML returns true when the request prefers HTML (e.g. browser navigation).
func AcceptsHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/html")
}
