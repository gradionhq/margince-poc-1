package crmapprovals

import "strings"

// The §4.4 sensitivity classes a send-body field can be tagged with.
const (
	classPII             = "pii"
	classSpecialCategory = "special_category"
	classFinancial       = "financial"
	classMasked          = "masked"
)

// EgressFlag marks a field in a send body as sensitive per §4.4.
type EgressFlag struct {
	FieldPath        string
	SensitivityClass string // classPII | classFinancial | classSpecialCategory | classMasked
}

// sensitivityMap is the §4.4 static sensitivity catalogue.
var sensitivityMap = map[string]string{
	"email":         classPII,
	"full_name":     classPII,
	"name":          classPII,
	"phone":         classPII,
	"address":       classPII,
	"ip_address":    classPII,
	"dob":           classPII,
	"date_of_birth": classPII,
	"ssn":           classSpecialCategory,
	"health":        classSpecialCategory,
	"medical":       classSpecialCategory,
	"religion":      classSpecialCategory,
	"ethnicity":     classSpecialCategory,
	"income":        classFinancial,
	"revenue":       classFinancial,
	"salary":        classFinancial,
	"bank_account":  classFinancial,
	"credit_card":   classFinancial,
	"card_number":   classFinancial,
	"credit":        classFinancial,
	"financial":     classFinancial,
	"password":      classMasked,
	"secret":        classMasked,
	"token":         classMasked,
	"api_key":       classMasked,
}

// FlagEgress scans the send body and fieldMap keys against the §4.4 sensitivity
// map and returns a flag for each matching field. The fieldMap keys are field
// paths (e.g. "email", "bank_account"); body is scanned for keyword occurrences
// not already covered by fieldMap.
func FlagEgress(body string, fieldMap map[string]string) []EgressFlag {
	seen := map[string]bool{}
	var flags []EgressFlag

	// 1. Flag any fieldMap key that matches the sensitivity map.
	for field := range fieldMap {
		if class, ok := sensitivityMap[strings.ToLower(field)]; ok {
			key := field + ":" + class
			if !seen[key] {
				seen[key] = true
				flags = append(flags, EgressFlag{FieldPath: field, SensitivityClass: class})
			}
		}
	}

	// 2. Scan the body for sensitivity keywords not already surfaced via fieldMap.
	lower := strings.ToLower(body)
	for keyword, class := range sensitivityMap {
		if _, alreadyFlagged := fieldMap[keyword]; alreadyFlagged {
			continue
		}
		if strings.Contains(lower, keyword) {
			key := keyword + ":" + class
			if !seen[key] {
				seen[key] = true
				flags = append(flags, EgressFlag{FieldPath: keyword, SensitivityClass: class})
			}
		}
	}

	return flags
}
