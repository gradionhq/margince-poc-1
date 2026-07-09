import { useState } from "react";
import type { components } from "../../../lib/api-client/generated/index.js";
import { RadioGroup } from "../../../shared/ui/forge.js";
import { ToastContainer } from "../../../shared/ui/ToastContainer.js";

type ComputedField = components["schemas"]["ComputedField"];
type Scenario = "212k" | "177k" | "lost";
type Toast = { id: string; variant: "info"; message: string };

const scenarioOptions = [
  { value: "212k", label: "212k" },
  { value: "177k", label: "177k" },
  { value: "lost", label: "lost" },
];

function formatEuroMinor(valueMinor: number) {
  return `${new Intl.NumberFormat("de-DE", {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(valueMinor / 100)} €`;
}

function formatSignedEuroMinor(valueMinor: number) {
  if (valueMinor > 0) return `+${formatEuroMinor(valueMinor)}`;
  if (valueMinor < 0) return `-${formatEuroMinor(Math.abs(valueMinor))}`;
  return formatEuroMinor(0);
}

function scenarioDeltaMinor(
  scenario: Scenario,
  openPipeline: ComputedField | undefined,
) {
  if (scenario === "177k") return -3500000;
  if (scenario === "lost") return -(openPipeline?.value_minor ?? 0);
  return 0;
}

function scenarioLabel(scenario: Scenario) {
  return scenario === "lost" ? "lost" : "40%";
}

export function RecomputeDriver({
  openPipeline,
  onFlash,
}: {
  openPipeline: ComputedField | undefined;
  onFlash: (deltaMinor: number) => void;
}) {
  const [selectedScenario, setSelectedScenario] = useState<Scenario>("212k");
  const [toasts, setToasts] = useState<Toast[]>([]);
  const baseValueMinor = openPipeline?.value_minor ?? 0;
  const deltaMinor = scenarioDeltaMinor(selectedScenario, openPipeline);
  const simulatedValueMinor = baseValueMinor + deltaMinor;

  function pushToast(message: string) {
    setToasts((current) => [
      ...current,
      { id: crypto.randomUUID(), variant: "info", message },
    ]);
  }

  function handleScenarioChange(next: Scenario) {
    setSelectedScenario(next);
    const nextDeltaMinor = scenarioDeltaMinor(next, openPipeline);
    onFlash(nextDeltaMinor);
    if (next === "lost") {
      pushToast("Simulation only - the open pipeline drops to zero.");
      return;
    }
    const change =
      nextDeltaMinor === 0 ? "no change" : formatSignedEuroMinor(nextDeltaMinor);
    pushToast(`Simulation only ${change}. Nothing is saved.`);
  }

  return (
    <section className="rounded-xl border border-gf-subtle bg-gf-card p-gf-md shadow-sm">
      <div className="flex items-start justify-between gap-gf-md">
        <div className="min-w-0">
          <p className="text-gf-caption uppercase tracking-wide text-gf-secondary">
            Right rail driver
          </p>
          <h3 className="text-gf-body font-semibold text-gf-primary">
            See it recompute
          </h3>
          <p className="mt-gf-xs text-gf-caption text-gf-secondary">
            Try it - nothing is saved.
          </p>
        </div>
        <div
          className="rounded-full border border-gf-subtle bg-gf-page px-gf-sm py-gf-xs text-gf-caption font-semibold text-gf-primary"
          aria-label="Win probability label"
        >
          {scenarioLabel(selectedScenario)}
        </div>
      </div>

      <div className="mt-gf-md">
        <RadioGroup
          label="Scenario"
          name="recompute-driver-scenario"
          value={selectedScenario}
          onChange={(value) => handleScenarioChange(value as Scenario)}
          options={scenarioOptions}
        />
      </div>

      <div className="mt-gf-md grid grid-cols-1 gap-gf-sm rounded-lg border border-gf-subtle bg-gf-page p-gf-md sm:grid-cols-2">
        <div>
          <p className="text-gf-caption text-gf-secondary">Simulated open pipeline</p>
          <p className="font-mono text-gf-body font-semibold text-gf-primary">
            {formatEuroMinor(simulatedValueMinor)}
          </p>
        </div>
        <div>
          <p className="text-gf-caption text-gf-secondary">Delta</p>
          <p className="font-mono text-gf-body font-semibold text-gf-primary">
            {formatEuroMinor(deltaMinor)}
          </p>
        </div>
      </div>

      <ToastContainer
        toasts={toasts}
        onDismiss={(id) => setToasts((current) => current.filter((toast) => toast.id !== id))}
      />
    </section>
  );
}
