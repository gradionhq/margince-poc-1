import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useStrengthSort } from "../../../shared/hooks/useStrengthSort.js";
import { FilterDropdown, TextInput } from "../../../shared/ui/forge.js";
import { RoleBadge } from "../../../shared/ui/RoleBadge.js";
import { useAuthStore } from "../../identity/store/authStore.js";
import { useOrganizations } from "../api/organizations.js";
import { CompanyList } from "../components/CompanyList.js";

export function CompaniesPage() {
  const navigate = useNavigate();
  const { sort, toggle } = useStrengthSort();
  const [q, setQ] = useState("");
  const { data, isLoading, isError, refetch } = useOrganizations({
    sort,
    q: q || undefined,
  });
  const { user, role } = useAuthStore();

  const companies = data?.data ?? [];

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
      <main className="p-gf-lg max-w-5xl mx-auto">
        <div className="flex items-center justify-between mb-gf-md">
          <div>
            <h2 className="text-gf-title font-semibold text-gf-primary">
              Companies
            </h2>
            <p className="text-gf-caption text-gf-secondary">
              {companies.length} in workspace · org strength = strongest contact
            </p>
          </div>
        </div>

        <div className="flex items-center gap-gf-sm mb-gf-md flex-wrap">
          <button
            type="button"
            onClick={toggle}
            className="px-gf-md py-1.5 rounded-md border border-gf-subtle text-gf-caption text-gf-secondary hover:bg-gf-hover"
          >
            Strength{" "}
            {sort === "-strength" ? "↓" : sort === "strength" ? "↑" : ""}
          </button>
          <FilterDropdown
            label="Filter"
            value=""
            options={[]}
            onChange={() => {}}
          />
          <TextInput
            placeholder="Search companies…"
            value={q}
            onChange={setQ}
            leadingIcon="Search"
            className="w-48"
          />
          <button
            type="button"
            className="ml-auto px-gf-md py-1.5 rounded-md border border-gf-subtle text-gf-caption text-gf-muted"
            title="Rare path — most companies enter via sync"
          >
            New
          </button>
        </div>

        <CompanyList
          companies={companies}
          isLoading={isLoading}
          isError={isError}
          onRetry={refetch}
          onRowClick={(id) => navigate(`/companies/${id}`)}
        />
      </main>
    </div>
  );
}
