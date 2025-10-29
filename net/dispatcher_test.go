package net

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/tracer"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/stealthrocket/netjail"

	"github.com/frain-dev/convoy/internal/pkg/license"

	"github.com/frain-dev/convoy/mocks"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/jarcoal/httpmock"

	"github.com/frain-dev/convoy/config"

	"github.com/stretchr/testify/require"
)

var successBody = []byte("received webhook successfully")

func TestDispatcher_SendRequest(t *testing.T) {
	client := http.DefaultClient

	buf := make([]byte, config.MaxResponseSize*2)
	configSignature := config.SignatureHeaderProvider(config.DefaultSignatureHeader.String())
	_, _ = rand.Read(buf)
	type args struct {
		endpoint string
		method   string
		jsonData json.RawMessage
		headers  httpheader.HTTPHeader
		project  *datastore.Project
		hmac     string
	}
	tests := []struct {
		name    string
		args    args
		want    *Response
		nFn     func() func()
		wantErr bool
	}{
		{
			name: "should_send_message",
			args: args{
				endpoint: "https://google.com",
				method:   http.MethodPost,
				jsonData: bytes.NewBufferString("testing").Bytes(),
				project: &datastore.Project{
					UID: "12345",
					Config: &datastore.ProjectConfig{
						Signature: &datastore.SignatureConfiguration{
							Header: configSignature,
						},
						ReplayAttacks: false,
					},
				},
				hmac: "12345",
			},
			want: &Response{
				Status:     "200 OK",
				StatusCode: http.StatusOK,
				Method:     http.MethodPost,
				URL:        nil,
				RequestHeader: http.Header{
					"Accept-Encoding":                      []string{"gzip"},
					"Content-Type":                         []string{"application/json"},
					"User-Agent":                           []string{defaultUserAgent()},
					config.DefaultSignatureHeader.String(): []string{"12345"}, // should equal hmac field above
				},
				ResponseHeader: nil,
				Body:           successBody,
				IP:             "",
				Error:          "",
			},
			nFn: func() func() {
				httpmock.Activate()

				httpmock.RegisterResponder(http.MethodPost, "https://google.com",
					httpmock.NewStringResponder(http.StatusOK, string(successBody)))

				return func() {
					httpmock.DeactivateAndReset()
				}
			},
			wantErr: false,
		},
		{
			name: "should_send_message_with_forwarded_headers",
			args: args{
				endpoint: "https://google.com",
				method:   http.MethodPost,
				jsonData: bytes.NewBufferString("testing").Bytes(),
				headers: map[string][]string{
					"X-Test-Sig": {"abcdef"},
				},
				project: &datastore.Project{
					UID: "12345",
					Config: &datastore.ProjectConfig{
						Signature: &datastore.SignatureConfiguration{
							Header: configSignature,
						},
						ReplayAttacks: false,
					},
				},
				hmac: "12345",
			},
			want: &Response{
				Status:     "200",
				StatusCode: http.StatusOK,
				Method:     http.MethodPost,
				URL:        nil,
				RequestHeader: http.Header{
					"Accept-Encoding":                      []string{"gzip"},
					"Content-Type":                         []string{"application/json"},
					"User-Agent":                           []string{defaultUserAgent()},
					"X-Test-Sig":                           []string{"abcdef"},
					config.DefaultSignatureHeader.String(): []string{"12345"}, // should equal hmac field above
				},
				ResponseHeader: nil,
				Body:           successBody,
				IP:             "",
				Error:          "",
			},
			nFn: func() func() {
				httpmock.Activate()

				httpmock.RegisterResponder(http.MethodPost, "https://google.com",
					httpmock.NewStringResponder(http.StatusOK, string(successBody)))

				return func() {
					httpmock.DeactivateAndReset()
				}
			},
			wantErr: false,
		},
		{
			name: "should_cut_down_oversized_response_body",
			args: args{
				endpoint: "https://google.com",
				method:   http.MethodPost,
				jsonData: bytes.NewBufferString("testing").Bytes(),
				project: &datastore.Project{
					UID: "12345",
					Config: &datastore.ProjectConfig{
						Signature: &datastore.SignatureConfiguration{
							Header: configSignature,
						},
						ReplayAttacks: false,
					},
				},
				hmac: "12345",
			},
			want: &Response{
				Status:     "200",
				StatusCode: http.StatusOK,
				Method:     http.MethodPost,
				URL:        nil,
				RequestHeader: http.Header{
					"Accept-Encoding":                      []string{"gzip"},
					"Content-Type":                         []string{"application/json"},
					"User-Agent":                           []string{defaultUserAgent()},
					config.DefaultSignatureHeader.String(): []string{"12345"}, // should equal hmac field above
				},
				ResponseHeader: nil,
				Body:           buf[:config.MaxResponseSize],
				IP:             "",
				Error:          "",
			},
			nFn: func() func() {
				httpmock.Activate()

				httpmock.RegisterResponder(http.MethodPost, "https://google.com",
					httpmock.NewBytesResponder(http.StatusOK, buf))

				return func() {
					httpmock.DeactivateAndReset()
				}
			},
			wantErr: false,
		},
		{
			name: "should_refuse_connection",
			args: args{
				endpoint: "http://localhost:3234",
				method:   http.MethodPost,
				jsonData: bytes.NewBufferString("bossman").Bytes(),
				project: &datastore.Project{
					UID: "12345",
					Config: &datastore.ProjectConfig{
						Signature: &datastore.SignatureConfiguration{
							Header: configSignature,
						},
						ReplayAttacks: false,
					},
				},
				hmac: "12345",
			},
			want: &Response{
				Status:     "",
				StatusCode: 0,
				Method:     http.MethodPost,
				RequestHeader: http.Header{
					"Accept-Encoding":                      []string{"gzip"},
					"Content-Type":                         []string{"application/json"},
					"User-Agent":                           []string{defaultUserAgent()},
					config.DefaultSignatureHeader.String(): []string{"12345"}, // should equal hmac field above
				},
				ResponseHeader: nil,
				Body:           nil,
				IP:             "",
				Error:          "connect: connection refused",
			},
			wantErr: true,
		},
		{
			name: "should_error_for_empty_signature_hmac",
			args: args{
				endpoint: "http://localhost:3234",
				method:   http.MethodPost,
				jsonData: bytes.NewBufferString("bossman").Bytes(),
				project: &datastore.Project{
					UID: "12345",
					Config: &datastore.ProjectConfig{
						Signature: &datastore.SignatureConfiguration{
							Header: configSignature,
						},
						ReplayAttacks: false,
					},
				},
				hmac: "",
			},
			want: &Response{
				Status:         "",
				StatusCode:     0,
				Method:         "",
				RequestHeader:  nil,
				ResponseHeader: nil,
				Body:           nil,
				IP:             "",
				Error:          "signature header and hmac are required",
			},
			wantErr: true,
		},
		{
			name: "should_error_for_empty_signature_header",
			args: args{
				endpoint: "http://localhost:3234",
				method:   http.MethodPost,
				jsonData: bytes.NewBufferString("bossman").Bytes(),
				project: &datastore.Project{
					UID: "12345",
					Config: &datastore.ProjectConfig{
						Signature: &datastore.SignatureConfiguration{
							Header: config.SignatureHeaderProvider(""),
						},
						ReplayAttacks: false,
					},
				},
				hmac: "css",
			},
			want: &Response{
				Status:         "",
				StatusCode:     0,
				Method:         "",
				RequestHeader:  nil,
				ResponseHeader: nil,
				Body:           nil,
				IP:             "",
				Error:          "signature header and hmac are required",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dispatcher{client: client, logger: log.NewLogger(os.Stdout), ff: fflag.NewFFlag([]string{}), tracer: tracer.NoOpBackend{}}

			if tt.nFn != nil {
				deferFn := tt.nFn()
				defer deferFn()
			}

			got, err := d.SendWebhook(context.Background(), tt.args.endpoint, tt.args.jsonData, tt.args.project.Config.Signature.Header.String(), tt.args.hmac, config.MaxResponseSize, tt.args.headers, "", time.Minute)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), tt.want.Error)
				require.Contains(t, got.Error, tt.want.Error)
			}

			require.Contains(t, got.Status, tt.want.Status)
			require.Equal(t, tt.want.StatusCode, got.StatusCode)
			require.Equal(t, tt.want.Method, got.Method)
			require.Equal(t, tt.want.IP, got.IP)
			require.Equal(t, tt.want.Body, got.Body)
			require.Equal(t, tt.want.RequestHeader, got.RequestHeader)
		})
	}
}

