import type { CustomField, Member } from "../../../lib/api-client/generated/index.js";
import { deriveAuditEntries, resolveMemberName } from "../lib/customFieldRules.js";
import { Skeleton } from "../../../shared/ui/forge.js";
import { FieldGuard } from "../../../shared/ui/FieldGuard.js";

export function CustomFieldAuditCard({
  fields,
  members,
  role,
  isLoading,
  isError,
}: {
  fields: CustomField[];
  members: Member[];
  role: string;
  isLoading: boolean;
  isError: boolean;
}) {
  const entries = isLoading || isError ? [] : deriveAuditEntries(fields);

  return (
    <div
      data-testid="audit-card"
      className="rounded-lg border border-gf-subtle bg-gf-card p-gf-md"
    >
      <h3 className="text-gf-body font-semibold text-gf-primary mb-gf-sm">
        Audit trail
      </h3>
      {isLoading ? (
        <div data-testid="audit-card-skeleton">
          <Skeleton height="60px" />
        </div>
      ) : isError ? (
        <p className="text-gf-body text-gf-status-danger">
          Something went wrong
        </p>
      ) : entries.length === 0 ? (
        <p className="text-gf-body text-gf-secondary">No changes yet.</p>
      ) : (
        <ul>
          {entries.map((entry) => (
            <li
              key={`${entry.id}-${entry.action}-${entry.occurredAt}`}
              className="py-gf-xs border-b border-gf-subtle last:border-b-0"
            >
              <p className="text-gf-body text-gf-primary">
                <FieldGuard
                  mode={role === "admin" ? "visible" : "masked"}
                >
                  {resolveMemberName(members, entry.actorId)}
                </FieldGuard>
                {" "}
                {entry.action === "added"
                  ? `added ${entry.label} (${entry.type}) to ${entry.object}`
                  : `retired ${entry.label}`}
              </p>
              <p className="text-gf-caption text-gf-secondary">
                {new Date(entry.occurredAt).toLocaleDateString()}
                {" · "}
                {entry.auditRef}
              </p>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
