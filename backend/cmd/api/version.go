package main

// Build-time variables injected via -ldflags.
// Default values are used when the binary is built without the ldflags stamp
// (e.g. during local `go run` or unit tests).
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)
