import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import {
  firstOpenStageId,
  ReopenConfirmDialog,
} from "./ReopenConfirmDialog.js";

describe("firstOpenStageId", () => {
  it("returns the lowest-position open stage, ignoring terminal stages", () => {
    const stages = [
      { id: "won", position: 2, semantic: "won" },
      { id: "s2", position: 1, semantic: "open" },
      { id: "s1", position: 0, semantic: "open" },
    ] as never[];
    expect(firstOpenStageId(stages)).toBe("s1");
  });

  it("returns undefined when there are no open stages", () => {
    expect(
      firstOpenStageId([
        { id: "won", position: 0, semantic: "won" },
      ] as never[]),
    ).toBeUndefined();
  });
});

describe("ReopenConfirmDialog", () => {
  it("confirms reopening", async () => {
    const onConfirm = vi.fn();
    render(
      <ReopenConfirmDialog
        open={true}
        dealName="Acme deal"
        onConfirm={onConfirm}
        onCancel={vi.fn()}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: /confirm/i }));
    expect(onConfirm).toHaveBeenCalled();
  });
});
