import { useState } from "react";
import { useParams } from "react-router-dom";
import { Button, Skeleton } from "../../../shared/ui/forge.js";
import { usePerson } from "../api/person.js";
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
export function PersonDetailPage() {
  const { id } = useParams<{ id: string }>();
  const [mergeOpen, setMergeOpen] = useState(false);
  const {
    data: person,
    isLoading,
    isError,
    error,
    refetch,
  } = usePerson(id ?? "");

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
      <div className="flex items-start justify-between gap-gf-md">
        <div className="flex-1">
          <PersonHeader person={person} />
        </div>
        <Button variant="ghost" size="sm" onClick={() => setMergeOpen(true)}>
          Merge…
        </Button>
      </div>
      <StrengthCard personId={person.id} strength={person.strength} />
      <PersonTabs personId={person.id} activities={person.activities ?? []} />
      <MergePersonDialog
        personId={person.id}
        open={mergeOpen}
        onClose={() => setMergeOpen(false)}
      />
    </div>
  );
}
