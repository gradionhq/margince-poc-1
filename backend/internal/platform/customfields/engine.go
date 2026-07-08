// Package customfields is the governed add-field engine (custom-fields.md):
// the single chokepoint in the system allowed to run a runtime ALTER TABLE.
// It validates a field definition against the closed type/object sets,
// derives its namespaced physical column identifier, generates the DDL only
// from the validated spec (never raw user text), detects a structural
// request and refuses it, and runs the ALTER TABLE + custom_field catalog
// INSERT + one audit_log row atomically. See create.go for the transaction
// orchestration and its role-switch note (margince_app has no ALTER
// privilege — the DDL must run before downgrading).
package customfields

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/lib/pq"
)

// The closed type/object sets (CUSTOM-FIELDS-PARAM-1/PARAM-2). No cap,
// no widening — the surface itself is the knob (custom-fields.md).
const (
	TypeText     = "text"
	TypeNumber   = "number"
	TypeDate     = "date"
	TypeCurrency = "currency"
	TypePicklist = "picklist"
	TypeBoolean  = "boolean"
)

var allowedObjects = map[string]bool{
	"person": true, "organization": true, "deal": true, "lead": true, "activity": true,
}

var allowedTypes = map[string]bool{
	TypeText: true, TypeNumber: true, TypeDate: true, TypeCurrency: true, TypePicklist: true, TypeBoolean: true,
}

var currencyCodeRe = regexp.MustCompile(`^[A-Z]{3}$`)

// FieldSpec is a candidate custom-field definition — the only source the
// DDL generator and catalog insert are permitted to read from. Source and
// CapturedBy are contract-required (createCustomField) but never stored on
// the custom_field catalog row itself; they feed the audit entry's evidence
// only (custom-fields.md: "provenance on create is captured for audit
// attribution only").
type FieldSpec struct {
	Object     string
	Label      string
	Type       string
	Currency   string
	Options    []string
	Source     string
	CapturedBy string
}

// FieldError is one field-level validation failure, matching the contract's
// ValidationError `details.errors[]` shape ({field, code}) — the json tags
// are load-bearing: without them the wire body would marshal as
// {"Field":...,"Code":...} (capitalised), violating the contract.
type FieldError struct {
	Field string `json:"field"`
	Code  string `json:"code"`
}

// FieldError.Field / audit-entry map keys shared across Validate, Create's
// audit entry (create.go), and the handler's diff map (handler.go) —
// extracted to a package-level const so the repeated literal satisfies
// golangci-lint's goconst rule (each string repeats 3+ times across this
// package's production code).
const (
	fieldObject = "object"
	fieldType   = "type"
	fieldLabel  = "label"
)

// codeRequired is the FieldError.Code value for a missing-required-field
// violation, reused by label/source/captured_by's presence checks below —
// extracted for the same goconst reason as the fieldXxx consts above.
const codeRequired = "required"

