import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import type { components } from "../../../lib/api-client/generated/index.js";
import { Chip } from "../../../shared/ui/forge.js";
import { ToastContainer } from "../../../shared/ui/ToastContainer.js";
import { useDeal, useDealActivities } from "../../deals/api/deals.js";
import { useAuthStore } from "../../identity/store/authStore.js";
import { useAttachments, useCreateAttachment } from "../api/attachments.js";
import { useToasts } from "../hooks/useToasts.js";
import { AttachmentList } from "./AttachmentList.js";
import { DetailsDrawer } from "./DetailsDrawer.js";
import { Dropzone } from "./Dropzone.js";
import { ExtractionPanel } from "./ExtractionPanel.js";

type Attachment = components["schemas"]["Attachment"];
type Activity = components["schemas"]["Activity"];

export type AttachmentsPanelProps =
  | {
      entityType: "deal";
      entityId: string;
      dealId: string;
    }
  | {
      entityType: "person" | "organization" | "lead" | "activity";
      entityId: string;
      dealId?: never;
    };

function labelForEntity(entityType: AttachmentsPanelProps["entityType"]) {
  return entityType === "deal"
    ? "deal"
    : entityType.charAt(0).toUpperCase() + entityType.slice(1);
}

function entityHref(
  entityType: AttachmentsPanelProps["entityType"],
  id: string,
) {
  if (entityType === "deal") return `/deals/${id}`;
  return undefined;
}

function roleCopy(role: string | null) {
  return role ? `${role} · sees deal-room files` : "role unavailable";
}

function firstAgentAttachment(attachments: Attachment[]) {
  return attachments.find((attachment) =>
    attachment.captured_by.startsWith("agent:"),
  );
}

