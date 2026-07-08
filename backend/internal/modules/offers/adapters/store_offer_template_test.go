//go:build integration

package adapters_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gradionhq/margince/backend/internal/modules/offers/adapters"
	"github.com/gradionhq/margince/backend/internal/modules/offers/domain"
	errs "github.com/gradionhq/margince/backend/internal/shared/apperrors"
	"github.com/gradionhq/margince/backend/internal/shared/kernel/pgtest"
)

func TestOfferTemplateStore_CreateGetUpdateArchive_RoundTrip(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID := seedProductWorkspace(t, db)
	s := adapters.NewOfferTemplateStore(db)

	tpl := domain.NewOfferTemplate("Standard DE", provTest())
	tpl.WorkspaceID = wsID
	tpl.Layout = map[string]interface{}{"logo_ref": nil}

	created, err := s.Create(context.Background(), tpl)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Locale != "de-DE" {
		t.Fatalf("expected default locale de-DE, got %q", created.Locale)
	}

	got, err := s.Get(context.Background(), created.ID, wsID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "Standard DE" {
		t.Fatalf("expected name round-trip, got %q", got.Name)
	}

	updated, err := s.Update(context.Background(), created.ID, wsID, map[string]any{"name": "Standard DE v2"}, created.Version)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "Standard DE v2" {
		t.Fatalf("expected updated name, got %q", updated.Name)
	}

	archived, err := s.Archive(context.Background(), created.ID, wsID)
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if archived.ArchivedAt == nil {
		t.Fatal("expected archived_at set")
	}
}

func TestOfferTemplateStore_Create_DuplicateName_Rejected(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID := seedProductWorkspace(t, db)
	s := adapters.NewOfferTemplateStore(db)

	a := domain.NewOfferTemplate("Dup Name", provTest())
	a.WorkspaceID = wsID
	if _, err := s.Create(context.Background(), a); err != nil {
		t.Fatalf("first create: %v", err)
	}
	b := domain.NewOfferTemplate("Dup Name", provTest())
	b.WorkspaceID = wsID
	_, err := s.Create(context.Background(), b)
	var dupErr *adapters.ErrDuplicateTemplateName
	if !errors.As(err, &dupErr) {
		t.Fatalf("expected ErrDuplicateTemplateName, got %v", err)
	}
}

func TestOfferTemplateStore_Create_SecondDefaultSameLocale_Rejected(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID := seedProductWorkspace(t, db)
	s := adapters.NewOfferTemplateStore(db)

	first := domain.NewOfferTemplate("First Default", provTest())
	first.WorkspaceID = wsID
	first.IsDefault = true
	if _, err := s.Create(context.Background(), first); err != nil {
		t.Fatalf("first create: %v", err)
	}

	second := domain.NewOfferTemplate("Second Default", provTest())
	second.WorkspaceID = wsID
	second.IsDefault = true
	_, err := s.Create(context.Background(), second)
	var conflictErr *adapters.ErrDefaultConflict
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected ErrDefaultConflict, got %v", err)
	}
}

func TestOfferTemplateStore_Create_DifferentLocaleDefaults_BothAllowed(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID := seedProductWorkspace(t, db)
	s := adapters.NewOfferTemplateStore(db)

	de := domain.NewOfferTemplate("DE Default", provTest())
	de.WorkspaceID = wsID
	de.Locale = "de-DE"
	de.IsDefault = true
	if _, err := s.Create(context.Background(), de); err != nil {
		t.Fatalf("de create: %v", err)
	}

	en := domain.NewOfferTemplate("EN Default", provTest())
	en.WorkspaceID = wsID
	en.Locale = "en-US"
	en.IsDefault = true
	if _, err := s.Create(context.Background(), en); err != nil {
		t.Fatalf("en create: expected different-locale defaults to both succeed, got %v", err)
	}
}

func TestOfferTemplateStore_Create_MissingProvenance_Rejected(t *testing.T) {
	db := pgtest.OpenTestDB(t)
	wsID := seedProductWorkspace(t, db)
	s := adapters.NewOfferTemplateStore(db)

	tpl := domain.OfferTemplate{WorkspaceID: wsID, Name: "No Prov"}
	_, err := s.Create(context.Background(), tpl)
	if !errors.Is(err, errs.ErrNullProvenance) {
		t.Fatalf("expected ErrNullProvenance, got %v", err)
	}
}
