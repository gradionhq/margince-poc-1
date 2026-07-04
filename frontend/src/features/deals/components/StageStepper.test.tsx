import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { StageStepper } from "./StageStepper.js";

const stages = [
  { id: "s1", name: "New", position: 0, semantic: "open", win_probability: 10 },
  {
    id: "s2",
    name: "Discovery",
    position: 1,
    semantic: "open",
    win_probability: 40,
  },
  {
    id: "s3",
    name: "Proposal",
    position: 2,
    semantic: "open",
    win_probability: 60,
  },
] as const;
const terminal = [
  {
    id: "won",
    name: "Closed Won",
    position: 100,
    semantic: "won",
    win_probability: 100,
  },
  {
    id: "lost",
    name: "Closed Lost",
    position: 101,
    semantic: "lost",
    win_probability: 0,
  },
] as const;

describe("StageStepper", () => {
  it("marks prior stages done, current stage highlighted, terminal nodes muted when open", () => {
    render(
      <StageStepper
        stages={[...stages, ...terminal]}
        currentStageId="s2"
        dealStatus="open"
      />,
    );
    expect(screen.getByTestId("stage-node-s1")).toHaveAttribute(
      "data-stage-state",
      "done",
    );
    expect(screen.getByTestId("stage-node-s2")).toHaveAttribute(
      "data-stage-state",
      "current",
    );
    expect(screen.getByTestId("stage-node-s3")).toHaveAttribute(
      "data-stage-state",
      "upcoming",
    );
    expect(screen.getByTestId("stage-node-won")).toHaveAttribute(
      "data-stage-state",
      "muted",
    );
    expect(screen.getByTestId("stage-node-lost")).toHaveAttribute(
      "data-stage-state",
      "muted",
    );
  });

  it("highlights Closed Won as current+active when the deal is actually won", () => {
    render(
      <StageStepper
        stages={[...stages, ...terminal]}
        currentStageId="won"
        dealStatus="won"
      />,
    );
    expect(screen.getByTestId("stage-node-won")).toHaveAttribute(
      "data-stage-state",
      "current",
    );
    expect(screen.getByTestId("stage-node-lost")).toHaveAttribute(
      "data-stage-state",
      "muted",
    );
  });
});
