import { useState } from "react";
import { useStrengthSort } from "../../../shared/hooks/useStrengthSort.js";
import { ArchiveConfirmDialog } from "../../../shared/ui/ArchiveConfirmDialog.js";
import { FieldGuard } from "../../../shared/ui/FieldGuard.js";
import { Button, FilterDropdown, TextInput } from "../../../shared/ui/forge.js";
import { RoleBadge } from "../../../shared/ui/RoleBadge.js";
import { ToastContainer } from "../../../shared/ui/ToastContainer.js";
import { useAuthStore } from "../../identity/store/authStore.js";
import { usePeople } from "../api/people.js";
import { useArchivePerson } from "../api/person.js";
import { PersonList } from "../components/PersonList.js";
import { classifySource } from "../components/SourceChip.js";

export function PeoplePage() {
  const [q, setQ] = useState("");
  const [archiveTarget, setArchiveTarget] = useState<{
    id: string;
    name: string;
  } | null>(null);
  const [toasts, setToasts] = useState<
    Array<{ id: string; variant: "success" | "error"; message: string }>
  >([]);
  const { sort, toggle } = useStrengthSort();
  const { data, isLoading, isError, refetch } = usePeople({
    sort,
    q: q || undefined,
  });
  const { user, role } = useAuthStore();
  const capturedByMode = role === "admin" ? "visible" : "masked";
  const archive = useArchivePerson(archiveTarget?.id ?? "");

  const people = data?.data ?? [];
  const connectorCount = people.filter(
    (p) => classifySource(p.source, p.captured_by) === "connector",
  ).length;
  const typedCount = people.filter(
    (p) => classifySource(p.source, p.captured_by) === "typed-by-you",
  ).length;

  function pushToast(variant: "success" | "error", message: string) {
    setToasts((t) => [...t, { id: crypto.randomUUID(), variant, message }]);
  }

  return (
    <div className="min-h-screen bg-gf-page">
      <header className="flex items-center justify-between px-gf-lg py-gf-md border-b border-gf-subtle bg-gf-card">
        <h1 className="text-gf-body font-semibold text-gf-primary">Margince</h1>
        <div className="flex items-center gap-gf-md">
          {role && <RoleBadge role={role} />}
          <span className="text-gf-caption text-gf-secondary">
            {user?.display_name}
          </span>
        </div>
      </header>
      <main className="p-gf-lg max-w-4xl mx-auto">
        <div className="flex items-start justify-between mb-gf-sm">
          <div>
            <h2 className="text-gf-title font-semibold text-gf-primary">
              Contacts we actually know
            </h2>
            {!isLoading && !isError && (
              <p className="text-gf-caption text-gf-secondary">
                {connectorCount} captured · {typedCount} hand-typed
              </p>
            )}
          </div>
          <FieldGuard mode={capturedByMode}>
            <span className="text-gf-caption text-gf-secondary italic">
              captured_by column: admin only
            </span>
          </FieldGuard>
        </div>
        <div className="flex items-center gap-gf-sm mb-gf-md flex-wrap">
          <button
            type="button"
            aria-label="Sort by strength"
            onClick={toggle}
            className="px-gf-sm py-gf-xs text-gf-caption border border-gf-subtle rounded-md text-gf-secondary hover:bg-gf-hover"
          >
            Strength{sort ? ` (${sort})` : ""}
          </button>
          <FilterDropdown
            label="Filter"
            value=""
            options={[]}
            onChange={() => {}}
          />
          <TextInput
            placeholder="Search contacts…"
            value={q}
            onChange={setQ}
            leadingIcon="Search"
          />
          <Button variant="ghost" size="sm">
            New contact
          </Button>
        </div>
        <PersonList
          people={people}
          isLoading={isLoading}
          isError={isError}
          onRetry={() => void refetch()}
          onArchive={(id) => {
            const target = people.find((p) => p.id === id);
            if (target) setArchiveTarget({ id, name: target.full_name });
          }}
        />
      </main>
      <ArchiveConfirmDialog
        open={!!archiveTarget}
        entityLabel={archiveTarget?.name ?? ""}
        isLoading={archive.isPending}
        onConfirm={() => {
          if (!archiveTarget) return;
          archive.mutate(undefined, {
            onSuccess: () => {
              pushToast("success", `${archiveTarget.name} archived`);
              setArchiveTarget(null);
              void refetch();
            },
            onError: () => {
              pushToast("error", "Failed to archive — please try again.");
              setArchiveTarget(null);
            },
          });
        }}
        onCancel={() => setArchiveTarget(null)}
      />
      <ToastContainer
        toasts={toasts}
        onDismiss={(id) => setToasts((t) => t.filter((x) => x.id !== id))}
      />
    </div>
  );
}
