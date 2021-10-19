package net

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"

	"github.com/frain-dev/convoy/server/models"

	fuzz "github.com/google/gofuzz"

	log "github.com/sirupsen/logrus"
)

const (
	host1 = "http://localhost:2091"
	host2 = "http://localhost:2092"
	host3 = "http://localhost:2093"
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
)

func TestMain(m *testing.M) {
	mux := http.DefaultServeMux
	logger := log.WithFields(map[string]interface{}{})
	mux.HandleFunc(successEndpoint, func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.WithError(err).Fatal("failed to read body")
		}

		logger.WithField("request_body", body)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("received webhook successfully"))

		h := w.Header()
		h["RequestID"] = []string{"abcd"}
	})

	mux.HandleFunc(failEndpoint, func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.WithError(err).Fatal("failed to read body")
		}

		logger.WithField("request_body", body)

		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error occurred"))

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

	code := m.Run()
	ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
	server1.Shutdown(ctx)
	server2.Shutdown(ctx)
	server3.Shutdown(ctx)

	os.Exit(code)
}

// serialize obj into json bytes
func serialize(obj interface{}) *bytes.Buffer {
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(obj); err != nil {
		log.WithError(err).Fatalf("unable to serialized obj")
	}
	return buf
}

func fuzzObj(obj interface{}) json.RawMessage {
	fuzz.New().Fuzz(obj)
	buf := serialize(obj)
	return buf.Bytes()
}

func TestDispatcher_SendRequest(t *testing.T) {
	type fields struct {
		client *http.Client
	}
	type args struct {
		endpoint        string
		method          string
		jsonData        json.RawMessage
		signatureHeader string
		hmac            string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *Response
		wantErr bool
	}{
		{
			name:   "should_send_message",
			fields: fields{client: http.DefaultClient},
			args: args{
				endpoint:        host1 + successEndpoint,
				method:          http.MethodPost,
				jsonData:        fuzzObj(&models.Message{}),
				signatureHeader: config.DefaultSignatureHeader.String(),
				hmac:            "",
			},
			want: &Response{
				Status:     http.StatusText(http.StatusOK),
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
				Body:  nil,
				IP:    server1.Addr,
				Error: "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Dispatcher{
				client: tt.fields.client,
			}
			got, err := d.SendRequest(tt.args.endpoint, tt.args.method, tt.args.jsonData, tt.args.signatureHeader, tt.args.hmac)
			if (err != nil) != tt.wantErr {
				t.Errorf("SendRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			require.Equal(t, tt.want.Error, got.Error)

		})
	}
}
