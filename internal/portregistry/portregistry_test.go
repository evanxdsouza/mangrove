package portregistry

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	mangrovedb "github.com/evanxdsouza/mangrove/internal/db"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	conn, err := mangrovedb.Open(filepath.Join(dir, "mangrove.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// seedService creates the minimal project -> deployment -> service chain
// AllocateForService needs to have a real row to update.
func seedService(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	ctx := context.Background()

	slug := fmt.Sprintf("test-project-%d", time.Now().UnixNano())
	res, err := db.ExecContext(ctx, `INSERT INTO projects (workspace_id, name, slug) VALUES (1, 'Test Project', ?)`, slug)
	if err != nil {
		t.Fatalf("insert project: %v", err)
	}
	projectID, _ := res.LastInsertId()

	res, err = db.ExecContext(ctx, `INSERT INTO deployments (project_id, name, slug, build_strategy) VALUES (?, 'app', 'app', 'dockerfile')`, projectID)
	if err != nil {
		t.Fatalf("insert deployment: %v", err)
	}
	deploymentID, _ := res.LastInsertId()

	res, err = db.ExecContext(ctx, `INSERT INTO services (deployment_id, name, container_name) VALUES (?, 'web', 'mangrove-test-web')`, deploymentID)
	if err != nil {
		t.Fatalf("insert service: %v", err)
	}
	serviceID, _ := res.LastInsertId()
	return serviceID
}

func TestAllocateForServiceAssignsLowestFreePort(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	serviceID := seedService(t, db)

	port, err := AllocateForService(ctx, db, serviceID, 20000, 20010)
	if err != nil {
		t.Fatalf("AllocateForService: %v", err)
	}
	if port != 20000 {
		t.Errorf("expected first allocation to take the lowest port 20000, got %d", port)
	}

	var hostPort sql.NullInt64
	if err := db.QueryRowContext(ctx, `SELECT host_port FROM services WHERE id = ?`, serviceID).Scan(&hostPort); err != nil {
		t.Fatalf("query services.host_port: %v", err)
	}
	if !hostPort.Valid || hostPort.Int64 != 20000 {
		t.Errorf("expected services.host_port = 20000, got %v", hostPort)
	}
}

func TestAllocateForServiceSkipsUsedPorts(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	svc1 := seedService(t, db)
	if _, err := AllocateForService(ctx, db, svc1, 20000, 20010); err != nil {
		t.Fatalf("first AllocateForService: %v", err)
	}

	svc2 := seedService(t, db)
	port2, err := AllocateForService(ctx, db, svc2, 20000, 20010)
	if err != nil {
		t.Fatalf("second AllocateForService: %v", err)
	}
	if port2 != 20001 {
		t.Errorf("expected second allocation to skip the taken port and get 20001, got %d", port2)
	}
}

func TestAllocateForServiceExhaustedRange(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	svc1 := seedService(t, db)
	if _, err := AllocateForService(ctx, db, svc1, 20000, 20000); err != nil {
		t.Fatalf("AllocateForService: %v", err)
	}

	svc2 := seedService(t, db)
	if _, err := AllocateForService(ctx, db, svc2, 20000, 20000); err != ErrNoPortsAvailable {
		t.Errorf("expected ErrNoPortsAvailable, got %v", err)
	}
}

func TestRegisterSystemPortIsIdempotentAndBlocksAllocation(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	if err := RegisterSystemPort(ctx, db, 7777, "mangrove API/webhook"); err != nil {
		t.Fatalf("RegisterSystemPort: %v", err)
	}
	if err := RegisterSystemPort(ctx, db, 7777, "mangrove API/webhook"); err != nil {
		t.Fatalf("RegisterSystemPort (idempotent call): %v", err)
	}

	svc := seedService(t, db)
	port, err := AllocateForService(ctx, db, svc, 7777, 7778)
	if err != nil {
		t.Fatalf("AllocateForService: %v", err)
	}
	if port != 7778 {
		t.Errorf("expected allocator to skip the registered system port 7777, got %d", port)
	}
}

func TestReserveManualBlocksAllocationAndRelease(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	if err := ReserveManual(ctx, db, 20005, "used by another app on this box", nil); err != nil {
		t.Fatalf("ReserveManual: %v", err)
	}

	svc := seedService(t, db)
	port, err := AllocateForService(ctx, db, svc, 20005, 20006)
	if err != nil {
		t.Fatalf("AllocateForService: %v", err)
	}
	if port != 20006 {
		t.Errorf("expected allocator to skip the manually reserved port 20005, got %d", port)
	}

	if err := Release(ctx, db, 20005); err != nil {
		t.Fatalf("Release: %v", err)
	}

	entries, err := List(ctx, db)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, e := range entries {
		if e.Port == 20005 {
			t.Errorf("expected port 20005 to be released, still found: %+v", e)
		}
	}
}

func TestReleaseClearsServiceHostPort(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	svc := seedService(t, db)

	port, err := AllocateForService(ctx, db, svc, 20000, 20010)
	if err != nil {
		t.Fatalf("AllocateForService: %v", err)
	}
	if err := Release(ctx, db, port); err != nil {
		t.Fatalf("Release: %v", err)
	}

	var hostPort sql.NullInt64
	if err := db.QueryRowContext(ctx, `SELECT host_port FROM services WHERE id = ?`, svc).Scan(&hostPort); err != nil {
		t.Fatalf("query services.host_port: %v", err)
	}
	if hostPort.Valid {
		t.Errorf("expected services.host_port to be cleared, got %v", hostPort.Int64)
	}
}

func TestReleaseRejectsSystemPort(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	if err := RegisterSystemPort(ctx, db, 7777, "mangrove API/webhook"); err != nil {
		t.Fatalf("RegisterSystemPort: %v", err)
	}
	if err := Release(ctx, db, 7777); err == nil {
		t.Fatal("expected Release to reject a system port, got nil error")
	}
}
