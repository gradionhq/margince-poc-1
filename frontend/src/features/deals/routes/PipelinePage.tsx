import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Button, RadioGroup } from "../../../shared/ui/forge.js";
import { ToastContainer } from "../../../shared/ui/ToastContainer.js";
import {
  useDeals,
  useDefaultPipeline,
  usePipelineRollup,
  useStages,
} from "../api/deals.js";
import { DealsTable } from "../components/DealsTable.js";
import { NewDealModal } from "../components/NewDealModal.js";
import { PipelineBoard } from "../components/PipelineBoard.js";
import { TotalsStrip } from "../components/TotalsStrip.js";

export function PipelinePage() {
  const navigate = useNavigate();
  const [view, setView] = useState<"board" | "table">("board");
  const [newDealOpen, setNewDealOpen] = useState(false);
  const [toasts, setToasts] = useState<
    Array<{ id: string; variant: "success" | "error"; message: string }>
  >([]);
  const { data: pipeline } = useDefaultPipeline();
  const pipelineId = pipeline?.id;
  const { data: allStages } = useStages(pipelineId);
  const openStages = (allStages ?? []).filter((s) => s.semantic === "open");
  const stagesById = Object.fromEntries(
    (allStages ?? []).map((s) => [s.id, s]),
  );
  const {
    data: dealPage,
    isLoading: dealsLoading,
    isError: dealsError,
    refetch: refetchDeals,
  } = useDeals({ pipelineId, status: "open" });
  const {
    data: rollup,
    isLoading: rollupLoading,
    isError: rollupError,
  } = usePipelineRollup(pipelineId);

  return (
    <div className="min-h-screen bg-gf-page">
      <header className="flex items-center justify-between px-gf-lg py-gf-md border-b border-gf-subtle bg-gf-card">
        <h2 className="text-gf-title font-semibold text-gf-primary">Deals</h2>
        <div className="flex items-center gap-gf-md">
          <RadioGroup
            label="View"
            name="pipeline-view"
            value={view}
            onChange={(v) => setView(v as "board" | "table")}
            options={[
              { value: "board", label: "Board" },
              { value: "table", label: "Table" },
            ]}
          />
          <Button variant="primary" onClick={() => setNewDealOpen(true)}>
            New deal
          </Button>
        </div>
      </header>
      <main className="p-gf-lg">
        <TotalsStrip
          rollup={rollup}
          isLoading={rollupLoading}
          isError={rollupError}
        />
        {view === "board" ? (
          <PipelineBoard
            pipelineId={pipelineId ?? ""}
            stages={openStages}
            terminalStages={(allStages ?? []).filter(
              (s) => s.semantic !== "open",
            )}
            deals={dealPage?.data ?? []}
            isLoading={dealsLoading}
            isError={dealsError}
            onRetry={refetchDeals}
            onCardClick={(dealId) => navigate(`/deals/${dealId}`)}
            onMoveError={(message) =>
              setToasts((t) => [
                ...t,
                { id: crypto.randomUUID(), variant: "error", message },
              ])
            }
          />
        ) : (
          <DealsTable deals={dealPage?.data ?? []} stagesById={stagesById} />
        )}
      </main>
      {newDealOpen && pipeline?.id && openStages[0] && (
        <NewDealModal
          open={newDealOpen}
          onClose={() => setNewDealOpen(false)}
          // No organizationId prop — the board is a generic entry point with no company already
          // in context, so NewDealModal renders its own org picker first. A future
          // company/contact-page launcher can pass organizationId directly to skip the picker.
          defaultPipelineId={pipeline.id}
          defaultStageId={openStages[0].id}
          onCreated={() => {
            setNewDealOpen(false);
            setToasts((t) => [
              ...t,
              {
                id: crypto.randomUUID(),
                variant: "success",
                message:
                  "Deal created from context · org + people + recent activity pre-attached",
              },
            ]);
          }}
        />
      )}
      <ToastContainer
        toasts={toasts}
        onDismiss={(id) => setToasts((t) => t.filter((x) => x.id !== id))}
      />
    </div>
  );
}
