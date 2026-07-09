import { useParams } from "react-router-dom";
import { Skeleton } from "../../../shared/ui/forge.js";
import {
  FieldHistoryForbiddenError,
  type EntityType,
  useEntityRecord,
  useFieldHistory,
} from "../api/fieldHistory.js";
import { FieldHistoryControls } from "../components/FieldHistoryControls.js";
import { FieldHistoryGroupCard } from "../components/FieldHistoryGroupCard.js";
import { useFieldHistoryView } from "../hooks/useFieldHistoryView.js";

const VALID_ENTITY_TYPES: EntityType[] = ["person", "organization", "deal", "lead", "activity"];

export function FieldHistoryPage() {
  const { entityType: rawEntityType, entityId } = useParams<{ entityType: string; entityId: string }>();
  const entityType = VALID_ENTITY_TYPES.includes(rawEntityType as EntityType)
    ? (rawEntityType as EntityType)
    : undefined;

  const historyQuery = useFieldHistory(entityType, entityId);
  const recordQuery = useEntityRecord(entityType, entityId);

  const isLoading = historyQuery.isLoading || recordQuery.isLoading;
  const isForbidden =
    historyQuery.error instanceof FieldHistoryForbiddenError ||
    recordQuery.error instanceof FieldHistoryForbiddenError;
  const isError = (historyQuery.isError || recordQuery.isError) && !isForbidden;

  const view = useFieldHistoryView(historyQuery.data ?? [], recordQuery.data);
  const currency =
    typeof recordQuery.data?.currency === "string" ? (recordQuery.data.currency as string) : "EUR";
  const fieldOptions = view.groups.map((g) => ({ field: g.field, label: g.label }));

  if (!entityType || !entityId) {
    return <p className="p-gf-lg text-gf-body text-gf-secondary">Unknown record.</p>;
  }

  return (
    <div className="min-h-screen bg-gf-page">
      <header className="px-gf-lg py-gf-md border-b border-gf-subtle bg-gf-card">
        <div className="flex items-center gap-gf-sm">
          <h2 className="text-gf-title font-semibold text-gf-primary">Field change history</h2>
          {!isLoading && !isForbidden && !isError && (
            <span className="ml-auto text-gf-caption text-gf-secondary">
              {view.header.fieldCount} fields · {view.header.changeCount} changes
            </span>
          )}
        </div>
        <p className="mt-gf-sm text-gf-caption text-gf-secondary">
          Reconstructed from the append-only audit log — this is a{" "}
          <b className="text-gf-primary">read-only projection, not editable here</b>.
        </p>
      </header>
      <main className="p-gf-lg">
        <FieldHistoryControls
          actor={view.actor}
          onActorChange={view.setActor}
          field={view.field}
          onFieldChange={view.setField}
          fieldOptions={fieldOptions}
          search={view.search}
          onSearchChange={view.setSearch}
          hasActiveFilters={view.hasActiveFilters}
          onClearFilters={view.clearFilters}
        />
        {isLoading ? (
          <div data-testid="field-history-skeleton" className="mt-gf-md flex flex-col gap-gf-sm">
            <Skeleton height="90px" />
            <Skeleton height="90px" />
          </div>
        ) : isForbidden ? (
          <p className="mt-gf-md text-gf-body text-gf-status-danger">
            You don't have access to this record's field history.
          </p>
        ) : isError ? (
          <div className="mt-gf-md text-gf-body text-gf-status-danger">
            <p>Couldn't load the change history.</p>
            <p className="mt-gf-xs text-gf-caption text-gf-secondary">
              This is a read of an append-only log — your data is intact and nothing was lost. We
              won't show a partial or guessed history.
            </p>
          </div>
        ) : view.filteredGroups.length === 0 ? (
          <div className="mt-gf-md text-center">
            <p className="text-gf-body text-gf-primary">No changes match this filter</p>
            <p className="mt-gf-xs text-gf-caption text-gf-secondary">
              No field matched your search, or no change in the selected scope was made by that
              actor.
            </p>
          </div>
        ) : (
          view.filteredGroups.map((g) => (
            <FieldHistoryGroupCard key={g.field} group={g} currency={currency} />
          ))
        )}
      </main>
    </div>
  );
}
