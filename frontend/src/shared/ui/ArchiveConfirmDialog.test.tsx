import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ArchiveConfirmDialog } from "./ArchiveConfirmDialog.js";

describe("ArchiveConfirmDialog", () => {
  it("names the entity in the confirm copy and confirms on click", () => {
    const onConfirm = vi.fn();
    render(
      <ArchiveConfirmDialog
        open
        entityLabel="Alice Smith"
        onConfirm={onConfirm}
        onCancel={vi.fn()}
        isLoading={false}
      />,
    );
    expect(screen.getByText(/Alice Smith/)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Archive" }));
    expect(onConfirm).toHaveBeenCalledOnce();
  });
});
