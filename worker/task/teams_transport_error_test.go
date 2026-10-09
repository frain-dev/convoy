package task

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/url"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

// signedTeamsURL mirrors the shape of a real Workflows webhook: the `sig` query
// parameter is bearer-equivalent, so no classified error may echo it.
const signedTeamsURL = "https://prod-12.westus.logic.azure.com:443/workflows/abc123/triggers/manual/paths/invoke?api-version=2016-06-01&sp=%2Ftriggers%2Fmanual%2Frun&sv=1.0&sig=S3cr3tSignatureValue"

// wrapAsClientError reproduces what http.Client returns: a *url.Error that
// quotes the full request URL and wraps the transport failure.
func wrapAsClientError(inner error) error {
	return &url.Error{Op: "Post", URL: signedTeamsURL, Err: inner}
}

func TestClassifyTeamsTransportError(t *testing.T) {
	tests := []struct {
		name  string
		inner error
		want  string
	}{
		{
			name:  "dns failure names the host only",
			inner: &net.DNSError{Err: "no such host", Name: "prod-12.westus.logic.azure.com", IsNotFound: true},
			want:  `dns failure for host "prod-12.westus.logic.azure.com"`,
		},
		{
			name:  "dns timeout is still reported as dns",
			inner: &net.DNSError{Err: "i/o timeout", Name: "prod-12.westus.logic.azure.com", IsTimeout: true},
			want:  `dns failure for host "prod-12.westus.logic.azure.com"`,
		},
		{
			name:  "net timeout",
			inner: &net.OpError{Op: "dial", Net: "tcp", Err: os.ErrDeadlineExceeded},
			want:  "timeout",
		},
		{
			name:  "context deadline",
			inner: context.DeadlineExceeded,
			want:  "timeout",
		},
		{
			name:  "connection refused",
			inner: &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)},
			want:  "connection refused",
		},
		{
			name:  "tls certificate verification",
			inner: &tls.CertificateVerificationError{Err: x509.UnknownAuthorityError{}},
			want:  "tls failure",
		},
		{
			name:  "tls record header",
			inner: tls.RecordHeaderError{Msg: "first record does not look like a TLS handshake"},
			want:  "tls failure",
		},
		{
			name:  "unrecognised falls back",
			inner: errors.New("something else went wrong"),
			want:  "transport failure",
		},
		{
			// The connect-time SSRF guard returns a plain error from the
			// dialer. It has no sentinel to match on, so it lands in the
			// generic category by design.
			name:  "ssrf rejection falls back",
			inner: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("ssrf guard: blocked connection to non-public address 10.0.0.1")},
			want:  "transport failure",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyTeamsTransportError(wrapAsClientError(tc.inner))
			require.Equal(t, tc.want, got)
		})
	}
}

// TestClassifyTeamsTransportError_NeverLeaksSignature is the invariant the
// classifier exists to protect: whatever the shape of the wrapped failure, the
// text handed back must not carry the URL or its query.
func TestClassifyTeamsTransportError_NeverLeaksSignature(t *testing.T) {
	inners := []error{
		&net.DNSError{Err: "no such host", Name: "prod-12.westus.logic.azure.com", IsNotFound: true},
		&net.OpError{Op: "dial", Net: "tcp", Err: os.ErrDeadlineExceeded},
		context.DeadlineExceeded,
		&net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)},
		&tls.CertificateVerificationError{Err: x509.UnknownAuthorityError{}},
		tls.RecordHeaderError{Msg: "first record does not look like a TLS handshake"},
		errors.New("something else went wrong"),
		// The worst case: an inner error that itself quotes the whole URL.
		errors.New("proxy rejected request to " + signedTeamsURL),
	}

	for _, inner := range inners {
		clientErr := wrapAsClientError(inner)

		// Guard the test itself: the raw client error must contain the
		// signature, or these assertions prove nothing.
		require.Contains(t, clientErr.Error(), "sig=S3cr3tSignatureValue")

		category := classifyTeamsTransportError(clientErr)
		require.NotContains(t, category, "sig=")
		require.NotContains(t, category, "S3cr3tSignatureValue")
		require.NotContains(t, category, "logic.azure.com:443")
		require.False(t, strings.Contains(category, signedTeamsURL))
	}
}
