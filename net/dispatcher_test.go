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
	host3 = "127.0.0.1:2093"
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

	server3 = &http.Server{
		Addr: host3,
	}

	successBody = []byte("received webhook successfully")
	failureBody = []byte("error occurred")
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

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(successBody)

		h := w.Header()
		h["RequestID"] = []string{"abcd"}
	})

	mux.HandleFunc(failEndpoint, func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.WithError(err).Fatal("failed to read body")
		}

		logger.WithField("request_body", string(body))

		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(failureBody)

		h := w.Header()
		h["RequestID"] = []string{"abcd"}
	})

	server1.Handler = mux
	server2.Handler = mux
	server3.Handler = mux

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

	go func() {
		if err := server3.ListenAndServe(); err != http.ErrServerClosed {
			log.WithError(err).Fatal("server3 exited")
		}
	}()

	// allow the servers start
	time.Sleep(2 * time.Second)
	code := m.Run()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_ = server1.Shutdown(ctx)
	_ = server2.Shutdown(ctx)
	_ = server3.Shutdown(ctx)

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
				hmac:            "",
			},
			want: &Response{
				Status:     "200 OK",
				StatusCode: http.StatusOK,
				Method:     http.MethodPost,
				URL:        nil,
				RequestHeader: http.Header{
					"Content-Type": []string{"application/json"},
					"User-Agent":   []string{string(DefaultUserAgent)},
				},
				ResponseHeader: http.Header{
					"RequestID": []string{"abcd"},
				},
				Body:  successBody,
				IP:    server1.Addr,
				Error: "",
			},
			wantErr: false,
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

			require.Equal(t, tt.want.Error, got.Error)
			require.Equal(t, tt.want.Status, got.Status)
			require.Equal(t, tt.want.StatusCode, got.StatusCode)
			require.Equal(t, tt.want.Method, got.Method)
			require.Equal(t, tt.want.IP, got.IP)
			require.Equal(t, tt.want.Body, got.Body)
		})
	}
}
