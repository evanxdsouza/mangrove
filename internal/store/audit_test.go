package store

import (
	"context"
	"path/filepath"
	"testing"

	mangrovedb "github.com/evanxdsouza/mangrove/internal/db"
)

func TestAuditEventRoundTrip(t *testing.T) {
	dir := t.TempDir()
	db, err := mangrovedb.Open(filepath.Join(dir, "mangrove.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := New(db)
	ctx := context.Background()

	userID, err := st.CreateUser(ctx, "owner@example.com", "hash", "owner")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	ws, err := st.CreateWorkspace(ctx, "Acme", "acme", userID)
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}

	resourceID := int64(42)
	if err := st.RecordAuditEvent(ctx, AuditEvent{
		ActorUserID:  &userID,
		ActorEmail:   "owner@example.com",
		Action:       "delete",
		ResourceType: "project",
		ResourceID:   &resourceID,
		WorkspaceID:  &ws.ID,
		Detail:       "test detail",
	}); err != nil {
		t.Fatalf("RecordAuditEvent: %v", err)
	}
	// An org-level event with no workspace at all (e.g. user management).
	if err := st.RecordAuditEvent(ctx, AuditEvent{
		ActorEmail:   "github-webhook",
		Action:       "deploy",
		ResourceType: "deployment",
		ResourceID:   &resourceID,
	}); err != nil {
		t.Fatalf("RecordAuditEvent (no workspace): %v", err)
	}

	wsEvents, err := st.ListAuditEventsForWorkspace(ctx, ws.ID, 10)
	if err != nil {
		t.Fatalf("ListAuditEventsForWorkspace: %v", err)
	}
	if len(wsEvents) != 1 {
		t.Fatalf("expected 1 workspace-scoped event, got %d", len(wsEvents))
	}
	e := wsEvents[0]
	if e.ActorEmail != "owner@example.com" || e.Action != "delete" || e.ResourceType != "project" ||
		e.ResourceID == nil || *e.ResourceID != 42 || e.WorkspaceID == nil || *e.WorkspaceID != ws.ID || e.Detail != "test detail" {
		t.Errorf("event fields didn't round-trip: %+v", e)
	}
	if e.ActorUserID == nil || *e.ActorUserID != userID {
		t.Errorf("expected actor_user_id %d, got %v", userID, e.ActorUserID)
	}

	all, err := st.ListAllAuditEvents(ctx, 10)
	if err != nil {
		t.Fatalf("ListAllAuditEvents: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 total events, got %d", len(all))
	}
	// Newest first.
	if all[0].Action != "deploy" || all[0].ActorEmail != "github-webhook" || all[0].WorkspaceID != nil {
		t.Errorf("expected the webhook-triggered event first (newest) with no workspace, got %+v", all[0])
	}
}

func TestListAuditEventsRespectsLimit(t *testing.T) {
	dir := t.TempDir()
	db, err := mangrovedb.Open(filepath.Join(dir, "mangrove.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := New(db)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if err := st.RecordAuditEvent(ctx, AuditEvent{ActorEmail: "x@example.com", Action: "deploy", ResourceType: "deployment"}); err != nil {
			t.Fatalf("RecordAuditEvent: %v", err)
		}
	}
	events, err := st.ListAllAuditEvents(ctx, 3)
	if err != nil {
		t.Fatalf("ListAllAuditEvents: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected limit to cap at 3, got %d", len(events))
	}
}
