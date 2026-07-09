import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { Icon, Skeleton } from "../../../shared/ui/forge.js";
import { ToastContainer } from "../../../shared/ui/ToastContainer.js";
import { useDeal, useDealOffers } from "../../deals/api/deals.js";
import { useAuthStore } from "../../identity/store/authStore.js";
import {
  offersKeys,
  useCreateLineItem,
  useDeleteLineItem,
  useOffer,
  useOfferLineItems,
  useUpdateLineItem,
} from "../api/offers.js";
import { ExplainTotalPanel } from "../components/ExplainTotalPanel.js";
import { LineItemEditor } from "../components/LineItemEditor.js";
import { OfferPreviewPanel } from "../components/OfferPreviewPanel.js";
import { RegenerateBanner } from "../components/RegenerateBanner.js";
import { SendCard } from "../components/SendCard.js";
import { StagedLinesPanel } from "../components/StagedLinesPanel.js";

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
  const { role, user } = useAuthStore();
  const [toasts, setToasts] = useState<
    { id: string; variant: "success" | "error"; message: string }[]
  >([]);
  const [stagedLineIds, setStagedLineIds] = useState<Set<string>>(new Set());
  const pushToast = (variant: "success" | "error", message: string) =>
    setToasts((current) => [
      ...current,
      { id: crypto.randomUUID(), variant, message },
    ]);
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
  const { data: deal } = useDeal(id);
  const { data: lineItems = [] } = useOfferLineItems(offerId);
  const createLineItem = useCreateLineItem(offerId);
  const updateLineItem = useUpdateLineItem(offerId);
  const deleteLineItem = useDeleteLineItem(offerId);

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
  const dealName = deal?.name ?? "Deal";
  const committedLines = lineItems.filter(
    (line) => !stagedLineIds.has(line.id),
  );
  const nextLinePosition =
    committedLines.reduce((max, line) => Math.max(max, line.position), 0) + 1;

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
                  {isLocked ? (
                    <span data-testid="locked-revision-icon">
                      <Icon name="Lock" size={12} />
                    </span>
                  ) : null}
                  v{revision.revision}
                </Link>
              </li>
            );
          })}
        </ul>
      </section>

      <RegenerateBanner
        dealId={id ?? ""}
        offer={offer}
        userRole={role}
        onRegenerated={(_, aiLineIds) => {
          setStagedLineIds(new Set(aiLineIds));
        }}
      />

      <LineItemEditor
        lines={committedLines}
        stagedLineIds={stagedLineIds}
        canMutateOffer={canEdit}
        onCreate={() =>
          createLineItem.mutate({
            position: nextLinePosition,
            description: "New line",
            quantity: 1,
            unit_price_minor: 0,
            discount_pct: 0,
            tax_rate: 0,
            source: "ui",
            captured_by: `human:${user?.id ?? "unknown"}`,
          })
        }
        onUpdate={(lineId, patch) => updateLineItem.mutate({ lineId, patch })}
        onDelete={(lineId) => deleteLineItem.mutate({ lineId })}
      />

      <StagedLinesPanel
        lines={lineItems}
        stagedLineIds={stagedLineIds}
        canMutateOffer={canEdit || stagedLineIds.size > 0}
        currentUserId={user?.id ?? ""}
        onAccept={(lineId, patch) =>
          updateLineItem.mutate(
            { lineId, patch },
            {
              onSuccess: () => {
                setStagedLineIds((prev) => {
                  const next = new Set(prev);
                  next.delete(lineId);
                  return next;
                });
              },
            },
          )
        }
        onDismiss={(lineId) =>
          deleteLineItem.mutate(
            { lineId },
            {
              onSuccess: () => {
                setStagedLineIds((prev) => {
                  const next = new Set(prev);
                  next.delete(lineId);
                  return next;
                });
              },
            },
          )
        }
      />

      <ExplainTotalPanel
        currency={offer.currency}
        lines={committedLines}
        stagedLineIds={stagedLineIds}
        grossMinor={offer.gross_minor}
      />

      <OfferPreviewPanel
        dealName={dealName}
        offer={offer}
        lines={committedLines}
        canMutateOffer={canEdit}
      />

      <SendCard
        offer={offer}
        canMutateOffer={canEdit}
        onSent={(next) => {
          qc.setQueryData(offersKeys.detail(offer.id), next);
          navigate(`/deals/${id}/offers/${next.id}`);
        }}
        pushToast={pushToast}
      />

      <ToastContainer
        toasts={toasts}
        onDismiss={(toastId) =>
          setToasts((current) => current.filter((t) => t.id !== toastId))
        }
      />

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
