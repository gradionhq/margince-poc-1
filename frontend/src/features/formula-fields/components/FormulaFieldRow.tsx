import type { components } from "../../../lib/api-client/generated/index.js";
import { Chip, Icon } from "../../../shared/ui/forge.js";
import { formatComputedFieldValue } from "../lib/format.js";

type ComputedField = components["schemas"]["ComputedField"];

export function FormulaFieldRow({ field }: { field: ComputedField }) {
  return (
    <div
      data-testid={`formula-field-row-${field.key}`}
      className="grid gap-gf-sm rounded-lg border border-gf-subtle bg-gf-card p-gf-md md:grid-cols-[minmax(0,1fr)_auto] md:items-center"
    >
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-gf-xs">
          <h4 className="text-gf-body font-semibold text-gf-primary">
            {field.label}
          </h4>
          <Chip variant="info">Σ Derived</Chip>
        </div>
        <p className="mt-1 text-gf-caption text-gf-secondary">
          {field.computable ? "Read-only computed value" : "Formula unavailable"}
        </p>
      </div>
      <div className="flex items-center justify-between gap-gf-sm md:justify-end">
        {field.computable && (
          <span title="Read-only — computed, cannot be edited">
            <Icon name="Lock" size={14} />
          </span>
        )}
        <span className="font-mono text-gf-body text-gf-primary">
          {formatComputedFieldValue(field)}
        </span>
      </div>
    </div>
  );
}
