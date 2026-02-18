package license

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
)

const gcmNonceSize = 12

// LicenseDataPayload is the structure stored in license_data (encrypted).
// It holds the org's license key and entitlements from the license service.
type LicenseDataPayload struct {
	Key          string                 `json:"key"`
	Entitlements map[string]interface{} `json:"entitlements,omitempty"`
}

// EncryptLicenseData encrypts a LicenseDataPayload for the given org and returns a base64 string.
func EncryptLicenseData(orgID string, payload *LicenseDataPayload) (string, error) {
	if payload == nil {
		return "", errors.New("payload is required")
	}
	plain, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	key := sha256.Sum256([]byte(orgID))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcmNonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, plain, nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

// DecryptLicenseData decrypts license_data for the given org and returns the payload.
func DecryptLicenseData(orgID, ciphertext string) (*LicenseDataPayload, error) {
	if ciphertext == "" {
		return nil, errors.New("ciphertext is required")
	}
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcmNonceSize {
		return nil, errors.New("ciphertext too short")
	}
	key := sha256.Sum256([]byte(orgID))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce, ct := raw[:gcmNonceSize], raw[gcmNonceSize:]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, err
	}
	var p LicenseDataPayload
	if err := json.Unmarshal(plain, &p); err != nil {
		return nil, err
	}
	return &p, nil
}
