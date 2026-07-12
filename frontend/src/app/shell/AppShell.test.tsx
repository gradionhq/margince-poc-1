import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { AppShell } from "./AppShell.js";

function renderAt(path: string, props = {}) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route element={<AppShell {...props} />}>
          <Route path="/home" element={<div>home body</div>} />
          <Route path="/deals" element={<div>deals body</div>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  );
}

describe("AppShell", () => {
  it("renders the rail, top bar, and routed outlet body", () => {
    renderAt("/home");
    expect(screen.getByTestId("workspace-rail")).not.toBeNull();
    expect(screen.getByTestId("top-bar")).not.toBeNull();
    expect(screen.getByText("home body")).not.toBeNull();
  });

  it("marks exactly one rail item active for the current route", () => {
    renderAt("/deals");
    const active = screen
      .getAllByTestId("rail-nav-item")
      .filter((el) => el.getAttribute("data-active") === "true");
    expect(active).toHaveLength(1);
    expect(active[0].getAttribute("data-nav-id")).toBe("deals");
  });

  it("renders an empty contextual action area at cold start", () => {
    renderAt("/home");
    expect(screen.getByTestId("top-bar-actions").children).toHaveLength(0);
  });

  it("skip link is first focusable, wired to main, and sr-only by default", () => {
    renderAt("/home");

    // (a) skip link present
    const link = screen.getByRole("link", { name: /skip to main content/i });
    expect(link).not.toBeNull();

    // (b) first focusable in DOM order (before rail items)
    const { container } = renderAt("/home");
    const focusable = container.querySelectorAll("a[href], button, [tabindex]");
    expect(focusable[0]).toBe(
      container.querySelector('a[href="#main-content"]'),
    );

    // (c) href wires to main's id
    expect(link.getAttribute("href")).toBe("#main-content");
    const main = document.querySelector("main");
    expect(main?.id).toBe("main-content");
    expect(main?.getAttribute("tabindex")).toBe("-1");

    // (d) sr-only by default (hidden until focused)
    expect(link.className).toContain("sr-only");
  });
});
