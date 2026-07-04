import {
  DndContext,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
} from "@dnd-kit/core";
import { useState } from "react";
import { useAdvanceDeal } from "../api/deals.js";
import type { Deal, Stage } from "../../../lib/api-client/generated/index.js";
import { Skeleton } from "../../../shared/ui/forge.js";
import { OutcomeDialog } from "./OutcomeDialog.js";
import { StageColumn } from "./StageColumn.js";

// The next stage in DEAL-FORM-1 position order after `currentStageId`, searching across
// ALL stages (open + terminal) — this is what both the Advance button (DEAL-AC-B4) and the
// drag-drop terminal check resolve against. Returns undefined only for the last stage.
export function nextStageId(
  currentStageId: string,
  allStagesByPosition: Stage[],
): string | undefined {
  const ordered = allStagesByPosition.slice().sort((a, b) => a.position - b.position);
  const idx = ordered.findIndex((s) => s.id === currentStageId);
  if (idx === -1 || idx === ordered.length - 1) return undefined;
  return ordered[idx + 1].id;
}

export function PipelineBoard({
  pipelineId,
  stages,
  terminalStages = [],
  deals,
  isLoading,
  isError,
  onRetry,
  onCardClick,
}: {
  pipelineId: string;
  stages: Stage[];
  terminalStages?: Stage[];
  deals: Deal[];
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  onCardClick: (dealId: string) => void;
}) {
  const advance = useAdvanceDeal(pipelineId);
  const [isDragging, setIsDragging] = useState(false);
  // Both the won-stage id and the lost-stage id are resolved once a terminal transition is
  // pending — the OutcomeDialog's OK/Cancel choice (AC-deal-6) decides which one to write,
  // independent of which drop zone (if any) triggered the pending state.
  const [pendingOutcome, setPendingOutcome] = useState<
    { dealId: string; dealName: string; wonStageId?: string; lostStageId?: string } | null
  >(null);
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 8 } }),
  );
  const wonStage = terminalStages.find((s) => s.semantic === "won");
  const lostStage = terminalStages.find((s) => s.semantic === "lost");
  const allStages = [...stages, ...terminalStages];

  // Shared by drag-drop and the Advance button: an open→open move applies directly (🟢); a
  // move where either endpoint is terminal opens the outcome dialog (🟡, either-endpoint rule,
  // DEAL-AC-N-1/DEAL-WIRE-4).
  function attemptAdvance(dealId: string, toStageId: string) {
    const deal = deals.find((d) => d.id === dealId);
    if (!deal || deal.stage_id === toStageId) return;
    const targetIsTerminal = terminalStages.some((s) => s.id === toStageId);
    const fromIsTerminal = stages.find((s) => s.id === deal.stage_id) === undefined;
    if (targetIsTerminal || fromIsTerminal) {
      setPendingOutcome({
        dealId,
        dealName: deal.name,
        wonStageId: wonStage?.id,
        lostStageId: lostStage?.id,
      });
      return;
    }
    advance.mutate({ dealId, toStageId });
  }

  function handleDragStart(_event: DragStartEvent) {
    setIsDragging(true);
  }

  function handleDragEnd(event: DragEndEvent) {
    setIsDragging(false);
    const dealId = String(event.active.id);
    const toStageId = event.over?.id ? String(event.over.id) : undefined;
    if (!toStageId) return;
    attemptAdvance(dealId, toStageId);
  }

  function handleAdvanceClick(dealId: string) {
    const deal = deals.find((d) => d.id === dealId);
    if (!deal) return;
    const target = nextStageId(deal.stage_id, allStages);
    if (!target) return; // already at the last stage — nothing to advance to
    attemptAdvance(dealId, target);
  }

  if (isLoading) {
    return (
      <div data-testid="board-skeleton" className="flex gap-gf-md p-gf-md">
        {[1, 2, 3, 4, 5].map((i) => (
          <Skeleton key={i} height="300px" />
        ))}
      </div>
    );
  }
  if (isError) {
    return (
      <div className="p-gf-md rounded-md border border-gf-status-danger-subtle bg-gf-status-danger-subtle">
        <p className="text-gf-body text-gf-status-danger mb-gf-sm">
          Failed to load the pipeline board.
        </p>
        <button
          type="button"
          onClick={onRetry}
          className="text-gf-caption text-gf-accent underline"
        >
          Retry
        </button>
      </div>
    );
  }
  if (stages.length === 0) {
    return (
      <p className="p-gf-md text-gf-body text-gf-secondary">
        No pipeline configured yet.
      </p>
    );
  }
  return (
    <div className="p-gf-md">
      {deals.length === 0 && (
        // A top-level honest empty state (STATE-1) distinct from a column's own "Drop a card
        // here" hint — empty stage columns still render below (they stay valid drop targets,
        // never collapsed).
        <p
          data-testid="board-empty-state"
          className="mb-gf-md text-gf-body text-gf-secondary"
        >
          No deals yet — drag a card here once you create one, or use "New deal" above.
        </p>
      )}
      <DndContext sensors={sensors} onDragStart={handleDragStart} onDragEnd={handleDragEnd}>
        <div className="flex gap-gf-md overflow-x-auto">
          {stages.map((stage) => (
            <StageColumn
              key={stage.id}
              stage={stage}
              deals={deals.filter((d) => d.stage_id === stage.id)}
              onCardClick={onCardClick}
              onAdvanceClick={handleAdvanceClick}
            />
          ))}
          {isDragging &&
            terminalStages.map((stage) => (
              <StageColumn
                key={stage.id}
                stage={stage}
                deals={[]}
                isTransient
                onCardClick={onCardClick}
                onAdvanceClick={handleAdvanceClick}
              />
            ))}
        </div>
      </DndContext>
      <OutcomeDialog
        open={pendingOutcome !== null}
        dealName={pendingOutcome?.dealName ?? ""}
        isLoading={advance.isPending}
        onWon={() => {
          if (!pendingOutcome?.wonStageId) return;
          advance.mutate({
            dealId: pendingOutcome.dealId,
            toStageId: pendingOutcome.wonStageId,
            status: "won",
          });
          setPendingOutcome(null);
        }}
        onLost={(reason) => {
          if (!pendingOutcome?.lostStageId) return;
          advance.mutate({
            dealId: pendingOutcome.dealId,
            toStageId: pendingOutcome.lostStageId,
            status: "lost",
            lostReason: reason,
          });
          setPendingOutcome(null);
        }}
        onCancel={() => setPendingOutcome(null)}
      />
    </div>
  );
}
