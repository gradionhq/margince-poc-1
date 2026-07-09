import { useEffect, useMemo, useState } from "react";
import { useParams } from "react-router-dom";
import { ToastContainer } from "../../../shared/ui/ToastContainer.js";
import { useMembers } from "../../custom-fields/api/members.js";
import {
  QuotaAttainmentForbiddenError,
  QuotaAttainmentTargetZeroError,
  QuotaForbiddenError,
  useQuota,
  useQuotaAttainment,
} from "../api/quotas.js";
import { AttainmentRing } from "../components/AttainmentRing.js";
import { ContributingDealsTable } from "../components/ContributingDealsTable.js";
import { PeriodBar } from "../components/PeriodBar.js";
import { QuotaExplainBox } from "../components/QuotaExplainBox.js";
import { TargetEditor } from "../components/TargetEditor.js";
import { TeamRollupRail } from "../components/TeamRollupRail.js";

type Toast = { id: string; variant: "success" | "error"; message: string };

function quotaLabel(
  quotaOwnerId: string | null | undefined,
  members: Map<string, string>,
) {
  return (quotaOwnerId && members.get(quotaOwnerId)) ?? "Unknown quota";
}

export function QuotaPage() {
  const { id } = useParams<{ id: string }>();
  const [toasts, setToasts] = useState<Toast[]>([]);
  const [toastSeq, setToastSeq] = useState(0);
  const [lastGoodComputeAt, setLastGoodComputeAt] = useState<
    string | undefined
  >();

  const {
    data: quota,
    isLoading: quotaLoading,
    isError: quotaIsError,
    error: quotaError,
  } = useQuota(id);
  const {
    data: attainment,
    isLoading: attainmentLoading,
    isError: attainmentIsError,
    error: attainmentError,
  } = useQuotaAttainment(id);
  const { data: membersPage } = useMembers();

  // F1: "honest failure card with cause + last successful compute time" — tracked as a plain
  // timestamp, never the stale attainment object itself (the ring never renders attainment data
  // on an error branch).
  useEffect(() => {
    if (attainment?.as_of_date) {
      setLastGoodComputeAt(attainment.as_of_date);
    }
  }, [attainment?.as_of_date]);

  const memberNameById = useMemo(
    () =>
      new Map(
        (membersPage?.data ?? []).map((member) => [
          member.user_id,
          member.display_name,
        ]),
      ),
    [membersPage],
  );

  function pushToast(variant: "success" | "error", message: string) {
    setToasts((current) => [
      ...current,
      { id: String(toastSeq), variant, message },
    ]);
    setToastSeq((current) => current + 1);
  }

  function dismissToast(toastId: string) {
    setToasts((current) => current.filter((toast) => toast.id !== toastId));
  }

  if (!id) {
    return (
      <div className="min-h-screen bg-gf-page p-gf-lg">
        <p className="text-gf-body text-gf-secondary">Quota not found.</p>
      </div>
    );
  }

  const quotaForbidden = quotaError instanceof QuotaForbiddenError;
  const attainmentForbidden =
    attainmentError instanceof QuotaAttainmentForbiddenError;
  const attainmentTargetZero =
    attainmentError instanceof QuotaAttainmentTargetZeroError;

  if (!quotaLoading && quotaForbidden) {
    return (
      <div className="min-h-screen bg-gf-page p-gf-lg">
        <p className="text-gf-body text-gf-status-danger">
          You don't have access to this quota.
        </p>
      </div>
    );
  }

  if (!quotaLoading && quotaIsError && !quota) {
    return (
      <div className="min-h-screen bg-gf-page p-gf-lg">
        <p className="text-gf-body text-gf-secondary">Quota not found.</p>
      </div>
    );
  }

  const isLoading = quotaLoading || attainmentLoading;

  return (
    <div className="min-h-screen bg-gf-page">
      <header className="border-b border-gf-subtle bg-gf-card px-gf-lg py-gf-md">
        <div className="flex flex-wrap items-center justify-between gap-gf-sm">
          <div>
            <h2 className="text-gf-title font-semibold text-gf-primary">
              Quota &amp; Attainment
            </h2>
            {quota && (
              <p className="text-gf-caption text-gf-secondary">
                {quotaLabel(quota.owner_id, memberNameById)} · {quota.currency}{" "}
                · {quota.period_start} to {quota.period_end}
              </p>
            )}
          </div>
        </div>
      </header>

      <main className="grid gap-gf-lg p-gf-lg lg:grid-cols-[minmax(0,1fr)_320px]">
        <section className="rounded-lg border border-gf-subtle bg-gf-card">
          <AttainmentRing
            attainment={attainment}
            isLoading={isLoading}
            isError={attainmentIsError}
            isForbidden={attainmentForbidden}
            isTargetZero={attainmentTargetZero}
          />
          {!isLoading && attainment && (
            <QuotaExplainBox attainment={attainment} />
          )}
          {attainmentIsError &&
            !attainmentForbidden &&
            !attainmentTargetZero && (
              <p className="px-gf-lg pb-gf-lg text-gf-caption text-gf-tertiary">
                {lastGoodComputeAt
                  ? `Last successful compute: ${lastGoodComputeAt}.`
                  : "No successful compute yet."}
              </p>
            )}
          {quota && (
            <div className="border-t border-gf-subtle px-gf-lg pb-gf-lg">
              <PeriodBar
                quota={quota}
                onToast={(message) => pushToast("error", message)}
              />
              <div className="mt-gf-lg grid gap-gf-lg md:grid-cols-[minmax(0,1fr)_280px]">
                <div className="min-w-0">
                  {attainment && (
                    <ContributingDealsTable attainment={attainment} />
                  )}
                </div>
                <div className="rounded-lg border border-gf-subtle bg-gf-elevated p-gf-md">
                  <TargetEditor
                    quota={quota}
                    onToast={(variant, message) => pushToast(variant, message)}
                  />
                </div>
              </div>
            </div>
          )}
        </section>

        <aside className="rounded-lg border border-gf-subtle bg-gf-card p-gf-md">
          <TeamRollupRail quota={quota} currentAttainment={attainment} />
        </aside>
      </main>

      <ToastContainer toasts={toasts} onDismiss={dismissToast} />
    </div>
  );
}
