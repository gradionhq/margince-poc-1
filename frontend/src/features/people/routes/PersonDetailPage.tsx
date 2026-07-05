import { useState } from "react";
import { useParams } from "react-router-dom";
import { ArchiveConfirmDialog } from "../../../shared/ui/ArchiveConfirmDialog.js";
import {
  ArchivedBanner,
  restoreErrorMessage,
} from "../../../shared/ui/ArchivedBanner.js";
import { Button, Skeleton } from "../../../shared/ui/forge.js";
import { ToastContainer } from "../../../shared/ui/ToastContainer.js";
import { useArchivePerson, usePerson, useRestorePerson } from "../api/person.js";
import { MergePersonDialog } from "../components/MergePersonDialog.js";
import { PersonHeader } from "../components/PersonHeader.js";
import { PersonTabs } from "../components/PersonTabs.js";
import { StrengthCard } from "../components/StrengthCard.js";

// Person 360 (T19). Known gaps carried honestly rather than worked around (see PR description):
//  - Company link (`PersonHeader`) points to /companies/{organization_id}, which has no route on
//    main yet (T20 builds it concurrently, not a dependency here).
//  - mergePerson has no If-Match/version param and its 409 uses the generic Conflict schema —
//    `mergeErrorMessage` reads the actual `code` at runtime rather than assuming version_skew.
//  - NotesTab has no confirmed create-note endpoint in the contract — Save is local-state only.
//  - ActivityRef (Person.activities) carries no source/captured_by — the Activity tab cannot
//    render a real per-row provenance chip from this field; omitted rather than fabricated.
//  - `GET /people/{id}` never populates `emails`/`phones` on the composite read — the backend's
//    assembleComposite (backend/internal/modules/people/transport/handler_person.go) and
//    PersonStore.Get/GetAny (backend/internal/modules/directory/store.go) never join
//    person_email/person_phone; those fields only ever appear transiently on Create's request
//    echo, never on a subsequent GET (confirmed live via curl against a real DB with seeded
//    rows). `PersonHeader`'s primaryEmail/primaryPhone branches are correct against the
//    contract's declared shape and will render real data the moment the backend joins it — a
//    pre-existing backend gap (predates T19), not something to fix from this frontend ticket.
export function PersonDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [mergeOpen, setMergeOpen] = useState(false);
  const [archiveOpen, setArchiveOpen] = useState(false);
  const [toasts, setToasts] = useState<
    Array<{ id: string; variant: "success" | "error"; message: string }>
  >([]);
  const [restoreConflict, setRestoreConflict] = useState<{
    message: string;
    existingId?: string;
  } | null>(null);
  const {
    data: person,
    isLoading,
    isError,
    error,
    refetch,
  } = usePerson(id ?? "");
  const archive = useArchivePerson(id ?? "");
  const restore = useRestorePerson(id ?? "");

  function pushToast(variant: "success" | "error", message: string) {
    setToasts((t) => [...t, { id: crypto.randomUUID(), variant, message }]);
  }

  if (isLoading) {
    return (
      <div
        data-testid="person-detail-loading"
        className="p-gf-lg flex flex-col gap-gf-sm"
      >
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-32 w-full" />
      </div>
    );
  }

  if (isError) {
    const detail =
      error && typeof error === "object" && "detail" in error
        ? String((error as { detail?: unknown }).detail)
        : "Failed to load this contact.";
    return (
      <div
        data-testid="person-detail-error"
        className="p-gf-lg flex flex-col gap-gf-sm"
      >
        <p className="text-gf-body text-gf-status-danger">{detail}</p>
        <button
          type="button"
          onClick={() => void refetch()}
          className="self-start px-gf-sm py-gf-xs text-gf-caption border border-gf-subtle rounded-md text-gf-secondary hover:bg-gf-hover"
        >
          Retry
        </button>
      </div>
    );
  }

  if (!person) {
    return (
      <div data-testid="person-detail-notfound" className="p-gf-lg">
        <p className="text-gf-body text-gf-secondary">Contact not found.</p>
      </div>
    );
  }

  return (
    <div
      data-testid="person-detail-loaded"
      className="p-gf-lg max-w-4xl mx-auto flex flex-col gap-gf-lg"
    >
      {person.archived_at && (
        <ArchivedBanner
          entityLabel="contact"
          isRestoring={restore.isPending}
          existingRecordId={restoreConflict?.existingId}
          existingRecordHref={
            restoreConflict?.existingId
              ? `/people/${restoreConflict.existingId}`
              : undefined
          }
          onRestore={() =>
            restore.mutate(undefined, {
              onSuccess: () => {
                pushToast("success", "Contact restored");
                setRestoreConflict(null);
              },
              onError: (err) => {
                const parsed = restoreErrorMessage(err);
                if (parsed.existingId) setRestoreConflict(parsed);
                else pushToast("error", parsed.message);
              },
            })
          }
        />
      )}
      <div className="flex items-start justify-between gap-gf-md">
        <div className="flex-1">
          <PersonHeader person={person} />
        </div>
        <div className="flex items-center gap-gf-sm">
          {!person.archived_at && (
            <Button variant="ghost" size="sm" onClick={() => setArchiveOpen(true)}>
              Archive…
            </Button>
          )}
          <Button variant="ghost" size="sm" onClick={() => setMergeOpen(true)}>
            Merge…
          </Button>
        </div>
      </div>
      <StrengthCard personId={person.id} strength={person.strength} />
      <PersonTabs personId={person.id} activities={person.activities ?? []} />
      <MergePersonDialog
        personId={person.id}
        open={mergeOpen}
        onClose={() => setMergeOpen(false)}
      />
      <ArchiveConfirmDialog
        open={archiveOpen}
        entityLabel={person.full_name}
        isLoading={archive.isPending}
        onConfirm={() =>
          archive.mutate(undefined, {
            onSuccess: () => {
              pushToast("success", "Contact archived");
              setArchiveOpen(false);
            },
            onError: () => {
              pushToast("error", "Failed to archive — please try again.");
              setArchiveOpen(false);
            },
          })
        }
        onCancel={() => setArchiveOpen(false)}
      />
      <ToastContainer
        toasts={toasts}
        onDismiss={(tid) => setToasts((t) => t.filter((x) => x.id !== tid))}
      />
    </div>
  );
}
