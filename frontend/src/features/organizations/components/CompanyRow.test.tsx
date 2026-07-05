import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { CompanyRow } from "./CompanyRow.js";

describe("CompanyRow", () => {
  it("renders an Archive row action that calls onArchive with the org id", () => {
    const onArchive = vi.fn();
    render(
      <table>
        <tbody>
          <CompanyRow
            org={{
              id: "org1",
              display_name: "Acme Inc",
              industry: "Software",
              contact_count: 4,
              open_deal_count: 2,
              org_strength: null,
            } as never}
            onClick={vi.fn()}
            onArchive={onArchive}
          />
        </tbody>
      </table>,
    );

    fireEvent.click(screen.getByRole("button", { name: /row actions/i }));
    fireEvent.click(
      within(screen.getByRole("menu")).getByRole("menuitem", {
        name: "Archive",
      }),
    );

    expect(onArchive).toHaveBeenCalledOnce();
    expect(onArchive).toHaveBeenCalledWith("org1");
  });
});
