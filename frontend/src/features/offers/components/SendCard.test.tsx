import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Offer } from "../../../lib/api-client/generated/index.js";
import { SendCard } from "./SendCard.js";

const mutateAsync = vi.fn();

let mockCanMutateOffer = true;
let mockOffer: Pick<Offer, "id" | "status" | "offer_number" | "revision"> & {
  currency: string;
  deal_id: string;
} = {
  id: "o1",
  status: "draft",
  currency: "EUR",
  offer_number: "OFF-1",
  revision: 2,
  deal_id: "d1",
};
const onSent = vi.fn();
const pushToast = vi.fn();

vi.mock("../../../shared/ui/forge.js", () => ({
  Button: ({
    children,
    onClick,
    disabled,
    type,
  }: {
    children: React.ReactNode;
    onClick?: () => void;
    disabled?: boolean;
    type?: "button" | "submit";
  }) => (
    <button type={type ?? "button"} onClick={onClick} disabled={disabled}>
      {children}
    </button>
  ),
  ConfirmDialog: ({
    open,
    onConfirm,
    onClose,
    title,
    description,
    confirmLabel,
  }: {
    open: boolean;
    onConfirm: () => void;
    onClose: () => void;
    title: string;
    description: string;
    confirmLabel: string;
  }) =>
    open ? (
      <div>
        <h2>{title}</h2>
        <p>{description}</p>
        <button type="button" onClick={onClose}>
          Cancel
        </button>
        <button type="button" onClick={onConfirm}>
          {confirmLabel}
        </button>
      </div>
    ) : null,
}));

vi.mock("../api/offers.js", () => ({
  useSendOffer: () => ({
    mutateAsync,
    isPending: false,
  }),
}));

describe("SendCard", () => {
  beforeEach(() => {
    mutateAsync.mockReset();
    onSent.mockReset();
    pushToast.mockReset();
    mockCanMutateOffer = true;
    mockOffer = {
      id: "o1",
      status: "draft",
      currency: "EUR",
      offer_number: "OFF-1",
      revision: 2,
      deal_id: "d1",
    };
  });

  function renderCard() {
    const qc = new QueryClient();
    return render(
      <QueryClientProvider client={qc}>
        <SendCard
          offer={mockOffer}
          canMutateOffer={mockCanMutateOffer}
          onSent={onSent}
          pushToast={pushToast}
        />
      </QueryClientProvider>,
    );
  }

  it("shows the 🟡-gate copy only for draft offers the user can send", () => {
    renderCard();
    expect(
      screen.getByText(/your own click here is the approval/i),
    ).toBeInTheDocument();
  });

  it("does not render for sent offers", () => {
    mockOffer = { ...mockOffer, status: "sent" };
    renderCard();
    expect(screen.queryByTestId("send-card")).not.toBeInTheDocument();
  });

  it("confirms, sends, and surfaces the locked toast text", async () => {
    const user = userEvent.setup();
    mutateAsync.mockResolvedValueOnce({ ...mockOffer, status: "sent" });
    renderCard();

    await user.click(screen.getByRole("button", { name: /send offer/i }));
    await user.click(screen.getByRole("button", { name: /confirm send/i }));

    expect(mutateAsync).toHaveBeenCalledOnce();
    expect(onSent).toHaveBeenCalledWith(
      expect.objectContaining({ status: "sent" }),
    );
    expect(pushToast).toHaveBeenCalledWith(
      "success",
      expect.stringMatching(/offer sent — locked/i),
    );
  });

  it("renders the approval_required and fx_rate_unavailable messages distinctly", () => {
    renderCard();
    const user = userEvent.setup();

    mutateAsync.mockRejectedValueOnce({ status: 403 });
    return user
      .click(screen.getByRole("button", { name: /send offer/i }))
      .then(() =>
        user.click(screen.getByRole("button", { name: /confirm send/i })),
      )
      .then(() => {
        expect(
          screen.getByText(/approval required unexpectedly/i),
        ).toBeInTheDocument();
      });
  });

  it("renders the fx rate unavailable message on 422", async () => {
    const user = userEvent.setup();
    mutateAsync.mockRejectedValueOnce({ status: 422 });
    renderCard();

    await user.click(screen.getByRole("button", { name: /send offer/i }));
    await user.click(screen.getByRole("button", { name: /confirm send/i }));

    expect(screen.getByText(/missing fx rate context/i)).toBeInTheDocument();
  });
});
