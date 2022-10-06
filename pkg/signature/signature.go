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
	"strings"
	"time"
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
	// Secret represents the list of currently active secrets used for
	// a signing scheme. It is used to implement rolled secrets.
	// It's order is irrelevant.
	Secret []string

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
	tBuf, err := s.encodePayload()
	if err != nil {
		return "", err
	}

	if !s.Advanced {
		sch := s.Schemes[len(s.Schemes)-1]
		sec := sch.Secret[len(sch.Secret)-1]

		sig, err := s.generateSignature(sch, sec, tBuf)
		if err != nil {
			return "", err
		}

		return sig, nil
	}

	var hStr strings.Builder

	// Add timestamp.
	t := fmt.Sprintf("t=%d,", time.Now().Unix())
	hStr.WriteString(t)

	for k, sch := range s.Schemes {
		v := fmt.Sprintf("v%d=", k)

		var hSig string
		for _, sec := range sch.Secret {
			sig, err := s.generateSignature(sch, sec, tBuf)
			if err != nil {
				return "", err
			}

			hSig = fmt.Sprintf("%s%s,", v, sig)
			hStr.WriteString(hSig)
		}

	}

	return hStr.String(), nil
}

func (s *Signature) generateSignature(sch Scheme, sec string, buf []byte) (string, error) {
	var sig string
	var err error
	switch sch.Encoding {
	case "hex":
		if sig, err = s.generateHexSignature(sch.Hash, sec, buf); err != nil {
			return "", err
		}
	case "base64":
		if sig, err = s.generateBase64Signature(sch.Hash, sec, buf); err != nil {
			return "", err
		}
	default:
		return "", ErrInvalidEncoding
	}

	return sig, nil
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

func (s *Signature) encodePayload() ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)

	err := encoder.Encode(s.Payload)
	if err != nil {
		return nil, err
	}

	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}
