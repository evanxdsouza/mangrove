// Package db owns Mangrove's SQLite connection and embedded migrations.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open opens (creating if needed) the SQLite database at path, applies
// pending migrations, and returns a ready-to-use connection pool.
//
// SQLite serializes writers regardless of driver, so the pool is kept small
// and WAL mode lets readers proceed alongside a writer without blocking.
func Open(path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	conn.SetMaxOpenConns(4)

	if _, err := conn.Exec(`PRAGMA journal_mode=WAL;`); err != nil {
		return nil, fmt.Errorf("enable WAL: %w", err)
	}
	if _, err := conn.Exec(`PRAGMA synchronous=NORMAL;`); err != nil {
		return nil, fmt.Errorf("set synchronous: %w", err)
	}

	if err := migrate(conn); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return conn, nil
}

func migrate(conn *sql.DB) error {
	if _, err := conn.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return err
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		var applied int
		if err := conn.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, name).Scan(&applied); err != nil {
			return err
		}
		if applied > 0 {
			continue
		}

		content, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return err
		}
		if err := applyMigration(conn, name, string(content)); err != nil {
			return err
		}
	}
	return nil
}

// applyMigration runs one migration file's SQL plus its schema_migrations
// bookkeeping insert as a single transaction, on a single pinned connection.
//
// Some migrations need to rebuild a table (e.g. to change a CHECK
// constraint SQLite won't let you ALTER in place), which means DROPping a
// table other tables hold a foreign key against. SQLite enforces FKs during
// DROP TABLE, so that fails while foreign_keys=ON. PRAGMA foreign_keys is
// also a documented no-op once a transaction is already open, so it has to
// be toggled on the same physical connection the transaction runs on --
// conn.Exec()/conn.Begin() from the pool make no such guarantee, hence
// pinning one connection here via conn.Conn(ctx).
func applyMigration(db *sql.DB, name, sqlText string) (err error) {
	ctx := context.Background()
	c, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer c.Close()

	if _, err := c.ExecContext(ctx, `PRAGMA foreign_keys=OFF`); err != nil {
		return fmt.Errorf("disable foreign_keys for migration %s: %w", name, err)
	}
	defer func() {
		if _, ferr := c.ExecContext(ctx, `PRAGMA foreign_keys=ON`); ferr != nil && err == nil {
			err = fmt.Errorf("re-enable foreign_keys after migration %s: %w", name, ferr)
		}
	}()

	tx, err := c.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, sqlText); err != nil {
		tx.Rollback()
		return fmt.Errorf("apply %s: %w", name, err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, name); err != nil {
		tx.Rollback()
		return err
	}
	// PRAGMA foreign_key_check reports violations as result rows, not as an
	// error -- with foreign_keys temporarily off during a rebuild, this is
	// the only thing standing between a migration and silently orphaning a
	// reference, so it's checked explicitly rather than trusted to error out.
	rows, err := tx.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("run foreign_key_check after migration %s: %w", name, err)
	}
	hasViolation := rows.Next()
	rows.Close()
	if hasViolation {
		tx.Rollback()
		return fmt.Errorf("migration %s left dangling foreign key references", name)
	}
	return tx.Commit()
}
