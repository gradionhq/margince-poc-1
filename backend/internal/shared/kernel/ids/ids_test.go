package ids

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewUnique(t *testing.T) {
	a, b := New(), New()
	if len(a) != 36 {
		t.Fatalf("want canonical UUID len 36, got %d (%q)", len(a), a)
	}
	if _, err := uuid.Parse(a); err != nil {
		t.Fatalf("New() must be a canonical UUID: %v", err)
	}
	if a == b {
		t.Fatal("ids should differ")
	}
}
