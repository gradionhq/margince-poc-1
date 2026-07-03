import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { UserAvatar } from "./UserAvatar.js";

describe("UserAvatar", () => {
  it("renders presence indicator when presence set", () => {
    render(<UserAvatar name="Alice" presence="online" />);
    expect(screen.getByTestId("presence-indicator")).toBeInTheDocument();
  });
  it("omits presence indicator when unset", () => {
    render(<UserAvatar name="Bob" />);
    expect(screen.queryByTestId("presence-indicator")).toBeNull();
  });
});
