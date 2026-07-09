import { Button, RadioGroup } from "../../../shared/ui/forge.js";
import { SearchField } from "../../../shared/ui/SearchField.js";
import type { ActorFilter } from "../hooks/useFieldHistoryView.js";

export function FieldHistoryControls({
  actor,
  onActorChange,
  field,
  onFieldChange,
  fieldOptions,
  search,
  onSearchChange,
  hasActiveFilters,
  onClearFilters,
}: {
  actor: ActorFilter;
  onActorChange: (a: ActorFilter) => void;
  field: string | null;
  onFieldChange: (f: string | null) => void;
  fieldOptions: { field: string; label: string }[];
  search: string;
  onSearchChange: (s: string) => void;
  hasActiveFilters: boolean;
  onClearFilters: () => void;
}) {
  return (
    <div className="flex flex-wrap items-center gap-gf-md py-gf-sm">
      <RadioGroup
        label="Actor"
        name="field-history-actor"
        value={actor}
        onChange={(v) => onActorChange(v as ActorFilter)}
        options={[
          { value: "all", label: "All actors" },
          { value: "human", label: "Human" },
          { value: "agent", label: "Agent" },
        ]}
      />
      <div className="flex flex-wrap items-center gap-gf-xs">
        <button
          type="button"
          onClick={() => onFieldChange(null)}
          className={`rounded-full border px-gf-sm py-gf-xs text-gf-caption ${
            field === null
              ? "border-gf-accent bg-gf-accent-subtle text-gf-accent"
              : "border-gf-subtle text-gf-secondary"
          }`}
        >
          All fields
        </button>
        {fieldOptions.map((opt) => (
          <button
            key={opt.field}
            type="button"
            onClick={() => onFieldChange(opt.field)}
            className={`rounded-full border px-gf-sm py-gf-xs text-gf-caption ${
              field === opt.field
                ? "border-gf-accent bg-gf-accent-subtle text-gf-accent"
                : "border-gf-subtle text-gf-secondary"
            }`}
          >
            {opt.label}
          </button>
        ))}
      </div>
      <div className="min-w-[220px]">
        <SearchField value={search} onChange={onSearchChange} placeholder="Search fields…" />
      </div>
      {hasActiveFilters && (
        <Button variant="ghost" size="sm" onClick={onClearFilters}>
          Clear filters
        </Button>
      )}
    </div>
  );
}
