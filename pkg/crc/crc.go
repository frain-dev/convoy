package crc

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
)

type Crc interface {
	ValidateRequest(r *http.Request) map[string]interface{}
}

type TwitterCrC struct {
	secret string
}

func NewTwitterCrc(secret string) *TwitterCrC {
	return &TwitterCrC{secret: secret}
}

func (tc *TwitterCrC) ValidateRequest(r *http.Request) map[string]interface{} {
	crcToken := r.URL.Query().Get("crc_token")

	h := hmac.New(sha256.New, []byte(tc.secret))
	h.Write([]byte(crcToken))
	computedMac := base64.StdEncoding.EncodeToString(h.Sum(nil))

	re := fmt.Sprintf("sha256=%s", computedMac)
	response := map[string]interface{}{"response_token": re}

	return response
}
