import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";
import { ArchivedBanner, restoreErrorMessage } from "./ArchivedBanner.js";

describe("restoreErrorMessage", () => {
  it("surfaces the existing-record id on a 409 dedupe refusal", () => {
    const result = restoreErrorMessage({
      status: 409,
      code: "duplicate_email",
      detail: "A live person already has this email.",
      details: { existing_id: "p-existing-1" },
    });
    expect(result.existingId).toBe("p-existing-1");
    expect(result.message).toBe("A live person already has this email.");
  });

  it("falls back to a generic message with no details", () => {
    const result = restoreErrorMessage({ status: 422, code: "already_live" });
    expect(result.existingId).toBeUndefined();
    expect(result.message).toBe("Restore failed (already_live).");
  });

  it("falls back further with no code at all", () => {
    expect(restoreErrorMessage(new Error("network")).message).toBe(
      "Restore failed — please try again.",
    );
  });
});

describe("ArchivedBanner", () => {
  it("renders the honest archived message and a Restore action", () => {
    render(
      <ArchivedBanner
        entityLabel="contact"
        onRestore={vi.fn()}
        isRestoring={false}
      />,
    );
    expect(screen.getByText("This contact is archived.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Restore" })).toBeInTheDocument();
  });

  it("calls onRestore when Restore is clicked", () => {
    const onRestore = vi.fn();
    render(
      <ArchivedBanner
        entityLabel="contact"
        onRestore={onRestore}
        isRestoring={false}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Restore" }));
    expect(onRestore).toHaveBeenCalledOnce();
  });

  it("renders the existing-record pointer instead of Restore on a 409 refusal", () => {
    render(
      <MemoryRouter>
        <ArchivedBanner
          entityLabel="contact"
          onRestore={vi.fn()}
          isRestoring={false}
          existingRecordId="p-existing-1"
          existingRecordHref="/people/p-existing-1"
        />
      </MemoryRouter>,
    );
    expect(
      screen.getByRole("link", { name: /already live as a different record/i }),
    ).toHaveAttribute("href", "/people/p-existing-1");
    expect(screen.queryByRole("button", { name: "Restore" })).not.toBeInTheDocument();
  });
});
