import type { components } from "../../../lib/api-client/generated/crm.js";

type ComputedField = components["schemas"]["ComputedField"];

export function FieldDefinitionRail({ field }: { field: ComputedField }) {
  return (
    <section className="rounded-xl border border-gf-subtle bg-gf-card p-gf-lg shadow-sm">
      <div className="flex flex-wrap items-start justify-between gap-gf-sm">
        <div>
          <p className="text-gf-caption uppercase tracking-wide text-gf-muted">
            Field definition
          </p>
          <h3 className="mt-gf-xs text-gf-title font-semibold text-gf-primary">
            {field.label}
          </h3>
        </div>
        <p className="text-gf-caption text-gf-secondary">computed:server</p>
      </div>

      <div className="mt-gf-md space-y-gf-md">
        <div>
          <p className="text-gf-caption text-gf-muted">Formula SQL</p>
          <pre
            data-testid="formula-sql"
            className="mt-gf-xs overflow-x-auto rounded-md border border-gf-subtle bg-gf-elevated px-gf-md py-gf-sm font-mono text-gf-caption text-gf-primary"
          >
            {field.formula_sql}
          </pre>
        </div>

        <div>
          <p className="text-gf-caption text-gf-muted">Dependencies</p>
          <ul
            data-testid="formula-dependencies"
            className="mt-gf-xs flex flex-wrap gap-gf-xs"
          >
            {field.dependencies.map((dependency: string) => (
              <li
                key={dependency}
                className="rounded-full border border-gf-subtle bg-gf-elevated px-gf-xs py-0.5 font-mono text-gf-caption text-gf-primary"
              >
                {dependency}
              </li>
            ))}
          </ul>
        </div>

        <p className="rounded-md border border-gf-subtle bg-gf-elevated px-gf-md py-gf-sm text-gf-caption text-gf-secondary">
          Authoring new formula logic is a reviewed source change, not a runtime
          builder. This needs the development path, not this screen.
        </p>
      </div>
    </section>
  );
}