func TestNewDispatcher(t *testing.T) {
	type args struct {
		httpProxy     string
		enforceSecure bool
	}
	tests := []struct {
		name       string
		args       args
		mockFn     func(license.Licenser)
		wantProxy  bool
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_set_proxy",
			args: args{
				httpProxy:     "https://21.3.32.33:443",
				enforceSecure: false,
			},
			mockFn: func(licenser license.Licenser) {
				l := licenser.(*mocks.MockLicenser)
				l.EXPECT().UseForwardProxy().Return(true)
				l.EXPECT().IpRules().Return(true)
				l.EXPECT().CustomCertificateAuthority().Return(false)
			},
			wantProxy:  true,
			wantErr:    false,
			wantErrMsg: "",
		},
		{
			name: "should_not_set_proxy",
			args: args{
				httpProxy:     "https://21.3.32.33:443",
				enforceSecure: false,
			},
			mockFn: func(licenser license.Licenser) {
				l := licenser.(*mocks.MockLicenser)
				l.EXPECT().UseForwardProxy().Return(false)
				l.EXPECT().IpRules().Return(true)
				l.EXPECT().CustomCertificateAuthority().Return(false)
			},
			wantProxy:  false,
			wantErr:    false,
			wantErrMsg: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			licenser := mocks.NewMockLicenser(ctrl)
			if tt.mockFn != nil {
				tt.mockFn(licenser)
			}

			d, err := NewDispatcher(
				licenser,
				fflag.NewFFlag([]string{string(fflag.IpRules)}),
				LoggerOption(log.NewLogger(os.Stdout)),
				TLSConfigOption(tt.args.enforceSecure, licenser, nil),
				ProxyOption(tt.args.httpProxy),
			)
			if tt.wantErr {
				require.Error(t, err)
				require.Equal(t, tt.wantErrMsg, err.Error())
				return
			}

			require.NoError(t, err)

			// Access the custom transport
			customTransport, ok := d.client.Transport.(*CustomTransport)
			require.True(t, ok, "Transport should be of type *CustomTransport")

			// Access the netjail.Transport
			netJailTransport := customTransport.netJailTransport
			require.NotNil(t, netJailTransport, "Underlying transport should be of type *netjail.Transport")

			if tt.wantProxy {
				require.NotNil(t, netJailTransport.New().Proxy)
			}
		})
	}
}

