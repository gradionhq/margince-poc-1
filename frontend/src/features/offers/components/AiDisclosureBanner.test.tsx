import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { AiDisclosureBanner } from "./AiDisclosureBanner.js";

describe("AiDisclosureBanner", () => {
  it("renders nothing when there are no evidence lines", () => {
    const { container } = render(
      <AiDisclosureBanner hasEvidenceLines={false} aiDisclosureText="ignored" />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("renders the provided disclosure text verbatim", () => {
    render(
      <AiDisclosureBanner
        hasEvidenceLines
        aiDisclosureText="AI disclosure from the server"
      />,
    );

    expect(
      screen.getByText("AI disclosure from the server"),
    ).toBeInTheDocument();
  });

  it("renders the fallback disclosure copy when none is provided", () => {
    render(<AiDisclosureBanner hasEvidenceLines aiDisclosureText={null} />);

    expect(
      screen.getByText(
        "This offer includes AI-proposed content — review every line before sending.",
      ),
    ).toBeInTheDocument();
  });
});
