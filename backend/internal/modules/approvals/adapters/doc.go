// Package adapters is the structural scaffold for the approvals module's storage adapters
// (WS-E-a). Implementation remains in the parent crmapprovals package during this migration
// phase; this directory marks the intended future home for DB adapters once the internal
// tests that reference private symbols (parseToken) are updated to use the external test pattern.
package adapters
