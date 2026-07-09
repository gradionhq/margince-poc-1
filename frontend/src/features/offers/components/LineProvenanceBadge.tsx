import { Chip, Icon } from "../../../shared/ui/forge.js";
import {
  classifySource,
  SourceChip,
} from "../../people/components/SourceChip.js";

export function LineProvenanceBadge({
  source,
  capturedBy,
  evidence,
}: {
  source: string;
  capturedBy: string;
  evidence: unknown | null;
}) {
  const kind = classifySource(source, capturedBy);

  if (evidence != null && capturedBy.startsWith("agent:")) {
    return (
      <Chip className="gap-gf-xs bg-gf-status-info/15 text-gf-status-info border-gf-status-info/30">
        <Icon name="Sparkles" size={12} />
        AI-proposed
      </Chip>
    );
  }

  if (kind === "typed-by-you") {
    return <Chip>typed by you</Chip>;
  }

  return <SourceChip source={source} capturedBy={capturedBy} />;
}