// Validate checks spec against the closed type/object sets and the
// conditional-required rules (CreateCustomFieldRequest doc: currency
// required iff type=currency, options required non-empty iff
// type=picklist). Returns every violation found, not just the first.
func Validate(spec FieldSpec) []FieldError {
	var errs []FieldError
	if !allowedObjects[spec.Object] {
		errs = append(errs, FieldError{Field: fieldObject, Code: "unsupported_object"})
	}
	if !allowedTypes[spec.Type] {
		errs = append(errs, FieldError{Field: fieldType, Code: "unsupported_type"})
	}
	if strings.TrimSpace(spec.Label) == "" {
		errs = append(errs, FieldError{Field: fieldLabel, Code: codeRequired})
	}
	if spec.Type == TypeCurrency && !currencyCodeRe.MatchString(spec.Currency) {
		errs = append(errs, FieldError{Field: "currency", Code: "required_for_type_currency"})
	}
	if spec.Type == TypePicklist && len(spec.Options) == 0 {
		errs = append(errs, FieldError{Field: "options", Code: "required_for_type_picklist"})
	}
	if strings.TrimSpace(spec.Source) == "" {
		errs = append(errs, FieldError{Field: "source", Code: codeRequired})
	}
	if strings.TrimSpace(spec.CapturedBy) == "" {
		errs = append(errs, FieldError{Field: "captured_by", Code: codeRequired})
	}
	return errs
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// maxSlugLen keeps `cf_<slug>_check` (the longest identifier this package
// generates) safely under Postgres's 63-byte identifier cap.
const maxSlugLen = 40

// DeriveSlug turns a display label into the admin-facing key the physical
// column name derives from (CUSTOM-FIELDS-PARAM-3): lowercased,
// non-alphanumeric runs collapsed to a single underscore, trimmed, and
// length-capped. Never returns raw label text.
func DeriveSlug(label string) string {
	s := strings.ToLower(strings.TrimSpace(label))
	s = nonAlnum.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if s == "" {
		s = "field"
	}
	if len(s) > maxSlugLen {
		s = strings.Trim(s[:maxSlugLen], "_")
	}
	return s
}

// ColumnName derives the cf_-prefixed physical column identifier from slug
// (CUSTOM-FIELDS-PARAM-3) — never client-supplied, immutable once live.
func ColumnName(slug string) string {
	return "cf_" + slug
}

var identifierRe = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// validIdentifier defensively re-checks an identifier even though both
// object and columnName are always server-derived by this point — belt and
// suspenders per the spec's explicit instruction.
func validIdentifier(s string) bool {
	return len(s) > 0 && len(s) <= 63 && identifierRe.MatchString(s)
}

// structuralKeywords are the AC-custom-fields-5 example phrases: a label
// smelling like a new object, relationship, or logic is refused, never
// silently accepted (CUSTOM-FIELDS-AC-4/AC-8).
var structuralKeywords = []string{
	"object", "relationship", "link to", "lookup to", "formula", "validation rule",
}

// IsStructural reports whether label smells like a structural request
// rather than a bounded scalar attribute.
func IsStructural(label string) bool {
	l := strings.ToLower(label)
	for _, kw := range structuralKeywords {
		if strings.Contains(l, kw) {
			return true
		}
	}
	return false
}

// sqlType maps a validated field type to its storage type
// (CUSTOM-FIELDS-PARAM-4): text->text, number->numeric (string round-trip,
// no float), date->date, currency->bigint minor-units (ISO-4217 code lives
// in the catalog row, not the column), boolean->boolean,
// picklist->text+generated CHECK.
func sqlType(fieldType string) (string, error) {
	switch fieldType {
	case TypeText:
		return "text", nil
	case TypeNumber:
		return "numeric", nil
	case TypeDate:
		return "date", nil
	case TypeCurrency:
		return "bigint", nil
	case TypePicklist:
		return "text", nil
	case TypeBoolean:
		return "boolean", nil
	default:
		return "", fmt.Errorf("customfields: unsupported type %q", fieldType)
	}
}

// BuildDDL returns the ALTER TABLE statement adding the validated field's
// column — CUSTOM-FIELDS-SCHEMA-2: `ALTER TABLE <object> ADD COLUMN
// cf_<slug> <mapped-type> NULL`, plus a generated CHECK constraint for a
// picklist's allowed values (CUSTOM-FIELDS-PARAM-4). Every token is derived
// only from spec (already closed-set validated by Validate) and
// pq-quoted identifiers/literals — never raw request text, so an
// injection attempt in Label cannot reach the database as free text
// (CUSTOM-FIELDS-AC-12).
func BuildDDL(object, columnName string, spec FieldSpec) (string, error) {
	if !validIdentifier(object) {
		return "", fmt.Errorf("customfields: invalid object identifier %q", object)
	}
	if !validIdentifier(columnName) {
		return "", fmt.Errorf("customfields: invalid column identifier %q", columnName)
	}
	colType, err := sqlType(spec.Type)
	if err != nil {
		return "", err
	}
	stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s NULL",
		pq.QuoteIdentifier(object), pq.QuoteIdentifier(columnName), colType)
	if spec.Type == TypePicklist {
		checkName := columnName + "_check"
		if !validIdentifier(checkName) {
			return "", fmt.Errorf("customfields: invalid check-constraint identifier %q", checkName)
		}
		quotedCol := pq.QuoteIdentifier(columnName)
		var quotedOpts []string
		for _, o := range spec.Options {
			quotedOpts = append(quotedOpts, pq.QuoteLiteral(o))
		}
		stmt += fmt.Sprintf(", ADD CONSTRAINT %s CHECK (%s IS NULL OR %s IN (%s))",
			pq.QuoteIdentifier(checkName), quotedCol, quotedCol, strings.Join(quotedOpts, ", "))
	}
	return stmt, nil
}
