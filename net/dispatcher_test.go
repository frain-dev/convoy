package net

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"

	"github.com/frain-dev/convoy/config"
	"github.com/stretchr/testify/require"
)

var (
	successBody = []byte("received webhook successfully")
)

func TestDispatcher_SendRequest(t *testing.T) {
	client := http.DefaultClient

	buf := make([]byte, config.MaxResponseSize*2)

	_, _ = rand.Read(buf)
	timestamp := fmt.Sprint(time.Now().Unix())

	type args struct {
		endpoint        string
		method          string
		jsonData        json.RawMessage
		signatureHeader string
		convoyTimestamp string
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
				convoyTimestamp: timestamp,
				hmac:            "12345",
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
					"Convoy-Timestamp":                     []string{timestamp},
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
				endpoint:        "https://google.com",
				method:          http.MethodPost,
				jsonData:        bytes.NewBufferString("testing").Bytes(),
				signatureHeader: config.DefaultSignatureHeader.String(),
				convoyTimestamp: timestamp,
				hmac:            "12345",
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
					"Convoy-Timestamp":                     []string{timestamp}},
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
				endpoint:        "http://localhost:3234",
				method:          http.MethodPost,
				jsonData:        bytes.NewBufferString("bossman").Bytes(),
				signatureHeader: config.DefaultSignatureHeader.String(),
				hmac:            "12345",
				convoyTimestamp: timestamp,
			},
			want: &Response{
				Status:     "",
				StatusCode: 0,
				Method:     http.MethodPost,
				RequestHeader: http.Header{
					"Content-Type":                         []string{"application/json"},
					"User-Agent":                           []string{defaultUserAgent()},
					config.DefaultSignatureHeader.String(): []string{"12345"}, // should equal hmac field above
					"Convoy-Timestamp":                     []string{timestamp},
				},
				ResponseHeader: nil,
				Body:           nil,
				IP:             "",
				Error:          "connect: connection refused",
			},
			wantErr: true,
		},
		{
			name: "should_error_for_empty_signaturehmac",
			args: args{
				endpoint:        "http://localhost:3234",
				method:          http.MethodPost,
				jsonData:        bytes.NewBufferString("bossman").Bytes(),
				signatureHeader: config.DefaultSignatureHeader.String(),
				hmac:            "",
				convoyTimestamp: timestamp,
			},
			want: &Response{
				Status:         "",
				StatusCode:     0,
				Method:         "",
				RequestHeader:  nil,
				ResponseHeader: nil,
				Body:           nil,
				IP:             "",
				Error:          "signature header, hmac signature and hmac timestamp are required",
			},
			wantErr: true,
		},
		{
			name: "should_error_for_empty_timestamphmac",
			args: args{
				endpoint:        "http://localhost:3234",
				method:          http.MethodPost,
				jsonData:        bytes.NewBufferString("bossman").Bytes(),
				signatureHeader: config.DefaultSignatureHeader.String(),
				hmac:            "css",
				convoyTimestamp: "",
			},
			want: &Response{
				Status:         "",
				StatusCode:     0,
				Method:         "",
				RequestHeader:  nil,
				ResponseHeader: nil,
				Body:           nil,
				IP:             "",
				Error:          "signature header, hmac signature and hmac timestamp are required",
			},
			wantErr: true,
		},
		{
			name: "should_error_for_empty_signature_header",
			args: args{
				endpoint:        "http://localhost:3234",
				method:          http.MethodPost,
				jsonData:        bytes.NewBufferString("bossman").Bytes(),
				signatureHeader: "",
				hmac:            "css",
				convoyTimestamp: timestamp,
			},
			want: &Response{
				Status:         "",
				StatusCode:     0,
				Method:         "",
				RequestHeader:  nil,
				ResponseHeader: nil,
				Body:           nil,
				IP:             "",
				Error:          "signature header, hmac signature and hmac timestamp are required",
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

			got, err := d.SendRequest(tt.args.endpoint, tt.args.method, tt.args.jsonData, tt.args.signatureHeader, tt.args.hmac, tt.args.convoyTimestamp, config.MaxResponseSize)
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
