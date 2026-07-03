package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

// Directory and file permissions for generated scaffolding. Generated source is
// not secret, but the gate (gosec G301/G306) wants the conservative bits; tools
// regenerate freely so group/other write is never needed.
const (
	dirPerm  os.FileMode = 0o750
	filePerm os.FileMode = 0o600
)

// tmplKeyName / tmplKeySnake are the template data keys every scaffold shares:
// PascalCase identifier and the original snake_case name.
const (
	tmplKeyName  = "Name"
	tmplKeySnake = "Snake"
)

// scaffold is one source file a generator emits: a parsed template plus the
// relative path (under the target dir) it renders to.
type scaffold struct {
	filename string // e.g. "<name>.go"
	tmpl     *template.Template
}

// emitPair renders the .go + _test.go scaffold pair for a generated artifact
// into dir, then prints the user-facing summary. It centralizes MkdirAll, the
// permission bits, and template execution so each gen* command stays a thin
// description of *what* it scaffolds. data carries the template keys.
func emitPair(dir string, data map[string]string, files []scaffold, summary string) error {
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return err
	}
	for _, f := range files {
		if err := renderTmpl(filepath.Join(dir, f.filename), f.tmpl, data); err != nil {
			return err
		}
	}
	printSummary(summary)
	return nil
}

// renderTmpl executes t with data and writes the result to path.
func renderTmpl(path string, t *template.Template, data any) error {
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), filePerm)
}

// printSummary writes a generator's user-facing summary to stdout. The CLI's
// whole job is to report what it scaffolded, so this is genuine stdout output
// (not logging) — routed through Fprintf to keep it off the fmt.Print* path.
func printSummary(s string) {
	// A failed write to stdout (e.g. a closed pipe) is not actionable for a
	// one-shot generator; the scaffold files are already on disk by now.
	_, _ = fmt.Fprint(os.Stdout, s)
}
