import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { Organization } from "../../../lib/api-client/generated/index.js";
import { EditOrgModal } from "./EditOrgModal.js";

vi.mock("../api/organizations.js", () => ({
  useUpdateOrganization: () => ({
    mutate: (_patch: unknown, opts?: { onSuccess?: () => void }) =>
      opts?.onSuccess?.(),
    isPending: false,
  }),
}));

const org: Organization = {
  id: "org1",
  workspace_id: "w1",
  display_name: "Acme",
  industry: "Software",
  size_band: "51-200",
  address: { city: "Berlin", country: "DE" },
  version: 3,
  source: "manual",
  captured_by: "human:u1",
  created_at: "",
  updated_at: "",
};

describe("EditOrgModal", () => {
  it("saving with a changed field calls onSaved with that field's name (AC-company-12)", () => {
    const onSaved = vi.fn();
    render(<EditOrgModal open onClose={vi.fn()} org={org} onSaved={onSaved} />);
    const industryInput = screen.getByDisplayValue("Software");
    fireEvent.change(industryInput, { target: { value: "Fintech" } });
    fireEvent.click(screen.getByRole("button", { name: /save/i }));
    expect(onSaved).toHaveBeenCalledWith(expect.arrayContaining(["industry"]));
  });

  it("saving with nothing changed calls onSaved with an empty list", () => {
    const onSaved = vi.fn();
    render(<EditOrgModal open onClose={vi.fn()} org={org} onSaved={onSaved} />);
    fireEvent.click(screen.getByRole("button", { name: /save/i }));
    expect(onSaved).toHaveBeenCalledWith([]);
  });
});
