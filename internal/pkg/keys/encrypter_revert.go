package keys

import (
	"fmt"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/jmoiron/sqlx"
)

func RevertEncryption(lo log.StdLogger, db database.Database, encryptionKey string, timeout int) error {
	// Start a transaction
	tx, err := db.GetDB().Beginx()
	if err != nil {
		lo.WithError(err).Error("failed to begin transaction")
		return err
	}

	for table, columns := range tablesAndColumns {
		lo.Infof("Processing table: %s", table)

		if err := lockTable(tx, table, timeout); err != nil {
			rollback(lo, tx)
			return err
		}

		isEncrypted, err := checkEncryptionStatus(tx, table)
		if err != nil {
			rollback(lo, tx)
			return err
		}

		if !isEncrypted {
			lo.Infof("Table %s is not encrypted. Skipping revert.", table)
			continue
		}

		for column, cipherColumn := range columns {
			if err := decryptAndRestoreColumn(tx, table, column, cipherColumn, encryptionKey); err != nil {
				rollback(lo, tx)
				return err
			}
		}

		if err := markTableDecrypted(tx, table); err != nil {
			rollback(lo, tx)
			return err
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		lo.WithError(err).Error("failed to commit transaction")
		return err
	}

	lo.Infof("Encryption revert completed successfully.")
	return nil
}

// decryptAndRestoreColumn decrypts the cipher column and restores the data to the plain column.
func decryptAndRestoreColumn(tx *sqlx.Tx, table, column, cipherColumn, encryptionKey string) error {
	// Decrypt the cipher column and update the plain column, casting as needed
	revertQuery := fmt.Sprintf(
		"UPDATE convoy.%s SET %s = pgp_sym_decrypt(%s::bytea, $1)::%s WHERE %s IS NOT NULL;",
		table, column, cipherColumn, getColumnType(tx, table, column), cipherColumn,
	)
	_, err := tx.Exec(revertQuery, encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to decrypt column %s in table %s: %w", cipherColumn, table, err)
	}

	// Clear the cipher column
	clearCipherQuery := fmt.Sprintf(
		"UPDATE convoy.%s SET %s = NULL WHERE %s IS NOT NULL;",
		table, cipherColumn, cipherColumn,
	)
	_, err = tx.Exec(clearCipherQuery)
	if err != nil {
		return fmt.Errorf("failed to clear cipher column %s in table %s: %w", cipherColumn, table, err)
	}

	return nil
}

func getColumnType(tx *sqlx.Tx, table, column string) string {
	query := `SELECT data_type FROM convoy.information_schema.columns WHERE table_name = $1 AND column_name = $2;`
	var columnType string
	err := tx.Get(&columnType, query, table, column)
	if err != nil {
		log.Infof("Failed to fetch column type for %s.%s: %v", table, column, err)
		return ""
	}
	return columnType
}

// markTableDecrypted sets the `is_encrypted` column to false.
func markTableDecrypted(tx *sqlx.Tx, table string) error {
	markQuery := fmt.Sprintf(
		"UPDATE convoy.%s SET is_encrypted = FALSE;", table,
	)
	_, err := tx.Exec(markQuery)
	if err != nil {
		return fmt.Errorf("failed to mark table %s as decrypted: %w", table, err)
	}
	return nil
}
