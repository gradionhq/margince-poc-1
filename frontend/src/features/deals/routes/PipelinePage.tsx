import { useNavigate } from "react-router-dom";
import { useDeals, useDefaultPipeline, usePipelineRollup, useStages } from "../api/deals.js";
import { PipelineBoard } from "../components/PipelineBoard.js";
import { TotalsStrip } from "../components/TotalsStrip.js";

export function PipelinePage() {
  const navigate = useNavigate();
  const { data: pipeline } = useDefaultPipeline();
  const pipelineId = pipeline?.id;
  const { data: allStages } = useStages(pipelineId);
  const openStages = (allStages ?? []).filter((s) => s.semantic === "open");
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
      </header>
      <main className="p-gf-lg">
        <TotalsStrip rollup={rollup} isLoading={rollupLoading} isError={rollupError} />
        <PipelineBoard
          pipelineId={pipelineId ?? ""}
          stages={openStages}
          deals={dealPage?.data ?? []}
          isLoading={dealsLoading}
          isError={dealsError}
          onRetry={refetchDeals}
          onCardClick={(dealId) => navigate(`/deals/${dealId}`)}
        />
      </main>
    </div>
  );
}
