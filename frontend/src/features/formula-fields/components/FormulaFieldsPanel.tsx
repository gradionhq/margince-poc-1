import { useEffect, useState } from "react";
import type { Organization } from "../../../lib/api-client/generated/index.js";
import { Chip, SectionHeader } from "../../../shared/ui/forge.js";
import { ExplainBox } from "./ExplainBox.js";
import { FieldDefinitionRail } from "./FieldDefinitionRail.js";
import { FormulaFieldRow } from "./FormulaFieldRow.js";
import { RecomputeDriver } from "./RecomputeDriver.js";
import { SendToDevelopmentCard } from "./SendToDevelopmentCard.js";

export function FormulaFieldsPanel({ org }: { org: Organization }) {
  const [flashKey, setFlashKey] = useState<string | null>(null);
  const computedFields = org.computed_fields;
  const openPipeline = computedFields?.find(
    (field) => field.key === "open_pipeline",
  );

  useEffect(() => {
    if (!flashKey) return;
    const timeout = window.setTimeout(() => setFlashKey(null), 1200);
    return () => window.clearTimeout(timeout);
  }, [flashKey]);

  if (!computedFields) return null;

  if (computedFields.length === 0) {
    return (
      <section
        data-testid="formula-fields-panel"
        className="rounded-lg border border-gf-subtle bg-gf-card p-gf-lg"
      >
        <SectionHeader label="Formula fields" />
        <p className="mt-gf-sm text-gf-body text-gf-muted">
          No computed fields yet.
        </p>
      </section>
    );
  }

  return (
    <section
      data-testid="formula-fields-panel"
      className="rounded-lg border border-gf-subtle bg-gf-card p-gf-lg"
    >
      <div className="flex flex-wrap items-start justify-between gap-gf-sm">
        <div>
          <SectionHeader label="Formula fields" />
          <p className="mt-gf-xs text-gf-caption text-gf-secondary">
            Recomputes on every write
          </p>
        </div>
        <Chip variant="info">read-only computed</Chip>
      </div>

      <div className="mt-gf-lg grid grid-cols-1 gap-gf-lg xl:grid-cols-[minmax(0,1fr)_22rem]">
        <div className="space-y-gf-md">
          {computedFields.map((field) => (
            <div
              key={field.key}
              data-flash={flashKey === field.key ? "true" : "false"}
              className={
                flashKey === field.key
                  ? "rounded-xl ring-2 ring-gf-accent ring-offset-2 ring-offset-gf-card transition"
                  : undefined
              }
            >
              <FormulaFieldRow field={field} />
              <div className="mt-gf-sm">
                <ExplainBox field={field} />
              </div>
            </div>
          ))}
        </div>

        <div className="space-y-gf-md">
          <RecomputeDriver
            openPipeline={openPipeline}
            onFlash={() => setFlashKey("open_pipeline")}
          />
          <SendToDevelopmentCard />
          {openPipeline && <FieldDefinitionRail field={openPipeline} />}
        </div>
      </div>
    </section>
  );
}
