package datasource

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gradionhq/margince/backend/internal/shared/apperrors"
)

// stub implements Provider for compile-time assertions.
type stub struct{}

func (stub) Search(_ context.Context, _ SearchQuery) (SearchResult, error) {
	return SearchResult{}, nil
}
func (stub) ListObjects(_ context.Context) ([]ObjectDef, error)             { return nil, nil }
func (stub) ListFields(_ context.Context, _ EntityType) ([]FieldDef, error) { return nil, nil }
func (stub) Create(_ context.Context, _ CreateInput) (EntityRef, error)     { return EntityRef{}, nil }

// Read and RunReport return an opaque `any` result; a nil result with no error is
// a valid empty response from this compile-assertion stub.
//
//nolint:nilnil // stub: nil opaque result + nil error is a valid empty response
func (stub) Read(_ context.Context, _ EntityRef) (Record, error) { return nil, nil }

//nolint:nilnil // stub: nil opaque result + nil error is a valid empty response
func (stub) RunReport(_ context.Context, _ ReportPlan) (ReportResult, error) { return nil, nil }

func (stub) Update(_ context.Context, _ UpdateInput) (EntityRef, error) { return EntityRef{}, nil }

func (stub) AdvanceDeal(_ context.Context, _ AdvanceDealInput) (EntityRef, error) {
	return EntityRef{}, nil
}

func (stub) Freshness(_ context.Context, _ EntityRef) (FreshnessInfo, error) {
	return FreshnessInfo{}, nil
}

func (stub) LinkConversation(_ context.Context, _ LinkConversationInput) (EntityRef, error) {
	return EntityRef{}, nil
}

func (stub) UnlinkConversation(_ context.Context, _ UnlinkConversationInput) error {
	return nil
}

var _ Provider = stub{}

func TestErrVersionSkewIsErrsSentinel(t *testing.T) {
	if !errors.Is(ErrVersionSkew, errs.ErrVersionSkew) {
		t.Error("datasource.ErrVersionSkew must satisfy errors.Is against errs.ErrVersionSkew")
	}
}

func TestTier0(t *testing.T) {
	// Construct every exported input/output type to prove no crm-core import needed.
	_ = EntityRef{Type: EntityPerson, ID: "1"}
	_ = SearchQuery{Type: EntityOrganization, Filter: map[string]any{}, Limit: 10}
	_ = SearchResult{Records: []Record{}, Total: 0}
	_ = ObjectDef{Type: EntityDeal, Label: "Deal"}
	_ = FieldDef{Name: "name", Type: "string", Label: "Name", Required: true}
	_ = ReportPlan{Name: "pipeline", Params: map[string]any{}}
	_ = CreateInput{Type: EntityActivity, Fields: nil, Source: "s", CapturedBy: "agent:1"}
	ifv := "v1"
	_ = UpdateInput{Type: EntityLead, ID: "2", Patch: nil, Source: "s", CapturedBy: "agent:1", IfVersion: &ifv}
	_ = AdvanceDealInput{DealID: "d1", ToStatus: "won", ChangedBy: "agent:1"}
	_ = FreshnessInfo{LastSyncedAt: time.Now(), Authoritative: true}
}

func TestUnsupportedByDatasourceError_ErrorMessage(t *testing.T) {
	e := UnsupportedError{Verb: "RunReport", Incumbent: "hubspot", Reason: "no run_report analogue on HubSpot"}
	got := e.Error()
	if got == "" {
		t.Fatal("Error() must not be empty")
	}
	if !strings.Contains(got, "RunReport") || !strings.Contains(got, "hubspot") {
		t.Errorf("Error() = %q; want verb+incumbent", got)
	}
}

func TestUnsupportedByDatasourceError_ErrorsIs(t *testing.T) {
	e := UnsupportedError{Verb: "ListObjects", Incumbent: "hubspot", Reason: "private-app only"}
	wrapped := fmt.Errorf("outer: %w", e)
	if !errors.Is(wrapped, ErrUnsupported) {
		t.Fatal("errors.Is must match ErrUnsupported through wrapping")
	}
}

func TestUnsupportedByDatasourceError_ErrorsAs(t *testing.T) {
	e := UnsupportedError{Verb: "ListFields", Incumbent: "hubspot", Reason: "private-app only"}
	wrapped := fmt.Errorf("outer: %w", e)
	var target UnsupportedError
	if !errors.As(wrapped, &target) {
		t.Fatal("errors.As must extract UnsupportedError")
	}
	if target.Verb != "ListFields" || target.Reason == "" {
		t.Errorf("extracted: %+v", target)
	}
}

func TestCapabilityManifest_Shape(t *testing.T) {
	m := CapabilityManifest{
		"search_records": CapabilitySupport{Supported: true},
		"run_report":     CapabilitySupport{Supported: false, Reason: "no run_report on HubSpot"},
	}
	if s := m["search_records"]; !s.Supported {
		t.Error("search_records should be supported")
	}
	if s := m["run_report"]; s.Supported || s.Reason == "" {
		t.Errorf("run_report: Supported=%v Reason=%q", s.Supported, s.Reason)
	}
}
