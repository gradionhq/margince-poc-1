import { useEffect, useMemo } from "react";
import type {
  Activity,
  components,
} from "../../../lib/api-client/generated/index.js";
import { Button, Chip, StatusBadge } from "../../../shared/ui/forge.js";
import { ScanStatusChip } from "./ScanStatusChip.js";

type Attachment = components["schemas"]["Attachment"];

function formatBytes(bytes: number) {
  return `${new Intl.NumberFormat(undefined).format(bytes)} bytes`;
}

function formatTimestamp(iso: string) {
  return new Date(iso).toLocaleString();
}

function provenanceLabel(attachment: Attachment) {
  if (attachment.captured_by.startsWith("agent:")) {
    return `Captured by ${attachment.captured_by}`;
  }
  return `Uploaded by ${attachment.captured_by.replace(/^human:/, "")}`;
}

function parseTimestamp(iso: string) {
  const value = new Date(iso).getTime();
  return Number.isNaN(value) ? undefined : value;
}

function hasStoredActivityId(
  attachment: Attachment,
): attachment is Attachment & { activity_id: string } {
  return "activity_id" in attachment && typeof attachment.activity_id === "string";
}

function closestTimelineActivity(
  attachment: Attachment,
  activities?: Activity[],
) {
  const attachmentTime = parseTimestamp(attachment.created_at);
  if (attachmentTime === undefined || !activities || activities.length === 0) {
    return undefined;
  }

  let closest: Activity | undefined;
  let closestDelta = Number.POSITIVE_INFINITY;

  for (const activity of activities) {
    const activityTime = parseTimestamp(activity.occurred_at);
    if (activityTime === undefined) continue;
    const delta = Math.abs(activityTime - attachmentTime);
    if (delta < closestDelta) {
      closest = activity;
      closestDelta = delta;
    }
  }

  return closest;
}

export function DetailsDrawer({
  attachment,
  open,
  onClose,
  activities,
}: {
  attachment: Attachment;
  open: boolean;
  onClose: () => void;
  activities?: Activity[];
}) {
  const linkedActivity = useMemo(() => {
    if (hasStoredActivityId(attachment)) {
      return {
        label: "Timeline activity id",
        value: attachment.activity_id,
      };
    }

    const derived = closestTimelineActivity(attachment, activities);
    if (!derived) return undefined;

    return {
      label: "Closest matching timeline entry",
      value: derived.id,
    };
  }, [activities, attachment]);

  useEffect(() => {
    if (!open) return undefined;

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        onClose();
      }
    }

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onClose, open]);

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-gf-modal bg-gf-primary/40"
      onClick={onClose}
      data-testid="details-drawer-backdrop"
    >
      <aside
        role="dialog"
        aria-modal="true"
        aria-labelledby="details-drawer-title"
        data-testid="details-drawer"
        className="absolute right-0 top-0 flex h-full w-full max-w-xl flex-col border-l border-gf-subtle bg-gf-surface shadow-2xl"
        onClick={(event) => event.stopPropagation()}
      >
        <header className="flex items-start justify-between gap-gf-md border-b border-gf-subtle px-gf-lg py-gf-md">
          <div className="min-w-0">
            <p className="text-gf-caption uppercase tracking-wide text-gf-secondary">
              Attachment details
            </p>
            <h2
              id="details-drawer-title"
              className="truncate text-gf-title font-semibold text-gf-primary"
            >
              {attachment.filename}
            </h2>
          </div>
          <Button variant="ghost" size="sm" onClick={onClose}>
            Close
          </Button>
        </header>

        <div className="flex-1 overflow-y-auto px-gf-lg py-gf-lg">
          <dl className="grid gap-gf-md">
            <div>
              <dt className="text-gf-caption text-gf-secondary">Type</dt>
              <dd className="text-gf-body text-gf-primary">
                {attachment.content_type}
              </dd>
            </div>
            <div>
              <dt className="text-gf-caption text-gf-secondary">Size</dt>
              <dd className="text-gf-body text-gf-primary">
                {formatBytes(attachment.byte_size)}
              </dd>
            </div>
            <div>
              <dt className="text-gf-caption text-gf-secondary">SHA-256</dt>
              <dd className="text-gf-body text-gf-primary">
                {attachment.checksum ?? "Not available"}
              </dd>
            </div>
            <div>
              <dt className="text-gf-caption text-gf-secondary">Provenance</dt>
              <dd className="text-gf-body text-gf-primary">
                {provenanceLabel(attachment)}
              </dd>
            </div>
            <div>
              <dt className="text-gf-caption text-gf-secondary">
                Attached at
              </dt>
              <dd className="text-gf-body text-gf-primary">
                {formatTimestamp(attachment.created_at)}
              </dd>
            </div>
            <div>
              <dt className="text-gf-caption text-gf-secondary">
                Scan result
              </dt>
              <dd className="text-gf-body text-gf-primary">
                <ScanStatusChip scanStatus={attachment.scan_status} />
              </dd>
            </div>
            <div>
              <dt className="text-gf-caption text-gf-secondary">
                Visibility scope
              </dt>
              <dd className="text-gf-body text-gf-primary">
                <StatusBadge
                  label={attachment.access ?? "visible"}
                  variant={
                    (attachment.access ?? "visible") === "restricted"
                      ? "warning"
                      : "success"
                  }
                />
              </dd>
            </div>
            <div>
              <dt className="text-gf-caption text-gf-secondary">
                {linkedActivity?.label ?? "Timeline activity id"}
              </dt>
              <dd className="text-gf-body text-gf-primary">
                {linkedActivity?.value ?? "Not available"}
              </dd>
            </div>
            <div>
              <dt className="text-gf-caption text-gf-secondary">
                Immutable fields
              </dt>
              <dd className="text-gf-body text-gf-primary">
                <Chip variant="neutral">Provenance is read-only</Chip>
                <span className="ml-gf-sm">
                  <Chip variant="neutral">Scan result is read-only</Chip>
                </span>
              </dd>
            </div>
          </dl>
        </div>
      </aside>
    </div>
  );
}
