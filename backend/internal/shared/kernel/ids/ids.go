// Package ids is the Tier-0 identifier kernel (UUIDv7 per the spec).
package ids

import "github.com/google/uuid"

// New returns a fresh canonical UUID string (time-ordered UUIDv7, hyphenated).
// Matches the DB's uuidv7() default and the canonical form used in seeds, so an
// app-generated id echoed back in a response (e.g. before any DB round-trip) is
// byte-for-byte identical to the same id read back from Postgres.
func New() string {
	if u, err := uuid.NewV7(); err == nil {
		return u.String()
	}
	// NewV7 only errors if the system RNG fails; fall back to a random v4.
	return uuid.NewString()
}