// TestDispatcherSendRequest tests the basic functionality of SendWebhook
func TestDispatcherSendRequest(t *testing.T) {
	// Start a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.Equal(t, "test-hmac", r.Header.Get("X-Signature"))
		require.Equal(t, "test-key", r.Header.Get("X-Convoy-Idempotency-Key"))

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status": "success"}`))
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	licenser := mocks.NewMockLicenser(ctrl)
	licenser.EXPECT().UseForwardProxy().Times(1).Return(true)
	licenser.EXPECT().IpRules().Times(4).Return(true)

	// Create a new dispatcher
	dispatcher, err := NewDispatcher(
		licenser,
		fflag.NewFFlag([]string{string(fflag.IpRules)}),
		LoggerOption(log.NewLogger(os.Stdout)),
		ProxyOption("nil"),
		AllowListOption([]string{"0.0.0.0/0"}),
		BlockListOption([]string{"10.0.0.0/8"}),
	)
	require.NoError(t, err)

	// Prepare request data
	jsonData := json.RawMessage(`{"key": "value"}`)
	headers := httpheader.HTTPHeader{
		"X-Custom-Header": []string{"custom-value"},
	}

	// Send request
	resp, err := dispatcher.SendWebhook(
		context.Background(),
		server.URL,
		jsonData,
		"X-Signature",
		"test-hmac",
		1024,
		headers,
		"test-key",
		5*time.Second,
	)

	// Assert response
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, `{"status": "success"}`, string(resp.Body))
	require.Equal(t, "custom-value", resp.RequestHeader.Get("X-Custom-Header"))
}

// TestDispatcherWithTimeout tests the timeout functionality
func TestDispatcherWithTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	licenser := mocks.NewMockLicenser(ctrl)
	licenser.EXPECT().UseForwardProxy().Times(1).Return(true)
	licenser.EXPECT().IpRules().Times(4).Return(true)

	dispatcher, err := NewDispatcher(
		licenser,
		fflag.NewFFlag([]string{string(fflag.IpRules)}),
		LoggerOption(log.NewLogger(os.Stdout)),
		ProxyOption("nil"),
		AllowListOption([]string{"0.0.0.0/0"}),
		BlockListOption([]string{"10.0.0.0/8"}),
	)
	require.NoError(t, err)

	// Send request with a short timeout
	_, err = dispatcher.SendWebhook(
		context.Background(),
		server.URL,
		nil,
		"X-Signature",
		"test-hmac",
		1024,
		nil,
		"",
		1*time.Second,
	)

	// Assert that we got a timeout error
	require.Error(t, err)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Contains(t, err.Error(), "context deadline exceeded")
}

