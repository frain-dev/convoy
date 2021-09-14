package util

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"github.com/frain-dev/convoy/config/algo"
	"golang.org/x/crypto/sha3"
	"hash"
)

func ComputeJSONHmac(hash string, data string, secret string, order bool) (string, error) {

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
