import { useParams } from "react-router-dom";
import { Skeleton } from "../../../shared/ui/forge.js";
import { usePerson } from "../api/person.js";

export function PersonDetailPage() {
  const { id } = useParams<{ id: string }>();
  const { data: person, isLoading, isError, error, refetch } = usePerson(id ?? "");

  if (isLoading) {
    return (
      <div data-testid="person-detail-loading" className="p-gf-lg flex flex-col gap-gf-sm">
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
      <div data-testid="person-detail-error" className="p-gf-lg flex flex-col gap-gf-sm">
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
    <div data-testid="person-detail-loaded" className="p-gf-lg max-w-4xl mx-auto flex flex-col gap-gf-lg">
      {/* Header/StrengthCard/Tabs/Merge are wired in Tasks 3-7. */}
    </div>
  );
}
