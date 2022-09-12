package util

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"strings"
	"time"

	"github.com/dchest/uniuri"

	"github.com/frain-dev/convoy/config/algo"
	"golang.org/x/crypto/sha3"
)

const (
	Prefix    string = "CO"
	Seperator string = "."
)

func ComputeJSONHmac(hash, data, secret string, order bool) (string, error) {

	if order {
		d, err := JsonReMarshalString(data)
		if err != nil {
			return "", err
		}
		data = d
	}

	fn, err := getHashFunction(hash)
	if err != nil {
		return "", err
	}

	h := hmac.New(fn, []byte(secret))
	h.Write([]byte(data))
	e := hex.EncodeToString(h.Sum(nil))

	return e, nil
}

type Signature struct {
	Timestamp   string
	Hmac        string
	EncodedData []byte
}

func GenerateSignatureHeader(replayAttacks bool, hash string, secret string, data json.RawMessage) (*Signature, error) {
	buf := bytes.NewBuffer([]byte{})
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)

	err := encoder.Encode(data)
	if err != nil {
		return nil, fmt.Errorf("failed to encode data: %v", err)
	}

	trimmedBuff := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))

	var signedPayload strings.Builder
	var timestamp string
	if replayAttacks {
		timestamp = fmt.Sprint(time.Now().Unix())
		signedPayload.WriteString(timestamp)
		signedPayload.WriteString(",")
	}
	signedPayload.WriteString(string(trimmedBuff))

	hmacStr, err := ComputeJSONHmac(hash, signedPayload.String(), secret, false)
	if err != nil {
		return nil, fmt.Errorf("error occurred while generating hmac: %v", err)
	}

	return &Signature{
		Timestamp:   timestamp,
		Hmac:        hmacStr,
		EncodedData: trimmedBuff,
	}, nil
}

func getHashFunction(algorithm string) (func() hash.Hash, error) {
	switch algorithm {
	case algo.MD5:
		return md5.New, nil
	case algo.SHA1:
		return sha1.New, nil
	case algo.SHA224:
		return sha256.New224, nil
	case algo.SHA256:
		return sha256.New, nil
	case algo.SHA384:
		return sha512.New384, nil
	case algo.SHA512:
		return sha512.New, nil
	case algo.SHA3_224:
		return sha3.New224, nil
	case algo.SHA3_256:
		return sha3.New256, nil
	case algo.SHA3_384:
		return sha3.New384, nil
	case algo.SHA3_512:
		return sha3.New512, nil
	case algo.SHA512_224:
		return sha512.New512_224, nil
	case algo.SHA512_256:
		return sha512.New512_256, nil
	}
	return nil, errors.New("unknown hash algorithm")
}

func GenerateSecret() (string, error) {
	return GenerateRandomString(25)
}

func GenerateAPIKey() (string, string) {
	mask := uniuri.NewLen(16)
	key := uniuri.NewLen(64)

	var api_key strings.Builder

	api_key.WriteString(Prefix)
	api_key.WriteString(Seperator)
	api_key.WriteString(mask)
	api_key.WriteString(Seperator)
	api_key.WriteString(key)

	return mask, api_key.String()
}
