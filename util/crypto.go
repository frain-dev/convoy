package util

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

func ComputeJSONHmac(secret, data string, order bool) (string, error) {

	if order {
		d, err := JsonReMarshalString(data)
		if err != nil {
			return "", err
		}
		data = d
	}

	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	sha := hex.EncodeToString(h.Sum(nil))

	return sha, nil
}

func GenerateSecret() (string, error) {
	return GenerateRandomString(25)
}
