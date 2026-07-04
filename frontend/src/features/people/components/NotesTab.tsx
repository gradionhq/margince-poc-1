import { useState } from "react";
import { Button } from "../../../shared/ui/forge.js";

interface LocalNote {
  id: string;
  text: string;
}

export function NotesTab() {
  const [draft, setDraft] = useState("");
  const [notes, setNotes] = useState<LocalNote[]>([]);

  function handleSave() {
    const text = draft.trim();
    if (!text) return;
    setNotes((prev) => [...prev, { id: crypto.randomUUID(), text }]);
    setDraft("");
  }

  return (
    <div className="flex flex-col gap-gf-sm">
      {notes.length === 0 ? (
        <p className="text-gf-body text-gf-secondary">No notes yet.</p>
      ) : (
        <ul className="flex flex-col gap-gf-xs">
          {notes.map((n) => (
            <li key={n.id} className="flex items-center gap-gf-xs border border-gf-subtle rounded-md p-gf-sm">
              <span className="text-gf-body text-gf-primary">{n.text}</span>
              <span className="inline-flex items-center px-gf-sm py-gf-xs rounded-full text-gf-caption font-medium bg-gf-accent-light text-gf-accent">
                typed by you
              </span>
            </li>
          ))}
        </ul>
      )}
      <label htmlFor="note-draft" className="text-gf-caption text-gf-secondary">
        will be typed-by-you
      </label>
      <textarea
        id="note-draft"
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        className="min-h-20 rounded-md border border-gf-subtle bg-gf-elevated text-gf-body text-gf-primary p-gf-sm"
      />
      <Button size="sm" onClick={handleSave} disabled={draft.trim().length === 0} className="self-start">
        Save
      </Button>
      <p className="text-gf-label text-gf-secondary italic">
        No notes-create endpoint exists in the contract yet — this is not yet persisted to the
        backend; it lives in local component state only.
      </p>
    </div>
  );
}