// TestDispatcherWithBlockedIP tests the IP blocking functionality
func TestDispatcherWithBlockedIP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	licenser := mocks.NewMockLicenser(ctrl)
	licenser.EXPECT().UseForwardProxy().Times(1).Return(true)
	licenser.EXPECT().IpRules().Times(4).Return(true)

	// Create a dispatcher with a blocklist that includes the test server's IP
	dispatcher, err := NewDispatcher(
		licenser,
		fflag.NewFFlag([]string{string(fflag.IpRules)}),
		LoggerOption(log.NewLogger(os.Stdout)),
		ProxyOption("nil"),
		AllowListOption([]string{"0.0.0.0/0"}),
		BlockListOption([]string{"127.0.0.0/8"}),
	)
	require.NoError(t, err)

	// Attempt to send a request
	_, err = dispatcher.SendWebhook(
		context.Background(),
		server.URL,
		nil,
		"X-Signature",
		"test-hmac",
		1024,
		nil,
		"",
		5*time.Second,
	)

	// Assert that the request was blocked
	require.Error(t, err)
	require.ErrorIs(t, err, netjail.ErrDenied)
	require.Contains(t, err.Error(), "127.0.0.1: address not allowed")
}

// TestDispatcherWithMTLSRespectsIPRules ensures that when an mTLS certificate is provided,
// the dispatcher still enforces IP rules (via netjail) and blocks connections accordingly.
func TestDispatcherWithMTLSRespectsIPRules(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Sample client certificate and key (testing only)
	clientCert := `-----BEGIN CERTIFICATE-----
MIIDXjCCAkagAwIBAgIBATANBgkqhkiG9w0BAQsFADBTMQswCQYDVQQGEwJVUzEO
MAwGA1UECAwFU3RhdGUxDTALBgNVBAcMBENpdHkxETAPBgNVBAoMCENsaWVudENB
MRIwEAYDVQQDDAlDbGllbnQtQ0EwHhcNMjUxMDI0MDc1MjM4WhcNMjgwNzIwMDc1
MjM4WjBOMQswCQYDVQQGEwJVUzEOMAwGA1UECAwFU3RhdGUxDTALBgNVBAcMBENp
dHkxDzANBgNVBAoMBkNsaWVudDEPMA0GA1UEAwwGY2xpZW50MIIBIjANBgkqhkiG
9w0BAQEFAAOCAQ8AMIIBCgKCAQEA9Bv0GzAzt8ijkjlVP+E66KaNk0f67T5UFiiT
ij4w9hPOzRPlyXhjsixlqNqkm5ASbycWKhHP67SO7Xn+IeKEXdk/N0BHR0pNlh9k
lXpetKnzvrSwm6ldPD9OrXxjYqouvQpEJ/pkKZsUaH5S5Si6tW+KqczPN9JerjIU
OTAPwDr7KN/MwF+Q2de+7UaZ7Chja41NB0lCIQAU18jGtqQpMISNtA2O3YcaXY8J
0DNdjh/yczu7ii3VKvzFNHDGUbkC7VXJbLziGxCFjDBev9IhMxzmpfQS8IsMWeic
iOBD/8Be9ENW0I2YEZfvMubH/rvJPgxIMSgq9jIE1LKuPAGRQwIDAQABo0IwQDAd
BgNVHQ4EFgQUZFs54K2y3wBzj8S8g9aN2ERcElAwHwYDVR0jBBgwFoAUBhhmTRZN
fHlRYCwezjxoQMKQxgswDQYJKoZIhvcNAQELBQADggEBAGV4PSkztoi1vd26oruO
4b11Ylrx0qON9nXj0RJpARoGr3NY674jBITe8ZhSQUc178z08BeaBD1s9joXsMx4
pmWCJSPLWL/h7d5VcT3x+HxOFXgek1q/L4CzbkExPkzu2655JzYcsI18KWSziZ5i
gJDE82c2rYBwMzbKW5yZERPib/EDJP5I1FckApNZepHIp0zaxdgbsSj72nq7YWEs
cGNwawU8GNLRl4b7a87FDoJj5UG9Yh3CRQejz7CVNsmney0bhmNmoB7T4W5NzUsP
S8+eiZZouOkyMjDYK+piAfzKSttLOW2jbFDASqp4EGXzKqG4tM8oVXYsIWfIReRu
ldg=
-----END CERTIFICATE-----`

	clientKey := `-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQD0G/QbMDO3yKOS
OVU/4Tropo2TR/rtPlQWKJOKPjD2E87NE+XJeGOyLGWo2qSbkBJvJxYqEc/rtI7t
ef4h4oRd2T83QEdHSk2WH2SVel60qfO+tLCbqV08P06tfGNiqi69CkQn+mQpmxRo
flLlKLq1b4qpzM830l6uMhQ5MA/AOvso38zAX5DZ177tRpnsKGNrjU0HSUIhABTX
yMa2pCkwhI20DY7dhxpdjwnQM12OH/JzO7uKLdUq/MU0cMZRuQLtVclsvOIbEIWM
MF6/0iEzHOal9BLwiwxZ6JyI4EP/wF70Q1bQjZgRl+8y5sf+u8k+DEgxKCr2MgTU
sq48AZFDAgMBAAECggEAB38vE7SuKe520Fm3Aga424Z3iGnoZwFuhxDijLDU3Rts
bJG4Kv22n7UYWPazoalrjE+/F2l21FTPvOa6hMmwA5fVhqz2ydXNpMojBl1jOJlg
yHiqu3Hlajr0suqqiYNGrmL5BxQVoAEVjKrKGr3E+iewsph9I3twKyZgYGwKJJhu
9nrCccyDZHkOjW0KfaL2ppP6WRSSMl4LotBJnc8C//dDxX+zZkYoksII6jRzvPM2
CeVxXILa43AP5rifguG+/wTyjP4PG2c42Ra+Ac4DzHkMwQrwa0gIHQYswN5CRU3G
6Aha8KFt1jjwdEm4muV8Db8ZyeWneUz1mWZdRw/emQKBgQD+2U2zRpb1WNOE9j4w
a9/TgHih1+OYUdFkV9u/4Zc1oCEhjBTUzpnxGlhAlco6Cjw0RjGmrYQ9gCscPdh/
Oz8dPfZZcxSCuw4PFYjGu8OOoYNNeLfj6V8aqAFhROxICkL5EkRv5mZ8YWaPOqIn
MbEBcSaezdk7cVPBbLFUH/2DaQKBgQD1NjssV/fzE+EUeIp6E75LA06nAX2xX1L2
2uDMt/IGEscZLjUhcYS+M+LkuNtW2Yjgy/fzAlw1bMFjHIH9TYsLqAc7u3WcNAXW
7L3DksBPhvknjoZ+i8nkaLFeUG3XnLQaI3drk/vhL5q+dR+KK9PALXeixjHmvFum
Ry57kgTVywKBgQD9u5ky3xs5l4CxJwHv79dfms+AQ5QkeYGC6D6wIokMKSwTXIb5
AeIfPN2VIA3CD6K1YRXaH3RETzGc4q6ErpY+JQz7LirDpj1vIz+Urikb/w7duU1N
K3M29QK6t4aQizb3CQr+ZmSvfcJA5F3BrCXRi7ip78VS+5gqQm+jlF4x0QKBgGMN
AgAalLzq9cuYGY/Qc9jHQDkz3/sLH2854P6w+yG66hPg13Nn8JAIU4nCpk9B1gnA
Oqs989Nc2A1aEaQpc5ZEzI8zXQG4/fbgcJMUr3wwcGqrJubtPqN2KteHM6eZ1CKO
2wlooKFI4oA2vYPJymJhu2bUGooy4e6b6EngJPXbAoGBAJG3A9IYS01CIEe3Aqch
JvevQh041JhSVv78fVtY0YJNE3WZQ5M1GM0PLHIKRJ54DqFq979XVhzSw/t4TeNk
POZjvwZTtrr1jOLClXXnNaM9y/Fo+fVcdEU1M2yEITJOxPfEmejB/4Qeji2ARKtm
C6azzwqUOSsfDcuAS5sfJp/6
-----END PRIVATE KEY-----`

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	licenser := mocks.NewMockLicenser(ctrl)
	licenser.EXPECT().UseForwardProxy().Return(true)
	licenser.EXPECT().IpRules().AnyTimes().Return(true)

	dispatcher, err := NewDispatcher(
		licenser,
		fflag.NewFFlag([]string{string(fflag.IpRules)}),
		LoggerOption(log.NewLogger(os.Stdout)),
		ProxyOption("nil"),
		AllowListOption([]string{"0.0.0.0/0"}),
		BlockListOption([]string{"127.0.0.0/8"}),
	)
	require.NoError(t, err)

	// Build a tls.Certificate from PEMs
	cert, err := config.LoadClientCertificate(clientCert, clientKey)
	require.NoError(t, err)

	// Attempt to send a request with mTLS enabled
	_, err = dispatcher.SendWebhookWithMTLS(
		context.Background(),
		server.URL,
		nil,
		"X-Signature",
		"test-hmac",
		1024,
		nil,
		"",
		5*time.Second,
		cert,
	)

	// Should be blocked by netjail due to blocklist
	require.Error(t, err)
	require.ErrorIs(t, err, netjail.ErrDenied)
}
