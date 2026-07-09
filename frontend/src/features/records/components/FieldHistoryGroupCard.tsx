import { useState } from "react";
import {
  COMPUTED_MONEY_FIELD,
  type FieldHistoryEntry,
  formatCurrentFieldValue,
  formatDiffFieldValue,
  originLabel,
} from "../api/fieldHistory.js";
import type { FieldHistoryGroup } from "../hooks/useFieldHistoryView.js";
import { FieldHistoryExplainBox } from "./FieldHistoryExplainBox.js";

function EvidencePanel({ evidence }: { evidence: Record<string, unknown> }) {
  const quote = typeof evidence.quote === "string" ? evidence.quote : undefined;
  const event = typeof evidence.event === "string" ? evidence.event : undefined;
  const sourceUrl =
    typeof evidence.source_url === "string" ? evidence.source_url : undefined;
  const confidence =
    typeof evidence.confidence === "string" ? evidence.confidence : undefined;
  const confidenceNote =
    typeof evidence.confidence_note === "string"
      ? evidence.confidence_note
      : undefined;

  return (
    <div
      data-testid="field-history-evidence-panel"
      className="mt-gf-xs rounded-md border border-gf-subtle bg-gf-card p-gf-sm text-gf-caption text-gf-secondary"
    >
      <p className="text-gf-primary">{quote ?? event ?? "Evidence provided"}</p>
      <div className="mt-gf-xs flex items-center gap-gf-sm">
        {sourceUrl && (
          <a href={sourceUrl} className="text-gf-accent underline">
            source
          </a>
        )}
        {confidence && (
          <span className="inline-flex items-center gap-gf-xs">
            <span
              className={`h-2 w-2 rounded-full ${
                confidence === "high"
                  ? "bg-gf-status-success"
                  : "bg-gf-status-warning"
              }`}
            />
            {confidence}
            {confidenceNote ? ` — ${confidenceNote}` : ""}
          </span>
        )}
      </div>
    </div>
  );
}

function DiffRow({
  entry,
  currency,
  field,
  fieldEntries,
}: {
  entry: FieldHistoryEntry;
  currency: string;
  field: string;
  fieldEntries: FieldHistoryEntry[];
}) {
  const [evidenceOpen, setEvidenceOpen] = useState(false);
  // old_value/new_value are `string | null` per the contract ("— empty —" is
  // represented as null) — the `?` in the generated type is an artifact of
  // an optional JSON key, not a third real state, so `== null` (not `===`)
  // is the type-safe way to treat "absent" the same as "explicitly empty".
  const from =
    entry.old_value == null
      ? originLabel(entry, fieldEntries)
      : formatDiffFieldValue(field, entry.old_value, currency);
  const to =
    entry.new_value == null
      ? "— removed —"
      : formatDiffFieldValue(field, entry.new_value, currency);
  const hasEvidence = entry.actor_type === "agent" && !!entry.evidence;

  return (
    <li
      data-testid={`field-history-row-${entry.id}`}
      className="py-gf-sm border-b border-gf-subtle last:border-b-0"
    >
      <div className="flex items-center gap-gf-sm font-mono text-gf-body">
        <span
          className={
            from.startsWith("—")
              ? "italic text-gf-muted"
              : "text-gf-tertiary line-through"
          }
        >
          {from}
        </span>
        <span aria-hidden="true">→</span>
        <span className="rounded bg-gf-status-success-subtle px-gf-xs font-medium text-gf-primary">
          {to}
        </span>
      </div>
      <p className="mt-gf-xs text-gf-caption text-gf-secondary">
        {new Date(entry.changed_at).toLocaleString()} · {entry.actor_type}
        {entry.passport_id ? ` · passport ${entry.passport_id}` : ""}
        {hasEvidence && (
          <button
            type="button"
            onClick={() => setEvidenceOpen((o) => !o)}
            className="ml-gf-sm text-gf-accent underline"
          >
            evidence
          </button>
        )}
      </p>
      {hasEvidence && evidenceOpen && (
        <EvidencePanel evidence={entry.evidence as Record<string, unknown>} />
      )}
    </li>
  );
}

export function FieldHistoryGroupCard({
  group,
  currency,
}: {
  group: FieldHistoryGroup;
  currency: string;
}) {
  return (
    <div
      data-testid={`field-history-group-${group.field}`}
      className="mt-gf-md rounded-lg border border-gf-subtle bg-gf-card p-gf-md"
    >
      <div className="flex items-center gap-gf-sm">
        <h3 className="text-gf-body font-semibold text-gf-primary">
          {group.label}
        </h3>
        <code className="text-gf-micro text-gf-tertiary">{group.field}</code>
        <span className="ml-auto text-gf-caption text-gf-tertiary">
          {group.allEntries.length} change
          {group.allEntries.length === 1 ? "" : "s"}
        </span>
      </div>
      <div className="mt-gf-sm flex items-center gap-gf-sm">
        <span className="text-gf-caption text-gf-secondary">Current</span>
        <span className="font-mono text-gf-body text-gf-primary">
          {formatCurrentFieldValue(group.field, group.currentValue, currency)}
        </span>
      </div>
      {group.field === COMPUTED_MONEY_FIELD &&
        typeof group.currentValue === "number" && (
          <FieldHistoryExplainBox
            grossMinor={group.currentValue}
            currency={currency}
          />
        )}
      {group.allEntries.length === 0 ? (
        <p className="mt-gf-sm text-gf-body text-gf-secondary">
          Set on create and never changed — the audit log records no edits. An
          empty history is honest, not a gap.
        </p>
      ) : (
        <ul className="mt-gf-sm">
          {group.visibleEntries.map((e) => (
            <DiffRow
              key={e.id}
              entry={e}
              currency={currency}
              field={group.field}
              fieldEntries={group.allEntries}
            />
          ))}
        </ul>
      )}
    </div>
  );
}
