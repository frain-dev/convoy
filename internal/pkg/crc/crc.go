package crc

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"gopkg.in/guregu/null.v4"
)

type Crc interface {
	HandleRequest(w http.ResponseWriter, r *http.Request, source *datastore.Source, sourceRepo datastore.SourceRepository) error
}

type TwitterCrc struct {
	secret string
}

type TwitterCrcResponse struct {
	ResponseToken string `json:"response_token"`
}

func NewTwitterCrc(secret string) *TwitterCrc {
	return &TwitterCrc{secret: secret}
}

func (tc *TwitterCrc) HandleRequest(w http.ResponseWriter, r *http.Request, source *datastore.Source, sourceRepo datastore.SourceRepository) error {
	crcToken := r.URL.Query().Get("crc_token")

	h := hmac.New(sha256.New, []byte(tc.secret))
	h.Write([]byte(crcToken))
	computedMac := base64.StdEncoding.EncodeToString(h.Sum(nil))

	re := fmt.Sprintf("sha256=%s", computedMac)
	tr := &TwitterCrcResponse{ResponseToken: re}

	data, err := json.Marshal(tr)
	if err != nil {
		return err
	}

	source.ProviderConfig.Twitter.CrcVerifiedAt = null.TimeFrom(time.Now())
	err = sourceRepo.UpdateSource(r.Context(), source)
	if err != nil {
		return err
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write(data)
	if err != nil {
		return err
	}

	return nil
}
