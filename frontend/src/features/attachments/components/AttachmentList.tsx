import { useState } from "react";
import { Skeleton } from "../../../shared/ui/forge.js";
import { useAttachments } from "../api/attachments.js";
import { AttachmentRow } from "./AttachmentRow.js";
import { VisibilityRail } from "./VisibilityRail.js";

const SKELETON_ROWS = [0, 1, 2];
const FILTERS = [
  { key: "all", label: "All" },
  { key: "visible", label: "Visible to me" },
] as const;

export function AttachmentList({
  entityType,
  entityId,
  onDownload,
  onDetails,
  onFilenameClick,
  currentUserId,
}: {
  entityType: string;
  entityId: string;
  onDownload?: Parameters<typeof AttachmentRow>[0]["onDownload"];
  onDetails?: Parameters<typeof AttachmentRow>[0]["onDetails"];
  onFilenameClick?: Parameters<typeof AttachmentRow>[0]["onFilenameClick"];
  currentUserId?: string;
}) {
  const [filter, setFilter] = useState<(typeof FILTERS)[number]["key"]>("all");
  const { data, isLoading, isError, refetch } = useAttachments({
    entityType,
    entityId,
  });
  const attachments = data ?? [];
  const filteredAttachments =
    filter === "visible"
      ? attachments.filter((attachment) => attachment.access !== "restricted")
      : attachments;

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

  const noAttachmentsAtAll = !data || data.length === 0;
  const emptyStateCopy = noAttachmentsAtAll
    ? "No files attached yet"
    : "No visible files";

  return (
    <div className="flex flex-col gap-gf-sm">
      <VisibilityRail />
      <div className="rounded-lg border border-gf-subtle bg-gf-card p-gf-md">
        <div className="flex items-center justify-between gap-gf-sm">
          <div
            role="tablist"
            aria-label="Attachment visibility filter"
            className="inline-flex rounded-md border border-gf-subtle bg-gf-page p-1"
          >
            {FILTERS.map((option) => (
              <button
                key={option.key}
                type="button"
                role="tab"
                aria-selected={filter === option.key}
                onClick={() => setFilter(option.key)}
                className={`rounded-sm px-gf-sm py-gf-xs text-gf-caption ${
                  filter === option.key
                    ? "bg-gf-card text-gf-primary shadow-sm"
                    : "text-gf-secondary"
                }`}
              >
                {option.label}
              </button>
            ))}
          </div>
        </div>
        {filteredAttachments.length === 0 ? (
          <p className="mt-gf-md text-gf-body text-gf-secondary">
            {emptyStateCopy}
          </p>
        ) : (
          <ul className="flex flex-col">
            {filteredAttachments.map((attachment) => (
              <AttachmentRow
                key={attachment.id}
                attachment={attachment}
                onDownload={onDownload}
                onDetails={onDetails}
                onFilenameClick={onFilenameClick}
                currentUserId={currentUserId}
              />
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}
