import type { AuditHistoryEntry } from "../../../lib/api-client/generated/index.js";
import { Skeleton } from "../../../shared/ui/forge.js";

export function StageHistoryCard({
  entries,
  isLoading,
  isError,
}: {
  entries: AuditHistoryEntry[];
  isLoading: boolean;
  isError: boolean;
}) {
  return (
    <div
      data-testid="stage-history-card"
      className="rounded-lg border border-gf-subtle bg-gf-card p-gf-md"
    >
      <h3 className="text-gf-body font-semibold text-gf-primary mb-gf-sm">
        Stage history
      </h3>
      {isLoading ? (
        <div data-testid="stage-history-skeleton">
          <Skeleton height="60px" />
        </div>
      ) : isError ? (
        <p className="text-gf-body text-gf-status-danger">
          Failed to load stage history.
        </p>
      ) : entries.length === 0 ? (
        <p className="text-gf-body text-gf-secondary">No stage history yet.</p>
      ) : (
        <ul>
          {entries.map((e) => (
            <li
              key={e.id}
              data-testid={`history-row-${e.id}`}
              className="py-gf-xs border-b border-gf-subtle last:border-b-0"
            >
              <p className="text-gf-body text-gf-primary">{e.summary}</p>
              <p className="text-gf-caption text-gf-secondary">
                {new Date(e.occurred_at).toLocaleDateString()}
              </p>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
