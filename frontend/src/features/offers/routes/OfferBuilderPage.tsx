import { useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate, useParams } from "react-router-dom";
import { Skeleton } from "../../../shared/ui/forge.js";
import { useAuthStore } from "../../identity/store/authStore.js";
import { useDealOffers, useOffer } from "../api/offers.js";

export function canMutateOffer(
  role: string | null | undefined,
  offer: { status?: string | null } | null | undefined,
) {
  return (
    (role === "admin" || role === "rep" || role === "manager") &&
    offer?.status === "draft"
  );
}

function OfferBuilderSkeleton() {
  return (
    <div data-testid="offer-builder-skeleton" className="p-gf-lg space-y-gf-lg">
      <Skeleton height="96px" />
      <Skeleton height="64px" />
      <Skeleton height="280px" />
    </div>
  );
}

export function OfferBuilderPage() {
  const { id, offerId } = useParams<{ id: string; offerId: string }>();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const { role } = useAuthStore();
  const {
    data: offer,
    isLoading,
    isError,
    error: offerError,
    refetch,
  } = useOffer(offerId);
  const {
    data: dealOffersResponse,
    isLoading: dealOffersLoading,
    isError: dealOffersError,
    refetch: refetchDealOffers,
  } = useDealOffers(id);

  const dealOffers = dealOffersResponse?.data ?? [];
  const currentChain = offer
    ? dealOffers
        .filter((row) => row.offer_number === offer.offer_number)
        .slice()
        .sort((a, b) => a.revision - b.revision)
    : [];
  const lockedOffer = offer && offer.status !== "draft";
  const canEdit = canMutateOffer(role, offer);
  const offerStatus = (offerError as { status?: number } | undefined)?.status;
  const hasPermissionError = offerStatus === 401 || offerStatus === 403;

  if (isLoading || dealOffersLoading) {
    return <OfferBuilderSkeleton />;
  }

  if (isError || dealOffersError) {
    return (
      <div className="p-gf-lg">
        {hasPermissionError ? (
          <div
            data-testid="offer-builder-permission-card"
            className="rounded-gf-lg border border-gf-subtle bg-gf-card p-gf-lg"
          >
            <h1 className="text-gf-title font-semibold text-gf-primary">
              You don't have permission to view this offer
            </h1>
            <p className="mt-gf-sm text-gf-body text-gf-secondary">
              Ask an admin or a rep to grant offer access.
            </p>
          </div>
        ) : (
          <div
            data-testid="offer-builder-error-card"
            className="rounded-gf-lg border border-gf-subtle bg-gf-card p-gf-lg"
          >
            <h1 className="text-gf-title font-semibold text-gf-primary">
              Failed to load this offer.
            </h1>
            <button
              type="button"
            onClick={() => {
              refetch();
              refetchDealOffers();
              qc.invalidateQueries({ queryKey: ["offers"] });
            }}
              className="mt-gf-sm text-gf-accent underline"
            >
              Try again
            </button>
          </div>
        )}
      </div>
    );
  }

  if (!offer) {
    return null;
  }

  return (
    <div className="p-gf-lg space-y-gf-lg">
      <header className="rounded-gf-lg border border-gf-subtle bg-gf-card p-gf-lg">
        <div className="flex items-center justify-between gap-gf-md">
          <div>
            <h1 className="text-gf-title font-semibold text-gf-primary">
              {offer.offer_number} v{offer.revision}
            </h1>
            <Link
              to={`/deals/${offer.deal_id}`}
              className="text-gf-caption text-gf-accent underline"
            >
              Back to deal
            </Link>
          </div>
          <span className="rounded-full border border-gf-subtle px-gf-sm py-gf-xs text-gf-caption">
            {offer.status}
          </span>
        </div>
      </header>

      <section
        data-testid="offer-versions-bar"
        className="rounded-gf-lg border border-gf-subtle bg-gf-card p-gf-lg"
      >
        <h2 className="text-gf-body font-medium text-gf-primary">Versions</h2>
        <ul className="mt-gf-sm flex flex-wrap gap-gf-sm">
          {currentChain.map((revision) => {
            const isCurrent = revision.id === offer.id;
            const isLocked = revision.status !== "draft";
            return (
              <li key={revision.id}>
                <Link
                  to={`/deals/${offer.deal_id}/offers/${revision.id}`}
                  className={`inline-flex items-center gap-gf-xs rounded-full border px-gf-sm py-gf-xs text-gf-caption ${
                    isCurrent
                      ? "border-gf-accent text-gf-accent"
                      : "border-gf-subtle text-gf-secondary"
                  }`}
                >
                  {isLocked ? "🔒" : null}
                  v{revision.revision}
                </Link>
              </li>
            );
          })}
        </ul>
      </section>

      <section data-testid="offer-builder-shell">
        <p className="text-gf-body text-gf-secondary">
          {lockedOffer
            ? "This revision is locked."
            : canEdit
              ? "This draft can be edited."
              : "This draft is view-only for your role."}
        </p>
      </section>
    </div>
  );
}
