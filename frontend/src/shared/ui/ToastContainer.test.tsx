import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ToastContainer } from "./ToastContainer.js";

describe("ToastContainer", () => {
  it("renders queued toasts and dismisses by id", () => {
    const onDismiss = vi.fn();
    render(
      <ToastContainer
        toasts={[{ id: "t1", message: "Saved" }]}
        onDismiss={onDismiss}
      />,
    );
    expect(screen.getByText("Saved")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /dismiss/i }));
    expect(onDismiss).toHaveBeenCalledWith("t1");
  });
});
