package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	mangrovedb "github.com/evanxdsouza/mangrove/internal/db"
)

// TestCreateWebhookEvent_DuplicateDeliveryID covers the race
// internal/api/webhook.go relies on: WebhookDeliveryExists is only a
// fast-path check, not a guarantee, since two concurrent deliveries with the
// same X-GitHub-Delivery ID (GitHub retries reuse the original ID) can both
// pass it before either has inserted. The delivery_id UNIQUE constraint is
// the real guarantee -- this confirms losing that race comes back as
// ErrDuplicate, not a generic error the handler would surface as a 500.
func TestCreateWebhookEvent_DuplicateDeliveryID(t *testing.T) {
	dir := t.TempDir()
	db, err := mangrovedb.Open(filepath.Join(dir, "mangrove.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := New(db)
	ctx := context.Background()

	id, err := st.CreateWebhookEvent(ctx, "delivery-1", "push", nil, true, `{}`)
	if err != nil {
		t.Fatalf("first CreateWebhookEvent: %v", err)
	}
	if id == 0 {
		t.Fatalf("expected a non-zero event id")
	}

	_, err = st.CreateWebhookEvent(ctx, "delivery-1", "push", nil, true, `{}`)
	if !errors.Is(err, ErrDuplicate) {
		t.Fatalf("expected ErrDuplicate for a repeated delivery id, got %v", err)
	}
}
