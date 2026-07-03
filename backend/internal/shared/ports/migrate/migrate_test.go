package migrate

import "testing"

// TestPackTable locks the per-pack tracking-table naming: each enabled pack
// applies into its own schema_migrations_<code> table, distinct from core's
// default schema_migrations. The runner derives <code> from Pack.Code() at
// runtime — no country literal lives in this package.
func TestPackTable(t *testing.T) {
	if got, want := packTable("de"), "schema_migrations_de"; got != want {
		t.Errorf("packTable(%q) = %q, want %q", "de", got, want)
	}
	if got, want := packTable("eu"), "schema_migrations_eu"; got != want {
		t.Errorf("packTable(%q) = %q, want %q", "eu", got, want)
	}
}
