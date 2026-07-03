package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// envReadRe matches direct process-environment reads.
var envReadRe = regexp.MustCompile(`os\.(Getenv|LookupEnv)\b`)

// TestConfigIsTheSoleEnvRoot enforces the cmd/server composition-root invariant:
// config.go is the ONLY file in this package that reads the process environment.
// Every other file must receive its configuration through the resolved Config
// struct. This guard is what stopped the env-template ↔ config.go drift that
// accumulated when features (STT, blobstore) each grew their own os.Getenv calls.
//
// If you legitimately need a new env var, add it to Config in config.go and
// thread the value through — do not call os.Getenv elsewhere.
func TestConfigIsTheSoleEnvRoot(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	const allowed = "config.go"
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") || name == allowed {
			continue
		}
		src, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatal(err)
		}
		if loc := envReadRe.FindIndex(src); loc != nil {
			line := 1 + strings.Count(string(src[:loc[0]]), "\n")
			t.Errorf("%s:%d reads the environment directly (os.Getenv/LookupEnv); "+
				"move it into Config in %s and pass the value through", name, line, allowed)
		}
	}
}
