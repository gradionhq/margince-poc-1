//go:build !nopacks

package main

// imports_juris.go is the compile-time jurisdiction switch (ADR-0042): blank-
// importing a pack links it into this binary and runs its init() registration.
// Remove a line to build a binary without that jurisdiction. This is the ONLY
// place packs are wired; the require-set in cmd/server/go.mod is the real
// compile-time boundary (ADR-0042).
import _ "github.com/gradionhq/margince/crm-de"
