package keys

import (
	"context"
	"fmt"
	"github.com/frain-dev/convoy/database"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/jmoiron/sqlx"
	"strings"
)

const NULL = "NULL"

func InitEncryption(lo log.StdLogger, db database.Database, km KeyManager, encryptionKey string, timeout int) error {
	// Start a transaction
	tx, err := db.GetDB().Beginx()
	if err != nil {
		lo.WithError(err).Error("failed to begin transaction")
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for table, columns := range tablesAndColumns {
		lo.Infof("Processing table: %s", table)

		if err := lockTable(ctx, tx, table, timeout); err != nil {
			rollback(lo, tx)
			lo.WithError(err).Error("failed to lock table")
			return err
		}

		isEncrypted, err := checkEncryptionStatus(ctx, tx, table)
		if err != nil {
			rollback(lo, tx)
			lo.WithError(err).Error("failed to check encryption status")
			return err
		}

		if isEncrypted {
			lo.Infof("Table %s is already encrypted. Skipping encryption.", table)
			continue
		}

		for column, cipherColumn := range columns {
			if err := encryptColumn(lo, ctx, tx, table, column, cipherColumn, encryptionKey); err != nil {
				rollback(lo, tx)
				lo.WithError(err).Error("failed to encrypt column")
				return fmt.Errorf("failed to encrypt column %s: %w", columns, err)
			}
		}

		if err := markTableEncrypted(ctx, tx, table); err != nil {
			rollback(lo, tx)
			lo.WithError(err).Error("failed to mark table")
			return fmt.Errorf("failed to mark encryption status for table %s: %w", table, err)
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		lo.WithError(err).Error("failed to commit transaction")
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	lo.Infof("Encryption initialization completed successfully.")
	return nil
}

func rollback(lo log.StdLogger, tx *sqlx.Tx) {
	rErr := tx.Rollback()
	if rErr != nil {
		lo.WithError(rErr).Error("failed to rollback transaction")
	}
}

// checkEncryptionStatus checks if the column is already encrypted.
func checkEncryptionStatus(ctx context.Context, tx *sqlx.Tx, table string) (bool, error) {
	checkQuery := fmt.Sprintf(
		"SELECT is_encrypted FROM convoy.%s WHERE is_encrypted=TRUE LIMIT 1;", table,
	)
	var isEncrypted bool
	err := tx.GetContext(ctx, &isEncrypted, checkQuery)
	if err != nil && err.Error() != "sql: no rows in result set" {
		return false, fmt.Errorf("failed to check encryption status of table %s: %w", table, err)
	}
	return isEncrypted, nil
}

// lockTable ensures the specified table is locked for exclusive access during the operation.
func lockTable(ctx context.Context, tx *sqlx.Tx, table string, timeout int) error {
	// Set a statement timeout to avoid indefinite hanging on the lock
	_, err := tx.ExecContext(ctx, fmt.Sprintf("SET statement_timeout = '%ds';", timeout))
	if err != nil {
		return fmt.Errorf("failed to set statement timeout: %w", err)
	}

	lockQuery := fmt.Sprintf("LOCK TABLE convoy.%s IN ACCESS EXCLUSIVE MODE;", table)
	_, err = tx.ExecContext(ctx, lockQuery)
	if err != nil {
		return fmt.Errorf("failed to lock table %s: %w", table, err)
	}
	return nil
}

// encryptColumn encrypts the specified column in the table.
func encryptColumn(lo log.StdLogger, ctx context.Context, tx *sqlx.Tx, table, column, cipherColumn, encryptionKey string) error {
	// Encrypt the column data and store it in the _cipher column
	columnZero, err := getColumnZero(lo, ctx, tx, table, column)
	if err != nil {
		return err
	}
	encryptQuery := fmt.Sprintf(
		"UPDATE convoy.%s SET %s = pgp_sym_encrypt(%s::text, $1), %s = %s WHERE %s IS NOT NULL;",
		table, cipherColumn, column, column, columnZero, column,
	)
	_, err = tx.ExecContext(ctx, encryptQuery, encryptionKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt column %s in table %s: %w", column, table, err)
	}

	return nil
}

func getColumnZero(lo log.StdLogger, ctx context.Context, tx *sqlx.Tx, table, column string) (string, error) {
	query := `SELECT is_nullable, data_type FROM information_schema.columns WHERE table_name = $1 AND column_name = $2;`
	var isNullable, columnType string
	err := tx.QueryRowContext(ctx, query, table, column).Scan(&isNullable, &columnType)
	if err != nil {
		lo.Errorf("Failed to fetch column info for %s.%s: %v", table, column, err)
		return NULL, err
	}

	if isNullable == "NO" {
		switch {
		case strings.Contains(columnType, "json"):
			return "'[]'::jsonb", nil
		case strings.Contains(columnType, "text") || strings.Contains(columnType, "char"):
			return "''", nil
		case strings.Contains(columnType, "int") || strings.Contains(columnType, "numeric"):
			return "0", nil
		case strings.Contains(columnType, "bool"):
			return "FALSE", nil
		default:
			lo.Warnf("Unknown type %s for %s.%s, defaulting to NULL", columnType, table, column)
			return NULL, nil
		}
	}

	return NULL, nil
}

// markTableEncrypted sets the `is_encrypted` column to true.
func markTableEncrypted(ctx context.Context, tx *sqlx.Tx, table string) error {
	markQuery := fmt.Sprintf(
		"UPDATE convoy.%s SET is_encrypted = TRUE;", table,
	)
	_, err := tx.ExecContext(ctx, markQuery)
	if err != nil {
		return fmt.Errorf("failed to mark table %s as encrypted: %w", table, err)
	}
	return nil
}
