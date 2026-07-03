import { FieldGuard } from "../../../shared/ui/FieldGuard.js";
import { RoleBadge } from "../../../shared/ui/RoleBadge.js";
import { useAuthStore } from "../../identity/store/authStore.js";
import { usePeople } from "../api/people.js";
import { PersonList } from "../components/PersonList.js";

export function PeoplePage() {
  const { data, isLoading, isError } = usePeople();
  const { user, role } = useAuthStore();
  const capturedByMode = role === "admin" ? "visible" : "masked";

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
      <main className="p-gf-lg max-w-2xl mx-auto">
        <div className="flex items-center justify-between mb-gf-md">
          <h2 className="text-gf-title font-semibold text-gf-primary">
            People
          </h2>
          <FieldGuard mode={capturedByMode}>
            <span className="text-gf-caption text-gf-secondary italic">
              captured_by column: admin only
            </span>
          </FieldGuard>
        </div>
        <PersonList
          people={data?.data ?? []}
          isLoading={isLoading}
          isError={isError}
        />
      </main>
    </div>
  );
}
