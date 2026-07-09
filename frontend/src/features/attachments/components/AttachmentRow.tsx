import { useState } from "react";
import type { components } from "../../../lib/api-client/generated/index.js";
import { Button, Chip, RailIcon } from "../../../shared/ui/forge.js";
import { ToastContainer } from "../../../shared/ui/ToastContainer.js";
import { useRequestAccess } from "../api/attachments.js";
import { ScanStatusChip } from "./ScanStatusChip.js";

type Attachment = components["schemas"]["Attachment"];
type Toast = { id: string; variant: "success" | "error" | "info"; message: string };

function formatBytes(bytes: number) {
  return new Intl.NumberFormat(undefined).format(bytes) + " bytes";
}

function formatTimestamp(iso: string) {
  return new Date(iso).toLocaleString();
}

function iconForContentType(contentType: string) {
  if (contentType.startsWith("image/")) return "Image";
  if (contentType.startsWith("video/")) return "Film";
  if (contentType.startsWith("audio/")) return "Music";
  if (contentType === "application/pdf" || contentType.startsWith("text/"))
    return "FileText";
  if (
    contentType === "application/zip" ||
    contentType === "application/x-zip-compressed"
  )
    return "Archive";
  return "File";
}

function sourceCanOpen(attachment: Attachment) {
  return attachment.source === "human" || attachment.captured_by.startsWith("agent:");
}

function provenanceLabel(attachment: Attachment) {
  if (attachment.captured_by.startsWith("agent:")) {
    return `captured by ${attachment.captured_by}`;
  }
  return `uploaded by ${attachment.captured_by.replace(/^human:/, "")}`;
}

export function AttachmentRow({
  attachment,
  onDownload,
  onDetails,
  onFilenameClick,
}: {
  attachment: Attachment;
  onDownload?: (attachment: Attachment) => void;
  onDetails?: (attachment: Attachment) => void;
  onFilenameClick?: (attachment: Attachment) => void;
}) {
  const requestAccess = useRequestAccess(attachment.id);
  const [toasts, setToasts] = useState<Toast[]>([]);
  const isRestricted = attachment.access === "restricted";
  const isBlocked = attachment.scan_status === "blocked";
  const canOpen = sourceCanOpen(attachment) && !isRestricted && !isBlocked;
  const openFilename = onFilenameClick ?? onDetails;
  const showDownload =
    Boolean(attachment.download_url) && !isRestricted && !isBlocked;

  function pushToast(variant: Toast["variant"], message: string) {
    setToasts((current) => [
      ...current,
      { id: crypto.randomUUID(), variant, message },
    ]);
  }

  async function handleRequestAccess() {
    try {
      await requestAccess.mutateAsync();
      pushToast("success", "Access request sent and logged.");
    } catch {
      pushToast("error", "Failed to request access.");
    }
  }

  function handleBlockedInfo() {
    pushToast("info", "Blocked because the file was quarantined.");
  }

  return (
    <li
      data-testid={`attachment-row-${attachment.id}`}
      className="grid gap-gf-sm border-b border-gf-subtle py-gf-sm last:border-b-0 lg:grid-cols-[1fr_auto]"
    >
      <div className="flex min-w-0 items-start gap-gf-sm">
        <div className="mt-0.5 shrink-0 text-gf-secondary">
          <RailIcon name={iconForContentType(attachment.content_type)} size={18} />
        </div>
        <div className="min-w-0 flex-1">
          {canOpen ? (
            <button
              type="button"
              className="block max-w-full truncate text-left text-gf-body font-medium text-gf-primary underline decoration-transparent hover:text-gf-accent hover:underline"
              onClick={() => openFilename?.(attachment)}
            >
              {attachment.filename}
            </button>
          ) : (
            <p className="truncate text-gf-body font-medium text-gf-primary">
              {attachment.filename}
            </p>
          )}
          <div className="mt-gf-xs flex flex-wrap items-center gap-x-gf-sm gap-y-gf-xs text-gf-caption text-gf-secondary">
            <ScanStatusChip scanStatus={attachment.scan_status} />
            {isRestricted && <Chip variant="info">Restricted</Chip>}
            <span>{formatBytes(attachment.byte_size)}</span>
            <span>{provenanceLabel(attachment)}</span>
            {isRestricted && (
              <span className="text-gf-status-warning">not your role</span>
            )}
            {isBlocked && (
              <span className="text-gf-status-danger">
                Quarantined - not downloadable
              </span>
            )}
            <time dateTime={attachment.created_at}>
              {formatTimestamp(attachment.created_at)}
            </time>
          </div>
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-gf-xs">
        {isRestricted ? (
          <Button
            variant="ghost"
            size="sm"
            icon="Lock"
            onClick={() => void handleRequestAccess()}
          >
            Request access
          </Button>
        ) : isBlocked ? (
          <Button
            variant="ghost"
            size="sm"
            icon="Info"
            onClick={handleBlockedInfo}
          >
            Why was this blocked?
          </Button>
        ) : (
          <>
            {showDownload && (
              <Button
                variant="ghost"
                size="sm"
                icon="Download"
                onClick={() => onDownload?.(attachment)}
              >
                Download
              </Button>
            )}
            <Button
              variant="ghost"
              size="sm"
              icon="Info"
              onClick={() => onDetails?.(attachment)}
            >
              Details
            </Button>
          </>
        )}
      </div>
      <ToastContainer
        toasts={toasts}
        onDismiss={(id) =>
          setToasts((current) => current.filter((toast) => toast.id !== id))
        }
      />
    </li>
  );
}
