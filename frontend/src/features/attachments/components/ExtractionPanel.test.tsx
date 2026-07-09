import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const acceptMutate = vi.fn();

vi.mock("../api/attachments.js", () => ({
  useAttachmentExtraction: vi.fn(),
  useAcceptExtraction: vi.fn(() => ({
    mutate: acceptMutate,
    isPending: false,
  })),
}));

import {
  useAcceptExtraction,
  useAttachmentExtraction,
} from "../api/attachments.js";
import { ExtractionPanel } from "./ExtractionPanel.js";

const mockedUseAttachmentExtraction = vi.mocked(useAttachmentExtraction);
const mockedUseAcceptExtraction = vi.mocked(useAcceptExtraction);

const populatedExtraction = {
  fields: [
    {
      field: "name",
      value: "Acme Deal",
      source_quote: "Acme Deal",
      page_or_section: "Page 1",
      confidence: "high",
    },
    {
      field: "amount_minor",
      value: "1000000",
      source_quote: "$10,000.00",
      page_or_section: "Section 2",
      confidence: "medium",
    },
  ],
  omitted: [
    {
      field: "expected_close_date",
      reason: "not_stated_in_file",
    },
  ],
};

describe("ExtractionPanel", () => {
  beforeEach(() => {
    acceptMutate.mockReset();
    mockedUseAttachmentExtraction.mockReset();
    mockedUseAcceptExtraction.mockReset();
    mockedUseAcceptExtraction.mockImplementation(() => ({
      mutate: acceptMutate,
      isPending: false,
    }));
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("renders nothing for the empty seam and stays gated when dealId is missing", () => {
    mockedUseAttachmentExtraction.mockReturnValue({
      data: { fields: [], omitted: [] },
      isLoading: false,
      isError: false,
    });

    const { rerender } = render(
      <ExtractionPanel attachmentId="att-1" dealId="deal-1" />,
    );
    expect(screen.queryByTestId("extraction-panel")).not.toBeInTheDocument();

    rerender(<ExtractionPanel attachmentId="att-1" />);
    expect(screen.queryByTestId("extraction-panel")).not.toBeInTheDocument();
    expect(useAttachmentExtraction).toHaveBeenLastCalledWith(undefined);
  });

  it("renders grounded and omitted fields, then accepts edits into the deal", async () => {
    const user = userEvent.setup();
    mockedUseAttachmentExtraction.mockReturnValue({
      data: populatedExtraction,
      isLoading: false,
      isError: false,
    });
    acceptMutate.mockImplementation((_body, options) => {
      options?.onSuccess?.({
        deal_id: "deal-1",
        accepted: [
          {
            field: "name",
            value: "Acme Deal (edited)",
            provenance: "human",
          },
          {
            field: "amount_minor",
            value: "1000000",
            provenance: "ai-extracted",
          },
        ],
      });
    });

    render(<ExtractionPanel attachmentId="att-1" dealId="deal-1" />);

    expect(
      screen.getByRole("heading", {
        name: /AI read this file — 2 fields it can ground, staged for your record \(accept to persist\)/i,
      }),
    ).toBeInTheDocument();

    const nameField = screen.getByTestId("extraction-field-name");
    expect(within(nameField).getByText("Acme Deal")).toBeInTheDocument();
    expect(within(nameField).getByText("grounded")).toBeInTheDocument();
    expect(within(nameField).getByText("Page 1")).toBeInTheDocument();
    expect(
      within(nameField).getByLabelText("high confidence"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "expected_close_date - omitted (not stated in this file)",
      ),
    ).toBeInTheDocument();

    await user.click(
      within(nameField).getByRole("button", { name: /^edit$/i }),
    );
    const input = within(nameField).getByDisplayValue("Acme Deal");
    await user.clear(input);
    await user.type(input, "Acme Deal (edited)");

    await user.click(
      screen.getByRole("button", { name: /^accept 2 fields$/i }),
    );

    expect(acceptMutate).toHaveBeenCalledWith(
      expect.objectContaining({
        field_keys: ["name", "amount_minor"],
        edits: { name: "Acme Deal (edited)" },
      }),
      expect.any(Object),
    );
    expect(
      screen.getByRole("heading", {
        name: /2 fields accepted to the deal — original snippets retained/i,
      }),
    ).toBeInTheDocument();
    const acceptedNameField = screen.getByTestId("extraction-field-name");
    expect(
      within(acceptedNameField).getByText("typed-by-you"),
    ).toBeInTheDocument();
    expect(
      within(acceptedNameField).getByText("Original snippet: Acme Deal"),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /^accept 2 fields$/i }),
    ).not.toBeInTheDocument();
    expect(
      within(screen.getByRole("alert")).getByText(
        /2 fields accepted to the deal/i,
      ),
    ).toBeInTheDocument();
  });

  it("dismiss removes the panel locally and toasts that nothing was written", async () => {
    const user = userEvent.setup();
    mockedUseAttachmentExtraction.mockReturnValue({
      data: populatedExtraction,
      isLoading: false,
      isError: false,
    });

    render(<ExtractionPanel attachmentId="att-1" dealId="deal-1" />);

    await user.click(screen.getByRole("button", { name: /^dismiss$/i }));

    expect(acceptMutate).not.toHaveBeenCalled();
    expect(screen.queryByTestId("extraction-panel")).not.toBeInTheDocument();
    expect(screen.getByText(/nothing was written/i)).toBeInTheDocument();
  });
});
