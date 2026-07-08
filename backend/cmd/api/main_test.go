package main

import (
	"testing"

	"github.com/gradionhq/margince/backend/pkg/shared/ports/jurisdiction"
)

func TestJurisdictionWired(t *testing.T) {
	if _, ok := jurisdiction.For("de"); !ok {
		t.Fatal("expected crm-de linked via imports_juris.go")
	}
}
