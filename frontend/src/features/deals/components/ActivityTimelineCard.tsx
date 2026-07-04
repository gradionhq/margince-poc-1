import type { Activity } from "../../../lib/api-client/generated/index.js";
import { Skeleton } from "../../../shared/ui/forge.js";

export function ActivityTimelineCard({
  activities,
  isLoading,
  isError,
}: {
  activities: Activity[];
  isLoading: boolean;
  isError: boolean;
}) {
  return (
    <div
      data-testid="activity-timeline-card"
      className="rounded-lg border border-gf-subtle bg-gf-card p-gf-md"
    >
      <h3 className="text-gf-body font-semibold text-gf-primary mb-gf-sm">
        Timeline
      </h3>
      {isLoading ? (
        <div data-testid="activity-timeline-skeleton">
          <Skeleton height="80px" />
        </div>
      ) : isError ? (
        <p className="text-gf-body text-gf-status-danger">
          Failed to load activity.
        </p>
      ) : activities.length === 0 ? (
        <p className="text-gf-body text-gf-secondary">
          You logged none of this
        </p>
      ) : (
        <ul>
          {activities.map((a) => (
            <li
              key={a.id}
              data-testid={`timeline-row-${a.id}`}
              className="py-gf-xs border-b border-gf-subtle last:border-b-0"
            >
              <p className="text-gf-body text-gf-primary">
                {a.subject ?? a.kind}
              </p>
              <p className="text-gf-caption text-gf-secondary">
                {new Date(a.occurred_at).toLocaleString()}
                {a.source_system ? ` · via ${a.source_system}` : ""}
              </p>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
