// Package portregistry tracks every port Mangrove has allocated (to a
// service, or to itself for the fixed webhook/API port) alongside manual
// reservations for ports used by things outside Mangrove's control, so
// nothing collides. SQLite's UNIQUE constraint on port_registry.port is
// the final backstop; allocation itself runs inside a transaction so two
// concurrent deploys can't race onto the same port.
package portregistry

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// DefaultRangeMin/Max bound the ports Mangrove auto-allocates to services.
// Kept well clear of common well-known ports and of MangrovePort itself.
const (
	DefaultRangeMin = 20000
	DefaultRangeMax = 21000
)

var ErrNoPortsAvailable = errors.New("portregistry: no free ports in range")

type Entry struct {
	ID             int64
	Port           int
	Status         string
	AllocationType string
	ServiceID      *int64
	Note           string
}

// AllocateForService reserves the lowest free port in [minPort, maxPort],
// records it against serviceID, and writes it onto services.host_port —
// all inside one transaction.
func AllocateForService(ctx context.Context, db *sql.DB, serviceID int64, minPort, maxPort int) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	port, err := firstFreePort(ctx, tx, minPort, maxPort)
	if err != nil {
		return 0, err
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO port_registry (port, status, allocation_type, service_id) VALUES (?, 'allocated', 'service_public', ?)`,
		port, serviceID,
	); err != nil {
		return 0, fmt.Errorf("insert port_registry row: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE services SET host_port = ? WHERE id = ?`, port, serviceID); err != nil {
		return 0, fmt.Errorf("update services.host_port: %w", err)
	}

	return port, tx.Commit()
}

// RegisterSystemPort records a fixed, never-reassignable port (Mangrove's
// own webhook/API port) so the allocator's collision check sees it as
// taken. It is a no-op if already registered.
func RegisterSystemPort(ctx context.Context, db *sql.DB, port int, note string) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO port_registry (port, status, allocation_type, note) VALUES (?, 'allocated', 'system', ?)
		 ON CONFLICT(port) DO NOTHING`,
		port, note,
	)
	return err
}

// ReserveManual marks a host port as used by something outside Mangrove's
// control (e.g. another service on the box), so it's excluded from
// auto-allocation.
func ReserveManual(ctx context.Context, db *sql.DB, port int, note string, reservedByUserID *int64) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO port_registry (port, status, allocation_type, note, reserved_by_user_id) VALUES (?, 'reserved', 'manual_external', ?, ?)`,
		port, note, reservedByUserID,
	)
	if err != nil {
		return fmt.Errorf("reserve port %d: %w", port, err)
	}
	return nil
}

// Release frees a port. For a service-owned port this also clears
// services.host_port; for a manual reservation it just removes the row.
func Release(ctx context.Context, db *sql.DB, port int) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var allocationType string
	var serviceID sql.NullInt64
	err = tx.QueryRowContext(ctx, `SELECT allocation_type, service_id FROM port_registry WHERE port = ?`, port).
		Scan(&allocationType, &serviceID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("port %d is not registered", port)
	}
	if err != nil {
		return err
	}
	if allocationType == "system" {
		return fmt.Errorf("port %d is a system port and cannot be released", port)
	}

	if serviceID.Valid {
		if _, err := tx.ExecContext(ctx, `UPDATE services SET host_port = NULL WHERE id = ?`, serviceID.Int64); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM port_registry WHERE port = ?`, port); err != nil {
		return err
	}
	return tx.Commit()
}

// List returns every tracked port, for the admin panel's port-registry view.
func List(ctx context.Context, db *sql.DB) ([]Entry, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, port, status, allocation_type, service_id, COALESCE(note, '') FROM port_registry ORDER BY port`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var serviceID sql.NullInt64
		if err := rows.Scan(&e.ID, &e.Port, &e.Status, &e.AllocationType, &serviceID, &e.Note); err != nil {
			return nil, err
		}
		if serviceID.Valid {
			e.ServiceID = &serviceID.Int64
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func firstFreePort(ctx context.Context, tx *sql.Tx, minPort, maxPort int) (int, error) {
	rows, err := tx.QueryContext(ctx, `SELECT port FROM port_registry WHERE port BETWEEN ? AND ?`, minPort, maxPort)
	if err != nil {
		return 0, err
	}
	used := make(map[int]bool)
	for rows.Next() {
		var p int
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			return 0, err
		}
		used[p] = true
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	rows.Close()

	for p := minPort; p <= maxPort; p++ {
		if !used[p] {
			return p, nil
		}
	}
	return 0, ErrNoPortsAvailable
}
