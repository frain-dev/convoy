package keys

import (
	"fmt"
	"os"
)

type LocalKeyManager struct {
	currentKey string
	isSet      bool
}

func NewLocalKeyManager() (*LocalKeyManager, error) {
	currentKey := os.Getenv("CONVOY_LOCAL_ENCRYPTION_KEY")
	if currentKey == "" {
		return nil, fmt.Errorf("current key must not be empty")
	}
	return &LocalKeyManager{
		currentKey: currentKey,
		isSet:      true,
	}, nil
}

func (l *LocalKeyManager) IsSet() bool {
	return l.isSet
}

func (l *LocalKeyManager) GetCurrentKey() (string, error) {
	if l.currentKey == "" {
		return "", fmt.Errorf("no current key configured")
	}
	return l.currentKey, nil
}

func (l *LocalKeyManager) GetCurrentKeyFromCache() (string, error) {
	return l.GetCurrentKey()
}

func (l *LocalKeyManager) SetKey(newKey string) error {
	if newKey == "" {
		return fmt.Errorf("new key must not be empty")
	}
	l.currentKey = newKey
	return nil
}
