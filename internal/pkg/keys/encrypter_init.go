package keys

import (
	"fmt"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/jmoiron/sqlx"
	"strings"
)

func InitEncryption(db database.Database, km KeyManager, encryptionKey string) error {
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

		if isEncrypted {
			log.Infof("Table %s is already encrypted. Skipping encryption.", table)
			continue
		}

		for column, cipherColumn := range columns {
			if err := encryptColumn(tx, table, column, cipherColumn, encryptionKey); err != nil {
				_ = tx.Rollback()
				return err
			}
		}

		if err := markTableEncrypted(tx, table); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	err = km.SetKey(encryptionKey)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("failed to update encryption key: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Infof("Encryption initialization completed successfully.")
	return nil
}

// checkEncryptionStatus checks if the column is already encrypted.
func checkEncryptionStatus(tx *sqlx.Tx, table string) (bool, error) {
	checkQuery := fmt.Sprintf(
		"SELECT is_encrypted FROM convoy.%s WHERE is_encrypted=TRUE LIMIT 1;", table,
	)
	var isEncrypted bool
	err := tx.Get(&isEncrypted, checkQuery)
	if err != nil && err.Error() != "sql: no rows in result set" {
		return false, fmt.Errorf("failed to check encryption status of table %s: %w", table, err)
	}
	return isEncrypted, nil
}

// lockTable ensures the specified table is locked for exclusive access during the operation.
func lockTable(tx *sqlx.Tx, table string) error {
	// Set a statement timeout to avoid indefinite hanging on the lock
	_, err := tx.Exec("SET statement_timeout = '120s';")
	if err != nil {
		return fmt.Errorf("failed to set statement timeout: %w", err)
	}

	lockQuery := fmt.Sprintf("LOCK TABLE convoy.%s IN ACCESS EXCLUSIVE MODE;", table)
	_, err = tx.Exec(lockQuery)
	if err != nil {
		return fmt.Errorf("failed to lock table %s: %w", table, err)
	}
	return nil
}

// encryptColumn encrypts the specified column in the table.
func encryptColumn(tx *sqlx.Tx, table, column, cipherColumn, encryptionKey string) error {
	// Encrypt the column data and store it in the _cipher column
	encryptQuery := fmt.Sprintf(
		"UPDATE convoy.%s SET %s = pgp_sym_encrypt(%s::text, $1), %s = %s WHERE %s IS NOT NULL;",
		table, cipherColumn, column, column, getColumnZero(tx, table, column), column,
	)
	_, err := tx.Exec(encryptQuery, encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt column %s in table %s: %w", column, table, err)
	}

	return nil
}

func getColumnZero(tx *sqlx.Tx, table, column string) string {
	query := `SELECT is_nullable, data_type FROM convoy.information_schema.columns WHERE table_name = $1 AND column_name = $2;`
	var isNullable, columnType string
	err := tx.QueryRow(query, table, column).Scan(&isNullable, &columnType)
	if err != nil {
		log.Infof("Failed to fetch column info for %s.%s: %v", table, column, err)
		return "NULL"
	}

	if isNullable == "NO" {
		switch {
		case strings.Contains(columnType, "json"):
			return "'[]'::jsonb"
		case strings.Contains(columnType, "text") || strings.Contains(columnType, "char"):
			return "''"
		case strings.Contains(columnType, "int") || strings.Contains(columnType, "numeric"):
			return "0"
		case strings.Contains(columnType, "bool"):
			return "FALSE"
		default:
			log.Warnf("Unknown type %s for %s.%s, defaulting to NULL", columnType, table, column)
			return "NULL"
		}
	}

	return "NULL"
}

// markTableEncrypted sets the `is_encrypted` column to true.
func markTableEncrypted(tx *sqlx.Tx, table string) error {
	markQuery := fmt.Sprintf(
		"UPDATE convoy.%s SET is_encrypted = TRUE;", table,
	)
	_, err := tx.Exec(markQuery)
	if err != nil {
		return fmt.Errorf("failed to mark table %s as encrypted: %w", table, err)
	}
	return nil
}
