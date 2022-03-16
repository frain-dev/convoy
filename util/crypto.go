package util

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"strings"
	"time"

	"github.com/dchest/uniuri"

	"github.com/frain-dev/convoy/config/algo"
	"golang.org/x/crypto/sha3"
)

var (
	prefix    []string = []string{"CO", "t=", "data="}
	seperator []string = []string{".", ","}
)

func ComputeJSONHmac(hash, data, secret string, order bool, timestamp bool) (string, error) {

	if order {
		d, err := JsonReMarshalString(data)
		if err != nil {
			return "", err
		}
		data = d
	}

	if timestamp {
		var d strings.Builder
		d.WriteString(prefix[1])
		d.WriteString(fmt.Sprint(time.Now().Unix()))
		d.WriteString(seperator[1])
		d.WriteString(prefix[2])
		d.WriteString(data)
		data = d.String()
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

	api_key.WriteString(prefix[0])
	api_key.WriteString(seperator[0])
	api_key.WriteString(mask)
	api_key.WriteString(seperator[0])
	api_key.WriteString(key)

	return mask, api_key.String()
}
