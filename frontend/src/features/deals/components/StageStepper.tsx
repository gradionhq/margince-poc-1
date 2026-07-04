import type { Stage } from "../../../lib/api-client/generated/index.js";

type StageState = "done" | "current" | "upcoming" | "muted";

function stateFor(
  stage: Stage,
  currentStageId: string,
  currentPosition: number,
  dealStatus: "open" | "won" | "lost",
): StageState {
  if (stage.semantic !== "open") {
    // Terminal node: highlighted only when the deal is actually in that terminal state.
    return stage.id === currentStageId && dealStatus !== "open"
      ? "current"
      : "muted";
  }
  if (stage.id === currentStageId) return "current";
  return stage.position < currentPosition ? "done" : "upcoming";
}

export function StageStepper({
  stages,
  currentStageId,
  dealStatus,
}: {
  stages: Stage[];
  currentStageId: string;
  dealStatus: "open" | "won" | "lost";
}) {
  const ordered = stages.slice().sort((a, b) => a.position - b.position);
  const current = ordered.find((s) => s.id === currentStageId);
  const currentPosition = current?.position ?? 0;

  const stateClasses: Record<StageState, string> = {
    done: "bg-gf-status-success-subtle text-gf-status-success-fg border-gf-status-success-fg",
    current: "bg-gf-accent text-gf-on-accent border-gf-accent font-semibold",
    upcoming: "bg-gf-card text-gf-secondary border-gf-subtle",
    muted: "bg-gf-card text-gf-muted border-gf-subtle opacity-50",
  };

  return (
    <ol
      data-testid="stage-stepper"
      className="flex items-center gap-gf-xs overflow-x-auto"
    >
      {ordered.map((stage) => {
        const state = stateFor(
          stage,
          currentStageId,
          currentPosition,
          dealStatus,
        );
        return (
          <li
            key={stage.id}
            data-testid={`stage-node-${stage.id}`}
            data-stage-state={state}
            className={`rounded-full border px-gf-sm py-gf-xs text-gf-caption whitespace-nowrap ${stateClasses[state]}`}
          >
            {state === "done" && "✓ "}
            {stage.name}
          </li>
        );
      })}
    </ol>
  );
}
