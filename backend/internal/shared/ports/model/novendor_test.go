package model

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestModelImportsNoVendorSDK(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	banned := []string{
		"anthropic", "openai", "/genai", "generativeai",
		"ollama", "mistral", "cohere", "sashabaranov",
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, imp := range f.Imports {
			p := strings.ToLower(imp.Path.Value)
			for _, b := range banned {
				if strings.Contains(p, b) {
					t.Errorf("%s imports banned vendor SDK %s", name, imp.Path.Value)
				}
			}
		}
	}
}
