package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/evanxdsouza/mangrove/internal/store"
)

// TestRemoveWorkspaceMemberRecordsAuditEvent confirms a real write path
// (not a direct store.RecordAuditEvent call) produces a readable audit
// entry -- the actual point of wiring s.audit into handlers, not just
// that the store layer round-trips a hand-built AuditEvent. Uses
// workspace-membership actions rather than delete/deploy because those
// are pure store writes with no s.Orchestrator dependency, which is nil
// in this lightweight test env (see roleTestEnv's own doc comment).
func TestRemoveWorkspaceMemberRecordsAuditEvent(t *testing.T) {
	env := newRoleTestEnv(t)
	ownerCookie, ownerID := env.cookieFor(t, "owner@example.com", "owner")
	_, colleagueID := env.cookieFor(t, "colleague@example.com", "member")
	workspaceID, _, _, _ := env.seedWorkspace(t, ownerID, "acme")

	rec := env.do(http.MethodPost, "/api/workspaces/"+itoa(workspaceID)+"/members", ownerCookie,
		map[string]any{"email": "colleague@example.com", "role": "viewer"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 adding member, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = env.do(http.MethodDelete, "/api/workspaces/"+itoa(workspaceID)+"/members/"+itoa(colleagueID), ownerCookie, nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 removing member, got %d: %s", rec.Code, rec.Body.String())
	}

	events, err := env.store.ListAuditEventsForWorkspace(context.Background(), workspaceID, 10)
	if err != nil {
		t.Fatalf("ListAuditEventsForWorkspace: %v", err)
	}
	if len(events) != 2 { // add_member, remove_member
		t.Fatalf("expected 2 audit events, got %d: %+v", len(events), events)
	}
	// Newest first: remove_member, then add_member.
	if events[0].Action != "remove_member" || events[0].ResourceType != "workspace" || events[0].ResourceID == nil || *events[0].ResourceID != workspaceID {
		t.Errorf("unexpected newest audit event: %+v", events[0])
	}
	if events[0].ActorEmail != "owner@example.com" {
		t.Errorf("expected actor_email owner@example.com, got %q", events[0].ActorEmail)
	}
	if events[1].Action != "add_member" {
		t.Errorf("expected the older event to be add_member, got %+v", events[1])
	}
}

// TestAuditLogEndpointsAreScoped covers both read endpoints: a workspace
// member (viewer+) can see their own workspace's trail but gets 403 for
// one they don't belong to, and only a global owner can see the
// all-workspaces admin view.
func TestAuditLogEndpointsAreScoped(t *testing.T) {
	env := newRoleTestEnv(t)
	ownerCookie, ownerID := env.cookieFor(t, "owner@example.com", "owner")
	memberCookie, memberID := env.cookieFor(t, "member@example.com", "member")

	workspaceA, _, _, _ := env.seedWorkspace(t, ownerID, "acme")
	env.setRole(t, workspaceA, memberID, "viewer")

	t.Run("workspace member can see their own workspace's log", func(t *testing.T) {
		rec := env.do(http.MethodGet, "/api/workspaces/"+itoa(workspaceA)+"/audit-log", memberCookie, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var events []store.AuditEvent
		if err := json.Unmarshal(rec.Body.Bytes(), &events); err != nil {
			t.Fatalf("decode: %v", err)
		}
	})

	t.Run("member with no membership is forbidden", func(t *testing.T) {
		otherWorkspaceID, _, _, _ := env.seedWorkspace(t, ownerID, "initech")
		rec := env.do(http.MethodGet, "/api/workspaces/"+itoa(otherWorkspaceID)+"/audit-log", memberCookie, nil)
		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("global admin audit-log is owner-only", func(t *testing.T) {
		rec := env.do(http.MethodGet, "/api/admin/audit-log", memberCookie, nil)
		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403 for member, got %d: %s", rec.Code, rec.Body.String())
		}
		rec = env.do(http.MethodGet, "/api/admin/audit-log", ownerCookie, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for owner, got %d: %s", rec.Code, rec.Body.String())
		}
		var events []store.AuditEvent
		if err := json.Unmarshal(rec.Body.Bytes(), &events); err != nil {
			t.Fatalf("decode: %v", err)
		}
	})
}
