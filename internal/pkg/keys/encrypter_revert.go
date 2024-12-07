package keys

import (
	"fmt"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/jmoiron/sqlx"
)

func RevertEncryption(db database.Database, km KeyManager, encryptionKey string) error {
	// Start a transaction
	tx, err := db.GetDB().Beginx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	for table, columns := range tablesAndColumns {
		log.Infof("Processing table: %s", table)

		if err := lockTable(tx, table); err != nil {
			_ = tx.Rollback()
			return err
		}

		isEncrypted, err := checkEncryptionStatus(tx, table)
		if err != nil {
			_ = tx.Rollback()
			return err
		}

		if !isEncrypted {
			log.Infof("Table %s is not encrypted. Skipping revert.", table)
			continue
		}

		for column, cipherColumn := range columns {
			if err := decryptAndRestoreColumn(tx, table, column, cipherColumn, encryptionKey); err != nil {
				_ = tx.Rollback()
				return err
			}
		}

		if err := markTableDecrypted(tx, table); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Infof("Encryption revert completed successfully.")
	return nil
}

// decryptAndRestoreColumn decrypts the cipher column and restores the data to the plain column.
func decryptAndRestoreColumn(tx *sqlx.Tx, table, column, cipherColumn, encryptionKey string) error {
	// Decrypt the cipher column and update the plain column, casting as needed
	revertQuery := fmt.Sprintf(
		"UPDATE %s SET %s = pgp_sym_decrypt(%s::bytea, $1)::%s WHERE %s IS NOT NULL;",
		table, column, cipherColumn, getColumnType(tx, table, column), cipherColumn,
	)
	_, err := tx.Exec(revertQuery, encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to decrypt column %s in table %s: %w", cipherColumn, table, err)
	}

	// Clear the cipher column
	clearCipherQuery := fmt.Sprintf(
		"UPDATE %s SET %s = NULL WHERE %s IS NOT NULL;",
		table, cipherColumn, cipherColumn,
	)
	_, err = tx.Exec(clearCipherQuery)
	if err != nil {
		return fmt.Errorf("failed to clear cipher column %s in table %s: %w", cipherColumn, table, err)
	}

	return nil
}

func getColumnType(tx *sqlx.Tx, table, column string) string {
	query := `SELECT data_type FROM information_schema.columns WHERE table_name = $1 AND column_name = $2;`
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
		"UPDATE %s SET is_encrypted = FALSE;", table,
	)
	_, err := tx.Exec(markQuery)
	if err != nil {
		return fmt.Errorf("failed to mark table %s as decrypted: %w", table, err)
	}
	return nil
}
