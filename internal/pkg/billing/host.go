package billing

import (
	"errors"
	"net/url"
	"strings"
)

// CanonicalOrigin validates and normalises a caller-supplied origin into a canonical
// "scheme://host[:port]" form. It requires an http or https scheme and a hostname,
// rejects userinfo, path, query, fragment, and non-ASCII hosts, lowercases the host, and
// strips default ports (80 for http, 443 for https). It is the single origin-validation
// source for billing checkout redirects.
func CanonicalOrigin(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("host is required")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", errors.New("host is invalid")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("host must include http or https scheme")
	}
	if u.Hostname() == "" {
		return "", errors.New("host must include a hostname")
	}
	if u.User != nil {
		return "", errors.New("host must not include userinfo")
	}
	if (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("host must not include path, query, or fragment")
	}
	if !isASCII(u.Hostname()) {
		return "", errors.New("host contains invalid characters")
	}

	host := strings.ToLower(u.Hostname())
	if port := canonicalPort(u); port != "" {
		host += ":" + port
	}
	return u.Scheme + "://" + host, nil
}

func canonicalPort(u *url.URL) string {
	port := u.Port()
	if port == "" {
		return ""
	}
	if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
		return ""
	}
	return port
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}
