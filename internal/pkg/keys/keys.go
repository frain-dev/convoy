package keys

import (
	"errors"
	"sync/atomic"
)

var (
	// Define the table and columns mapping
	tablesAndColumns = map[string]map[string]string{
		"endpoints": {
			"secrets": "secrets_cipher",
			"authentication_type_api_key_header_value": "authentication_type_api_key_header_value_cipher",
		},
	}
)

type KeyManager interface {
	IsSet() bool
	GetCurrentKey() (string, error)
	SetKey(newKey string) error
}

var kmSingleton atomic.Value

func Set(km KeyManager) error {
	kmSingleton.Store(&km)
	return nil
}

// Get fetches the KeyManager at runtime. Set must have been called previously for this to work.
func Get() (KeyManager, error) {
	km, ok := kmSingleton.Load().(*KeyManager)
	if !ok {
		return nil, errors.New("call Set before this function")
	}

	return *km, nil
}
