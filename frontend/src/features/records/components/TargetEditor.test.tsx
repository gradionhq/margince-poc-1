import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Quota } from "../api/quotas.js";

vi.mock("../../../lib/api-client/client.js", () => ({
  apiClient: { GET: vi.fn(), POST: vi.fn(), DELETE: vi.fn(), PATCH: vi.fn() },
}));

import { apiClient } from "../../../lib/api-client/client.js";
import { TargetEditor } from "./TargetEditor.js";

const QUOTA: Quota = {
  id: "q1",
  workspace_id: "ws1",
  owner_id: "u1",
  team_id: null,
  period_start: "2026-07-01",
  period_end: "2026-09-30",
  target_minor: 28000000,
  currency: "EUR",
  version: 3,
  created_at: "2026-06-28T00:00:00Z",
  updated_at: "2026-07-01T00:00:00Z",
  archived_at: null,
};

function wrapper({ children }: { children: ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

beforeEach(() => vi.clearAllMocks());

describe("TargetEditor", () => {
  it("AC-quota-6: saving a new German-grouped value PATCHes target_minor and toasts the human-typed confirmation", async () => {
    (apiClient.PATCH as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      data: { ...QUOTA, target_minor: 30000000, version: 4 },
      error: undefined,
    });
    const onToast = vi.fn();
    const user = userEvent.setup();
    render(<TargetEditor quota={QUOTA} onToast={onToast} />, { wrapper });
    const input = screen.getByRole("textbox");
    await user.clear(input);
    await user.type(input, "300.000");
    await user.click(screen.getByRole("button", { name: /save target/i }));
    expect(apiClient.PATCH).toHaveBeenCalledWith(
      "/quotas/{id}",
      expect.objectContaining({
        params: { path: { id: "q1" }, header: { "If-Match": "3" } },
        body: { target_minor: 30000000 },
      }),
    );
    expect(onToast).toHaveBeenCalledWith(
      "success",
      expect.stringMatching(/target saved as human-typed/i),
    );
  });

  it("AC-quota-6: a zero/empty entry toasts the refusal and never PATCHes", async () => {
    const onToast = vi.fn();
    const user = userEvent.setup();
    render(<TargetEditor quota={QUOTA} onToast={onToast} />, { wrapper });
    const input = screen.getByRole("textbox");
    await user.clear(input);
    await user.click(screen.getByRole("button", { name: /save target/i }));
    expect(onToast).toHaveBeenCalledWith(
      "error",
      expect.stringMatching(/enter a target amount in eur/i),
    );
    expect(apiClient.PATCH).not.toHaveBeenCalled();
  });
});
