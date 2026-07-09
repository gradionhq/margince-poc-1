import { Skeleton } from "../../../shared/ui/forge.js";
import { useAttachments } from "../api/attachments.js";
import { AttachmentRow } from "./AttachmentRow.js";

const SKELETON_ROWS = [0, 1, 2];

export function AttachmentList({
  entityType,
  entityId,
  onDownload,
  onDetails,
  onFilenameClick,
}: {
  entityType: string;
  entityId: string;
  onDownload?: Parameters<typeof AttachmentRow>[0]["onDownload"];
  onDetails?: Parameters<typeof AttachmentRow>[0]["onDetails"];
  onFilenameClick?: Parameters<typeof AttachmentRow>[0]["onFilenameClick"];
}) {
  const { data, isLoading, isError, refetch } = useAttachments({
    entityType,
    entityId,
  });

  if (isLoading) {
    return (
      <div
        data-testid="attachment-list-skeleton"
        className="flex flex-col gap-gf-sm rounded-lg border border-gf-subtle bg-gf-card p-gf-md"
      >
        {SKELETON_ROWS.map((row) => (
          <Skeleton key={row} className="h-12 w-full" />
        ))}
      </div>
    );
  }

  if (isError) {
    return (
      <div
        data-testid="attachment-list-error"
        className="rounded-lg border border-gf-subtle bg-gf-card p-gf-md"
      >
        <p className="text-gf-body text-gf-status-danger">
          Failed to load files.
        </p>
        <button
          type="button"
          onClick={() => refetch()}
          className="mt-gf-sm rounded-md border border-gf-subtle px-gf-sm py-gf-xs text-gf-caption text-gf-secondary hover:bg-gf-hover"
        >
          Retry
        </button>
      </div>
    );
  }

  if (!data || data.length === 0) {
    return (
      <div className="rounded-lg border border-gf-subtle bg-gf-card p-gf-md">
        <p className="text-gf-body text-gf-secondary">
          No files attached yet
        </p>
      </div>
    );
  }

  return (
    <div className="rounded-lg border border-gf-subtle bg-gf-card p-gf-md">
      <ul className="flex flex-col">
        {data.map((attachment) => (
          <AttachmentRow
            key={attachment.id}
            attachment={attachment}
            onDownload={onDownload}
            onDetails={onDetails}
            onFilenameClick={onFilenameClick}
          />
        ))}
      </ul>
    </div>
  );
}
