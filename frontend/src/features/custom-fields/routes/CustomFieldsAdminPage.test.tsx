import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { CustomField } from "../../../lib/api-client/generated/index.js";

// Setup mocks with configurable behavior
let mockFieldsData: CustomField[] = [
  {
    id: "f1",
    workspace_id: "w1",
    label: "Custom Field 1",
    type: "text",
    object: "deal",
    slug: "custom_field_1",
    column_name: "cf_custom_field_1",
    status: "active",
    created_by: "user1",
    created_at: "2026-07-01T00:00:00Z",
    updated_at: "2026-07-01T00:00:00Z",
  },
];
let mockFieldsIsLoading = false;
let mockFieldsIsError = false;

const mockMembersData = [
  {
    user_id: "user1",
    display_name: "John Doe",
  },
];

let mockAuthRole = "admin";

const createFieldMutate = vi.fn();
const renameFieldMutate = vi.fn();
const retireFieldMutate = vi.fn();
const refetchFields = vi.fn();

vi.mock("../api/customFields.js", () => ({
  useCustomFields: (object: string) => ({
    data: {
      data: mockFieldsData.filter((f) => f.object === object),
      page: { total: mockFieldsData.length },
    },
    isLoading: mockFieldsIsLoading,
    isError: mockFieldsIsError,
    refetch: refetchFields,
  }),
  useCreateCustomField: () => ({
    mutate: createFieldMutate,
    isPending: false,
  }),
  useRenameCustomField: () => ({
    mutate: renameFieldMutate,
    isPending: false,
  }),
  useRetireCustomField: () => ({
    mutate: retireFieldMutate,
    isPending: false,
  }),
  useUpdateCustomFieldOptions: () => ({
    mutate: vi.fn(),
    isPending: false,
  }),
  customFieldsKeys: {
    all: ["custom-fields"],
    list: (object: string) => ["custom-fields", "list", object],
  },
}));

vi.mock("../api/members.js", () => ({
  useMembers: () => ({
    data: { data: mockMembersData },
  }),
}));

vi.mock("../../identity/store/authStore.js", () => ({
  useAuthStore: () => ({
    user: { id: "user1", email: "test@example.com" },
    role: mockAuthRole,
    roles: [mockAuthRole],
    loading: false,
  }),
}));

import { CustomFieldsAdminPage } from "./CustomFieldsAdminPage.js";

