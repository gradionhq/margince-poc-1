import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import type { Stage } from "../../../lib/api-client/generated/index.js";
import { ArchiveConfirmDialog } from "../../../shared/ui/ArchiveConfirmDialog.js";
import {
  ArchivedBanner,
  restoreErrorMessage,
} from "../../../shared/ui/ArchivedBanner.js";
import { Button, Skeleton } from "../../../shared/ui/forge.js";
import { ToastContainer } from "../../../shared/ui/ToastContainer.js";
import {
  dealsKeys,
  useAdvanceDeal,
  useArchiveDeal,
  useDeal,
  useDealActivities,
  useDealHistory,
  useRestoreDeal,
  useStages,
} from "../api/deals.js";
import { ActivityTimelineCard } from "../components/ActivityTimelineCard.js";
import { OutcomeDialog } from "../components/OutcomeDialog.js";
import {
  advanceErrorMessage,
  nextStageId,
} from "../components/PipelineBoard.js";
import {
  firstOpenStageId,
  ReopenConfirmDialog,
} from "../components/ReopenConfirmDialog.js";
import { StageHistoryCard } from "../components/StageHistoryCard.js";
import { StageStepper } from "../components/StageStepper.js";
import { StakeholdersRail } from "../components/StakeholdersRail.js";
import { TasksCard } from "../components/TasksCard.js";
import { WeightedValueExplainer } from "../components/WeightedValueExplainer.js";
import { AttachmentsPanel } from "../../attachments/index.js";

type Toast = { id: string; variant: "success" | "error"; message: string };

const TIMELINE_KINDS = new Set(["email", "call", "meeting"]);

