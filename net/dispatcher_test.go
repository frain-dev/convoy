package net

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"

	"github.com/frain-dev/convoy/config"
	"github.com/stretchr/testify/require"
)

var (
	successBody      = []byte("received webhook successfully")
	failureBody      = []byte("error occurred")
	pageNotFoundBody = []byte("404 page not found\n")
)

func TestDispatcher_SendRequest(t *testing.T) {
	client := http.DefaultClient
	type args struct {
		endpoint        string
		method          string
		jsonData        json.RawMessage
		signatureHeader string
		hmac            string
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
				endpoint:        "https://google.com",
				method:          http.MethodPost,
				jsonData:        bytes.NewBufferString("testing").Bytes(),
				signatureHeader: config.DefaultSignatureHeader.String(),
				hmac:            "12345",
			},
			want: &Response{
				Status:     "200",
				StatusCode: http.StatusOK,
				Method:     http.MethodPost,
				URL:        nil,
				RequestHeader: http.Header{
					"Content-Type":                         []string{"application/json"},
					"User-Agent":                           []string{string(DefaultUserAgent)},
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
			name: "should_error_for_wrong_endpoint",
			args: args{
				endpoint:        "https://localhost:9080",
				method:          http.MethodPost,
				jsonData:        bytes.NewBufferString("lol").Bytes(),
				signatureHeader: config.DefaultSignatureHeader.String(),
				hmac:            "12345",
			},
			want: &Response{
				Status:     "404",
				StatusCode: http.StatusNotFound,
				Method:     http.MethodPost,
				RequestHeader: http.Header{
					"Content-Type":                         []string{"application/json"},
					"User-Agent":                           []string{string(DefaultUserAgent)},
					config.DefaultSignatureHeader.String(): []string{"12345"}, // should equal hmac field above
				},
				ResponseHeader: nil,
				Body:           pageNotFoundBody,
				IP:             "",
				Error:          "",
			},
			nFn: func() func() {
				httpmock.Activate()

				httpmock.RegisterResponder(http.MethodPost, "https://localhost:9080",
					httpmock.NewStringResponder(http.StatusNotFound, string(pageNotFoundBody)))

				return func() {
					httpmock.DeactivateAndReset()
				}
			},
			wantErr: false,
		},
		{
			name: "should_refuse_connection_to_wrong_endpoint",
			args: args{
				endpoint:        "http://localhost:3234",
				method:          http.MethodPost,
				jsonData:        bytes.NewBufferString("bossman").Bytes(),
				signatureHeader: config.DefaultSignatureHeader.String(),
				hmac:            "12345",
			},
			want: &Response{
				Status:     "",
				StatusCode: 0,
				Method:     http.MethodPost,
				RequestHeader: http.Header{
					"Content-Type":                         []string{"application/json"},
					"User-Agent":                           []string{string(DefaultUserAgent)},
					config.DefaultSignatureHeader.String(): []string{"12345"}, // should equal hmac field above
				},
				ResponseHeader: nil,
				Body:           nil,
				IP:             "",
				Error:          "connect: connection refused",
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

			got, err := d.SendRequest(tt.args.endpoint, tt.args.method, tt.args.jsonData, tt.args.signatureHeader, tt.args.hmac)
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
