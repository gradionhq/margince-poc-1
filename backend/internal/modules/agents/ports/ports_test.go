package ports_test

import (
	"context"
	"testing"
	"time"

	"github.com/gradionhq/margince/backend/internal/modules/agents/domain"
	"github.com/gradionhq/margince/backend/internal/modules/agents/ports"
)

func TestFixtureAssembler_ImplementsContextAssembler(t *testing.T) {
	var _ ports.ContextAssembler = ports.FixtureAssembler{}
}

func TestFixtureAssembler_ReturnsItsCannedView(t *testing.T) {
	want := domain.AssembledView{WorkspaceID: "ws-1", Facts: []domain.Fact{{EntityType: "activity", EntityID: "1"}}}
	f := ports.FixtureAssembler{View: want}
	got, err := f.Assemble(context.Background(), "ws-1", time.Now())
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	if got.WorkspaceID != want.WorkspaceID || len(got.Facts) != len(want.Facts) {
		t.Fatalf("Assemble() = %+v, want %+v", got, want)
	}
}

func TestFixtureAssembler_PropagatesAssemblerError(t *testing.T) {
	wantErr := context.DeadlineExceeded
	f := ports.FixtureAssembler{Err: wantErr}
	_, err := f.Assemble(context.Background(), "ws-1", time.Now())
	if err != wantErr {
		t.Fatalf("Assemble() err = %v, want %v", err, wantErr)
	}
}
