import { Avatar } from "../../../shared/ui/forge.js";

// logo_object_key does not exist yet (S-E02.8) — always takes the monogram path today.
export function OrgLogo({
  name,
  size = "md",
}: {
  name: string;
  size?: "sm" | "md" | "lg";
}) {
  return <Avatar name={name} size={size} />;
}
