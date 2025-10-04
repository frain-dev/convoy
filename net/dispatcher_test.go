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
	"github.com/frain-dev/convoy/pkg/constants"
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

func TestDispatcher_Ping(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLicenser := mocks.NewMockLicenser(ctrl)
	fflag := fflag.NewFFlag([]string{})

	// Setup default mocks
	mockLicenser.EXPECT().IpRules().Return(false).AnyTimes()

	// Test cases
	tests := []struct {
		name          string
		endpoint      string
		contentType   string
		serverHandler http.HandlerFunc
		expectedError bool
	}{
		{
			name:        "HEAD method succeeds",
			endpoint:    "/test",
			contentType: "",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "HEAD" {
					w.WriteHeader(http.StatusOK)
				} else {
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			},
			expectedError: false,
		},
		{
			name:        "GET method succeeds after HEAD fails",
			endpoint:    "/test",
			contentType: "",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" {
					w.WriteHeader(http.StatusOK)
				} else {
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			},
			expectedError: false,
		},
		{
			name:        "POST method succeeds with JSON content type",
			endpoint:    "/test",
			contentType: constants.ContentTypeJSON,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "POST" && r.Header.Get("Content-Type") == constants.ContentTypeJSON {
					w.WriteHeader(http.StatusOK)
				} else {
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			},
			expectedError: false,
		},
		{
			name:        "POST method succeeds with form content type",
			endpoint:    "/test",
			contentType: constants.ContentTypeFormURLEncoded,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "POST" && r.Header.Get("Content-Type") == constants.ContentTypeFormURLEncoded {
					w.WriteHeader(http.StatusOK)
				} else {
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			},
			expectedError: false,
		},
		{
			name:        "All methods fail",
			endpoint:    "/test",
			contentType: "",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(tt.serverHandler)
			defer server.Close()

			// Create dispatcher
			dispatcher, err := NewDispatcher(mockLicenser, fflag, LoggerOption(log.NewLogger(os.Stdout)))
			require.NoError(t, err)

			// Test ping
			ctx := context.Background()
			err = dispatcher.Ping(ctx, server.URL+tt.endpoint, 5*time.Second, tt.contentType)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDispatcher_tryPingMethod(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLicenser := mocks.NewMockLicenser(ctrl)
	fflag := fflag.NewFFlag([]string{})

	// Setup default mocks
	mockLicenser.EXPECT().IpRules().Return(false).AnyTimes()

	dispatcher, err := NewDispatcher(mockLicenser, fflag, LoggerOption(log.NewLogger(os.Stdout)))
	require.NoError(t, err)

	tests := []struct {
		name           string
		method         string
		contentType    string
		serverHandler  http.HandlerFunc
		expectedError  bool
		expectedStatus int
	}{
		{
			name:        "HEAD method succeeds",
			method:      "HEAD",
			contentType: "",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "HEAD", r.Method)
				w.WriteHeader(http.StatusOK)
			},
			expectedError: false,
		},
		{
			name:        "GET method succeeds",
			method:      "GET",
			contentType: "",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "GET", r.Method)
				w.WriteHeader(http.StatusOK)
			},
			expectedError: false,
		},
		{
			name:        "POST method with JSON content type",
			method:      "POST",
			contentType: constants.ContentTypeJSON,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "POST", r.Method)
				require.Equal(t, constants.ContentTypeJSON, r.Header.Get("Content-Type"))
				w.WriteHeader(http.StatusOK)
			},
			expectedError: false,
		},
		{
			name:        "POST method with form content type",
			method:      "POST",
			contentType: constants.ContentTypeFormURLEncoded,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "POST", r.Method)
				require.Equal(t, constants.ContentTypeFormURLEncoded, r.Header.Get("Content-Type"))
				w.WriteHeader(http.StatusOK)
			},
			expectedError: false,
		},
		{
			name:        "Method returns 404",
			method:      "GET",
			contentType: "",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			expectedError: true,
		},
		{
			name:        "Method returns 500",
			method:      "GET",
			contentType: "",
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(tt.serverHandler)
			defer server.Close()

			// Test tryPingMethod
			ctx := context.Background()
			err := dispatcher.tryPingMethod(ctx, server.URL, tt.method, tt.contentType)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDispatcher_PingWithDefaultMethods(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLicenser := mocks.NewMockLicenser(ctrl)
	fflag := fflag.NewFFlag([]string{})

	// Setup default mocks
	mockLicenser.EXPECT().IpRules().Return(false).AnyTimes()

	dispatcher, err := NewDispatcher(mockLicenser, fflag, LoggerOption(log.NewLogger(os.Stdout)))
	require.NoError(t, err)

	// Test with default ping methods (HEAD, GET, POST)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	ctx := context.Background()
	err = dispatcher.Ping(ctx, server.URL, 5*time.Second, "")
	require.NoError(t, err) // Should succeed with default methods
}

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
					"Content-Type":                         []string{constants.ContentTypeJSON},
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
					"Content-Type":                         []string{constants.ContentTypeJSON},
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
					"Content-Type":                         []string{constants.ContentTypeJSON},
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
					"Content-Type":                         []string{constants.ContentTypeJSON},
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

			got, err := d.SendWebhook(context.Background(), tt.args.endpoint, tt.args.jsonData, tt.args.project.Config.Signature.Header.String(), tt.args.hmac, config.MaxResponseSize, tt.args.headers, "", time.Minute, "application/json")
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
		constants.ContentTypeJSON,
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
		constants.ContentTypeJSON,
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
		constants.ContentTypeJSON,
	)

	// Assert that the request was blocked
	require.Error(t, err)
	require.ErrorIs(t, err, netjail.ErrDenied)
	require.Contains(t, err.Error(), "127.0.0.1: address not allowed")
}
