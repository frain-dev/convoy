package net

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/server/models"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const (
	host1 = "127.0.0.1:2091"
	host2 = "127.0.0.1:2092"
)

const (
	successEndpoint = "/success"
	failEndpoint    = "/failure"
)

var (
	server1 = &http.Server{
		Addr: host1,
	}

	server2 = &http.Server{
		Addr: host2,
	}

	successBody      = []byte("received webhook successfully")
	failureBody      = []byte("error occurred")
	pageNotFoundBody = []byte("404 page not found\n")
)

func TestMain(m *testing.M) {
	mux := http.DefaultServeMux
	logger := log.WithFields(map[string]interface{}{})
	mux.HandleFunc(successEndpoint, func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.WithError(err).Fatal("failed to read body")
		}

		logger.WithField("request_body", string(body))

		w.Header()[http.CanonicalHeaderKey("Request_ID")] = []string{"abcd"}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(successBody)

	})

	mux.HandleFunc(failEndpoint, func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.WithError(err).Fatal("failed to read body")
		}

		logger.WithField("request_body", string(body))

		w.Header()[http.CanonicalHeaderKey("Request_ID")] = []string{"abcd"}

		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(failureBody)
	})

	server1.Handler = mux
	server2.Handler = mux

	go func() {
		if err := server1.ListenAndServe(); err != http.ErrServerClosed {
			log.WithError(err).Fatal("server1 exited")
		}
	}()

	go func() {
		if err := server2.ListenAndServe(); err != http.ErrServerClosed {
			log.WithError(err).Fatal("server2 exited")
		}
	}()

	// allow the servers start
	time.Sleep(2 * time.Second)
	code := m.Run()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_ = server1.Shutdown(ctx)
	_ = server2.Shutdown(ctx)

	os.Exit(code)
}

// serialize obj into json bytes
func serialize(obj interface{}) *bytes.Buffer {
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(obj); err != nil {
		log.WithError(err).Fatalf("unable to serialize obj")
	}
	return buf
}

func rawMessage(obj interface{}) json.RawMessage {
	buf := serialize(obj)
	return buf.Bytes()
}

func formatEndpoint(host, endpoint string) string {
	return fmt.Sprintf("http://%s%s", host, endpoint)
}

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
		wantErr bool
	}{
		{
			name: "should_send_message",
			args: args{
				endpoint: formatEndpoint(host1, successEndpoint),
				method:   http.MethodPost,
				jsonData: rawMessage(&models.Message{
					MessageID:  "12322",
					AppID:      "24244",
					EventType:  "test.charge",
					ProviderID: "",
					Data:       nil,
					Status:     "success",
					CreatedAt:  int64(primitive.NewDateTimeFromTime(time.Now())),
				}),
				signatureHeader: config.DefaultSignatureHeader.String(),
				hmac:            "12345",
			},
			want: &Response{
				Status:     "200 OK",
				StatusCode: http.StatusOK,
				Method:     http.MethodPost,
				URL:        nil,
				RequestHeader: http.Header{
					"Content-Type":                         []string{"application/json"},
					"User-Agent":                           []string{string(DefaultUserAgent)},
					config.DefaultSignatureHeader.String(): []string{"12345"}, // should equal hmac field above
				},
				ResponseHeader: http.Header{
					http.CanonicalHeaderKey("Request_ID"): []string{"abcd"},
				},
				Body:  successBody,
				IP:    server1.Addr,
				Error: "",
			},
			wantErr: false,
		},
		{
			name: "should_error_for_wrong_endpoint",
			args: args{
				endpoint: formatEndpoint(host2, "/undefined"),
				method:   http.MethodPost,
				jsonData: rawMessage(&models.Message{
					MessageID:  "12322",
					AppID:      "24244",
					EventType:  "test.charge",
					ProviderID: "",
					Data:       nil,
					Status:     "success",
					CreatedAt:  int64(primitive.NewDateTimeFromTime(time.Now())),
				}),
				signatureHeader: config.DefaultSignatureHeader.String(),
				hmac:            "12345",
			},
			want: &Response{
				Status:     "404 Not Found",
				StatusCode: http.StatusNotFound,
				Method:     http.MethodPost,
				RequestHeader: http.Header{
					"Content-Type":                         []string{"application/json"},
					"User-Agent":                           []string{string(DefaultUserAgent)},
					config.DefaultSignatureHeader.String(): []string{"12345"}, // should equal hmac field above
				},
				ResponseHeader: http.Header{},
				Body:           pageNotFoundBody,
				IP:             server2.Addr,
				Error:          "",
			},
			wantErr: false,
		},
		{
			name: "should_refuse_connection_to_wrong_endpoint",
			args: args{
				endpoint: formatEndpoint("localhost:3023", "/undefined"),
				method:   http.MethodPost,
				jsonData: rawMessage(&models.Message{
					MessageID:  "12322",
					AppID:      "24244",
					EventType:  "test.charge",
					ProviderID: "",
					Data:       nil,
					Status:     "success",
					CreatedAt:  int64(primitive.NewDateTimeFromTime(time.Now())),
				}),
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
				ResponseHeader: http.Header{},
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

			got, err := d.SendRequest(tt.args.endpoint, tt.args.method, tt.args.jsonData, tt.args.signatureHeader, tt.args.hmac)
			if (err != nil) != tt.wantErr {
				t.Errorf("SendRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			require.Contains(t, got.Error, tt.want.Error)
			require.Equal(t, tt.want.Status, got.Status)
			require.Equal(t, tt.want.StatusCode, got.StatusCode)
			require.Equal(t, tt.want.Method, got.Method)
			require.Equal(t, tt.want.IP, got.IP)
			require.Equal(t, tt.want.Body, got.Body)

			require.Equal(t, tt.want.RequestHeader, got.RequestHeader)
			require.Equal(t, tt.want.ResponseHeader[http.CanonicalHeaderKey("Request_ID")], got.ResponseHeader[http.CanonicalHeaderKey("Request_ID")])
		})
	}
}
