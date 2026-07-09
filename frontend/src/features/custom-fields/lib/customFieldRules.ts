import type { CustomField, Member } from "../../../lib/api-client/generated/index.js";

export type ObjectKey = "deal" | "organization" | "person" | "lead" | "activity";

export const OBJECT_CHIPS = [
  { value: "deal" as ObjectKey, label: "Deal" },
  { value: "organization" as ObjectKey, label: "Company" },
  { value: "person" as ObjectKey, label: "Contact" },
  { value: "lead" as ObjectKey, label: "Lead" },
  { value: "activity" as ObjectKey, label: "Activity" },
] as const;

export function slugify(label: string): string {
  return (
    label
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, "_")
      .replace(/^_+|_+$/g, "")
  );
}

export function buildApiKey(object: string, slug: string): string {
  if (!slug) return "";
  return `${object}.cf_${slug}`;
}

export function buildDdlPreview(object: string, slug: string, type: string): string {
  return `ALTER ${object} ADD COLUMN cf_${slug} (${type}) · backfilled NULL · reversible`;
}

const STRUCTURAL_WORDS = ["object", "relationship", "link to", "lookup to"] as const;

export function detectStructuralWord(label: string): string | null {
  const lower = label.toLowerCase();
  for (const word of STRUCTURAL_WORDS) {
    if (lower.includes(word)) {
      return word;
    }
  }
  return null;
}

export function resolveMemberName(members: Member[], userId: string): string {
  const member = members.find((m) => m.user_id === userId);
  return member ? member.display_name : "Unknown";
}

export interface CustomFieldAuditEntry {
  id: string;
  actorId: string;
  label: string;
  type: string;
  object: string;
  occurredAt: string;
  auditRef: string;
  action: "added" | "retired";
}

export function deriveAuditEntries(fields: CustomField[]): CustomFieldAuditEntry[] {
  const entries: CustomFieldAuditEntry[] = [];

  for (const field of fields) {
    const idPrefix = field.id.slice(0, 8).replace(/-/g, "");

    entries.push({
      id: field.id,
      actorId: field.created_by,
      label: field.label,
      type: field.type,
      object: field.object,
      occurredAt: field.created_at,
      auditRef: `audit#${idPrefix}-created`,
      action: "added",
    });

    if (field.status === "retired") {
      entries.push({
        id: field.id,
        actorId: field.created_by,
        label: field.label,
        type: field.type,
        object: field.object,
        occurredAt: field.updated_at,
        auditRef: `audit#${idPrefix}-retired`,
        action: "retired",
      });
    }
  }

  return entries.sort((a, b) => new Date(b.occurredAt).getTime() - new Date(a.occurredAt).getTime());
}
