//go:build nopacks

package main

// imports_juris_nopacks.go is selected when building with -tags nopacks (non-DACH
// config). No jurisdiction packs are blank-imported, so For("de") returns false
// and no pack init() runs. This proves the compile-time switch compiles cleanly
// with zero packs linked.
