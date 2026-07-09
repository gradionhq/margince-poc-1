import type { Meta, StoryObj } from "@storybook/react-vite";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { MemoryRouter } from "react-router-dom";
import type {
  CustomField,
  CustomFieldListResponse,
  Member,
  MemberListResponse,
} from "../../../lib/api-client/generated/index.js";
import { setAuth } from "../../identity/store/authStore.js";
import { customFieldsKeys } from "../api/customFields.js";
import { CustomFieldsAdminPage } from "./CustomFieldsAdminPage.js";

const MEMBERS: Member[] = [
  {
    user_id: "user-1",
    email: "admin@example.com",
    display_name: "Admin User",
    status: "active",
    is_agent: false,
    roles: ["admin"],
  },
  {
    user_id: "user-2",
    email: "bob@example.com",
    display_name: "Bob Johnson",
    status: "active",
    is_agent: false,
    roles: ["rep"],
  },
];

const FIELDS: CustomField[] = [
  {
    id: "field-1",
    workspace_id: "ws-1",
    object: "deal",
    label: "Renewal Date",
    slug: "renewal_date",
    type: "date",
    status: "active",
    column_name: "cf_renewal_date",
    created_by: "user-1",
    created_at: "2026-07-01T10:00:00Z",
    updated_at: "2026-07-01T10:00:00Z",
  },
  {
    id: "field-2",
    workspace_id: "ws-1",
    object: "deal",
    label: "Old Field",
    slug: "old_field",
    type: "text",
    status: "retired",
    column_name: "cf_old_field",
    created_by: "user-2",
    created_at: "2026-06-01T10:00:00Z",
    updated_at: "2026-07-05T14:30:00Z",
  },
];

// A fresh QueryClient per story, pre-seeded with the exact query keys the
// page's hooks read (customFieldsKeys.list / ["members"]) so the page renders
// its data state straight away — staleTime: Infinity keeps react-query from
// ever issuing the real network refetch-on-mount it would normally do (there
// is no backend behind this static Storybook build to answer it).
function makeClient(): QueryClient {
  const client = new QueryClient({
    defaultOptions: { queries: { staleTime: Infinity, retry: false } },
  });
  client.setQueryData<CustomFieldListResponse>(customFieldsKeys.list("deal"), {
    data: FIELDS,
    page: { has_more: false },
  });
  client.setQueryData<MemberListResponse>(["members"], { data: MEMBERS });
  return client;
}

// Seeds the real zustand authStore via its own public setter (setAuth) — the
// exact same call the app's real login flow makes — rather than mocking the
// store module.
function Demo({ role }: { role: "admin" | "rep" }) {
  setAuth(
    {
      id: "user-1",
      workspace_id: "ws-1",
      email: "admin@example.com",
      display_name: "Admin User",
      timezone: "UTC",
      status: "active",
      is_agent: false,
    },
    role,
    [role],
  );

  return (
    <QueryClientProvider client={makeClient()}>
      <MemoryRouter initialEntries={["/admin/custom-fields"]}>
        <CustomFieldsAdminPage />
      </MemoryRouter>
    </QueryClientProvider>
  );
}

const meta: Meta<typeof Demo> = {
  component: Demo,
  title: "CRM/CustomFields/CustomFieldsAdminPage",
  parameters: { surface: "fullscreen" },
};
export default meta;
type Story = StoryObj<typeof Demo>;

// Admin: "+ Add field" visible, row actions visible, actor names unmasked.
export const Admin: Story = { args: { role: "admin" } };

// Non-admin: "+ Add field" hidden, row actions omitted, actor names masked.
export const NonAdmin: Story = { args: { role: "rep" } };
