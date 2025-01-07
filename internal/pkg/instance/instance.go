package instance

import (
	"context"
	"encoding/json"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/oklog/ulid/v2"
	"os"
)

const (
	DefaultInstancePassphrase = "default-instance-passphrase"
	OrganisationScope         = "organisation"
	ProjectScope              = "project"
	KeyInstanceIngestRate     = "InstanceIngestRate"
	KeyRetentionPolicy        = "RetentionPolicy"
)

// IngestRate is a Wrapper for InstanceIngestRate int
type IngestRate struct {
	Value int `json:"value" envconfig:"CONVOY_INSTANCE_INGEST_RATE"`
}

type Defaults struct {
	InstanceIngestRate IngestRate                          `json:"instance_ingest_rate"`
	RetentionPolicy    config.RetentionPolicyConfiguration `json:"retention_policy"`
}

func GetInstanceDefaults() *Defaults {
	return &Defaults{
		InstanceIngestRate: IngestRate{
			Value: 25,
		},
		RetentionPolicy: config.RetentionPolicyConfiguration{
			Policy:                   "720h",
			IsRetentionPolicyEnabled: false,
		},
	}
}

func EncryptAndStoreInstanceDefaults(ctx context.Context, db database.Database, lo log.StdLogger) error {

	encryptionPassphrase := GetEncryptionPassphrase()
	defaults := GetInstanceDefaults()

	ingestRateJSON, err := json.Marshal(defaults.InstanceIngestRate)
	if err != nil {
		lo.WithError(err).Error("error marshaling ingest rate defaults")
		return err
	}

	retentionJSON, err := json.Marshal(defaults.RetentionPolicy)
	if err != nil {
		lo.WithError(err).Error("error marshaling retention defaults")
		return err
	}

	plaintextDefaults := map[string]map[string]string{
		OrganisationScope: {
			KeyInstanceIngestRate: string(ingestRateJSON),
			KeyRetentionPolicy:    string(retentionJSON),
		},
	}

	for scopeType, instanceDefaults := range plaintextDefaults {
		for key, plaintext := range instanceDefaults {
			errI := InsertEncryptedDefault(ctx, db, scopeType, key, plaintext, encryptionPassphrase)
			if errI != nil {
				lo.WithError(errI).Error("error inserting encrypted default")
				return errI
			}
		}
	}

	lo.Info("Encrypted defaults inserted successfully!")
	return nil
}

func GetEncryptionPassphrase() string {
	encryptionPassphrase := os.Getenv("CONVOY_INSTANCE_ENCRYPTION_PASSPHRASE")
	if encryptionPassphrase == "" {
		encryptionPassphrase = DefaultInstancePassphrase
	}
	return encryptionPassphrase
}

func InsertEncryptedDefault(ctx context.Context, db database.Database, scopeType, key, plaintext, encryptionPassphrase string) error {
	id := ulid.Make().String()

	query := `
        INSERT INTO convoy.instance_defaults (id, scope_type, key, default_value_cipher, created_at, updated_at)
        VALUES ($1, $2, $3, pgp_sym_encrypt($4::text, $5), CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
    `

	_, err := db.GetDB().ExecContext(ctx, query, id, scopeType, key, plaintext, encryptionPassphrase)
	return err
}

func FetchDecryptedDefaults(ctx context.Context, db database.Database, key, scopeType string, model interface{}) (string, error) {
	encryptionPassphrase := GetEncryptionPassphrase()

	var decryptedValue string

	query := `
        SELECT pgp_sym_decrypt(default_value_cipher::bytea, $1)
        FROM convoy.instance_defaults
        WHERE key = $2 AND scope_type = $3
    `

	err := db.GetReadDB().QueryRowContext(ctx, query, encryptionPassphrase, key, scopeType).Scan(&decryptedValue)
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

func FetchDecryptedOverrides(ctx context.Context, db database.Database, key string, scopeType, scopeId string, model interface{}) (string, error) {
	encryptionPassphrase := GetEncryptionPassphrase()

	var decryptedValue string

	query := `
        SELECT pgp_sym_decrypt(value_cipher::bytea, $1)
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
