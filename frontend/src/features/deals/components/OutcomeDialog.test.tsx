import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { OutcomeDialog } from "./OutcomeDialog.js";

describe("OutcomeDialog", () => {
  it("OK records Closed Won", async () => {
    const onWon = vi.fn();
    render(
      <OutcomeDialog
        open={true}
        dealName="Acme deal"
        onWon={onWon}
        onLost={vi.fn()}
        onCancel={vi.fn()}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: /^won$/i }));
    expect(onWon).toHaveBeenCalled();
  });

  it("Cancel opens the lost-reason prompt; non-blank submit records Closed Lost with the reason", async () => {
    const onLost = vi.fn();
    render(
      <OutcomeDialog
        open={true}
        dealName="Acme deal"
        onWon={vi.fn()}
        onLost={onLost}
        onCancel={vi.fn()}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: /cancel/i }));
    const input = screen.getByPlaceholderText(/reason/i);
    await userEvent.type(input, "Budget cut");
    await userEvent.click(screen.getByRole("button", { name: /confirm lost/i }));
    expect(onLost).toHaveBeenCalledWith("Budget cut");
  });

  it("a blank lost-reason submit keeps the deal open — nothing changes", async () => {
    const onLost = vi.fn();
    const onCancel = vi.fn();
    render(
      <OutcomeDialog
        open={true}
        dealName="Acme deal"
        onWon={vi.fn()}
        onLost={onLost}
        onCancel={onCancel}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: /cancel/i }));
    await userEvent.click(screen.getByRole("button", { name: /confirm lost/i }));
    expect(onLost).not.toHaveBeenCalled();
    expect(onCancel).toHaveBeenCalled();
  });
});
