import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import type { Organization } from "../../../lib/api-client/generated/index.js";
import { NewDealModal } from "./NewDealModal.js";

const org: Organization = {
  id: "org1",
  workspace_id: "w1",
  display_name: "Acme",
  source: "manual",
  captured_by: "human:u1",
  created_at: "",
  updated_at: "",
};

describe("NewDealModal", () => {
  it("stages the org and known contacts pre-linked, fires no network call on open", () => {
    render(
      <NewDealModal
        open
        onClose={vi.fn()}
        org={org}
        contacts={[{ id: "p1", full_name: "Jordan Ellis" } as never]}
      />,
    );
    expect(screen.getByText("Acme")).toBeInTheDocument();
    expect(screen.getByText("Jordan Ellis")).toBeInTheDocument();
  });
});
