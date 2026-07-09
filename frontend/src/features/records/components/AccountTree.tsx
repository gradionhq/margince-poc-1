import type { components } from "../../../lib/api-client/generated/index.js";
import { DataTable } from "../../../shared/ui/DataTable.js";
import { FieldGuard } from "../../../shared/ui/FieldGuard.js";
import { Icon } from "../../../shared/ui/forge.js";
import type { flattenTree } from "../api/accountTree.js";

type FlatRow = ReturnType<typeof flattenTree>[number];
type RestrictedExcludedEntry =
  components["schemas"]["OrganizationHierarchyRollup"]["restricted_excluded"][number];

export function AccountTree({
  rows,
  expandedIds,
  onToggleExpand,
  restrictedNodes,
}: {
  rows: FlatRow[];
  expandedIds: ReadonlySet<string>;
  onToggleExpand: (id: string) => void;
  restrictedNodes: RestrictedExcludedEntry[];
}) {
  const columns = [
    {
      key: "name",
      header: "Account",
      render: (row: FlatRow) => (
        <div
          className="flex items-center gap-gf-xs"
          style={{ paddingLeft: `${row.depth * 20}px` }}
        >
          {row.hasChildren ? (
            <button
              type="button"
              aria-label={
                expandedIds.has(row.node.org.id) ? "Collapse" : "Expand"
              }
              className="text-gf-caption text-gf-secondary"
              onClick={() => onToggleExpand(row.node.org.id)}
            >
              {expandedIds.has(row.node.org.id) ? "▾" : "▸"}
            </button>
          ) : (
            <span className="w-[1em]" />
          )}
          <span>{row.node.org.display_name}</span>
        </div>
      ),
    },
  ];

  return (
    <div>
      <p className="mb-gf-xs text-gf-caption text-gf-muted">
        Shows up to 200 accounts (RD-PARAM-1 tree-size bound — the account list
        is fetched as one bounded page; sub-accounts beyond this bound are not
        shown).
      </p>
      <DataTable
        columns={columns}
        rows={rows}
        getRowKey={(r) => r.node.org.id}
      />
      {restrictedNodes.length > 0 && (
        <div className="mt-gf-md">
          <p className="mb-gf-xs text-gf-caption text-gf-secondary font-medium">
            Restricted
          </p>
          <DataTable
            columns={[
              {
                key: "name",
                header: "Account",
                render: (entry: RestrictedExcludedEntry) => (
                  <div className="flex items-center gap-gf-xs">
                    <Icon name="Lock" size={16} />
                    <span>{entry.display_name}</span>
                    <span className="ml-gf-xs text-gf-caption text-gf-muted">
                      — restricted
                    </span>
                  </div>
                ),
              },
              {
                key: "figures",
                header: "Figures",
                render: () => <FieldGuard mode="masked">{null}</FieldGuard>,
              },
              {
                key: "note",
                header: "Note",
                render: () => (
                  <span className="text-gf-caption text-gf-muted">
                    Excluded from roll-up (no access)
                  </span>
                ),
              },
            ]}
            rows={restrictedNodes}
            getRowKey={(e) => e.id}
          />
        </div>
      )}
    </div>
  );
}
