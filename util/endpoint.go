package util

import (
    "context"
    "crypto/tls"
    "errors"
    "fmt"
    "net"
    "net/url"
    "strings"
    "time"
)

const tlsHandshakeTimeout = 10 * time.Second

func ValidateEndpoint(s string, enforceSecure bool) (string, error) {
    if IsStringEmpty(s) {
        return "", errors.New("please provide the endpoint url")
    }

    u, err := url.Parse(s)
    if err != nil {
        return "", err
    }

    switch u.Scheme {
    case "http":
        if enforceSecure {
            return "", errors.New("only https endpoints allowed")
        }
    case "https":
        if err := checkLiveness(u); err != nil {
            return "", err
        }
    default:
        return "", errors.New("invalid endpoint scheme")
    }

    switch strings.ToLower(u.Hostname()) {
    case "localhost", "127.0.0.1":
        return "", errors.New("cannot use localhost or 127.0.0.1")
    }

    return u.String(), nil
}

// checkLiveness verifies the endpoint is live and terminates TLS by completing a
// TLS handshake against it.
func checkLiveness(u *url.URL) error {
    port := u.Port()
    if port == "" {
        port = "443"
    }

    ctx, cancel := context.WithTimeout(context.Background(), tlsHandshakeTimeout)
    defer cancel()

    dialer := &tls.Dialer{Config: &tls.Config{MinVersion: tls.VersionTLS12}}

    conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(u.Hostname(), port))
    if err != nil {
        return fmt.Errorf("failed to ping tls endpoint: %v", err)
    }

    defer conn.Close()

    return nil
}
