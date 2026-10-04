package sqlitestore

import (
	"context"
	"database/sql"
	"fmt"
)

var migrations = []string{
	`CREATE TABLE IF NOT EXISTS links (
		code       TEXT PRIMARY KEY,
		url        TEXT    NOT NULL,
		created_at TEXT    NOT NULL,
		visits     INTEGER NOT NULL DEFAULT 0
	)`,
	`ALTER TABLE links ADD COLUMN expires_at TEXT;
	 CREATE INDEX idx_links_expires_at ON links (expires_at) WHERE expires_at IS NOT NULL`,
}

func migrate(ctx context.Context, db *sql.DB) error {
	var version int
	if err := db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	for i := version; i < len(migrations); i++ {
		if err := applyMigration(ctx, db, i+1, migrations[i]); err != nil {
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
	}
	return nil
}

func applyMigration(ctx context.Context, db *sql.DB, version int, stmt string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, stmt); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, version)); err != nil {
		return err
	}
	return tx.Commit()
}
