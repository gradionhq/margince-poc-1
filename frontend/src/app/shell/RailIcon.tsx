import { Building2, CheckSquare, type LucideProps, Target } from "lucide-react";
import type { ComponentType } from "react";
import { Icon } from "../../shared/ui/forge.js";

// Lucide icons the shell needs that Forge's iconMap does NOT register.
// Forge `Icon` would render its empty-box fallback for these, so resolve
// them directly from lucide-react. PascalCase only — same vocabulary as Forge.
const extraIcons: Record<string, ComponentType<LucideProps>> = {
  Building2,
  Target,
  CheckSquare,
};

export interface RailIconProps {
  name: string;
  size?: number;
  className?: string;
}

export function RailIcon({ name, size = 20, className = "" }: RailIconProps) {
  const Extra = extraIcons[name];
  if (Extra) {
    return <Extra size={size} className={className} data-testid="icon" />;
  }
  return <Icon name={name} size={size} className={className} />;
}
