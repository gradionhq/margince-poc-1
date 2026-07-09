import { SectionHeader } from "../../../shared/ui/forge.js";

export function VisibilityRail() {
  return (
    <div
      data-testid="attachments-visibility-rail"
      className="rounded-lg border border-gf-subtle bg-gf-card p-gf-md"
    >
      <SectionHeader label="Visibility" />
      <p className="mt-gf-sm text-gf-caption text-gf-secondary">
        Attachment visibility follows the parent record&apos;s RBAC. There is no
        per-file ACL in V1.
      </p>
    </div>
  );
}
