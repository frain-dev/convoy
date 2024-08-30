package net

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"testing"
	"time"

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
				Status:     "200",
				StatusCode: http.StatusOK,
				Method:     http.MethodPost,
				URL:        nil,
				RequestHeader: http.Header{
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
			d := &Dispatcher{client: client}

			if tt.nFn != nil {
				deferFn := tt.nFn()
				defer deferFn()
			}

			got, err := d.SendRequest(context.Background(), tt.args.endpoint, tt.args.method, tt.args.jsonData, tt.args.project.Config.Signature.Header.String(), tt.args.hmac, config.MaxResponseSize, tt.args.headers, "", time.Minute)
			if tt.wantErr {
				require.NotNil(t, err)
				require.Contains(t, err.Error(), tt.want.Error)
				require.Contains(t, got.Error, tt.want.Error)
			}

			require.Equal(t, tt.want.Status, got.Status)
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
		mockFn     func(licenser license.Licenser)
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
			d, err := NewDispatcher(tt.args.httpProxy, licenser, tt.args.enforceSecure)
			if tt.wantErr {
				require.Error(t, err)
				require.Equal(t, tt.wantErrMsg, err.Error())
				return
			}

			require.NoError(t, err)

			if tt.wantProxy {
				require.NotNil(t, d.client.Transport.(*http.Transport).Proxy)
			}
		})
	}
}
