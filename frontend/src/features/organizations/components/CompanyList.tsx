import type { Organization } from "../../../lib/api-client/generated/index.js";
import { Skeleton } from "../../../shared/ui/forge.js";
import { CompanyRow } from "./CompanyRow.js";

const COLUMNS = ["Company", "Contacts", "Open deals", "Org strength"];

interface CompanyListProps {
  companies: Organization[];
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  onRowClick: (id: string) => void;
}

export function CompanyList({
  companies,
  isLoading,
  isError,
  onRetry,
  onRowClick,
}: CompanyListProps) {
  if (isLoading) {
    return (
      <div
        data-testid="company-list-skeleton"
        className="flex flex-col gap-gf-sm p-gf-md"
      >
        {[1, 2, 3].map((i) => (
          <Skeleton key={i} height="40px" />
        ))}
      </div>
    );
  }

  if (isError) {
    return (
      <div className="p-gf-md rounded-md border border-gf-status-danger-subtle bg-gf-status-danger-subtle">
        <p className="text-gf-body text-gf-status-danger mb-gf-sm">
          Failed to load companies.
        </p>
        <button
          type="button"
          onClick={onRetry}
          className="text-gf-caption text-gf-accent underline"
        >
          Retry
        </button>
      </div>
    );
  }

  if (companies.length === 0) {
    return (
      <p className="p-gf-md text-gf-body text-gf-secondary">
        No companies yet.
      </p>
    );
  }

  return (
    <div className="overflow-auto">
      <table className="w-full text-gf-body text-gf-primary">
        <thead className="sticky top-0 bg-gf-elevated">
          <tr>
            {COLUMNS.map((col) => (
              <th
                key={col}
                className="p-gf-sm text-left text-gf-label font-medium text-gf-secondary"
              >
                {col}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {companies.map((org) => (
            <CompanyRow
              key={org.id}
              org={org}
              onClick={() => onRowClick(org.id)}
            />
          ))}
        </tbody>
      </table>
    </div>
  );
}
