import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { LineProvenanceBadge } from "./LineProvenanceBadge.js";

describe("LineProvenanceBadge", () => {
  it("shows an AI-proposed badge when evidence is present on an agent-captured line", () => {
    render(
      <LineProvenanceBadge
        source="ai"
        capturedBy="agent:regen"
        evidence={{ snippet: "draft scope" }}
      />,
    );

    expect(screen.getByText(/ai-proposed/i)).toBeInTheDocument();
  });

  it("shows a typed-by-you chip for a human-captured line", () => {
    render(
      <LineProvenanceBadge
        source="ui"
        capturedBy="human:018f3a1b"
        evidence={null}
      />,
    );

    expect(screen.getByText(/typed by you/i)).toBeInTheDocument();
  });

  it("falls back to the source chip for other provenance", () => {
    render(
      <LineProvenanceBadge
        source="email:abc"
        capturedBy="connector:sync"
        evidence={null}
      />,
    );

    expect(screen.getByText(/connector/i)).toBeInTheDocument();
  });
});
