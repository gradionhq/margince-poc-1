import type { ReactNode } from "react";

type Column<T> = {
  key: string;
  header: string;
  render: (row: T) => ReactNode;
};

export function DataTable<T>({
  columns,
  rows,
  getRowKey,
}: {
  columns: ReadonlyArray<Column<T>>;
  rows: ReadonlyArray<T>;
  getRowKey: (row: T) => string;
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
              className="border-t border-gf-subtle hover:bg-gf-hover"
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
