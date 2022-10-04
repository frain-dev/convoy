package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
)

var (
	// ErrFailedToEncodePayload is the error we return when we cannot encode webhook payload
	ErrFailedToEncodePayload = errors.New("Failed to encode payload")

	// ErrInvalidEncoding is the error returned when an invalid encoding is provided.
	// TODO(subomi): Can I use this format where I'm using this error
	ErrInvalidEncoding = errors.New("Invalid encoding format - %s")

	// ErrInvalidHash is the error returned when a unsupported hash is supplied.
	ErrInvalidHash = errors.New("Hash not supported")
)

type Scheme struct {
	Secret   []string
	Hash     string
	Encoding string
}

type Signature struct {
	Payload json.RawMessage

	// The order of this Schemes is a core part of this API.
	// We use the index as the version number. That is:
	// Index 0 = v0, Index 1 = v1
	Schemes []Scheme

	// This flag allows for backward-compatible implemtation
	// of this type. You're either generating a simplistic header
	// or a complex header.
	Advanced bool

	// Cached value
	computedValue string
}

func (s *Signature) ComputeHeaderValue() (string, error) {
	fmt.Println(string(s.Payload))
	buf := bytes.NewBuffer([]byte{})
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)

	err := encoder.Encode(s.Payload)
	if err != nil {
		return "", err
	}

	fmt.Println(buf.String())

	tBuf := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))

	sch := s.Schemes[len(s.Schemes)-1]
	sec := sch.Secret[len(sch.Secret)-1]

	var hStr string
	switch sch.Encoding {
	case "hex":
		if hStr, err = s.generateHexSignature(sch.Hash, sec, tBuf); err != nil {
			return "", err
		}
	case "base64":
		if hStr, err = s.generateBase64Signature(sch.Hash, sec, tBuf); err != nil {
			return "", err
		}
	default:
		return "", ErrInvalidEncoding
	}

	return hStr, nil
}

func (s *Signature) generateHexSignature(hash, secret string, buf []byte) (string, error) {
	h, err := s.signPayload(hash, secret, buf)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(h), nil
}

func (s *Signature) generateBase64Signature(hash, secret string, buf []byte) (string, error) {
	h, err := s.signPayload(hash, secret, buf)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(h), nil
}

func (s *Signature) signPayload(hash, secret string, buf []byte) ([]byte, error) {
	fn, err := s.getHashFunction(hash)
	if err != nil {
		return nil, err
	}

	h := hmac.New(fn, []byte(secret))
	h.Write(buf)

	return h.Sum(nil), nil
}

func (s *Signature) getHashFunction(algo string) (func() hash.Hash, error) {
	switch algo {
	case "SHA256":
		return sha256.New, nil
	case "SHA512":
		return sha512.New, nil
	default:
		return nil, ErrInvalidHash
	}
}
