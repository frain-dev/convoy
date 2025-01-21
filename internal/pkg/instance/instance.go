package instance

import (
	"context"
	"encoding/json"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/pkg/log"
	"os"
)

const (
	DefaultInstancePassphrase = "default-instance-passphrase"
	OrganisationScope         = "organisation"
	ProjectScope              = "project"
	KeyInstanceIngestRate     = "InstanceIngestRate"
	KeyRetentionPolicy        = "RetentionPolicy"
	KeyStaticIP               = "StaticIP"
	KeyEnterpriseSSO          = "EnterpriseSSO"
)

type Boolean struct {
	Value bool `json:"value"`
}

// IngestRate is a Wrapper for InstanceIngestRate int
type IngestRate struct {
	Value int `json:"value"`
}

type Defaults struct {
	InstanceIngestRate IngestRate                          `json:"instance_ingest_rate"`
	RetentionPolicy    config.RetentionPolicyConfiguration `json:"retention_policy"`
}

func GetEncryptionPassphrase() string {
	encryptionPassphrase := os.Getenv("CONVOY_INSTANCE_ENCRYPTION_PASSPHRASE")
	if encryptionPassphrase == "" {
		encryptionPassphrase = DefaultInstancePassphrase
	}
	return encryptionPassphrase
}

func FetchDecryptedOverrides(ctx context.Context, db database.Database, key string, scopeType, scopeId string, model interface{}) (string, error) {
	encryptionPassphrase := GetEncryptionPassphrase()

	var decryptedValue string

	query := `
        SELECT pgp_sym_decrypt(value_cipher::bytea, CONCAT($1::text, '-', id))
        FROM convoy.instance_overrides
        WHERE key = $2 AND scope_type = $3 AND scope_id = $4
    `

	err := db.GetReadDB().QueryRowContext(ctx, query, encryptionPassphrase, key, scopeType, scopeId).Scan(&decryptedValue)
	if err != nil {
		return "", err
	}

	err = json.Unmarshal([]byte(decryptedValue), model)
	if err != nil {
		log.WithError(err).Error("error unmarshalling decrypted JSON")
		return "", err
	}

	return decryptedValue, nil
}
