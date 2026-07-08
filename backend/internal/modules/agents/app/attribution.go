package app

// ActorOvernight is the attribution string for every audit row, approval
// request, and domain event this module produces (OVN-AC-8) — the
// established "agent:<name>" convention (backend/internal/shared/kernel/
// prov/prov.go; grep "agent:" elsewhere in the tree, e.g. "agent:sdr").
const ActorOvernight = "agent:overnight"

// TopicOvernightApplied is the domain event this module emits whenever an
// effect actually lands — from the narrow 🟢 lane (apply.go) or from the
// approval-decided executor (executor.go) — separate from the approvals
// module's own approval.requested/decided bookkeeping events.
const TopicOvernightApplied = "overnight.applied"

// entityTypeFromTarget splits a "kind:id" TargetEntity into its entity type
// (e.g. "deal:abc" -> "deal") for the audit_log entity_type column.
func entityTypeFromTarget(target string) string {
	for i := 0; i < len(target); i++ {
		if target[i] == ':' {
			return target[:i]
		}
	}
	return target
}

// entityIDFromTarget splits a "kind:id" TargetEntity into its entity id
// (e.g. "deal:abc" -> "abc") for the event_outbox entity_id column, which is
// uuid NOT NULL (000016_event_outbox.up.sql) — unlike audit_log.entity_id,
// there is no nullable escape hatch, so the id portion must be extracted.
func entityIDFromTarget(target string) string {
	for i := 0; i < len(target); i++ {
		if target[i] == ':' {
			return target[i+1:]
		}
	}
	return target
}
