package keys

import (
	"fmt"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/jmoiron/sqlx"
)

func RotateEncryptionKey(db database.Database, km KeyManager, oldKey, newKey string) error {

	tx, err := db.GetDB().Beginx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	for table, columns := range tablesAndColumns {
		log.Infof("Processing table: %s", table)

		err = lockTable(tx, table)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to lock table %s: %w", table, err)
		}

		isEncrypted, err := checkEncryptionStatus(tx, table)
		if err != nil {
			_ = tx.Rollback()
			return err
		}

		if !isEncrypted {
			_ = tx.Rollback()
			return fmt.Errorf("table %s has not been encrypted. Please initialize encryption first", table)
		}

		for plainColumn, cipherColumn := range columns {
			log.Infof("Re-encrypting column %s (%s) in table %s", plainColumn, cipherColumn, table)

			err = reEncryptColumn(tx, table, cipherColumn, oldKey, newKey)
			if err != nil {
				_ = tx.Rollback()
				return err
			}
		}
	}

	err = km.SetKey(newKey)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to update encryption key: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Infof("Key rotation completed successfully.")
	return nil
}

func reEncryptColumn(tx *sqlx.Tx, table, cipherColumn, oldKey, newKey string) error {
	// Re-encrypt the cipher column with the new key
	reEncryptQuery := fmt.Sprintf(
		"UPDATE convoy.%s SET %s = pgp_sym_encrypt(pgp_sym_decrypt(%s::bytea, $1), $2) WHERE %s IS NOT NULL;",
		table, cipherColumn, cipherColumn, cipherColumn,
	)
	_, err := tx.Exec(reEncryptQuery, oldKey, newKey)
	if err != nil {
		return fmt.Errorf("failed to re-encrypt column %s in table %s: %w", cipherColumn, table, err)
	}

	return nil
}
