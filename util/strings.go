package util

import (
	"crypto/rand"
	"strings"
)

// IsStringEmpty checks if the given string s is empty or not
func IsStringEmpty(s string) bool { return len(strings.TrimSpace(s)) == 0 }

var letterBytes = []byte("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_-")

const (
	letterIdxMask = byte(63)
)

func GenerateRandomString(n int) (string, error) {
	if n <= 0 {
		return "", nil
	}

	b := make([]byte, n)
	randomBytes := make([]byte, n)
	for i := 0; i < n; {
		if _, err := rand.Read(randomBytes); err != nil {
			return "", err
		}
		for _, rb := range randomBytes {
			if idx := int(rb & letterIdxMask); idx < len(letterBytes) {
				b[i] = letterBytes[idx]
				i++
				if i == n {
					break
				}
			}
		}
	}

	return string(b), nil
}

func StringSliceContains(sl []string, s string) bool {
	for _, v := range sl {
		if v == s {
			return true
		}
	}

	return false
}