function AttachmentsPanelShell({
  entityType,
  entityId,
  dealId,
  recordLabel,
  recordHref,
  extractionAttachmentId,
  activityFeed,
}: {
  entityType: AttachmentsPanelProps["entityType"];
  entityId: string;
  dealId?: string;
  recordLabel: string;
  recordHref?: string;
  extractionAttachmentId?: string;
  activityFeed?: Activity[];
}) {
  const { role, user } = useAuthStore();
  const createAttachment = useCreateAttachment();
  const [selectedAttachment, setSelectedAttachment] =
    useState<Attachment | null>(null);
  const { toasts, pushToast, dismissToast } = useToasts();
  // Reuses the same `useAttachments` polling AttachmentList already drives
  // (shared react-query cache key, no second poll loop) to detect when a
  // just-uploaded file's scan finishes.
  const { data: liveAttachments } = useAttachments({ entityType, entityId });
  const [pendingScanIds, setPendingScanIds] = useState<ReadonlySet<string>>(
    new Set(),
  );

  useEffect(() => {
    if (pendingScanIds.size === 0 || !liveAttachments) return;

    const stillPending = new Set(pendingScanIds);
    for (const attachment of liveAttachments) {
      if (!stillPending.has(attachment.id)) continue;
      if (attachment.scan_status === "scanning") continue;

      stillPending.delete(attachment.id);
      if (attachment.scan_status === "clean") {
        pushToast(
          "success",
          `${attachment.filename} attached and written to the timeline with provenance`,
        );
      }
    }

    if (stillPending.size !== pendingScanIds.size) {
      setPendingScanIds(stillPending);
    }
  }, [liveAttachments, pendingScanIds, pushToast]);

  async function handleFilesSelected(files: FileList) {
    if (entityType !== "deal" || !dealId) return;
    if (!user) {
      pushToast("error", "Sign in to upload attachments");
      return;
    }

    const queued = Array.from(files);
    for (const file of queued) {
      try {
        const attachment = await createAttachment.mutateAsync({
          request: {
            entity_type: entityType,
            entity_id: entityId,
            filename: file.name,
            content_type: file.type || "application/octet-stream",
            byte_size: file.size,
            source: "ui",
            captured_by: `human:${user.id}`,
          },
          file,
        });
        pushToast("info", "Virus scan in progress");
        setPendingScanIds((current) => new Set(current).add(attachment.id));
      } catch {
        pushToast("error", `Failed to attach ${file.name}`);
      }
    }
  }

  function handleDownload(attachment: Attachment) {
    if (!attachment.download_url) return;

    const anchor = document.createElement("a");
    anchor.href = attachment.download_url;
    anchor.target = "_blank";
    anchor.rel = "noopener noreferrer";
    anchor.download = attachment.filename;
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
    pushToast("success", "Downloaded — access logged");
  }

  return (
    <>
      <section
        data-testid="attachments-panel"
        className="rounded-lg border border-gf-subtle bg-gf-card p-gf-lg"
      >
        <div className="flex flex-col gap-gf-lg">
          <header className="flex flex-col gap-gf-sm border-b border-gf-subtle pb-gf-md">
            <p className="text-gf-caption uppercase tracking-wide text-gf-secondary">
              Attachments
            </p>
            <div className="flex flex-wrap items-center gap-gf-sm">
              {recordHref ? (
                <Link
                  to={recordHref}
                  className="truncate text-gf-title font-semibold text-gf-primary underline decoration-transparent hover:text-gf-accent hover:underline"
                >
                  {recordLabel}
                </Link>
              ) : (
                <h2 className="truncate text-gf-title font-semibold text-gf-primary">
                  {recordLabel}
                </h2>
              )}
              <Chip variant="info">Your role: {roleCopy(role)}</Chip>
            </div>
            <p className="text-gf-caption text-gf-secondary">
              Files inherit the parent record&apos;s RBAC and are written to the
              record timeline with provenance.
            </p>
          </header>

          <Dropzone
            onFilesSelected={(files) => void handleFilesSelected(files)}
          />

          <AttachmentList
            entityType={entityType}
            entityId={entityId}
            onDownload={handleDownload}
            onDetails={(attachment) => setSelectedAttachment(attachment)}
            onFilenameClick={(attachment) => setSelectedAttachment(attachment)}
          />

          {entityType === "deal" && dealId && extractionAttachmentId && (
            <ExtractionPanel
              attachmentId={extractionAttachmentId}
              dealId={dealId}
            />
          )}
        </div>
      </section>

      {selectedAttachment && (
        <DetailsDrawer
          attachment={selectedAttachment}
          open
          onClose={() => setSelectedAttachment(null)}
          activities={activityFeed}
        />
      )}

      <ToastContainer toasts={toasts} onDismiss={dismissToast} />
    </>
  );
}

function DealAttachmentsPanel(
  props: Extract<AttachmentsPanelProps, { entityType: "deal" }>,
) {
  const deal = useDeal(props.entityId);
  const activities = useDealActivities(props.entityId);
  const { data: attachments = [] } = useAttachments({
    entityType: props.entityType,
    entityId: props.entityId,
  });
  const extractionAttachment = firstAgentAttachment(attachments);

  return (
    <AttachmentsPanelShell
      entityType={props.entityType}
      entityId={props.entityId}
      dealId={props.dealId}
      recordLabel={deal.data?.name ?? props.entityId}
      recordHref={entityHref(props.entityType, props.entityId)}
      extractionAttachmentId={extractionAttachment?.id}
      activityFeed={activities.data}
    />
  );
}

function GenericAttachmentsPanel(
  props: Extract<
    AttachmentsPanelProps,
    { entityType: "person" | "organization" | "lead" | "activity" }
  >,
) {
  return (
    <AttachmentsPanelShell
      entityType={props.entityType}
      entityId={props.entityId}
      recordLabel={`${labelForEntity(props.entityType)} ${props.entityId}`}
    />
  );
}

export function AttachmentsPanel(props: AttachmentsPanelProps) {
  return props.entityType === "deal" ? (
    <DealAttachmentsPanel {...props} />
  ) : (
    <GenericAttachmentsPanel {...props} />
  );
}
