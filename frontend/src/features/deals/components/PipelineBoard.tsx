import {
  DndContext,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core";
import { useAdvanceDeal } from "../api/deals.js";
import type { Deal, Stage } from "../../../lib/api-client/generated/index.js";
import { Skeleton } from "../../../shared/ui/forge.js";
import { StageColumn } from "./StageColumn.js";

export function PipelineBoard({
  pipelineId,
  stages,
  deals,
  isLoading,
  isError,
  onRetry,
  onCardClick,
}: {
  pipelineId: string;
  stages: Stage[];
  deals: Deal[];
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  onCardClick: (dealId: string) => void;
}) {
  const advance = useAdvanceDeal(pipelineId);
  const sensors = useSensors(
    useSensor(PointerSensor, {
      // Activation distance separates a click from a drag-tail (AC-pipeline-4): pointer must
      // move >8px before dnd-kit starts a drag; a plain click never fires onDragEnd, so
      // DealCard's own onClick handles it.
      activationConstraint: { distance: 8 },
    }),
  );

  function handleDragEnd(event: DragEndEvent) {
    const dealId = String(event.active.id);
    const toStageId = event.over?.id ? String(event.over.id) : undefined;
    if (!toStageId) return;
    const deal = deals.find((d) => d.id === dealId);
    if (!deal || deal.stage_id === toStageId) return;
    advance.mutate({ dealId, toStageId });
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
      <DndContext sensors={sensors} onDragEnd={handleDragEnd}>
        <div className="flex gap-gf-md overflow-x-auto">
          {stages.map((stage) => (
            <StageColumn
              key={stage.id}
              stage={stage}
              deals={deals.filter((d) => d.stage_id === stage.id)}
              onCardClick={onCardClick}
            />
          ))}
        </div>
      </DndContext>
    </div>
  );
}
