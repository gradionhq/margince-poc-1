import { useState } from "react";
import { Button, ConfirmDialog, Modal } from "../../../shared/ui/forge.js";

export function OutcomeDialog({
  open,
  dealName,
  onWon,
  onLost,
  onCancel,
  isLoading = false,
}: {
  open: boolean;
  dealName: string;
  onWon: () => void;
  onLost: (reason: string) => void;
  onCancel: () => void;
  isLoading?: boolean;
}) {
  const [stage, setStage] = useState<"confirm" | "reason">("confirm");
  const [reason, setReason] = useState("");

  function reset() {
    setStage("confirm");
    setReason("");
  }

  if (stage === "reason") {
    return (
      <Modal
        open={open}
        onClose={() => {
          reset();
          onCancel();
        }}
        title="Mark as Closed Lost?"
        width="sm"
        footer={
          <>
            <Button
              variant="secondary"
              onClick={() => {
                reset();
                onCancel();
              }}
            >
              Keep open
            </Button>
            <Button
              variant="primary"
              loading={isLoading}
              onClick={() => {
                const trimmed = reason.trim();
                if (trimmed === "") {
                  // Blank submit — deal stays open, nothing changes (AC-deal-6).
                  reset();
                  onCancel();
                  return;
                }
                onLost(trimmed);
                reset();
              }}
            >
              Confirm Lost
            </Button>
          </>
        }
      >
        <div className="px-gf-xl py-gf-lg">
          <p className="text-gf-body text-gf-secondary mb-gf-md">
            Optional — why was {dealName} lost?
          </p>
          <input
            type="text"
            value={reason}
            onChange={(e) => setReason(e.target.value)}
            placeholder="Lost reason (optional)"
            className="h-10 w-full rounded-md bg-gf-elevated border border-gf-subtle text-gf-body text-gf-primary px-gf-md"
          />
        </div>
      </Modal>
    );
  }

  return (
    <ConfirmDialog
      open={open}
      onClose={() => setStage("reason")}
      onConfirm={() => {
        onWon();
        reset();
      }}
      title="Confirm the outcome"
      description={`${dealName} is moving to a terminal stage — a high-value transition. Confirm records Closed Won.`}
      confirmLabel="Won"
      isLoading={isLoading}
    />
  );
}
