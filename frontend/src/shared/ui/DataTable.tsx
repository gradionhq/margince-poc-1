import type { KeyboardEvent, ReactNode } from "react";

type Column<T> = {
  key: string;
  header: string;
  render: (row: T) => ReactNode;
};

export function DataTable<T>({
  columns,
  rows,
  getRowKey,
  onRowClick,
}: {
  columns: ReadonlyArray<Column<T>>;
  rows: ReadonlyArray<T>;
  getRowKey: (row: T) => string;
  onRowClick?: (row: T) => void;
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
          {rows.map((row) => (
            <tr
              key={getRowKey(row)}
              className={`border-t border-gf-subtle hover:bg-gf-hover ${onRowClick ? "cursor-pointer" : ""}`}
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
            >
              {columns.map((col) => (
                <td key={col.key} className="p-gf-sm">
                  {col.render(row)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
