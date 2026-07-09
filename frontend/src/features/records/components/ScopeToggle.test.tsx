import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ScopeToggle } from "./ScopeToggle.js";

describe("ScopeToggle", () => {
  it("reflects the scope prop as the active option", () => {
    render(<ScopeToggle scope="tree" onChange={vi.fn()} />);
    const treeOption = screen.getByRole("radio", { name: /whole tree/i });
    const selfOption = screen.getByRole("radio", {
      name: /this account only/i,
    });
    expect(treeOption).toBeChecked();
    expect(selfOption).not.toBeChecked();
  });

  it("clicking the inactive option calls onChange with the other scope", async () => {
    const onChange = vi.fn();
    render(<ScopeToggle scope="tree" onChange={onChange} />);
    await userEvent.click(
      screen.getByRole("radio", { name: /this account only/i }),
    );
    expect(onChange).toHaveBeenCalledWith("self");
  });

  it("when scope is self, self is checked and tree is not", () => {
    render(<ScopeToggle scope="self" onChange={vi.fn()} />);
    expect(
      screen.getByRole("radio", { name: /this account only/i }),
    ).toBeChecked();
    expect(
      screen.getByRole("radio", { name: /whole tree/i }),
    ).not.toBeChecked();
  });
});
