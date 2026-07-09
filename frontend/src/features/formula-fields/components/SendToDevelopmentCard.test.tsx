import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { SendToDevelopmentCard } from "./SendToDevelopmentCard.js";

describe("SendToDevelopmentCard", () => {
  it("shows the AI-proposed disclosure label", () => {
    render(<SendToDevelopmentCard />);
    expect(screen.getByText("AI-proposed")).toBeInTheDocument();
  });

  it("routes the proposal to development and shows the reviewed-source copy", async () => {
    const user = userEvent.setup();
    render(<SendToDevelopmentCard />);

    await user.click(
      screen.getByRole("button", { name: /send to development/i }),
    );

    expect(
      screen.getByText(
        /this logic ships as a reviewed source change, not as runtime editor state/i,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /development path/i }),
    ).toHaveAttribute("href", "/development");
    expect(screen.getByRole("status")).toHaveTextContent(
      "Formula logic is reviewed code, not runtime.",
    );
    expect(screen.getByText("AI-proposed")).toBeInTheDocument();
  });

  it("toasts the edit action without opening an editor", async () => {
    const user = userEvent.setup();
    render(<SendToDevelopmentCard />);

    await user.click(screen.getByRole("button", { name: /edit formula/i }));

    expect(screen.getByRole("status")).toHaveTextContent(
      "Draft edit - formula logic ships as reviewed source, not edited here",
    );
    expect(
      screen.queryByRole("textbox", { name: /formula/i }),
    ).not.toBeInTheDocument();
  });

  it("dismisses the card from the DOM", async () => {
    const user = userEvent.setup();
    render(<SendToDevelopmentCard />);

    await user.click(screen.getByRole("button", { name: /dismiss/i }));

    expect(screen.queryByText("AI-proposed")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /dismiss/i }),
    ).not.toBeInTheDocument();
  });
});
