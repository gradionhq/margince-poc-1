import { Avatar, PresenceIndicator } from "@shared/ui";

export function UserAvatar({
  name,
  src,
  presence,
  size = "md",
}: {
  name: string;
  src?: string;
  presence?: "online" | "away" | "offline";
  size?: "sm" | "md" | "lg";
}) {
  return (
    <div className="relative inline-flex shrink-0">
      <Avatar name={name} avatarUrl={src} size={size} />
      {presence && (
        <span className="absolute bottom-0 right-0">
          <PresenceIndicator status={presence} size="sm" />
        </span>
      )}
    </div>
  );
}
