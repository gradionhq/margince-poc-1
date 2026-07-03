// Package httpserver holds the shared HTTP error writer and middleware stack
// extracted from the cmd/api composition root (1c restructure,
// task-3-brief.md). package main → package httpserver is the one authorized
// rename for this extraction; behavior is unchanged.
package httpserver

import (
	"encoding/json"
	"net/http"

	"github.com/gradionhq/margince/backend/internal/contracts/types"
)

// Problem machine-codes shared across the cmd/api handlers. These mirror the
// codes the contract documents and the crm-core handlers emit; keeping them in
// one place is both the goconst answer and the single source the error writer
// reads from.
const (
	CodeInternal       = "internal"
	CodeBadRequest     = "bad_request"
	CodeUnauthorized   = "unauthorized"
	CodeForbidden      = "forbidden"
	CodeNotFound       = "not_found"
	CodeConflict       = "conflict"
	CodeValidation     = "validation_error"
	CodeScopeExceeded  = "scope_exceeds_grantor"
	CodeAlreadyDecided = "already_decided"
)

// WriteProblem is the centralized RFC 7807 problem+json writer for cmd/api —
// the single choke point §4 mandates instead of http.Error shipping a JSON body.
// It sets application/problem+json, the status line, and a minimal {code,status}
// body.
func WriteProblem(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(types.Problem{Code: code, Status: status})
}

// WriteInternal is the shorthand for the pervasive 500 problem body that the
// handlers previously shipped via http.Error(`{"code":"internal"}`, 500).
func WriteInternal(w http.ResponseWriter) {
	WriteProblem(w, http.StatusInternalServerError, CodeInternal)
}
