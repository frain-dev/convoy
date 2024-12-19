package keys

import (
	"fmt"
	"github.com/hashicorp/vault/api"
)

type VaultKeyManager struct {
	client *api.Client
}

// NewVaultKeyManager initializes the Vault client
func NewVaultKeyManager(vaultAddr, token string) (*VaultKeyManager, error) {
	// Configure Vault client
	config := &api.Config{
		Address: vaultAddr,
	}
	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault client: %w", err)
	}

	// Set token for authentication
	client.SetToken(token)

	return &VaultKeyManager{client: client}, nil
}

// GetCurrentKey retrieves the current encryption key from Vault
func (vkm *VaultKeyManager) GetCurrentKey() (string, error) {
	secret, err := vkm.client.Logical().Read("encryption/data/current_key")
	if err != nil {
		return "", fmt.Errorf("failed to read key from Vault: %w", err)
	}
	if secret == nil || secret.Data == nil {
		return "", fmt.Errorf("no encryption key found in Vault")
	}

	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("malformed key data in Vault")
	}

	key, ok := data["key"].(string)
	if !ok {
		return "", fmt.Errorf("key not found in Vault response")
	}

	return key, nil
}

// SetKey updates the current encryption key in Vault
func (vkm *VaultKeyManager) SetKey(newKey string) error {
	data := map[string]interface{}{
		"data": map[string]interface{}{
			"key": newKey,
		},
	}
	_, err := vkm.client.Logical().Write("encryption/data/current_key", data)
	if err != nil {
		return fmt.Errorf("failed to write key to Vault: %w", err)
	}
	return nil
}
