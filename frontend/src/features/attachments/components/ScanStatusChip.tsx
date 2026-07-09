import { Chip, StatusDot } from "../../../shared/ui/forge.js";

const STATUS_META = {
  scanning: {
    label: "Scanning…",
    variant: "neutral",
    dot: "running",
  },
  clean: {
    label: "Clean",
    variant: "success",
    dot: "success",
  },
  blocked: {
    label: "Blocked",
    variant: "danger",
    dot: "error",
  },
} as const;

type ScanStatus = keyof typeof STATUS_META;

export function ScanStatusChip({ scanStatus }: { scanStatus: ScanStatus }) {
  const meta = STATUS_META[scanStatus];

  return (
    <Chip variant={meta.variant}>
      <span className="inline-flex items-center gap-1.5">
        <StatusDot state={meta.dot} ariaLabel={meta.label} />
        <span>{meta.label}</span>
      </span>
    </Chip>
  );
}
