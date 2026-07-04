import type { Stage } from "../../../lib/api-client/generated/index.js";
import { ConfirmDialog } from "../../../shared/ui/forge.js";

// Documented reopen-target rule (DEAL-AC-R1): always the lowest-position open stage. See
// Global Constraints in the plan for why we don't parse audit before/after payloads instead.
export function firstOpenStageId(stages: Stage[]): string | undefined {
  const open = stages
    .filter((s) => s.semantic === "open")
    .slice()
    .sort((a, b) => a.position - b.position);
  return open[0]?.id;
}

export function ReopenConfirmDialog({
  open,
  dealName,
  onConfirm,
  onCancel,
  isLoading = false,
}: {
  open: boolean;
  dealName: string;
  onConfirm: () => void;
  onCancel: () => void;
  isLoading?: boolean;
}) {
  return (
    <ConfirmDialog
      open={open}
      onClose={onCancel}
      onConfirm={onConfirm}
      title="Reopen this deal?"
      description={`${dealName} is closed. Reopening moves it back to the pipeline's first open stage and requires approval.`}
      confirmLabel="Confirm"
      isLoading={isLoading}
    />
  );
}
