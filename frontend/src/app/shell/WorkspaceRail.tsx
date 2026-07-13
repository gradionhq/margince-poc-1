import { Link } from "react-router-dom";
import { Avatar, RailIcon } from "../../shared/ui/forge.js";
import { RAIL_NAV, type RailCounts } from "./railNav.js";

export function WorkspaceRail({
  activeId,
  counts,
  isAdmin = false,
  userName,
}: {
  activeId?: string;
  counts?: RailCounts;
  isAdmin?: boolean;
  /** Authenticated user's display name; omit to suppress the avatar (evidence-or-omit). */
  userName?: string;
}) {
  const visibleNav = RAIL_NAV.filter((item) => !item.adminOnly || isAdmin);

  return (
    <nav
      aria-label="Workspace"
      data-testid="workspace-rail"
      className="flex h-full w-16 flex-col items-center justify-between bg-gf-rail py-gf-md"
    >
      {/* Margin-rule "M" mark -> home */}
      <Link
        to="/home"
        data-testid="rail-mark"
        aria-label="Margince home"
        className="flex h-10 w-10 items-center justify-center rounded-md font-display text-lg font-semibold text-gf-on-accent"
      >
        M
      </Link>

      <ul className="flex flex-1 flex-col items-center gap-gf-xs pt-gf-lg">
        {visibleNav.map((item) => {
          const active = item.id === activeId;
          const count = item.badgeKey ? counts?.[item.badgeKey] : undefined;
          const hasCount = typeof count === "number" && count > 0;
          const label = hasCount ? `${item.label}, ${count}` : item.label;
          return (
            <li
              key={item.id}
              data-testid={`rail-nav-item-${item.id}`}
              className="relative"
            >
              <Link
                to={item.to}
                aria-label={label}
                aria-current={active ? "page" : undefined}
                data-testid="rail-nav-item"
                data-nav-id={item.id}
                data-active={active}
                className={`relative flex h-10 w-10 items-center justify-center rounded-md ${
                  active
                    ? "bg-gf-on-accent/15 text-gf-on-accent"
                    : "text-gf-on-accent/70 hover:bg-gf-on-accent/10"
                }`}
              >
                <RailIcon name={item.icon} />
                {hasCount && (
                  <span
                    data-testid="rail-badge"
                    aria-hidden="true"
                    className="absolute -right-0.5 -top-0.5 flex min-w-gf-lg items-center justify-center rounded-full bg-gf-accent px-gf-xs text-gf-mini leading-4 text-gf-on-accent"
                  >
                    {count}
                  </span>
                )}
              </Link>
            </li>
          );
        })}
      </ul>

      {/* User avatar -> settings. Omit entirely if we don't know who is signed in
          (evidence-or-omit) rather than fabricating a placeholder identity. */}
      {userName && (
        <Link to="/settings" data-testid="rail-avatar" aria-label="Settings">
          <Avatar name={userName} size="sm" />
        </Link>
      )}
    </nav>
  );
}
