import { Skeleton } from "../../../shared/ui/forge.js";
import { useMembers } from "../../custom-fields/api/members.js";
import type { Quota, QuotaAttainment } from "../api/quotas.js";
import { useTeamRollup } from "../api/quotas.js";

function barColorClass(pct: number): string {
  if (pct >= 100) return "bg-gf-status-success";
  if (pct >= 60) return "bg-gf-accent";
  return "bg-gf-status-danger";
}

export function TeamRollupRail({
  quota,
  currentAttainment,
}: {
  quota: Quota | undefined;
  currentAttainment: QuotaAttainment | undefined;
}) {
  const { reps, isLoading } = useTeamRollup(quota, currentAttainment);
  const { data: membersPage } = useMembers();
  const nameByUserId = new Map(
    (membersPage?.data ?? []).map((member) => [
      member.user_id,
      member.display_name,
    ]),
  );

  if (!quota) return null;
  if (isLoading) {
    return (
      <div data-testid="team-rollup-rail-skeleton">
        <Skeleton height="16px" />
      </div>
    );
  }

  return (
    <div>
      <h4 className="mb-gf-sm text-gf-label font-semibold uppercase tracking-wide text-gf-tertiary">
        Team attainment
      </h4>
      {reps.map((rep) => {
        const pct = rep.attainment
          ? Math.round(rep.attainment.attainment_pct)
          : 0;
        const name =
          (rep.quota.owner_id && nameByUserId.get(rep.quota.owner_id)) ??
          "Unknown rep";
        return (
          <div
            key={rep.quota.id}
            className="flex items-center gap-gf-md border-b border-gf-subtle py-gf-sm last:border-b-0"
          >
            <div className="min-w-0 flex-1">
              <b className="block text-gf-body font-semibold text-gf-primary">
                {name}
              </b>
              <span className="font-mono text-gf-micro text-gf-tertiary">
                {rep.isCurrent
                  ? "this quota"
                  : `${(rep.quota.target_minor / 100).toLocaleString("de-DE")} EUR target`}
              </span>
              <div className="mt-gf-xs h-1.5 w-full overflow-hidden rounded-full bg-gf-card">
                <div
                  className={`h-full rounded-full ${barColorClass(pct)}`}
                  style={{ width: `${Math.min(pct, 100)}%` }}
                />
              </div>
            </div>
            <span className="font-mono text-gf-body font-medium text-gf-primary">
              {pct}%
            </span>
          </div>
        );
      })}
      <p className="mt-gf-sm font-mono text-gf-micro text-gf-tertiary">
        team roll-up = Σ closed-won ÷ Σ targets · auditable
      </p>
    </div>
  );
}
