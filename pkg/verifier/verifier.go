package verifier

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"hash"
	"net/http"
	"strings"
)

var (
	ErrAlgoNotFound                       = errors.New("Algorithm not found")
	ErrInvalidIP                          = errors.New("Source IP not supported")
	ErrCannotReadRequestBody              = errors.New("Failed to read request body")
	ErrHashDoesNotMatch                   = errors.New("Invalid Signature - Hash does not match")
	ErrCannotDecodeHexEncodedMACHeader    = errors.New("Cannot decode hex encoded MAC header")
	ErrCannotDecodeBase64EncodedMACHeader = errors.New("Cannot decode base64 encoded MAC header")
	ErrSignatureCannotBeEmpty             = errors.New("Signature cannot be empty")
	ErrAuthHeader                         = errors.New("Invalid Authorization header")
	ErrAuthHeaderCannotBeEmpty            = errors.New("Auth header cannot be empty")
	ErrInvalidHeaderStructure             = errors.New("Invalid header structure")
	ErrInvalidAuthLength                  = errors.New("Invalid Basic Auth Length")
	ErrInvalidEncoding                    = errors.New("Invalid header encoding")
)

type Verifier interface {
	VerifyRequest(r *http.Request, payload []byte) error
}

type HmacOptions struct {
	Header       string
	GetSignature func(string) string
	Hash         string
	Secret       string
	Encoding     string
}

type HmacVerifier struct {
	opts *HmacOptions
}

func NewHmacVerifier(opts *HmacOptions) *HmacVerifier {
	// TODO(subomi): assert that they're all non-nil values.

	return &HmacVerifier{opts}
}

func (hV *HmacVerifier) VerifyRequest(r *http.Request, payload []byte) error {
	hash, err := hV.getHashFunction(hV.opts.Hash)
	if err != nil {
		return err
	}

	signature := r.Header.Get(hV.opts.Header)

	if hV.opts.GetSignature != nil {
		signature = hV.opts.GetSignature(signature)
	}

	if len(strings.TrimSpace(signature)) == 0 {
		return ErrSignatureCannotBeEmpty
	}

	mac := hmac.New(hash, []byte(hV.opts.Secret))
	mac.Write(payload)
	computedMAC := mac.Sum(nil)

	var sentMAC []byte

	if hV.opts.Encoding == "hex" {
		sentMAC, err = hex.DecodeString(signature)
		if err != nil {
			return ErrCannotDecodeHexEncodedMACHeader
		}
	} else if hV.opts.Encoding == "base64" {
		sentMAC, err = base64.StdEncoding.DecodeString(signature)
		if err != nil {
			return ErrCannotDecodeBase64EncodedMACHeader
		}
	} else {
		return ErrInvalidEncoding
	}

	validMAC := hmac.Equal(sentMAC, computedMAC)
	if !validMAC {
		return ErrHashDoesNotMatch
	}

	return nil
}

func (hV *HmacVerifier) getHashFunction(algo string) (func() hash.Hash, error) {
	switch algo {
	case "SHA256":
		return sha256.New, nil
	case "SHA512":
		return sha512.New, nil
	default:
		return nil, ErrAlgoNotFound
	}
}

type BasicAuthVerifier struct {
	username string
	password string
}

func NewBasicAuthVerifier(username, password string) *BasicAuthVerifier {
	return &BasicAuthVerifier{
		username: username,
		password: password,
	}
}

func (baV *BasicAuthVerifier) VerifyRequest(r *http.Request, payload []byte) error {
	val := r.Header.Get("Authorization")
	authInfo := strings.Split(val, " ")

	if len(authInfo) != 2 {
		return ErrInvalidHeaderStructure
	}

	credentials, err := base64.StdEncoding.DecodeString(authInfo[1])
	if err != nil {
		return ErrInvalidHeaderStructure
	}

	creds := strings.Split(string(credentials), ":")

	if len(creds) != 2 {
		return ErrInvalidAuthLength
	}

	if creds[0] != baV.username || creds[1] != baV.password {
		return ErrAuthHeader
	}

	return nil
}

type APIKeyVerifier struct {
	key    string
	header string
}

func NewAPIKeyVerifier(key, header string) *APIKeyVerifier {
	return &APIKeyVerifier{
		key:    key,
		header: header,
	}
}

func (aV *APIKeyVerifier) VerifyRequest(r *http.Request, payload []byte) error {
	authHeader := "Authorization"

	if len(strings.TrimSpace(aV.header)) != 0 {
		authHeader = aV.header
		val := r.Header.Get(authHeader)

		if len(strings.TrimSpace(val)) == 0 {
			return ErrAuthHeader
		}

		if val != aV.key {
			return ErrAuthHeader
		}

		return nil
	}

	val := r.Header.Get(authHeader)
	authInfo := strings.Split(val, " ")

	if len(authInfo) != 2 {
		return ErrInvalidHeaderStructure
	}

	if authInfo[1] != aV.key {
		return ErrAuthHeader
	}

	return nil
}

type GithubVerifier struct {
	HmacOpts *HmacOptions
}

func NewGithubVerifier(secret string) *GithubVerifier {
	gv := &GithubVerifier{}
	gv.HmacOpts = &HmacOptions{
		Header:       "X-Hub-Signature-256",
		Hash:         "SHA256",
		GetSignature: gv.getSignature,
		Secret:       secret,
		Encoding:     "hex",
	}

	return gv
}

func (gV *GithubVerifier) VerifyRequest(r *http.Request, payload []byte) error {
	v := HmacVerifier{gV.HmacOpts}
	return v.VerifyRequest(r, payload)
}

func (gV *GithubVerifier) getSignature(sig string) string {
	values := strings.Split(sig, "sha256=")
	if len(values) < 2 {
		return ""
	}

	return values[1]
}

type ShopifyVerifier struct {
	HmacOpts *HmacOptions
}

func NewShopifyVerifier(secret string) *ShopifyVerifier {
	sv := &ShopifyVerifier{}
	sv.HmacOpts = &HmacOptions{
		Header:       "X-Shopify-Hmac-SHA256",
		Hash:         "SHA256",
		GetSignature: nil,
		Secret:       secret,
		Encoding:     "base64",
	}

	return sv
}

func (sv *ShopifyVerifier) VerifyRequest(r *http.Request, payload []byte) error {
	v := HmacVerifier{sv.HmacOpts}
	return v.VerifyRequest(r, payload)
}

type TwitterVerifier struct {
	HmacOpts *HmacOptions
}

func NewTwitterVerifier(secret string) *TwitterVerifier {
	tv := &TwitterVerifier{}
	tv.HmacOpts = &HmacOptions{
		Header:       "X-Twitter-Webhooks-Signature",
		Hash:         "SHA256",
		GetSignature: tv.getSignature,
		Secret:       secret,
		Encoding:     "base64",
	}

	return tv
}

func (tv *TwitterVerifier) VerifyRequest(r *http.Request, payload []byte) error {
	v := HmacVerifier{tv.HmacOpts}
	return v.VerifyRequest(r, payload)
}

func (tV *TwitterVerifier) getSignature(sig string) string {
	return strings.Split(sig, "sha256=")[1]
}

type NoopVerifier struct{}

func (nV *NoopVerifier) VerifyRequest(r *http.Request, payload []byte) error {
	return nil
}
