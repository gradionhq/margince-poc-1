const CONNECTOR_PREFIXES = ["email:", "calendar:", "connector:"];

export function classifySource(
  source: string,
  capturedBy: string,
): "connector" | "typed-by-you" | "other" {
  if (capturedBy.startsWith("agent:") || capturedBy.startsWith("connector:"))
    return "connector";
  if (CONNECTOR_PREFIXES.some((p) => source.startsWith(p))) return "connector";
  if (capturedBy.startsWith("human:")) return "typed-by-you";
  return "other";
}

export function SourceChip({
  source,
  capturedBy,
}: {
  source: string;
  capturedBy: string;
}) {
  const kind = classifySource(source, capturedBy);
  const label =
    kind === "connector"
      ? "connector"
      : kind === "typed-by-you"
        ? "typed by you"
        : source;
  return (
    <span className="inline-flex items-center px-gf-sm py-gf-xs rounded-full text-gf-caption font-medium bg-gf-card border border-gf-subtle text-gf-secondary">
      {label}
    </span>
  );
}
