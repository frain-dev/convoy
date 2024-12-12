package keys

import (
	"context"
	"fmt"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/jmoiron/sqlx"
)

func RotateEncryptionKey(lo log.StdLogger, db database.Database, km KeyManager, oldKey, newKey string, timeout int) error {

	tx, err := db.GetDB().Beginx()
	if err != nil {
		lo.WithError(err).Error("failed to begin transaction")
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for table, columns := range tablesAndColumns {
		lo.Infof("Processing table: %s", table)

		err = lockTable(ctx, tx, table, timeout)
		if err != nil {
			rollback(lo, tx)
			log.WithError(err).Error("failed to lock table")
			return err
		}

		isEncrypted, err := checkEncryptionStatus(ctx, tx, table)
		if err != nil {
			rollback(lo, tx)
			log.WithError(err).Error("failed to check encryption status")
			return err
		}

		if !isEncrypted {
			rollback(lo, tx)
			return fmt.Errorf("table %s has not been encrypted. Please initialize encryption first", table)
		}

		for plainColumn, cipherColumn := range columns {
			lo.Infof("Re-encrypting column %s (%s) in table %s", plainColumn, cipherColumn, table)

			err = reEncryptColumn(ctx, tx, table, cipherColumn, oldKey, newKey)
			if err != nil {
				rollback(lo, tx)
				log.WithError(err).Error("failed to re-encrypt column")
				return err
			}
		}
	}

	err = km.SetKey(newKey)
	if err != nil {
		rollback(lo, tx)
		return fmt.Errorf("failed to update encryption key: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		lo.WithError(err).Error("failed to commit transaction")
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	lo.Infof("Key rotation completed successfully.")
	return nil
}

func reEncryptColumn(ctx context.Context, tx *sqlx.Tx, table, cipherColumn, oldKey, newKey string) error {
	// Re-encrypt the cipher column with the new key
	reEncryptQuery := fmt.Sprintf(
		"UPDATE convoy.%s SET %s = pgp_sym_encrypt(pgp_sym_decrypt(%s::bytea, $1), $2) WHERE %s IS NOT NULL;",
		table, cipherColumn, cipherColumn, cipherColumn,
	)
	_, err := tx.ExecContext(ctx, reEncryptQuery, oldKey, newKey)
	if err != nil {
		return fmt.Errorf("failed to re-encrypt column %s in table %s: %w", cipherColumn, table, err)
	}

	return nil
}
