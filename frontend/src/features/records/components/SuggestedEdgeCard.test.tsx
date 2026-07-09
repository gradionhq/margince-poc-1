import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { Organization } from "../../../lib/api-client/generated/index.js";
import { SuggestedEdgeCard } from "./SuggestedEdgeCard.js";

const candidate: Organization = {
  id: "org-cand",
  workspace_id: "ws-1",
  display_name: "Acme Subsidiary",
  source: "test",
  captured_by: "human:test",
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
  parent_org_id: null,
  domains: [],
};

describe("SuggestedEdgeCard", () => {
  it("renders 'Add {candidate.display_name} as a child' with Accept/Dismiss buttons", () => {
    render(
      <SuggestedEdgeCard
        candidate={candidate}
        parentId="org-root"
        status="staged"
        onAccept={vi.fn()}
        onDismiss={vi.fn()}
      />,
    );
    expect(screen.getByText(/Acme Subsidiary/)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /accept edge/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /dismiss/i }),
    ).toBeInTheDocument();
  });

  it("clicking neither button fires no network call on render (AC-6)", () => {
    const onAccept = vi.fn();
    const onDismiss = vi.fn();
    render(
      <SuggestedEdgeCard
        candidate={candidate}
        parentId="org-root"
        status="staged"
        onAccept={onAccept}
        onDismiss={onDismiss}
      />,
    );
    // No action taken — verify callbacks are not called on render alone
    expect(onAccept).not.toHaveBeenCalled();
    expect(onDismiss).not.toHaveBeenCalled();
  });

  it("clicking 'Accept edge' calls onAccept callback", async () => {
    const onAccept = vi.fn();
    render(
      <SuggestedEdgeCard
        candidate={candidate}
        parentId="org-root"
        status="staged"
        onAccept={onAccept}
        onDismiss={vi.fn()}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: /accept edge/i }));
    expect(onAccept).toHaveBeenCalledOnce();
  });

  it("shows 'edge written · audited' and no buttons when status is accepted", () => {
    render(
      <SuggestedEdgeCard
        candidate={candidate}
        parentId="org-root"
        status="accepted"
        onAccept={vi.fn()}
        onDismiss={vi.fn()}
      />,
    );
    expect(screen.getByText(/edge written · audited/i)).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /accept edge/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /dismiss/i }),
    ).not.toBeInTheDocument();
  });

  it("clicking 'Dismiss' calls onDismiss and fires no network call (AC-7)", async () => {
    const onDismiss = vi.fn();
    const onAccept = vi.fn();
    render(
      <SuggestedEdgeCard
        candidate={candidate}
        parentId="org-root"
        status="staged"
        onAccept={onAccept}
        onDismiss={onDismiss}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: /dismiss/i }));
    expect(onDismiss).toHaveBeenCalledOnce();
    // onAccept was never called (no network mutation)
    expect(onAccept).not.toHaveBeenCalled();
  });
});
