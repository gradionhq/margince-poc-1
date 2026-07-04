import { useState } from "react";
import { Button, Modal } from "../../../shared/ui/forge.js";
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
    const detail =
      typeof problem.detail === "string" ? problem.detail : undefined;
    if (code === "version_skew") {
      // Canned concurrent-merge copy — the raw `detail` is often a terse technical string; the
      // PO-AC-M5 affordance is the human-readable "lost the race" message.
      return {
        isVersionSkew: true,
        message: "Another merge already landed — this record lost the race.",
      };
    }
    if (detail) return { isVersionSkew: false, message: detail };
    if (code)
      return { isVersionSkew: false, message: `Merge failed (${code}).` };
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
      <Modal
        open={open}
        onClose={handleClose}
        title="Merge this contact"
        width="sm"
      >
        <div className="px-gf-xl py-gf-lg flex flex-col gap-gf-md">
          {/* Native input with htmlFor/id — the Forge TextInput takes no id, so a real
              label→control association (needed for a11y + getByLabelText) uses a raw input,
              mirroring NotesTab's labeled textarea. */}
          <label
            htmlFor="merge-target-id"
            className="flex flex-col gap-gf-xs text-gf-caption text-gf-secondary"
          >
            Target person id
            <input
              id="merge-target-id"
              value={targetId}
              onChange={(e) => setTargetId(e.target.value)}
              placeholder="Surviving person's UUID"
              className="h-10 w-full rounded-md bg-gf-elevated border border-gf-subtle text-gf-body text-gf-primary placeholder:text-gf-muted px-gf-md focus:border-gf-accent focus:ring-1 focus:ring-gf-accent focus:outline-none"
            />
          </label>
          <div className="flex justify-end gap-gf-sm">
            <Button variant="secondary" onClick={handleClose}>
              Cancel
            </Button>
            <Button
              onClick={() => setStep("confirm")}
              disabled={targetId.trim().length === 0}
            >
              Continue
            </Button>
          </div>
        </div>
      </Modal>
    );
  }

  const failure = error ? mergeErrorMessage(error) : undefined;

  // ConfirmDialog only takes a plain `description` string with no slot for
  // extra content, so the failure banner is rendered via Modal directly
  // (mirroring ConfirmDialog's own body/footer structure) rather than as a
  // ConfirmDialog + sibling <p>. Modal opens a native <dialog> via
  // showModal(), which puts it in the browser's top layer with its own
  // ::backdrop — any failure message rendered as a document-level sibling of
  // ConfirmDialog would sit behind that backdrop and never be visible to the
  // user. Rendering it as a child of Modal keeps it inside the dialog's own
  // top-layer content, so it's actually visible on failure (PO-AC-M3/M4/M5).
  return (
    <Modal
      open={open}
      onClose={handleClose}
      title="Confirm merge"
      width="sm"
      footer={
        <>
          <Button variant="secondary" onClick={handleClose}>
            Cancel
          </Button>
          <Button
            onClick={() => mutate({ targetId }, { onSuccess: handleClose })}
            loading={isPending}
          >
            Confirm
          </Button>
        </>
      }
    >
      <div className="px-gf-xl py-gf-lg flex flex-col gap-gf-md">
        <p className="text-gf-body text-gf-secondary">
          The target record survives; this record is archived. On any
          primary-email/primary-phone/current-employer conflict, the survivor's
          values win — this record's conflicting primary rows are demoted to
          non-primary.
        </p>
        {failure && (
          <p role="alert" className="text-gf-body text-gf-status-danger">
            {failure.message}
          </p>
        )}
      </div>
    </Modal>
  );
}
