import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

const mutateAsync = vi.fn().mockResolvedValue({ id: "d9" });
const employmentRelationships = [
  { person_id: "p1", kind: "employment" },
  { person_id: "p2", kind: "employment" },
];
vi.mock("../api/deals.js", () => ({
  useCreateDeal: () => ({ mutateAsync, isPending: false }),
  useOpenDealsForOrg: (orgId: string | undefined) => ({
    data: orgId ? { data: [{ id: "d-existing" }] } : undefined,
    isLoading: false,
  }),
  useRecentActivityCount: (orgId: string | undefined) => ({
    data: orgId ? 3 : undefined,
    isLoading: false,
  }),
  useOrgEmploymentRelationships: (orgId: string | undefined) => ({
    data: orgId ? { data: employmentRelationships } : undefined,
    isLoading: false,
  }),
}));
vi.mock("../../organizations/api/organizations.js", () => ({
  useOrganizations: () => ({
    data: {
      data: [
        { id: "o1", display_name: "Acme" },
        { id: "o2", display_name: "Globex" },
      ],
    },
    isLoading: false,
  }),
}));
vi.mock("../../identity/store/authStore.js", () => ({
  useAuthStore: () => ({ user: { id: "u1" } }),
}));
const relationshipPost = vi.fn().mockResolvedValue({ data: { id: "r1" } });
vi.mock("../../../lib/api-client/client.js", () => ({
  apiClient: {
    POST: (...args: unknown[]) => relationshipPost(...args),
  },
}));

import { NewDealModal } from "./NewDealModal.js";

describe("NewDealModal — no organizationId prop (board entry point)", () => {
  it("renders an org picker first; picking an org reveals the form + duplicate warning", async () => {
    render(
      <NewDealModal
        open={true}
        onClose={vi.fn()}
        defaultPipelineId="p1"
        defaultStageId="s0"
        onCreated={vi.fn()}
      />,
    );
    expect(
      screen.queryByRole("button", { name: /confirm & create/i }),
    ).not.toBeInTheDocument();
    await userEvent.click(screen.getByText("Acme"));
    expect(screen.getByText(/already has an open deal/i)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /confirm & create/i }),
    ).toBeInTheDocument();
  });
});

describe("NewDealModal — organizationId prop already provided (pre-linked context)", () => {
  it("skips the picker and shows the form + duplicate warning immediately", () => {
    render(
      <NewDealModal
        open={true}
        onClose={vi.fn()}
        organizationId="o1"
        defaultPipelineId="p1"
        defaultStageId="s0"
        onCreated={vi.fn()}
      />,
    );
    expect(screen.getByText(/already has an open deal/i)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /confirm & create/i }),
    ).toBeInTheDocument();
  });

  it("Confirm & create posts the deal (org + pipeline + stage) and calls onCreated", async () => {
    const onCreated = vi.fn();
    render(
      <NewDealModal
        open={true}
        onClose={vi.fn()}
        organizationId="o1"
        defaultPipelineId="p1"
        defaultStageId="s0"
        onCreated={onCreated}
      />,
    );
    await userEvent.click(
      screen.getByRole("button", { name: /confirm & create/i }),
    );
    expect(mutateAsync).toHaveBeenCalledWith(
      expect.objectContaining({
        organization_id: "o1",
        pipeline_id: "p1",
        stage_id: "s0",
      }),
    );
    expect(onCreated).toHaveBeenCalled();
  });

  it("shows an honest pre-attach stakeholder count from the org's employment relationships (AC-pipeline-9/10)", () => {
    render(
      <NewDealModal
        open={true}
        onClose={vi.fn()}
        organizationId="o1"
        defaultPipelineId="p1"
        defaultStageId="s0"
        onCreated={vi.fn()}
      />,
    );
    expect(
      screen.getByText(/2 stakeholders will be pre-attached/i),
    ).toBeInTheDocument();
  });

  it("Confirm & create pre-attaches one deal_stakeholder relationship per employment relationship's person_id", async () => {
    relationshipPost.mockClear();
    mutateAsync.mockResolvedValueOnce({ id: "d9" });
    render(
      <NewDealModal
        open={true}
        onClose={vi.fn()}
        organizationId="o1"
        defaultPipelineId="p1"
        defaultStageId="s0"
        onCreated={vi.fn()}
      />,
    );
    await userEvent.click(
      screen.getByRole("button", { name: /confirm & create/i }),
    );
    expect(relationshipPost).toHaveBeenCalledTimes(2);
    expect(relationshipPost).toHaveBeenCalledWith(
      "/relationships",
      expect.objectContaining({
        body: expect.objectContaining({
          kind: "deal_stakeholder",
          deal_id: "d9",
          person_id: "p1",
        }),
      }),
    );
    expect(relationshipPost).toHaveBeenCalledWith(
      "/relationships",
      expect.objectContaining({
        body: expect.objectContaining({
          kind: "deal_stakeholder",
          deal_id: "d9",
          person_id: "p2",
        }),
      }),
    );
  });

  it("shows an honest error and keeps the modal open when the deal create fails (dropped-error-path regression)", async () => {
    const onCreated = vi.fn();
    mutateAsync.mockRejectedValueOnce({
      code: "validation_error",
      detail: "name is required",
    });
    render(
      <NewDealModal
        open={true}
        onClose={vi.fn()}
        organizationId="o1"
        defaultPipelineId="p1"
        defaultStageId="s0"
        onCreated={onCreated}
      />,
    );
    await userEvent.click(
      screen.getByRole("button", { name: /confirm & create/i }),
    );
    expect(screen.getByText(/name is required/i)).toBeInTheDocument();
    expect(onCreated).not.toHaveBeenCalled();
    // The modal stays open (Confirm & create is still present) so the rep can retry
    // without losing their input.
    expect(
      screen.getByRole("button", { name: /confirm & create/i }),
    ).toBeInTheDocument();
  });

  it("shows an honest error when the deal creates but a stakeholder pre-attach fails, without calling onCreated", async () => {
    const onCreated = vi.fn();
    mutateAsync.mockResolvedValueOnce({ id: "d9" });
    relationshipPost.mockRejectedValueOnce(new Error("network down"));
    render(
      <NewDealModal
        open={true}
        onClose={vi.fn()}
        organizationId="o1"
        defaultPipelineId="p1"
        defaultStageId="s0"
        onCreated={onCreated}
      />,
    );
    await userEvent.click(
      screen.getByRole("button", { name: /confirm & create/i }),
    );
    expect(screen.getByText(/deal was created, but/i)).toBeInTheDocument();
    expect(onCreated).not.toHaveBeenCalled();
  });

  it("Cancel makes no server call and changes nothing", async () => {
    const onClose = vi.fn();
    render(
      <NewDealModal
        open={true}
        onClose={onClose}
        organizationId="o1"
        defaultPipelineId="p1"
        defaultStageId="s0"
        onCreated={vi.fn()}
      />,
    );
    mutateAsync.mockClear();
    await userEvent.click(screen.getByRole("button", { name: /cancel/i }));
    expect(mutateAsync).not.toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
  });
});
