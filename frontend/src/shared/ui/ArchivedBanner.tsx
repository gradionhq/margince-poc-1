import { Link } from "react-router-dom";
import { Button } from "./forge.js";

type RestoreProblem = {
  code?: unknown;
  detail?: unknown;
  details?: {
    existing_id?: unknown;
  };
};

export function restoreErrorMessage(error: unknown): {
  message: string;
  existingId?: string;
} {
  if (error && typeof error === "object") {
    const problem = error as RestoreProblem;
    const code = typeof problem.code === "string" ? problem.code : undefined;
    const detail = typeof problem.detail === "string" ? problem.detail : undefined;
    const existingId =
      typeof problem.details?.existing_id === "string"
        ? problem.details.existing_id
        : undefined;

    if (detail) return { message: detail, existingId };
    if (code) return { message: `Restore failed (${code}).`, existingId };
  }

  return { message: "Restore failed — please try again." };
}

export function ArchivedBanner({
  entityLabel,
  onRestore,
  isRestoring,
  existingRecordId,
  existingRecordHref,
}: {
  entityLabel: string;
  onRestore: () => void;
  isRestoring: boolean;
  existingRecordId?: string;
  existingRecordHref?: string;
}) {
  const showExistingRecordLink = !!existingRecordId && !!existingRecordHref;

  return (
    <div
      role="status"
      data-testid="archived-banner"
      className="flex items-center justify-between gap-gf-md rounded-md border border-gf-status-warning-subtle bg-gf-status-warning-subtle px-gf-md py-gf-sm"
    >
      <p className="text-gf-body text-gf-primary">
        This {entityLabel} is archived.
      </p>
      {showExistingRecordLink ? (
        <Link
          to={existingRecordHref}
          className="text-gf-body text-gf-accent underline"
        >
          Already live as a different record →
        </Link>
      ) : (
        <Button variant="secondary" size="sm" loading={isRestoring} onClick={onRestore}>
          Restore
        </Button>
      )}
    </div>
  );
}
