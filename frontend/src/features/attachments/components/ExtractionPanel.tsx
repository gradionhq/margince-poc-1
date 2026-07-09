import { useEffect, useState } from "react";
import type { components } from "../../../lib/api-client/generated/index.js";
import {
  Button,
  Chip,
  StatusDot,
  TextInput,
} from "../../../shared/ui/forge.js";
import { ToastContainer } from "../../../shared/ui/ToastContainer.js";
import {
  useAcceptExtraction,
  useAttachmentExtraction,
} from "../api/attachments.js";
import { useToasts } from "../hooks/useToasts.js";

type AttachmentExtraction = components["schemas"]["AttachmentExtraction"];
type ExtractedField = AttachmentExtraction["fields"][number];
type OmittedField = AttachmentExtraction["omitted"][number];
type AcceptedField =
  components["schemas"]["AttachmentExtractionAcceptResponse"]["accepted"][number];

type ExtractionPanelProps = {
  attachmentId: string;
  dealId?: string;
};

function confidenceDotState(confidence: ExtractedField["confidence"]) {
  return confidence === "high" ? "success" : "running";
}

function singularize(count: number, word: string) {
  return `${count} ${word}${count === 1 ? "" : "s"}`;
}

export function ExtractionPanel({
  attachmentId,
  dealId,
}: ExtractionPanelProps) {
  const extractionId = dealId ? attachmentId : undefined;
  const { data, isLoading, isError } = useAttachmentExtraction(extractionId);
  const acceptExtraction = useAcceptExtraction(attachmentId);
  const [dismissed, setDismissed] = useState(false);
  const [accepted, setAccepted] = useState<AcceptedField[] | null>(null);
  const [editingField, setEditingField] = useState<string | null>(null);
  const [draftValues, setDraftValues] = useState<Record<string, string>>({});
  const { toasts, pushToast, dismissToast } = useToasts();

  useEffect(() => {
    setDismissed(false);
    setAccepted(null);
    setEditingField(null);
    setDraftValues({});
  }, []);

  const groundedFields = data?.fields ?? [];
  const omittedFields = data?.omitted ?? [];
  const acceptedByField = new Map<string, AcceptedField>();
  for (const field of accepted ?? []) {
    acceptedByField.set(field.field, field);
  }
  const groundedByField = new Map<string, ExtractedField>();
  for (const field of groundedFields) {
    groundedByField.set(field.field, field);
  }

  if (!dealId) {
    return null;
  }

  if (dismissed) {
    return <ToastContainer toasts={toasts} onDismiss={dismissToast} />;
  }

  if (
    isLoading ||
    isError ||
    (!accepted && groundedFields.length === 0 && omittedFields.length === 0)
  ) {
    return <ToastContainer toasts={toasts} onDismiss={dismissToast} />;
  }

  const activeAcceptedFields = accepted ?? [];
  const isAccepted = activeAcceptedFields.length > 0;
  const groundedCount = groundedFields.length;
  const acceptLabel = `Accept ${singularize(groundedCount, "field")}`;
  const heading = isAccepted
    ? `${singularize(activeAcceptedFields.length, "field")} accepted to the deal — original snippets retained`
    : `AI read this file — ${singularize(groundedCount, "field")} it can ground, staged for your record (accept to persist)`;

  function handleDismiss() {
    setDismissed(true);
    pushToast("info", "Nothing was written");
  }

  function handleAccept() {
    const fieldKeys = groundedFields.map((field) => field.field);
    const edits: Record<string, string> = {};
    for (const [fieldName, value] of Object.entries(draftValues)) {
      const original = groundedByField.get(fieldName)?.value;
      if (original !== undefined && value !== original) {
        edits[fieldName] = value;
      }
    }

    acceptExtraction.mutate(
      {
        field_keys: fieldKeys,
        ...(Object.keys(edits).length > 0 ? { edits } : {}),
      },
      {
        onSuccess: (response) => {
          setAccepted(response.accepted);
          setEditingField(null);
          setDraftValues({});
          pushToast(
            "success",
            `${singularize(response.accepted.length, "field")} accepted to the deal`,
          );
        },
        onError: () => {
          pushToast("error", "Failed to accept staged extraction");
        },
      },
    );
  }

  function renderFieldBody(field: ExtractedField) {
    if (acceptedByField.has(field.field)) {
      const acceptedField = acceptedByField.get(field.field);
      const provenance =
        acceptedField?.provenance === "human"
          ? "typed-by-you"
          : "original snippet retained";

      return (
        <div className="flex flex-col gap-gf-xs">
          <div className="flex flex-wrap items-center gap-gf-xs">
            <span className="font-medium text-gf-primary">
              {acceptedField?.value ?? field.value}
            </span>
            <Chip variant="success">{provenance}</Chip>
          </div>
          <p className="text-gf-caption text-gf-secondary">
            Original snippet: {field.source_quote}
          </p>
        </div>
      );
    }

    if (editingField === field.field) {
      return (
        <div className="flex flex-col gap-gf-xs">
          <TextInput
            value={draftValues[field.field] ?? field.value}
            onChange={(value) =>
              setDraftValues((current) => ({
                ...current,
                [field.field]: value,
              }))
            }
          />
          <p className="text-gf-caption text-gf-secondary">
            Editing will send this value to the deal.
          </p>
        </div>
      );
    }

    return (
      <div className="flex flex-col gap-gf-xs">
        <p className="font-medium text-gf-primary">{field.value}</p>
        <p className="text-gf-caption text-gf-secondary">
          Original snippet: {field.source_quote}
        </p>
      </div>
    );
  }

  return (
    <>
      <section
        data-testid="extraction-panel"
        className={`rounded-lg border p-gf-lg ${isAccepted ? "border-gf-status-success-fg bg-gf-status-success-subtle" : "border-gf-subtle bg-gf-card"}`}
        aria-labelledby="extraction-panel-title"
      >
        <div className="flex flex-col gap-gf-md">
          <div className="flex flex-col gap-gf-sm">
            <h2
              id="extraction-panel-title"
              className="text-gf-title font-semibold text-gf-primary"
            >
              {heading}
            </h2>
            {!isAccepted && (
              <p className="text-gf-caption text-gf-secondary">
                Grounded fields are staged until you accept them. Dismiss only
                closes the panel locally.
              </p>
            )}
          </div>

          {groundedFields.length > 0 && (
            <div className="flex flex-col gap-gf-sm">
              {groundedFields.map((field) => (
                <article
                  key={field.field}
                  data-testid={`extraction-field-${field.field}`}
                  className="rounded-md border border-gf-subtle bg-gf-elevated p-gf-md"
                >
                  <div className="flex items-start justify-between gap-gf-md">
                    <div className="flex min-w-0 flex-1 flex-col gap-gf-xs">
                      <div className="flex flex-wrap items-center gap-gf-xs">
                        <h3 className="text-gf-body font-medium text-gf-primary">
                          {field.field}
                        </h3>
                        <Chip>grounded</Chip>
                        <span className="text-gf-caption text-gf-secondary">
                          {field.page_or_section}
                        </span>
                        <span className="inline-flex items-center gap-1 text-gf-caption text-gf-secondary">
                          <StatusDot
                            state={confidenceDotState(field.confidence)}
                            ariaLabel={`${field.confidence} confidence`}
                          />
                          <span>{field.confidence}</span>
                        </span>
                      </div>
                      {renderFieldBody(field)}
                    </div>
                    {!isAccepted && (
                      <div className="shrink-0">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => {
                            setEditingField(field.field);
                            setDraftValues((current) => ({
                              ...current,
                              [field.field]:
                                current[field.field] ?? field.value,
                            }));
                          }}
                        >
                          Edit
                        </Button>
                      </div>
                    )}
                  </div>
                </article>
              ))}
            </div>
          )}

          {omittedFields.length > 0 && (
            <div className="flex flex-col gap-gf-sm">
              {omittedFields.map((field: OmittedField) => (
                <p
                  key={field.field}
                  data-testid={`extraction-omitted-${field.field}`}
                  className="rounded-md border border-dashed border-gf-subtle px-gf-md py-gf-sm text-gf-caption text-gf-secondary"
                >
                  {field.field} - omitted (not stated in this file)
                </p>
              ))}
            </div>
          )}

          {!isAccepted && groundedFields.length > 0 && (
            <div className="flex flex-wrap items-center gap-gf-sm">
              <Button
                variant="primary"
                loading={acceptExtraction.isPending}
                onClick={handleAccept}
              >
                {acceptLabel}
              </Button>
              <Button variant="secondary" onClick={handleDismiss}>
                Dismiss
              </Button>
              {editingField && (
                <Button variant="ghost" onClick={() => setEditingField(null)}>
                  Done editing
                </Button>
              )}
            </div>
          )}
        </div>
      </section>

      <ToastContainer toasts={toasts} onDismiss={dismissToast} />
    </>
  );
}
