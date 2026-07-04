import { Skeleton } from "../../../shared/ui/forge.js";
import { usePersonDeals } from "../api/person.js";

function formatAmount(
  minor: number | null | undefined,
  currency: string | null | undefined,
): string {
  if (minor === null || minor === undefined || !currency) return "—";
  return new Intl.NumberFormat("en-US", { style: "currency", currency }).format(
    minor / 100,
  );
}

export function PersonDealsTab({ personId }: { personId: string }) {
  const { data: deals, isLoading, isError } = usePersonDeals(personId);

  if (isLoading) {
    return (
      <div
        data-testid="person-deals-loading"
        className="flex flex-col gap-gf-sm"
      >
        <Skeleton className="h-8 w-full" />
        <Skeleton className="h-8 w-full" />
      </div>
    );
  }

  if (isError) {
    return (
      <p className="text-gf-body text-gf-status-danger">
        Failed to load deals for this person.
      </p>
    );
  }

  if (!deals || deals.length === 0) {
    return (
      <p className="text-gf-body text-gf-secondary">
        No deals for this person yet.
      </p>
    );
  }

  return (
    <div className="flex flex-col gap-gf-sm">
      {deals.map((d) => (
        <div
          key={d.id}
          className="flex items-center justify-between border border-gf-subtle rounded-md p-gf-sm"
        >
          <span className="text-gf-body font-medium text-gf-primary">
            {d.name}
          </span>
          <span className="text-gf-caption text-gf-secondary">{d.status}</span>
          <span className="text-gf-body text-gf-primary">
            {formatAmount(d.amount_minor, d.currency)}
          </span>
        </div>
      ))}
    </div>
  );
}
