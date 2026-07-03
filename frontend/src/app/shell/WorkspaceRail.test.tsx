import { render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { RAIL_NAV } from "./railNav.js";
import { WorkspaceRail } from "./WorkspaceRail.js";

function renderRail(props: Parameters<typeof WorkspaceRail>[0] = {}) {
  return render(
    <MemoryRouter>
      <WorkspaceRail {...props} />
    </MemoryRouter>,
  );
}

const NON_ADMIN_NAV = RAIL_NAV.filter((i) => !i.adminOnly);

describe("WorkspaceRail", () => {
  it("renders the canonical nav items in order with their Lucide icon names", () => {
    expect(NON_ADMIN_NAV.map((i) => i.id)).toEqual([
      "home",
      "contacts",
      "companies",
      "leads",
      "deals",
      "tasks",
      "inbox",
      "reports",
      "ask-ai",
    ]);
    expect(NON_ADMIN_NAV.map((i) => i.icon)).toEqual([
      "Home",
      "Users",
      "Building2",
      "UserPlus",
      "Target",
      "CheckSquare",
      "Inbox",
      "BarChart3",
      "Sparkles",
    ]);
  });

  it("renders 9 non-admin items in DOM order and no icon-fallback boxes for rep", () => {
    renderRail();
    const items = screen.getAllByTestId("rail-nav-item");
    expect(items).toHaveLength(9);
    expect(items.map((el) => el.getAttribute("data-nav-id"))).toEqual(
      NON_ADMIN_NAV.map((i) => i.id),
    );
    expect(screen.queryByTestId("icon-fallback")).toBeNull();
  });

  it("renders the M mark (-> /home) at top and avatar (-> /settings) at bottom", () => {
    renderRail({ userName: "Ada Lovelace" });
    expect(screen.getByTestId("rail-mark").getAttribute("href")).toBe("/home");
    expect(screen.getByTestId("rail-avatar").getAttribute("href")).toBe(
      "/settings",
    );
  });

  it("uses the authenticated user's name for the avatar (no fabricated identity)", () => {
    renderRail({ userName: "Ada Lovelace" });
    // Avatar derives its glyph from the real name's initial, not a placeholder "Y" for "You".
    const avatar = screen.getByTestId("rail-avatar");
    expect(avatar).toHaveTextContent("A");
    expect(avatar).not.toHaveTextContent("Y");
  });

  it("omits the avatar entirely when no user name is known", () => {
    renderRail();
    expect(screen.queryByTestId("rail-avatar")).toBeNull();
  });

  it("renders count badges on Tasks and Inbox when count > 0, and not when 0/absent", () => {
    renderRail({ counts: { tasks: 3, inbox: 0 } });
    const tasks = screen.getByTestId("rail-nav-item-tasks");
    const inbox = screen.getByTestId("rail-nav-item-inbox");
    expect(within(tasks).getByTestId("rail-badge").textContent).toBe("3");
    expect(within(inbox).queryByTestId("rail-badge")).toBeNull();
  });

  it("admin sees the Members nav item", () => {
    renderRail({ isAdmin: true });
    expect(screen.getByTestId("rail-nav-item-members")).toBeInTheDocument();
    expect(
      screen.getByTestId("rail-nav-item-members").querySelector("a"),
    ).toHaveAttribute("href", "/admin/members");
  });

  it("rep does not see the Members nav item", () => {
    renderRail({ isAdmin: false });
    expect(screen.queryByTestId("rail-nav-item-members")).toBeNull();
  });
});
