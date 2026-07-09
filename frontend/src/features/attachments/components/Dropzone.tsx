import { useRef, useState } from "react";

export function Dropzone({
  onFilesSelected,
}: {
  onFilesSelected: (files: FileList) => void;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [isDragging, setIsDragging] = useState(false);

  function openPicker() {
    inputRef.current?.click();
  }

  function handleFiles(files: FileList | null) {
    if (!files || files.length === 0) return;
    onFilesSelected(files);
  }

  return (
    <fieldset
      aria-label="Upload files dropzone"
      data-testid="dropzone"
      data-dragging={isDragging ? "true" : "false"}
      className={`m-0 min-w-0 rounded-lg border border-dashed p-gf-lg transition-colors ${isDragging ? "border-gf-accent bg-gf-accent/10" : "border-gf-subtle bg-gf-card"}`}
      onDragOver={(event) => {
        event.preventDefault();
        setIsDragging(true);
      }}
      onDragEnter={(event) => {
        event.preventDefault();
        setIsDragging(true);
      }}
      onDragLeave={() => setIsDragging(false)}
      onDrop={(event) => {
        event.preventDefault();
        setIsDragging(false);
        handleFiles(event.dataTransfer.files);
      }}
    >
      <input
        ref={inputRef}
        type="file"
        multiple
        className="sr-only"
        aria-label="Upload files"
        onChange={(event) => handleFiles(event.currentTarget.files)}
      />
      <div className="flex flex-col items-start gap-gf-sm">
        <p className="text-gf-body font-medium text-gf-primary">
          Drop files here
        </p>
        <p className="text-gf-caption text-gf-secondary">
          Drag and drop, or browse to attach files.
        </p>
        <button
          type="button"
          aria-label="Browse files"
          className="rounded-md border border-gf-subtle bg-gf-elevated px-gf-sm py-gf-xs text-gf-caption font-medium text-gf-primary hover:bg-gf-card"
          onClick={(event) => {
            event.stopPropagation();
            openPicker();
          }}
        >
          Browse files
        </button>
      </div>
    </fieldset>
  );
}
