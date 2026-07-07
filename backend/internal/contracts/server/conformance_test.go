package server

import (
	"testing"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
)

// TestAllOperationsSatisfiesServerInterface is the compile-time conformance
// check for AC-D3: every operationId in crm.yaml (via the oapi-codegen
// chi-server ServerInterface) must have a concrete Go method somewhere in
// AllOperations. If this file fails to compile, either the generated
// ServerInterface gained an operation with no adapter method, or an adapter
// method's signature drifted from the generated one.
var _ types.ServerInterface = (*AllOperations)(nil)

func TestAllOperationsSatisfiesServerInterface(t *testing.T) {
	// The var _ assertion above is the actual test; a passing compile is a
	// passing test. This func exists so `go test` reports it explicitly.
}
