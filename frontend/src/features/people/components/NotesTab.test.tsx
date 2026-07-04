import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { NotesTab } from "./NotesTab.js";

describe("NotesTab", () => {
  it("renders the honest empty state — no notes model exists yet (STATE-1)", () => {
    render(<NotesTab />);
    expect(screen.getByText(/no notes yet/i)).toBeInTheDocument();
  });

  it("disables Save for an empty draft and enables it once text is typed", async () => {
    const { default: userEvent } = await import("@testing-library/user-event");
    const user = userEvent.setup();
    render(<NotesTab />);
    const saveBtn = screen.getByRole("button", { name: /save/i });
    expect(saveBtn).toBeDisabled();
    await user.type(
      screen.getByLabelText(/will be typed-by-you/i),
      "Follow up next week",
    );
    expect(saveBtn).toBeEnabled();
  });

  it("saving marks the note typed-by-you locally and flags the no-backend gap", async () => {
    const { default: userEvent } = await import("@testing-library/user-event");
    const user = userEvent.setup();
    render(<NotesTab />);
    await user.type(
      screen.getByLabelText(/will be typed-by-you/i),
      "Follow up next week",
    );
    await user.click(screen.getByRole("button", { name: /save/i }));
    expect(screen.getByText("Follow up next week")).toBeInTheDocument();
    expect(screen.getByText(/typed by you/i)).toBeInTheDocument();
    expect(
      screen.getByText(/not yet persisted to the backend/i),
    ).toBeInTheDocument();
  });
});
