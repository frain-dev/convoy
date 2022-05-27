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

var ErrAlgoNotFound = errors.New("Algorithm not found")
var ErrInvalidIP = errors.New("Source IP not supported")
var ErrCannotReadRequestBody = errors.New("Failed to read request body")
var ErrHashDoesNotMatch = errors.New("Invalid Signature - Hash does not match")
var ErrCannotDecodeHexEncodedMACHeader = errors.New("Cannot decode hex encoded MAC header")
var ErrCannotDecodeBase64EncodedMACHeader = errors.New("Cannot decode base64 encoded MAC header")
var ErrSignatureCannotBeEmpty = errors.New("Signature cannot be empty")
var ErrAuthHeader = errors.New("Invalid Authorization header")
var ErrAuthHeaderCannotBeEmpty = errors.New("Auth header cannot be empty")
var ErrInvalidHeaderStructure = errors.New("Invalid header structure")
var ErrInvalidAuthLength = errors.New("Invalid Basic Auth Length")
var ErrInvalidEncoding = errors.New("Invalid header encoding")

type Verifier interface {
	VerifyRequest(r *http.Request, payload []byte) error
}

type HmacVerifier struct {
	header   string
	hash     string
	secret   string
	encoding string
}

func NewHmacVerifier(header, hash, secret, encoding string) *HmacVerifier {
	// TODO(subomi): assert that they're all non-nil values.

	return &HmacVerifier{
		header:   header,
		hash:     hash,
		secret:   secret,
		encoding: encoding,
	}
}

func (hV *HmacVerifier) VerifyRequest(r *http.Request, payload []byte) error {
	hash, err := hV.getHashFunction(hV.hash)
	if err != nil {
		return err
	}

	rHeader := r.Header.Get(hV.header)

	if len(strings.TrimSpace(rHeader)) == 0 {
		return ErrSignatureCannotBeEmpty
	}

	mac := hmac.New(hash, []byte(hV.secret))
	mac.Write(payload)
	computedMAC := mac.Sum(nil)

	var sentMAC []byte

	if hV.encoding == "hex" {
		sentMAC, err = hex.DecodeString(rHeader)
		if err != nil {
			return ErrCannotDecodeHexEncodedMACHeader
		}
	} else if hV.encoding == "base64" {
		sentMAC, err = base64.StdEncoding.DecodeString(rHeader)
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

	if creds[0] != baV.username && creds[1] != baV.password {
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
