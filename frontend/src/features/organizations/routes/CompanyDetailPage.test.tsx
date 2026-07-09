import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  components,
  Organization,
} from "../../../lib/api-client/generated/index.js";

const baseOrgData = {
  id: "org1",
  workspace_id: "ws1",
  display_name: "Acme Corp",
  industry: "Software",
  size_band: "51-200",
  domains: [
    {
      id: "dom-1",
      domain: "acme.com",
      is_primary: true,
      source: "manual",
      captured_by: "human:u1",
    },
  ],
  address: { city: "Berlin", country: "DE" },
  org_strength: null,
  deals: [],
  relationships: [],
  activities: [],
  contact_count: 0,
  open_deal_count: 0,
  version: 3,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
  source: "manual",
  captured_by: "human:u1",
  archived_at: null as string | null,
} satisfies Organization;

let mockPartner: unknown = null;
let mockOrg: Organization = { ...baseOrgData };
let mockUpdateMutate = vi.fn(
  (_patch: unknown, opts?: { onSuccess?: () => void }) => {
    opts?.onSuccess?.();
    return undefined;
  },
);
let mockArchiveMutate = vi.fn(
  (_vars: undefined, opts?: { onSuccess?: () => undefined }) => {
    opts?.onSuccess?.();
    return undefined;
  },
);
let mockRestoreMutate = vi.fn(
  (
    _vars: undefined,
    opts?: { onSuccess?: () => undefined; onError?: (err: unknown) => void },
  ) => {
    opts?.onSuccess?.();
    return undefined;
  },
);

type ComputedField = components["schemas"]["ComputedField"];

const baseComputedFields = [
  {
    key: "open_pipeline",
    label: "Open pipeline",
    kind: "currency_minor",
    value_minor: 212000,
    formula_sql:
      "COALESCE(organization_open_pipeline_rollup.open_pipeline_minor_base, 0)",
    dependencies: [
      "organization_open_pipeline_rollup.open_pipeline_minor_base",
      "deal.amount_minor_base",
    ],
    computable: true,
  },
  {
    key: "weighted_pipeline",
    label: "Weighted pipeline",
    kind: "currency_minor",
    formula_sql: "",
    dependencies: [],
    computable: false,
    reason: "not_yet_built",
  },
  {
    key: "customer_age",
    label: "Customer age",
    kind: "duration_months",
    formula_sql: "",
    dependencies: [],
    computable: false,
    reason: "not_yet_built",
  },
  {
    key: "net_revenue_retention",
    label: "Net revenue retention",
    kind: "percent",
    formula_sql: "",
    dependencies: [],
    computable: false,
    reason: "not_yet_built",
  },
  {
    key: "blended_gross_margin",
    label: "Blended gross margin",
    kind: "percent",
    formula_sql: "",
    dependencies: [],
    computable: false,
    reason: "not_yet_built",
  },
] satisfies ComputedField[];

vi.mock("../api/organizations.js", () => ({
  useOrganization: () => ({
    data: mockOrg,
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  }),
  useOrgPartner: () => ({
    data: mockPartner,
    isLoading: false,
    isError: false,
  }),
  useOrgContacts: () => ({ contacts: [], isLoading: false }),
  useSourcedDeals: () => ({ data: [], isLoading: false, isError: false }),
  useUpdateOrganization: () => ({ mutate: mockUpdateMutate, isPending: false }),
  useArchiveOrganization: () => ({
    mutate: mockArchiveMutate,
    isPending: false,
  }),
  useRestoreOrganization: () => ({
    mutate: mockRestoreMutate,
    isPending: false,
  }),
}));

import { CompanyDetailPage } from "./CompanyDetailPage.js";

