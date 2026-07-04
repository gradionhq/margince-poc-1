import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

vi.mock("../../people/api/people.js", () => ({
  usePerson: (id: string) => ({
    data:
      id === "p1"
        ? { id: "p1", full_name: "Dana Lee", title: "VP Sales" }
        : { id: "p2", full_name: "Sam Ito", title: "Blocker Bob" },
    isLoading: false,
    isError: false,
  }),
}));

import { roleBadge, StakeholdersRail } from "./StakeholdersRail.js";

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient();
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("roleBadge", () => {
  it("maps champion/economic_buyer/influencer/user/blocker to the AC-deal-7 vocabulary", () => {
    expect(roleBadge("champion").label).toBe("Champion");
    expect(roleBadge("economic_buyer").label).toBe("Stakeholder");
    expect(roleBadge("influencer").label).toBe("Stakeholder");
    expect(roleBadge("user").label).toBe("Stakeholder");
    expect(roleBadge("blocker").label).toBe("Blocker");
  });
});

describe("StakeholdersRail", () => {
  const stakeholders = [
    { id: "r1", person_id: "p1", role: "economic_buyer" },
    { id: "r2", person_id: "p2", role: "blocker" },
  ] as never[];

  it("lists each stakeholder with name/title + role badge, multi-threaded framing", () => {
    render(
      <StakeholdersRail stakeholders={stakeholders} stakeholderCount={2} />,
      {
        wrapper,
      },
    );
    expect(screen.getByText("Dana Lee")).toBeInTheDocument();
    expect(screen.getByText("VP Sales")).toBeInTheDocument();
    expect(screen.getByText("Sam Ito")).toBeInTheDocument();
    expect(screen.getByText("Multi-threaded")).toBeInTheDocument();
    expect(
      screen.queryByText("No economic buyer identified yet"),
    ).not.toBeInTheDocument();
  });

  it("shows the No-economic-buyer notice when no economic_buyer role exists", () => {
    render(
      <StakeholdersRail
        stakeholders={
          [{ id: "r2", person_id: "p2", role: "blocker" }] as never[]
        }
        stakeholderCount={1}
      />,
      { wrapper },
    );
    expect(
      screen.getByText("No economic buyer identified yet"),
    ).toBeInTheDocument();
  });

  it("shows single-threaded risk framing when stakeholderCount is 1", () => {
    render(
      <StakeholdersRail
        stakeholders={
          [{ id: "r2", person_id: "p2", role: "champion" }] as never[]
        }
        stakeholderCount={1}
      />,
      { wrapper },
    );
    expect(screen.getByText(/single-threaded/i)).toBeInTheDocument();
  });

  it("honest-empty when there are no stakeholders — never claims single-threaded on zero", () => {
    render(<StakeholdersRail stakeholders={[]} stakeholderCount={0} />, {
      wrapper,
    });
    expect(screen.getByText(/no stakeholders captured/i)).toBeInTheDocument();
    expect(screen.queryByText(/single-threaded/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/multi-threaded/i)).not.toBeInTheDocument();
  });
});
