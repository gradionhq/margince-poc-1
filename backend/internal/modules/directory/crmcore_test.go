package crmcore

import (
	"testing"

	"github.com/gradionhq/margince/backend/internal/shared/kernel/prov"
)

func testProv() prov.Provenance { return prov.Provenance{Source: "test", CapturedBy: "human:test"} }

func TestNewPerson_HasID(t *testing.T) {
	person := NewPerson("Alice Smith", testProv())
	if person.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if person.FullName != "Alice Smith" {
		t.Fatalf("expected FullName=Alice Smith, got %q", person.FullName)
	}
	if person.Version != 1 {
		t.Fatalf("expected Version=1, got %d", person.Version)
	}
}

func TestNewOrganization_HasID(t *testing.T) {
	org := NewOrganization("Acme GmbH", testProv())
	if org.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if org.DisplayName != "Acme GmbH" {
		t.Fatalf("expected DisplayName=Acme GmbH, got %q", org.DisplayName)
	}
}

func TestNewDeal_StatusOpen(t *testing.T) {
	deal := NewDeal("Big Deal", "pipe-1", "stage-1", testProv())
	if deal.Status != "open" {
		t.Fatalf("expected Status=open, got %q", deal.Status)
	}
}

func TestNewActivity_HasKind(t *testing.T) {
	a := NewActivity("email", testProv())
	if a.Kind != "email" {
		t.Fatalf("expected Kind=email, got %q", a.Kind)
	}
}

func TestNewLead_StatusNew(t *testing.T) {
	l := NewLead(testProv())
	if l.Status != "new" {
		t.Fatalf("expected Status=new, got %q", l.Status)
	}
}