function renderPage() {
  const qc = new QueryClient();
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter initialEntries={["/companies/org1"]}>
        <Routes>
          <Route path="/companies/:id" element={<CompanyDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CompanyDetailPage", () => {
  beforeEach(() => {
    mockPartner = null;
    mockOrg = { ...baseOrgData };
    mockUpdateMutate = vi.fn(
      (_patch: unknown, opts?: { onSuccess?: () => void }) => {
        opts?.onSuccess?.();
        return undefined;
      },
    );
    mockArchiveMutate = vi.fn(
      (_vars: undefined, opts?: { onSuccess?: () => undefined }) => {
        opts?.onSuccess?.();
        return undefined;
      },
    );
    mockRestoreMutate = vi.fn(
      (
        _vars: undefined,
        opts?: {
          onSuccess?: () => undefined;
          onError?: (err: unknown) => void;
        },
      ) => {
        opts?.onSuccess?.();
        return undefined;
      },
    );
  });

  it("renders the header card with name, industry, website, staff, location", () => {
    renderPage();
    expect(screen.getByText("Acme Corp")).toBeInTheDocument();
    expect(screen.getByText("Software")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /acme\.com/i })).toHaveAttribute(
      "href",
      "https://acme.com",
    );
    expect(screen.getByText(/51-200/)).toBeInTheDocument();
    expect(screen.getByText("Berlin, DE")).toBeInTheDocument();
  });

  it("renders the no-signal org strength card", () => {
    renderPage();
    expect(screen.getByText(/no signal yet/i)).toBeInTheDocument();
  });

  it("renders honest empty states for PeopleRail and DealRail when there are none", () => {
    renderPage();
    expect(screen.getByText("No known contacts yet.")).toBeInTheDocument();
    expect(
      screen.getByText("No open or won deals for this org."),
    ).toBeInTheDocument();
  });

  it("renders honest empty states for ActivityCard, AccountSignalCard, and QuickFactsRail", () => {
    renderPage();
    expect(screen.getByText(/no activity yet/i)).toBeInTheDocument();
    expect(
      screen.getByText(/no account signal to flag right now/i),
    ).toBeInTheDocument();
    expect(screen.getByText(/owner/i)).toBeInTheDocument();
  });

  it("renders no partner panel when the org is not a partner (STATE-1, not an error)", () => {
    mockPartner = null;
    renderPage();
    expect(screen.queryByText("Cert status")).not.toBeInTheDocument();
  });

  it("renders the partner panel when the org IS a partner", () => {
    mockPartner = {
      id: "pt1",
      organization_id: "org1",
      cert_status: "certified",
      partner_role: "consulting",
    };
    renderPage();
    expect(screen.getByText("Cert status")).toBeInTheDocument();
    expect(screen.getByText(/certified/i)).toBeInTheDocument();
  });

  it("shows Summarize this account disabled with an explanatory title", () => {
    renderPage();
    const summarizeBtn = screen.getByRole("button", {
      name: /summarize this account/i,
    });
    expect(summarizeBtn).toBeDisabled();
    expect(summarizeBtn).toHaveAttribute(
      "title",
      "Account summaries ship in a later chapter",
    );
  });

  it("clicking Edit opens EditOrgModal, saving a changed field marks it typed-by-you (AC-company-12)", () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /^edit$/i }));
    const industryInput = screen.getByDisplayValue("Software");
    fireEvent.change(industryInput, { target: { value: "Fintech" } });
    fireEvent.click(screen.getByRole("button", { name: /^save$/i }));
    expect(screen.getByTitle("Typed by you this session")).toBeInTheDocument();
  });

  it("renders the archived banner and no blank/404-shaped content for an archived company", () => {
    mockOrg = { ...baseOrgData, archived_at: "2026-07-05T00:00:00Z" };
    renderPage();
    expect(screen.getByText(/this company is archived/i)).toBeInTheDocument();
    expect(
      screen.queryByText(/failed to load this company/i),
    ).not.toBeInTheDocument();
  });

  it("clicking Restore calls the restore mutation for an archived company", () => {
    mockOrg = { ...baseOrgData, archived_at: "2026-07-05T00:00:00Z" };
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /restore/i }));
    expect(mockRestoreMutate).toHaveBeenCalledOnce();
  });

  it("shows the existing-record pointer link on a 409 restore refusal", () => {
    mockOrg = { ...baseOrgData, archived_at: "2026-07-05T00:00:00Z" };
    mockRestoreMutate.mockImplementationOnce(
      (
        _vars: undefined,
        opts?: {
          onSuccess?: () => undefined;
          onError?: (err: unknown) => void;
        },
      ): undefined => {
        opts?.onError?.({
          status: 409,
          code: "duplicate_domain",
          details: { existing_id: "org2" },
          detail: "A live company already has this domain.",
        });
        return undefined;
      },
    );
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /restore/i }));
    expect(
      screen.getByRole("link", { name: /already live as a different record/i }),
    ).toHaveAttribute("href", "/companies/org2");
    expect(
      screen.queryByText(/failed to load this company/i),
    ).not.toBeInTheDocument();
  });

  it("shows Archive… for a live company and no archived banner", () => {
    mockOrg = { ...baseOrgData, archived_at: null };
    renderPage();
    expect(
      screen.getByRole("button", { name: /archive…/i }),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/this company is archived/i),
    ).not.toBeInTheDocument();
  });

  it("renders the formula fields panel when computed fields are present", () => {
    mockOrg = { ...baseOrgData, computed_fields: baseComputedFields };
    renderPage();

    expect(screen.getByTestId("formula-fields-panel")).toBeInTheDocument();
    expect(screen.getByTestId("formula-fields-panel")).toHaveTextContent(
      /read-only computed/i,
    );
    expect(screen.getByText(/recomputes on every write/i)).toBeInTheDocument();
    expect(
      screen.getByTestId("formula-field-row-open_pipeline"),
    ).toBeInTheDocument();
    expect(screen.getByText("See it recompute")).toBeInTheDocument();
    expect(screen.getByText("AI-proposed")).toBeInTheDocument();
    expect(screen.getByText("computed:server")).toBeInTheDocument();
  });

  it("does not render the formula fields panel when computed_fields is undefined", () => {
    mockOrg = { ...baseOrgData, computed_fields: undefined };
    renderPage();

    expect(
      screen.queryByTestId("formula-fields-panel"),
    ).not.toBeInTheDocument();
  });
});
