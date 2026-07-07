//go:build integration

package crmauth_test

import (
	"context"
	"testing"
	"time"

	crmauth "github.com/gradionhq/margince/backend/internal/modules/identity"
)

// TestPassportRestoredColumns proves AC-C5/D4: PassportStore.Create persists
// on_behalf_of/label, and Lookup returns them.
func TestPassportRestoredColumns(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	granter := seedUser(t, db, wsID)
	agent := seedUser(t, db, wsID)
	store := crmauth.NewPassportStore(db)

	_, rec, err := store.Create(context.Background(), wsID, granter, agent, "agent seat for X",
		[]string{"read:person"}, time.Hour)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rec.OnBehalfOf == nil || *rec.OnBehalfOf != agent {
		t.Fatalf("Create: OnBehalfOf = %v, want %q", rec.OnBehalfOf, agent)
	}
	if rec.Label == nil || *rec.Label != "agent seat for X" {
		t.Fatalf("Create: Label = %v, want %q", rec.Label, "agent seat for X")
	}

	rawToken, _, err := store.Create(context.Background(), wsID, granter, agent, "agent seat for X",
		[]string{"read:person"}, time.Hour)
	if err != nil {
		t.Fatalf("Create (2nd): %v", err)
	}
	looked, err := store.Lookup(context.Background(), rawToken)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if looked.OnBehalfOf == nil || *looked.OnBehalfOf != agent {
		t.Fatalf("Lookup: OnBehalfOf = %v, want %q", looked.OnBehalfOf, agent)
	}
	if looked.Label == nil || *looked.Label != "agent seat for X" {
		t.Fatalf("Lookup: Label = %v, want %q", looked.Label, "agent seat for X")
	}
}

// TestPassportCreate_EmptyLabel_StoresNull proves an empty caller-supplied
// label maps to SQL NULL, not the literal empty string.
func TestPassportCreate_EmptyLabel_StoresNull(t *testing.T) {
	db := testDB(t)
	wsID := seedWorkspace(t, db)
	granter := seedUser(t, db, wsID)
	store := crmauth.NewPassportStore(db)

	_, rec, err := store.Create(context.Background(), wsID, granter, granter, "", []string{"read:person"}, time.Hour)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rec.Label != nil {
		t.Fatalf("Label = %v, want nil for empty input", rec.Label)
	}
	if rec.OnBehalfOf == nil || *rec.OnBehalfOf != granter {
		t.Fatalf("OnBehalfOf = %v, want %q (defaulted to granter)", rec.OnBehalfOf, granter)
	}
}
