package task

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"syscall"
)

// classifyTeamsTransportError reduces a transport failure to a fixed category.
//
// http.Client returns a *url.Error whose Error() quotes the request URL, and a
// Workflows webhook URL carries its authorisation in a `sig` query parameter.
// No branch here may therefore include the wrapped error's own text. Each
// returns a constant, except the DNS case, which names the host only: a typo in
// the hostname is otherwise indistinguishable from a network outage, and the
// host carries no credential.
//
// Anything unrecognised, including a connect-time SSRF rejection, reports the
// generic category rather than the underlying message.
func classifyTeamsTransportError(err error) string {
	// Checked before the generic timeout branch: *net.DNSError also reports
	// Timeout() for a resolver timeout, and naming the host is more useful
	// than calling it a timeout.
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return fmt.Sprintf("dns failure for host %q", dnsErr.Name)
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}

	if errors.Is(err, syscall.ECONNREFUSED) {
		return "connection refused"
	}

	var certErr *tls.CertificateVerificationError
	if errors.As(err, &certErr) {
		return "tls failure"
	}
	var recordErr tls.RecordHeaderError
	if errors.As(err, &recordErr) {
		return "tls failure"
	}

	return "transport failure"
}
