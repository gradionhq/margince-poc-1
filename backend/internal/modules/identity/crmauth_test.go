package crmauth

import (
	"errors"
	"testing"
)

func TestHas(t *testing.T) {
	p := Passport{Scopes: []string{"read:deal"}}
	if !p.Has("read:deal") || p.Has("write:deal") {
		t.Fatal("scope check wrong")
	}
}

func TestPasswordHashRoundTrip(t *testing.T) {
	hash, err := HashPassword("s3cr3t!")
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(hash, "s3cr3t!") {
		t.Fatal("correct password should verify")
	}
	if VerifyPassword(hash, "wrong") {
		t.Fatal("wrong password must not verify")
	}
}

func TestPermissionsValidatorRejectsUnknownObject(t *testing.T) {
	_, err := ValidatePermissions(map[string]any{
		"notanobject": map[string]any{"read": map[string]any{"row_scope": "all"}},
	})
	if err == nil {
		t.Fatal("unknown object should be rejected")
	}
}

func TestPermissionsValidatorRejectsInvalidRowScope(t *testing.T) {
	_, err := ValidatePermissions(map[string]any{
		"person": map[string]any{"read": map[string]any{"row_scope": "bogus"}},
	})
	if err == nil {
		t.Fatal("invalid row_scope should be rejected")
	}
}

func TestScopeExceedsGrantorRejected(t *testing.T) {
	grantorScopes := []string{"read:person"}
	err := CheckScopeSubset([]string{"write:person"}, grantorScopes)
	if err == nil {
		t.Fatal("scope exceeding grantor should be rejected")
	}
}

func TestValidatePermissions_WorkspaceManageMembers(t *testing.T) {
	raw := map[string]any{
		"workspace": map[string]any{
			"manage_members": map[string]any{"row_scope": "all"},
		},
	}
	perms, err := ValidatePermissions(raw)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if err := AuthorizePerms(perms, "workspace", "manage_members"); err != nil {
		t.Fatalf("authorize manage_members: %v", err)
	}
	// A principal without it is denied.
	none, _ := ValidatePermissions(map[string]any{"person": map[string]any{"read": map[string]any{"row_scope": "own"}}})
	if err := AuthorizePerms(none, "workspace", "manage_members"); err == nil {
		t.Fatal("expected denial for principal without manage_members")
	}
}

func TestValidatePermissions_ApprovalObjectAndDecideAction(t *testing.T) {
	// approval object with the read + decide actions must validate and authorize.
	raw := map[string]any{
		"approval": map[string]any{
			"read":   map[string]any{"row_scope": "all"},
			"decide": map[string]any{"row_scope": "all"},
		},
	}
	perms, err := ValidatePermissions(raw)
	if err != nil {
		t.Fatalf("approval read+decide should validate, got error: %v", err)
	}
	if err := AuthorizePerms(perms, "approval", "read"); err != nil {
		t.Fatalf("authorize approval read: %v", err)
	}
	if err := AuthorizePerms(perms, "approval", "decide"); err != nil {
		t.Fatalf("authorize approval decide: %v", err)
	}

	// A read-only approval grant CANNOT decide (returns ErrForbidden).
	readOnly, err := ValidatePermissions(map[string]any{
		"approval": map[string]any{"read": map[string]any{"row_scope": "all"}},
	})
	if err != nil {
		t.Fatalf("approval read-only should validate: %v", err)
	}
	if err := AuthorizePerms(readOnly, "approval", "read"); err != nil {
		t.Fatalf("read-only seat must still authorize approval read: %v", err)
	}
	if err := AuthorizePerms(readOnly, "approval", "decide"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("read-only seat must be denied approval decide with ErrForbidden, got: %v", err)
	}
}

func TestValidatePermissions_DraftingAssetObjectAndCurateAction(t *testing.T) {
	// drafting_asset object with curate action must validate and authorize for admin-shaped perms.
	raw := map[string]any{
		"drafting_asset": map[string]any{
			"read":   map[string]any{"row_scope": "all"},
			"curate": map[string]any{"row_scope": "all"},
		},
	}
	perms, err := ValidatePermissions(raw)
	if err != nil {
		t.Fatalf("drafting_asset read+curate should validate, got error: %v", err)
	}
	if err := AuthorizePerms(perms, "drafting_asset", "read"); err != nil {
		t.Fatalf("authorize drafting_asset read: %v", err)
	}
	if err := AuthorizePerms(perms, "drafting_asset", "curate"); err != nil {
		t.Fatalf("authorize drafting_asset curate: %v", err)
	}

	// A read-only drafting_asset grant CANNOT curate (returns ErrForbidden).
	readOnly, err := ValidatePermissions(map[string]any{
		"drafting_asset": map[string]any{"read": map[string]any{"row_scope": "all"}},
	})
	if err != nil {
		t.Fatalf("drafting_asset read-only should validate: %v", err)
	}
	if err := AuthorizePerms(readOnly, "drafting_asset", "read"); err != nil {
		t.Fatalf("read-only seat must still authorize drafting_asset read: %v", err)
	}
	if err := AuthorizePerms(readOnly, "drafting_asset", "curate"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("read-only seat must be denied drafting_asset curate with ErrForbidden, got: %v", err)
	}
}

func TestValidatePermissions_ProductAccepted(t *testing.T) {
	raw := map[string]any{
		"product": map[string]any{
			"read": map[string]any{"row_scope": "all"},
		},
	}
	perms, err := ValidatePermissions(raw)
	if err != nil {
		t.Fatalf("product should be a known object, got error: %v", err)
	}
	if err := AuthorizePerms(perms, "product", "read"); err != nil {
		t.Fatalf("authorize product read: %v", err)
	}
	// Unknown object is still rejected.
	_, err = ValidatePermissions(map[string]any{
		"bogus_object": map[string]any{"read": map[string]any{"row_scope": "all"}},
	})
	if err == nil {
		t.Fatal("unknown object should still be rejected after adding product")
	}
}

func TestValidatePermissions_ConversationLinkAccepted(t *testing.T) {
	// conversation_link must be a known object so RBAC middleware does not silently
	// drop it from the merged permissions map (B-E14.2).
	raw := map[string]any{
		"conversation_link": map[string]any{
			"read":    map[string]any{"row_scope": "all"},
			"create":  map[string]any{"row_scope": "all"},
			"archive": map[string]any{"row_scope": "all"},
		},
	}
	perms, err := ValidatePermissions(raw)
	if err != nil {
		t.Fatalf("conversation_link should be a known object, got error: %v", err)
	}
	if err := AuthorizePerms(perms, "conversation_link", "create"); err != nil {
		t.Fatalf("authorize conversation_link create: %v", err)
	}
	if err := AuthorizePerms(perms, "conversation_link", "archive"); err != nil {
		t.Fatalf("authorize conversation_link archive: %v", err)
	}
	// read_only: no create permission → ErrForbidden.
	readOnly, err := ValidatePermissions(map[string]any{
		"conversation_link": map[string]any{"read": map[string]any{"row_scope": "all"}},
	})
	if err != nil {
		t.Fatalf("conversation_link read-only should validate: %v", err)
	}
	if err := AuthorizePerms(readOnly, "conversation_link", "create"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("read-only seat must be denied conversation_link create, got: %v", err)
	}
}
