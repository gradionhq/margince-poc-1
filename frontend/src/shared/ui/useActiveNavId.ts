import { useLocation } from "react-router-dom";
import { RAIL_NAV } from "../../app/shell/railNav.js";

// Longest-prefix match: a path activates the single nav item whose `to` is the
// longest prefix of the path. Guarantees at most one active id.
export function activeNavIdForPath(pathname: string): string | undefined {
  let best: { id: string; len: number } | undefined;
  for (const item of RAIL_NAV) {
    const matches = pathname === item.to || pathname.startsWith(`${item.to}/`);
    if (matches && (!best || item.to.length > best.len)) {
      best = { id: item.id, len: item.to.length };
    }
  }
  return best?.id;
}

export function useActiveNavId(): string | undefined {
  const { pathname } = useLocation();
  return activeNavIdForPath(pathname);
}
