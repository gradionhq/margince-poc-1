import type { Organization } from "../../../lib/api-client/generated/index.js";
import { Button } from "../../../shared/ui/forge.js";

export function SuggestedEdgeCard({
  candidate,
  status,
  onAccept,
  onDismiss,
}: {
  candidate: Organization;
  parentId: string;
  status: "staged" | "accepted";
  onAccept: () => void;
  onDismiss: () => void;
}) {
  if (status === "accepted") {
    return (
      <div className="rounded-md border border-gf-subtle bg-gf-card p-gf-md">
        <p className="text-gf-body text-gf-primary">
          Add {candidate.display_name} as a child
        </p>
        <p className="mt-gf-xs text-gf-caption text-gf-status-success">
          edge written · audited
        </p>
      </div>
    );
  }

  return (
    <div className="rounded-md border border-gf-subtle bg-gf-card p-gf-md">
      <p className="text-gf-body text-gf-primary">
        Add {candidate.display_name} as a child
      </p>
      <div className="mt-gf-sm flex gap-gf-sm">
        <Button variant="primary" onClick={onAccept}>
          Accept edge
        </Button>
        <Button variant="secondary" onClick={onDismiss}>
          Dismiss
        </Button>
      </div>
    </div>
  );
}
