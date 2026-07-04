import { useState } from "react";
import type { Person } from "../../../lib/api-client/generated/index.js";
import { Button, TextInput, Tooltip } from "../../../shared/ui/forge.js";
import { useOrganizationName, useUpdatePerson } from "../api/person.js";
import { SourceChip } from "./SourceChip.js";

function formatCaptureDate(iso: string | undefined): string {
  if (!iso) return "unknown date";
  return new Date(iso).toISOString().slice(0, 10);
}

export function PersonHeader({ person }: { person: Person }) {
  const [editing, setEditing] = useState(false);
  const [fullName, setFullName] = useState(person.full_name);
  const [title, setTitle] = useState(person.title ?? "");

  const primaryEmployment = (person.relationships ?? []).find(
    (r) => r.kind === "employment" && r.is_current_primary,
  );
  const { data: companyName } = useOrganizationName(
    primaryEmployment?.organization_id ?? undefined,
  );
  const { mutate: save, isPending } = useUpdatePerson(person.id);

  const primaryEmail = person.emails?.find((e) => e.is_primary) ?? person.emails?.[0];
  const primaryPhone = person.phones?.find((p) => p.is_primary) ?? person.phones?.[0];

  function handleSave() {
    save(
      {
        body: { full_name: fullName, title: title || null },
        ifMatch: person.version !== undefined ? String(person.version) : undefined,
      },
      { onSuccess: () => setEditing(false) },
    );
  }

  return (
    <header className="flex flex-col gap-gf-sm border-b border-gf-subtle pb-gf-md">
      <div className="flex items-start justify-between">
        <div className="flex flex-col gap-gf-xs">
          {editing ? (
            <TextInput value={fullName} onChange={setFullName} placeholder="Full name" />
          ) : (
            <h1 className="text-gf-title font-semibold text-gf-primary">{person.full_name}</h1>
          )}
          <div className="flex items-center gap-gf-xs text-gf-body text-gf-secondary">
            {editing ? (
              <TextInput value={title} onChange={setTitle} placeholder="Title" />
            ) : (
              person.title && <span>{person.title}</span>
            )}
            {primaryEmployment && (
              <>
                <span>·</span>
                <a
                  data-testid="company-link"
                  href={`/companies/${primaryEmployment.organization_id}`}
                  className="text-gf-accent hover:underline"
                >
                  {companyName ?? "this company"}
                </a>
              </>
            )}
            <SourceChip source={person.source} capturedBy={person.captured_by} />
            <span className="text-gf-label text-gf-secondary">
              {formatCaptureDate(person.created_at)}
            </span>
          </div>
        </div>
        <div className="flex items-center gap-gf-sm">
          {editing ? (
            <>
              <Button variant="ghost" size="sm" onClick={() => setEditing(false)}>
                Cancel
              </Button>
              <Button size="sm" onClick={handleSave} disabled={isPending}>
                Save
              </Button>
            </>
          ) : (
            <Button variant="ghost" size="sm" onClick={() => setEditing(true)}>
              Edit
            </Button>
          )}
          <Tooltip content="Email drafting ships in a later chapter">
            <span>
              <Button variant="ghost" size="sm" disabled>
                Draft email
              </Button>
            </span>
          </Tooltip>
          <span className="text-gf-label text-gf-secondary italic">
            Email drafting ships in a later chapter
          </span>
        </div>
      </div>
      <div className="flex flex-col gap-gf-xs">
        {primaryEmail && (
          <div className="flex items-center gap-gf-xs text-gf-body font-mono text-gf-primary">
            <span>{primaryEmail.email}</span>
            <SourceChip source={primaryEmail.source} capturedBy={primaryEmail.captured_by} />
            <span className="text-gf-label text-gf-secondary">
              {formatCaptureDate(primaryEmail.created_at)}
            </span>
          </div>
        )}
        {primaryPhone && (
          <div className="flex items-center gap-gf-xs text-gf-body font-mono text-gf-primary">
            <span>{primaryPhone.phone}</span>
            <SourceChip source={primaryPhone.source} capturedBy={primaryPhone.captured_by} />
            <span className="text-gf-label text-gf-secondary">
              {formatCaptureDate(primaryPhone.created_at)}
            </span>
          </div>
        )}
      </div>
    </header>
  );
}