export function DealDetailPage() {
  const { id } = useParams<{ id: string }>();
  const qc = useQueryClient();
  const { data: deal, isLoading, isError, refetch } = useDeal(id);
  const { data: allStages, isLoading: stagesLoading } = useStages(
    deal?.pipeline_id,
  );
  const {
    data: activities,
    isLoading: activitiesLoading,
    isError: activitiesError,
  } = useDealActivities(id);
  const {
    data: history,
    isLoading: historyLoading,
    isError: historyError,
  } = useDealHistory(id);
  const advance = useAdvanceDeal(deal?.pipeline_id);
  const archive = useArchiveDeal(id ?? "");
  const restore = useRestoreDeal(id ?? "");
  const [outcomeOpen, setOutcomeOpen] = useState(false);
  const [reopenOpen, setReopenOpen] = useState(false);
  const [archiveOpen, setArchiveOpen] = useState(false);
  const [toasts, setToasts] = useState<Toast[]>([]);

  function pushToast(variant: Toast["variant"], message: string) {
    setToasts((t) => [...t, { id: crypto.randomUUID(), variant, message }]);
  }

  // useAdvanceDeal's own onSettled (Task 1, unchanged from T21) only invalidates the pipeline
  // list/rollup queries it was built for — it never touches this screen's detail/history reads.
  // Every successful advance/close/reopen on THIS screen must explicitly refresh both, or the
  // stepper, KPI, Advance/Reopen buttons, and stage-history card go stale after their own mutation.
  function refreshDetailAndHistory() {
    refetch();
    qc.invalidateQueries({ queryKey: dealsKeys.history(id) });
  }

  if (isLoading) {
    return (
      <div data-testid="deal-detail-skeleton" className="p-gf-lg">
        <Skeleton height="120px" />
      </div>
    );
  }
  if (isError || !deal) {
    return (
      <div className="p-gf-lg">
        <p className="text-gf-body text-gf-status-danger mb-gf-sm">
          Failed to load this deal.
        </p>
        <button
          type="button"
          onClick={() => refetch()}
          className="text-gf-accent underline"
        >
          Retry
        </button>
      </div>
    );
  }

  const stages: Stage[] = allStages ?? [];
  const target = nextStageId(deal.stage_id, stages);
  const targetIsTerminal =
    stages.find((s) => s.id === target)?.semantic !== "open";
  const currentStage = stages.find((s) => s.id === deal.stage_id);

  function handleAdvanceClick() {
    if (!target) return;
    if (targetIsTerminal) {
      setOutcomeOpen(true);
      return;
    }
    const currentDeal = deal;
    if (!currentDeal) return;
    advance.mutate(
      { dealId: currentDeal.id, toStageId: target },
      {
        onSuccess: () => {
          pushToast("success", "Deal advanced");
          refreshDetailAndHistory();
        },
        onError: (err) => pushToast("error", advanceErrorMessage(err)),
      },
    );
  }

  const wonStage = stages.find((s) => s.semantic === "won");
  const lostStage = stages.find((s) => s.semantic === "lost");
  const reopenTarget = firstOpenStageId(stages);

  const timelineActivities = (activities ?? []).filter((a) =>
    TIMELINE_KINDS.has(a.kind),
  );
  const taskActivities = (activities ?? []).filter((a) => a.kind === "task");
  const historyEntries = (history ?? []).filter(
    (h) => h.action === "create" || h.action === "advance_stage",
  );

  return (
    <div className="p-gf-lg grid grid-cols-1 lg:grid-cols-3 gap-gf-lg">
      <div className="lg:col-span-2 flex flex-col gap-gf-lg">
        <header className="rounded-lg border border-gf-subtle bg-gf-card p-gf-lg">
          {deal.archived_at && (
            <ArchivedBanner
              entityLabel="deal"
              isRestoring={restore.isPending}
              onRestore={() =>
                restore.mutate(undefined, {
                  onSuccess: () => {
                    pushToast("success", "Deal restored");
                  },
                  onError: (err) => {
                    const parsed = restoreErrorMessage(err);
                    pushToast("error", parsed.message);
                  },
                })
              }
            />
          )}
          <h1 className="text-gf-title font-semibold text-gf-primary">
            {deal.name}
          </h1>
          {deal.organization_id && (
            <Link
              to={`/companies/${deal.organization_id}`}
              className="text-gf-caption text-gf-accent underline"
            >
              View company
            </Link>
          )}
          {stagesLoading ? (
            <Skeleton height="32px" />
          ) : (
            <div className="mt-gf-sm">
              <StageStepper
                stages={stages}
                currentStageId={deal.stage_id}
                dealStatus={deal.status}
              />
            </div>
          )}
          <div className="mt-gf-md">
            <WeightedValueExplainer
              amountMinor={deal.amount_minor}
              currency={deal.currency}
              winProbability={currentStage?.win_probability ?? 0}
              stageName={currentStage?.name ?? "—"}
            />
          </div>
          <div className="mt-gf-md flex gap-gf-sm">
            {!deal.archived_at && (
              <Button
                variant="secondary"
                size="sm"
                onClick={() => setArchiveOpen(true)}
              >
                Archive…
              </Button>
            )}
            {deal.status === "open" && target && (
              <Button variant="primary" onClick={handleAdvanceClick}>
                Advance
              </Button>
            )}
            {deal.status !== "open" && (
              <Button variant="secondary" onClick={() => setReopenOpen(true)}>
                Reopen
              </Button>
            )}
          </div>
        </header>

        <ActivityTimelineCard
          activities={timelineActivities}
          isLoading={activitiesLoading}
          isError={activitiesError}
        />
        <StageHistoryCard
          entries={historyEntries}
          isLoading={historyLoading}
          isError={historyError}
        />
        <TasksCard
          tasks={taskActivities}
          dealId={deal.id}
          isLoading={activitiesLoading}
          isError={activitiesError}
          onTaskDone={() => pushToast("success", "Task completed")}
        />
        <AttachmentsPanel entityType="deal" entityId={deal.id} dealId={deal.id} />
      </div>

      <div>
        <StakeholdersRail
          stakeholders={deal.stakeholders}
          stakeholderCount={deal.stakeholder_count}
        />
      </div>

      <OutcomeDialog
        open={outcomeOpen}
        dealName={deal.name}
        isLoading={advance.isPending}
        onWon={() => {
          if (!wonStage) return;
          advance.mutate(
            { dealId: deal.id, toStageId: wonStage.id, status: "won" },
            {
              onSuccess: () => {
                pushToast("success", "Deal closed — weighted 100%");
                refreshDetailAndHistory();
              },
              onError: (err) => pushToast("error", advanceErrorMessage(err)),
            },
          );
          setOutcomeOpen(false);
        }}
        onLost={(reason) => {
          if (!lostStage) return;
          advance.mutate(
            {
              dealId: deal.id,
              toStageId: lostStage.id,
              status: "lost",
              lostReason: reason,
            },
            {
              onSuccess: () => {
                pushToast("success", "Deal closed — weighted 0%");
                refreshDetailAndHistory();
              },
              onError: (err) => pushToast("error", advanceErrorMessage(err)),
            },
          );
          setOutcomeOpen(false);
        }}
        onCancel={() => setOutcomeOpen(false)}
      />

      <ReopenConfirmDialog
        open={reopenOpen}
        dealName={deal.name}
        isLoading={advance.isPending}
        onConfirm={() => {
          if (!reopenTarget) return;
          advance.mutate(
            { dealId: deal.id, toStageId: reopenTarget, status: "open" },
            {
              onSuccess: () => {
                pushToast("success", "Deal reopened");
                refreshDetailAndHistory();
              },
              onError: (err) => pushToast("error", advanceErrorMessage(err)),
            },
          );
          setReopenOpen(false);
        }}
        onCancel={() => setReopenOpen(false)}
      />

      <ArchiveConfirmDialog
        open={archiveOpen}
        entityLabel={deal.name}
        isLoading={archive.isPending}
        onConfirm={() =>
          archive.mutate(undefined, {
            onSuccess: () => {
              pushToast("success", `${deal.name} archived`);
              setArchiveOpen(false);
            },
            onError: () => {
              pushToast("error", "Failed to archive — please try again.");
              setArchiveOpen(false);
            },
          })
        }
        onCancel={() => setArchiveOpen(false)}
      />

      <ToastContainer
        toasts={toasts}
        onDismiss={(tid) => setToasts((t) => t.filter((x) => x.id !== tid))}
      />
    </div>
  );
}
