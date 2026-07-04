import { useState } from "react";
import { Button, ConfirmDialog, Modal, TextInput } from "../../../shared/ui/forge.js";
import { useMergePerson } from "../api/person.js";

// mergePerson's 409 uses the generic Conflict schema, not a dedicated VersionConflict — this
// reads the actual `code` off the thrown problem+json body rather than assuming version_skew is
// reachable (PO-AC-M5 gap, mirrors T21's advanceDeal If-Match gap; flagged in the PR description).
export function mergeErrorMessage(error: unknown): {
  message: string;
  isVersionSkew: boolean;
} {
  if (error && typeof error === "object") {
    const problem = error as { code?: unknown; detail?: unknown };
    const code = typeof problem.code === "string" ? problem.code : undefined;
    const detail = typeof problem.detail === "string" ? problem.detail : undefined;
    if (code === "version_skew") {
      // Canned concurrent-merge copy — the raw `detail` is often a terse technical string; the
      // PO-AC-M5 affordance is the human-readable "lost the race" message.
      return {
        isVersionSkew: true,
        message: "Another merge already landed — this record lost the race.",
      };
    }
    if (detail) return { isVersionSkew: false, message: detail };
    if (code) return { isVersionSkew: false, message: `Merge failed (${code}).` };
  }
  return { isVersionSkew: false, message: "Merge failed — please try again." };
}

export function MergePersonDialog({
  personId,
  open,
  onClose,
}: {
  personId: string;
  open: boolean;
  onClose: () => void;
}) {
  const [targetId, setTargetId] = useState("");
  const [step, setStep] = useState<"input" | "confirm">("input");
  const { mutate, isPending, error, reset } = useMergePerson(personId);

  function handleClose() {
    setStep("input");
    setTargetId("");
    reset();
    onClose();
  }

  if (!open) return null;

  if (step === "input") {
    return (
      <Modal open={open} onClose={handleClose} title="Merge this contact" width="sm">
        <div className="px-gf-xl py-gf-lg flex flex-col gap-gf-md">
          {/* Label wraps the input so it associates with the Forge TextInput (which takes no id). */}
          <label className="flex flex-col gap-gf-xs text-gf-caption text-gf-secondary">
            Target person id
            <TextInput
              value={targetId}
              onChange={setTargetId}
              placeholder="Surviving person's UUID"
            />
          </label>
          <div className="flex justify-end gap-gf-sm">
            <Button variant="secondary" onClick={handleClose}>
              Cancel
            </Button>
            <Button onClick={() => setStep("confirm")} disabled={targetId.trim().length === 0}>
              Continue
            </Button>
          </div>
        </div>
      </Modal>
    );
  }

  const failure = error ? mergeErrorMessage(error) : undefined;

  return (
    <>
      <ConfirmDialog
        open={open}
        onClose={handleClose}
        onConfirm={() => mutate({ targetId }, { onSuccess: handleClose })}
        title="Confirm merge"
        description="The target record survives; this record is archived. On any primary-email/primary-phone/current-employer conflict, the survivor's values win — this record's conflicting primary rows are demoted to non-primary."
        confirmLabel="Confirm"
        isLoading={isPending}
      />
      {failure && (
        <p role="alert" className="text-gf-body text-gf-status-danger px-gf-xl">
          {failure.message}
        </p>
      )}
    </>
  );
}
