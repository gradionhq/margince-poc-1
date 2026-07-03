import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ContextMenu } from "./ContextMenu.js";

describe("ContextMenu", () => {
  it("opens, navigates with arrows, selects with Enter", () => {
    const onSelect = vi.fn();
    render(
      <ContextMenu
        trigger={<button type="button">open</button>}
        items={[
          { id: "a", label: "Alpha", onSelect },
          { id: "b", label: "Beta", onSelect: () => {} },
        ]}
      />,
    );
    fireEvent.click(screen.getByText("open"));
    const menu = screen.getByRole("menu");
    fireEvent.keyDown(menu, { key: "ArrowDown" });
    fireEvent.keyDown(menu, { key: "Enter" });
    expect(onSelect).toHaveBeenCalled();
  });
});
