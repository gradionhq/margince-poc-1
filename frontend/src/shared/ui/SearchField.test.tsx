import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { SearchField } from "./SearchField.js";

describe("SearchField", () => {
  it("calls onChange on input", () => {
    const onChange = vi.fn();
    render(<SearchField value="" onChange={onChange} placeholder="Search" />);
    fireEvent.change(screen.getByPlaceholderText("Search"), {
      target: { value: "ab" },
    });
    expect(onChange).toHaveBeenCalledWith("ab");
  });
  it("shows a clear affordance only when value is non-empty and clears", () => {
    const onChange = vi.fn();
    const onClear = vi.fn();
    const { rerender } = render(
      <SearchField value="" onChange={onChange} onClear={onClear} />,
    );
    expect(screen.queryByRole("button", { name: /clear/i })).toBeNull();
    rerender(<SearchField value="x" onChange={onChange} onClear={onClear} />);
    fireEvent.click(screen.getByRole("button", { name: /clear/i }));
    expect(onChange).toHaveBeenCalledWith("");
    expect(onClear).toHaveBeenCalled();
  });
  it("renders real glyphs (not the icon-fallback) for the search and clear icons", () => {
    render(<SearchField value="x" onChange={vi.fn()} placeholder="Search" />);
    // A non-empty value renders both the leading search icon (via TextInput)
    // and the clear button's X icon. Both go through Forge's iconMap, whose
    // keys are PascalCase — a casing typo would render data-testid="icon-fallback".
    expect(screen.queryByTestId("icon-fallback")).toBeNull();
    // Both icons resolve, so two real glyphs are present.
    expect(screen.getAllByTestId("icon")).toHaveLength(2);
  });
});
