import { useParams } from "react-router-dom";
import { Skeleton } from "../../../shared/ui/forge.js";
import { useDeal } from "../api/deals.js";
import { formatMoney } from "../components/DealCard.js";

export function DealDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { data: deal, isLoading, isError, refetch } = useDeal(id);

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
  return (
    <div className="p-gf-lg">
      <h1 className="text-gf-title font-semibold text-gf-primary">
        {deal.name}
      </h1>
      <p className="text-gf-body text-gf-secondary">
        {formatMoney(deal.amount_minor, deal.currency)} · {deal.status}
      </p>
    </div>
  );
}
