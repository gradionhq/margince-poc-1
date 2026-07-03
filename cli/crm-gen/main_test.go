package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoFieldName(t *testing.T) {
	cases := map[string]string{"nickname": "Nickname", "expected_close_date": "ExpectedCloseDate", "owner_id": "OwnerId"}
	for in, want := range cases {
		if got := goFieldName(in); got != want {
			t.Errorf("goFieldName(%q)=%q want %q", in, got, want)
		}
	}
}

func TestOpenAPIFor(t *testing.T) {
	if openAPIFor("bigint") != "type: integer" {
		t.Error("bigint -> integer")
	}
	if openAPIFor("text NOT NULL") != "type: string" {
		t.Error("text -> string")
	}
	if openAPIFor("boolean") != "type: boolean" {
		t.Error("boolean -> boolean")
	}
}

func TestIdentValidation(t *testing.T) {
	if identRe.MatchString("Person") || !identRe.MatchString("person_email") {
		t.Error("ident validation wrong")
	}
}

func TestGenConnectorEmitsCurrentSeam(t *testing.T) {
	root := t.TempDir()
	if err := genConnector([]string{"acme_demo"}, root); err != nil {
		t.Fatalf("genConnector() error = %v", err)
	}

	dir := filepath.Join(root, "crm", "crm-capture", "connectors")
	goSrc, err := os.ReadFile(filepath.Join(dir, "acme_demo.go"))
	if err != nil {
		t.Fatalf("read generated connector: %v", err)
	}
	testSrc, err := os.ReadFile(filepath.Join(dir, "acme_demo_test.go"))
	if err != nil {
		t.Fatalf("read generated connector test: %v", err)
	}

	goText := string(goSrc)
	for _, want := range []string{
		"func (*AcmeDemoConnector) Descriptor() connector.Descriptor",
		"func (*AcmeDemoConnector) Normalize(context.Context, []byte) ([]connector.NormalizedRecord, error)",
		"func init() { connector.Register(&AcmeDemoConnector{}) }",
	} {
		if !strings.Contains(goText, want) {
			t.Fatalf("generated connector missing %q\n%s", want, goText)
		}
	}
	for _, disallowed := range []string{
		"func (*AcmeDemoConnector) Name() string",
		"(any, error)",
	} {
		if strings.Contains(goText, disallowed) {
			t.Fatalf("generated connector still contains %q\n%s", disallowed, goText)
		}
	}

	testText := string(testSrc)
	for _, want := range []string{
		"d := c.Descriptor()",
		"if d.Name != \"acme_demo\"",
		"if !errors.Is(err, connector.ErrSkip)",
	} {
		if !strings.Contains(testText, want) {
			t.Fatalf("generated connector test missing %q\n%s", want, testText)
		}
	}
}
