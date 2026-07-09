import { Icon } from "../../../shared/ui/forge.js";

export function AiDisclosureBanner({
  hasEvidenceLines,
  aiDisclosureText,
}: {
  hasEvidenceLines: boolean;
  aiDisclosureText?: string | null;
}) {
  if (!hasEvidenceLines) return null;

  return (
    <div className="rounded-gf-lg border border-gf-subtle bg-gf-card px-gf-md py-gf-sm text-gf-body text-gf-secondary">
      <div className="flex items-start gap-gf-sm">
        <Icon name="AlertCircle" size={16} className="mt-0.5 shrink-0" />
        <p>{aiDisclosureText ?? "This offer includes AI-proposed content — review every line before sending."}</p>
      </div>
    </div>
  );
}
