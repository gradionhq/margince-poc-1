import { useState } from "react";
import type { Offer } from "../../../lib/api-client/generated/index.js";
import { Button, ConfirmDialog } from "../../../shared/ui/forge.js";
import { useSendOffer } from "../api/offers.js";

function sendErrorMessage(error: { status?: number } | null) {
  if (error?.status === 422) {
    return "FX rate unavailable: the offer cannot be sent until the missing FX rate context is present.";
  }
  if (error?.status === 403) {
    return "Approval required unexpectedly for a human's own click; this path should not need an approval token.";
  }
  return "Unable to send this offer right now.";
}

export function SendCard({
  offer,
  canMutateOffer,
  onSent,
  pushToast,
}: {
  offer: Pick<Offer, "id" | "status" | "offer_number" | "revision">;
  canMutateOffer: boolean;
  onSent: (next: Offer) => void;
  pushToast: (variant: "success" | "error", message: string) => void;
}) {
  const sendOffer = useSendOffer(offer.id);
  const [open, setOpen] = useState(false);
  const [error, setError] = useState<{ status?: number } | null>(null);

  if (!canMutateOffer || offer.status !== "draft") return null;

  const confirmSend = async () => {
    try {
      const next = await sendOffer.mutateAsync();
      setError(null);
      onSent(next);
      pushToast(
        "success",
        "Offer sent — locked. Any further change starts the next revision (regenerate).",
      );
      setOpen(false);
    } catch (err) {
      const nextError = err as { status?: number };
      setError(nextError);
      pushToast("error", sendErrorMessage(nextError));
    }
  };

  return (
    <section
      data-testid="send-card"
      className="rounded-gf-lg border border-gf-subtle bg-gf-card p-gf-lg"
    >
      <div className="flex flex-wrap items-start justify-between gap-gf-sm">
        <div>
          <h2 className="text-gf-body font-medium text-gf-primary">Send</h2>
          <p className="text-gf-caption text-gf-secondary">
            Sending queues this offer to the approval inbox for an automated or
            agent send; your own click here is the approval and sends
            immediately.
          </p>
        </div>
        <Button
          type="button"
          onClick={() => setOpen(true)}
          disabled={sendOffer.isPending}
        >
          Send offer
        </Button>
      </div>

      {error ? (
        <p className="mt-gf-sm text-gf-caption text-gf-status-danger">
          {sendErrorMessage(error)}
        </p>
      ) : null}

      <ConfirmDialog
        open={open}
        onClose={() => setOpen(false)}
        onConfirm={confirmSend}
        title="Send this offer?"
        description="Your click is the approval for this human-operated builder."
        confirmLabel="Confirm send"
        isLoading={sendOffer.isPending}
      />
    </section>
  );
}
