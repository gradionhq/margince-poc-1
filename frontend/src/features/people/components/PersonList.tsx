import { useNavigate } from "react-router-dom";
import type { Person } from "../../../lib/api-client/generated/index.js";
import { ContextMenu } from "../../../shared/ui/ContextMenu.js";
import { DataTable } from "../../../shared/ui/DataTable.js";
import { Avatar, IconButton, Skeleton } from "../../../shared/ui/forge.js";
import { SourceChip } from "./SourceChip.js";
import { StrengthCell } from "./StrengthCell.js";

interface PersonListProps {
  people: Person[];
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  onArchive: (id: string) => void;
}

function formatLastActivity(iso: string | null | undefined): string {
  if (!iso) return "no activity";
  const d = new Date(iso);
  return d.toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

const SKELETON_ROWS = [0, 1, 2, 3, 4];

export function PersonList({
  people,
  isLoading,
  isError,
  onRetry,
  onArchive,
}: PersonListProps) {
  const navigate = useNavigate();

  if (isLoading) {
    return (
      <div
        data-testid="person-list-skeleton"
        className="flex flex-col gap-gf-sm p-gf-md"
      >
        {SKELETON_ROWS.map((n) => (
          <Skeleton key={n} className="h-8 w-full" />
        ))}
      </div>
    );
  }

  if (isError) {
    return (
      <div
        className="p-gf-md flex flex-col gap-gf-sm"
        data-testid="person-list-error"
      >
        <p className="text-gf-body text-gf-status-danger">
          Failed to load contacts.
        </p>
        <button
          type="button"
          onClick={onRetry}
          className="self-start px-gf-sm py-gf-xs text-gf-caption border border-gf-subtle rounded-md text-gf-secondary hover:bg-gf-hover"
        >
          Retry
        </button>
      </div>
    );
  }

  if (people.length === 0) {
    return (
      <p className="p-gf-md text-gf-body text-gf-secondary">No contacts yet.</p>
    );
  }

  const columns = [
    {
      key: "contact",
      header: "Contact",
      render: (p: Person) => (
        <div className="flex items-center gap-gf-sm min-w-0">
          <Avatar name={p.full_name} size="sm" />
          <div className="min-w-0">
            <p className="text-gf-body font-medium text-gf-primary truncate">
              {p.full_name}
            </p>
            {(p.title || p.emails?.[0]?.email) && (
              <p className="text-gf-caption text-gf-secondary truncate">
                {[p.title, p.emails?.[0]?.email].filter(Boolean).join(" · ")}
              </p>
            )}
          </div>
        </div>
      ),
    },
    {
      key: "relationship",
      header: "Relationship",
      render: (p: Person) =>
        p.strength ? (
          <StrengthCell
            score={p.strength.score}
            bucket={p.strength.bucket}
            recency={p.strength.recency}
            frequency={p.strength.frequency}
            reciprocity={p.strength.reciprocity}
          />
        ) : (
          <StrengthCell noSignalYet />
        ),
    },
    {
      key: "last",
      header: "Last",
      render: (p: Person) => (
        <span className="text-gf-body text-gf-secondary">
          {formatLastActivity(p.last_activity_at)}
        </span>
      ),
    },
    {
      key: "source",
      header: "Source",
      render: (p: Person) => (
        <SourceChip source={p.source} capturedBy={p.captured_by} />
      ),
    },
    {
      key: "actions",
      header: "",
      render: (p: Person) => (
        <div
          onClick={(e) => e.stopPropagation()}
          onKeyDown={(e) => e.stopPropagation()}
        >
          <ContextMenu
            trigger={<IconButton icon="MoreVertical" label="Row actions" />}
            items={[
              {
                id: "archive",
                label: "Archive",
                onSelect: () => onArchive(p.id),
              },
            ]}
          />
        </div>
      ),
    },
  ] as const;

  return (
    <DataTable
      columns={columns}
      rows={people}
      getRowKey={(p) => p.id}
      onRowClick={(p) => navigate(`/people/${p.id}`)}
    />
  );
}
