import type { HTMLAttributes, KeyboardEvent, ReactNode } from "react";

type Column<T> = {
  key: string;
  header: string;
  render: (row: T) => ReactNode;
};

// Intersected with an index signature so callers can also return arbitrary
// `data-*` attributes (e.g. `data-staged`) without TypeScript rejecting them —
// `HTMLAttributes` alone has no index signature for custom data attributes.
type RowProps = HTMLAttributes<HTMLTableRowElement> & Record<string, unknown>;

export function DataTable<T>({
  columns,
  rows,
  getRowKey,
  onRowClick,
  getRowProps,
}: {
  columns: ReadonlyArray<Column<T>>;
  rows: ReadonlyArray<T>;
  getRowKey: (row: T) => string;
  onRowClick?: (row: T) => void;
  getRowProps?: (row: T) => RowProps;
}) {
  return (
    <div className="overflow-auto">
      <table className="w-full text-gf-body text-gf-primary">
        <thead className="sticky top-0 bg-gf-elevated">
          <tr>
            {columns.map((col) => (
              <th
                key={col.key}
                className="p-gf-sm text-left text-gf-label font-medium text-gf-secondary"
              >
                {col.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => {
            const { className: extraClassName, ...extraRowProps } =
              getRowProps?.(row) ?? {};
            const baseClassName = `border-t border-gf-subtle hover:bg-gf-hover ${onRowClick ? "cursor-pointer" : ""}`;
            return (
              <tr
                key={getRowKey(row)}
                className={`${baseClassName} ${extraClassName ?? ""}`.trim()}
                {...(onRowClick
                  ? {
                      tabIndex: 0,
                      onClick: () => onRowClick(row),
                      onKeyDown: (e: KeyboardEvent) => {
                        if (e.target !== e.currentTarget) return;
                        if (e.key === "Enter" || e.key === " ") {
                          e.preventDefault();
                          onRowClick(row);
                        }
                      },
                    }
                  : {})}
                {...extraRowProps}
              >
                {columns.map((col) => (
                  <td key={col.key} className="p-gf-sm">
                    {col.render(row)}
                  </td>
                ))}
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
