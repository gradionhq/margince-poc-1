import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeAll, describe, expect, it, vi } from "vitest";

// jsdom does not implement the native <dialog> modal methods that Forge's Modal/ConfirmDialog
// rely on — polyfill them so the merge dialog can actually mount and be exercised.
beforeAll(() => {
  if (!HTMLDialogElement.prototype.showModal) {
    HTMLDialogElement.prototype.showModal = function showModal(
      this: HTMLDialogElement,
    ) {
      this.open = true;
    };
  }
  if (!HTMLDialogElement.prototype.close) {
    HTMLDialogElement.prototype.close = function close(
      this: HTMLDialogElement,
    ) {
      this.open = false;
    };
  }
});

vi.mock("../api/person.js", () => ({ useMergePerson: vi.fn() }));

import * as personApi from "../api/person.js";
import { MergePersonDialog, mergeErrorMessage } from "./MergePersonDialog.js";

const mockMerge = vi.mocked(personApi.useMergePerson);

function renderDialog(
  mutateOverrides: Partial<ReturnType<typeof personApi.useMergePerson>> = {},
) {
  const mutate = vi.fn();
  mockMerge.mockReturnValue({
    mutate,
    isPending: false,
    error: null,
    reset: vi.fn(),
    ...mutateOverrides,
  } as never);
  const qc = new QueryClient();
  const onClose = vi.fn();
  render(
    <QueryClientProvider client={qc}>
      <MergePersonDialog personId="p1" open onClose={onClose} />
    </QueryClientProvider>,
  );
  return { mutate, onClose };
}

describe("mergeErrorMessage", () => {
  it("renders the concurrent-merge-loss copy for code: version_skew (PO-AC-M5)", () => {
    const result = mergeErrorMessage({ code: "version_skew", detail: "stale" });
    expect(result.isVersionSkew).toBe(true);
    expect(result.message).toMatch(/lost the race/i);
  });

  it("renders self-merge/archived-target 422 detail honestly (PO-AC-M3/M4), no paraphrase", () => {
    const result = mergeErrorMessage({
      code: "validation_error",
      detail: "Cannot merge a person into themself",
    });
    expect(result.isVersionSkew).toBe(false);
    expect(result.message).toBe("Cannot merge a person into themself");
  });

  it("renders a generic honest conflict for any other 409 code, never fabricating version_skew", () => {
    const result = mergeErrorMessage({ code: "idempotency_key_conflict" });
    expect(result.isVersionSkew).toBe(false);
    expect(result.message).toMatch(/idempotency_key_conflict/);
  });

  it("falls back to a generic message when the error has no code/detail at all", () => {
    const result = mergeErrorMessage(new Error("network down"));
    expect(result.message).toMatch(/merge failed/i);
  });
});

describe("MergePersonDialog", () => {
  it("requires a target id before showing the survivor-wins confirm step (PO-AC-M1/M2)", async () => {
    const { default: userEvent } = await import("@testing-library/user-event");
    const user = userEvent.setup();
    const { mutate } = renderDialog();
    expect(screen.queryByText(/survive/i)).not.toBeInTheDocument();
    await user.type(screen.getByLabelText(/target person id/i), "p2");
    await user.click(screen.getByRole("button", { name: /continue/i }));
    expect(screen.getByText(/survive/i)).toBeInTheDocument();
    expect(screen.getByText(/archived/i)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /^confirm$/i }));
    expect(mutate).toHaveBeenCalledWith({ targetId: "p2" }, expect.anything());
  });

  it("renders the actual problem code on failure without silently retrying", async () => {
    const { default: userEvent } = await import("@testing-library/user-event");
    const user = userEvent.setup();
    renderDialog({
      error: { code: "version_skew", detail: "lost the race" } as never,
    });
    await user.type(screen.getByLabelText(/target person id/i), "p2");
    await user.click(screen.getByRole("button", { name: /continue/i }));
    await user.click(screen.getByRole("button", { name: /^confirm$/i }));
    expect(screen.getByText(/lost the race/i)).toBeInTheDocument();
  });
});