function renderPage() {
  const qc = new QueryClient();
  return render(
    <QueryClientProvider client={qc}>
      <MemoryRouter>
        <CustomFieldsAdminPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("CustomFieldsAdminPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Reset mock data to default
    mockFieldsData = [
      {
        id: "f1",
        workspace_id: "w1",
        label: "Custom Field 1",
        type: "text",
        object: "deal",
        slug: "custom_field_1",
        column_name: "cf_custom_field_1",
        status: "active",
        created_by: "user1",
        created_at: "2026-07-01T00:00:00Z",
        updated_at: "2026-07-01T00:00:00Z",
      },
    ];
    mockFieldsIsLoading = false;
    mockFieldsIsError = false;
    mockAuthRole = "admin";
  });

  describe("Structure and headers", () => {
    it("renders the main heading and description", () => {
      renderPage();
      expect(
        screen.getByRole("heading", { name: /custom fields/i }),
      ).toBeInTheDocument();
      expect(
        screen.getByText(/manage fields for deal, organization/i),
      ).toBeInTheDocument();
    });

    it("renders the object chips", () => {
      renderPage();
      expect(screen.getByTestId("chip-deal")).toBeInTheDocument();
      expect(screen.getByTestId("chip-organization")).toBeInTheDocument();
      expect(screen.getByTestId("chip-person")).toBeInTheDocument();
      expect(screen.getByTestId("chip-lead")).toBeInTheDocument();
      expect(screen.getByTestId("chip-activity")).toBeInTheDocument();
    });

    it("renders the CustomFieldsTable with fields", () => {
      renderPage();
      expect(screen.getByRole("table")).toBeInTheDocument();
      expect(screen.getByText("Custom Field 1")).toBeInTheDocument();
    });

    it("renders the audit card", () => {
      renderPage();
      expect(screen.getByTestId("audit-card")).toBeInTheDocument();
    });

    it("shows Add field button for admins", () => {
      renderPage();
      expect(
        screen.getByRole("button", { name: /add field/i }),
      ).toBeInTheDocument();
    });

    it("hides Add field button for non-admins", () => {
      mockAuthRole = "rep";
      renderPage();
      expect(
        screen.queryByRole("button", { name: /add field/i }),
      ).not.toBeInTheDocument();
    });
  });

  describe("Create flow", () => {
    it("opens NewCustomFieldModal when Add field button is clicked", async () => {
      const user = userEvent.setup();
      renderPage();

      const addButton = screen.getByRole("button", { name: /add field/i });
      await user.click(addButton);

      expect(screen.getByText(/new custom field/i)).toBeInTheDocument();
    });

    it("calls createField mutation and shows success toast", async () => {
      const user = userEvent.setup();
      createFieldMutate.mockImplementation((_req, opts) => {
        opts?.onSuccess?.({
          id: "f2",
          label: "Renewal Date",
          type: "text",
          object: "deal",
          slug: "renewal_date",
          status: "active",
          created_by: "user1",
          created_at: "2026-07-09T00:00:00Z",
          updated_at: "2026-07-09T00:00:00Z",
        });
      });

      renderPage();

      const addButton = screen.getByRole("button", { name: /add field/i });
      await user.click(addButton);

      const labelInputs = screen.getAllByRole("textbox");
      await user.type(labelInputs[0], "Renewal Date");

      const confirmButton = screen.getByRole("button", {
        name: /confirm & create/i,
      });
      await user.click(confirmButton);

      await waitFor(() => {
        expect(createFieldMutate).toHaveBeenCalled();
        expect(
          screen.getByText(
            /renewal date is live on the 360, filters, export & api/i,
          ),
        ).toBeInTheDocument();
      });
    });

    it("shows error toast on creation failure", async () => {
      const user = userEvent.setup();
      createFieldMutate.mockImplementation((_req, opts) => {
        opts?.onError?.(new Error("Network error"));
      });

      renderPage();

      const addButton = screen.getByRole("button", { name: /add field/i });
      await user.click(addButton);

      const labelInputs = screen.getAllByRole("textbox");
      await user.type(labelInputs[0], "Test Field");

      const confirmButton = screen.getByRole("button", {
        name: /confirm & create/i,
      });
      await user.click(confirmButton);

      await waitFor(() => {
        expect(screen.getByText(/network error/i)).toBeInTheDocument();
      });
    });

    it("prevents field creation when structural word is detected", async () => {
      const user = userEvent.setup();
      renderPage();

      const addButton = screen.getByRole("button", { name: /add field/i });
      await user.click(addButton);

      const labelInputs = screen.getAllByRole("textbox");
      await user.type(labelInputs[0], "link to company");

      const confirmButton = screen.getByRole("button", {
        name: /confirm & create/i,
      });
      expect(confirmButton).toBeDisabled();

      expect(
        screen.getByText(/this looks like a new object, relationship/i),
      ).toBeInTheDocument();
    });
  });

  describe("Rename flow", () => {
    it("opens RenameCustomFieldModal when Edit is clicked", async () => {
      const user = userEvent.setup();
      renderPage();

      const actionButton = screen.getByRole("button", { name: /row actions/i });
      await user.click(actionButton);

      const editMenuItem = screen.getByRole("menuitem", { name: "Edit" });
      await user.click(editMenuItem);

      expect(screen.getByText(/rename field/i)).toBeInTheDocument();
    });

    it("calls renameField mutation with label change and shows success toast", async () => {
      const user = userEvent.setup();
      renameFieldMutate.mockImplementation((_req, opts) => {
        opts?.onSuccess?.({
          id: "f1",
          label: "Updated Name",
          type: "text",
          object: "deal",
          slug: "updated_name",
          status: "active",
          created_by: "user1",
          created_at: "2026-07-01T00:00:00Z",
          updated_at: "2026-07-09T00:00:00Z",
        });
      });

      renderPage();

      const actionButton = screen.getByRole("button", { name: /row actions/i });
      await user.click(actionButton);

      const editMenuItem = screen.getByRole("menuitem", { name: "Edit" });
      await user.click(editMenuItem);

      const labelInput = screen.getByDisplayValue(/custom field 1/i);
      await user.clear(labelInput);
      await user.type(labelInput, "Updated Name");

      const saveButton = screen.getByRole("button", { name: "Save" });
      await user.click(saveButton);

      await waitFor(() => {
        expect(renameFieldMutate).toHaveBeenCalledWith(
          expect.objectContaining({
            id: "f1",
            label: "Updated Name",
          }),
          expect.any(Object),
        );
        expect(screen.getByText(/field renamed/i)).toBeInTheDocument();
      });
    });
  });

  describe("Retire flow", () => {
    it("opens RetireCustomFieldDialog when Archive is clicked", async () => {
      const user = userEvent.setup();
      renderPage();

      const actionButton = screen.getByRole("button", { name: /row actions/i });
      await user.click(actionButton);

      const archiveMenuItem = screen.getByRole("menuitem", { name: "Archive" });
      await user.click(archiveMenuItem);

      expect(screen.getByText(/retire this field/i)).toBeInTheDocument();
    });

    it("calls retireField mutation and shows success toast", async () => {
      const user = userEvent.setup();
      retireFieldMutate.mockImplementation((fieldId, opts) => {
        opts?.onSuccess?.({
          id: fieldId,
          label: "Custom Field 1",
          type: "text",
          object: "deal",
          slug: "custom_field_1",
          status: "retired",
          created_by: "user1",
          created_at: "2026-07-01T00:00:00Z",
          updated_at: "2026-07-09T00:00:00Z",
        });
      });

      renderPage();

      const actionButton = screen.getByRole("button", { name: /row actions/i });
      await user.click(actionButton);

      const archiveMenuItem = screen.getByRole("menuitem", { name: "Archive" });
      await user.click(archiveMenuItem);

      const confirmButton = screen.getByRole("button", { name: "Confirm" });
      await user.click(confirmButton);

      await waitFor(() => {
        expect(retireFieldMutate).toHaveBeenCalled();
        expect(screen.getByText(/field retired/i)).toBeInTheDocument();
      });
    });
  });

  describe("Object selection", () => {
    it("updates chip selection when clicking a different object", async () => {
      const user = userEvent.setup();
      // Add an organization field so the chip switch doesn't trigger EmptyState
      mockFieldsData = [
        {
          id: "f1",
          workspace_id: "w1",
          label: "Custom Field 1",
          type: "text",
          object: "deal",
          slug: "custom_field_1",
          column_name: "cf_custom_field_1",
          status: "active",
          created_by: "user1",
          created_at: "2026-07-01T00:00:00Z",
          updated_at: "2026-07-01T00:00:00Z",
        },
        {
          id: "f2",
          workspace_id: "w1",
          label: "Company Field",
          type: "text",
          object: "organization",
          slug: "company_field",
          column_name: "cf_company_field",
          status: "active",
          created_by: "user1",
          created_at: "2026-07-01T00:00:00Z",
          updated_at: "2026-07-01T00:00:00Z",
        },
      ];

      renderPage();

      expect(screen.getByTestId("chip-deal")).toHaveAttribute(
        "data-selected",
        "true",
      );

      const orgChip = screen.getByTestId("chip-organization");
      await user.click(orgChip);

      expect(orgChip).toHaveAttribute("data-selected", "true");
      expect(screen.getByTestId("chip-deal")).toHaveAttribute(
        "data-selected",
        "false",
      );
    });
  });

  describe("Toast management", () => {
    it("displays success toast after field creation", async () => {
      const user = userEvent.setup();
      createFieldMutate.mockImplementation((_req, opts) => {
        opts?.onSuccess?.({
          id: "f2",
          label: "New Field",
          type: "text",
          object: "deal",
          slug: "new_field",
          status: "active",
          created_by: "user1",
          created_at: "2026-07-09T00:00:00Z",
          updated_at: "2026-07-09T00:00:00Z",
        });
      });

      renderPage();

      const addButton = screen.getByRole("button", { name: /add field/i });
      await user.click(addButton);

      const labelInputs = screen.getAllByRole("textbox");
      await user.type(labelInputs[0], "New Field");

      const confirmButton = screen.getByRole("button", {
        name: /confirm & create/i,
      });
      await user.click(confirmButton);

      await waitFor(() => {
        expect(screen.getByText(/is live on the 360/i)).toBeInTheDocument();
      });
    });
  });
});
