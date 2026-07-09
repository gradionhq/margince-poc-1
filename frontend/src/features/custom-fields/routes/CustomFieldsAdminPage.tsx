import { useState } from "react";
import type {
  CreateCustomFieldRequest,
  CustomField,
} from "../../../lib/api-client/generated/index.js";
import {
  Button,
  InlineErrorFallback,
  Skeleton,
} from "../../../shared/ui/forge.js";
import { ToastContainer } from "../../../shared/ui/ToastContainer.js";
import { useAuthStore } from "../../identity/store/authStore.js";
import {
  useCreateCustomField,
  useCustomFields,
  useRenameCustomField,
  useRetireCustomField,
} from "../api/customFields.js";
import { useMembers } from "../api/members.js";
import { CustomFieldAuditCard } from "../components/CustomFieldAuditCard.js";
import { CustomFieldsTable } from "../components/CustomFieldsTable.js";
import { NewCustomFieldModal } from "../components/NewCustomFieldModal.js";
import { RenameCustomFieldModal } from "../components/RenameCustomFieldModal.js";
import { RetireCustomFieldDialog } from "../components/RetireCustomFieldDialog.js";
import { OBJECT_CHIPS, type ObjectKey } from "../lib/customFieldRules.js";

interface Toast {
  id: string;
  variant: "success" | "error";
  message: string;
}

export function CustomFieldsAdminPage() {
  const [selectedObject, setSelectedObject] = useState<ObjectKey>("deal");
  const [stagedRow, setStagedRow] = useState<{
    label: string;
    type: string;
  } | null>(null);
  const [newCustomFieldOpen, setNewCustomFieldOpen] = useState(false);
  const [renameOpen, setRenameOpen] = useState(false);
  const [renameField, setRenameField] = useState<CustomField | null>(null);
  const [retireOpen, setRetireOpen] = useState(false);
  const [retireField, setRetireField] = useState<CustomField | null>(null);
  const [toasts, setToasts] = useState<Toast[]>([]);

  // Hooks
  const {
    data: listResponse,
    isLoading,
    isError,
    refetch,
  } = useCustomFields(selectedObject);
  const fields = listResponse?.data || [];
  const { data: membersResponse } = useMembers();
  const members = membersResponse?.data || [];
  const { mutate: createField, isPending: createPending } =
    useCreateCustomField();
  const { mutate: renameFieldMutate, isPending: renamePending } =
    useRenameCustomField();
  const { mutate: retireFieldMutate, isPending: retirePending } =
    useRetireCustomField();

  const { role = "rep", user } = useAuthStore();
  const userId = user?.id || "";
  const isAdmin = role === "admin";

  // Toast management
  function pushToast(variant: "success" | "error", message: string) {
    setToasts((t) => [...t, { id: crypto.randomUUID(), variant, message }]);
  }

  // Create flow
  function handleNewFieldConfirm(req: CreateCustomFieldRequest) {
    // Set staged row for optimistic UI
    setStagedRow({ label: req.label, type: req.type });

    createField(req, {
      onSuccess: (field) => {
        // Clear staged row
        setStagedRow(null);
        // Close modal
        setNewCustomFieldOpen(false);
        // Show success toast with exact message format
        pushToast(
          "success",
          `${field.label} is live on the 360, filters, export & API.`,
        );
      },
      onError: (error) => {
        // Clear staged row on error
        setStagedRow(null);
        // Show error toast
        const errorMessage =
          error instanceof Error ? error.message : "Failed to create field";
        pushToast("error", errorMessage);
      },
    });
  }

  // Rename flow
  function handleRenameOpen(field: CustomField) {
    setRenameField(field);
    setRenameOpen(true);
  }

  function handleRenameSave(newLabel: string) {
    if (!renameField) return;

    renameFieldMutate(
      { id: renameField.id, label: newLabel },
      {
        onSuccess: () => {
          setRenameOpen(false);
          setRenameField(null);
          pushToast("success", "Field renamed");
        },
        onError: (error) => {
          const errorMessage =
            error instanceof Error ? error.message : "Failed to rename field";
          pushToast("error", errorMessage);
          setRenameOpen(false);
          setRenameField(null);
        },
      },
    );
  }

  // Retire flow
  function handleRetireOpen(field: CustomField) {
    setRetireField(field);
    setRetireOpen(true);
  }

  function handleRetireConfirm() {
    if (!retireField) return;

    retireFieldMutate(retireField.id, {
      onSuccess: () => {
        setRetireOpen(false);
        setRetireField(null);
        pushToast("success", "Field retired");
      },
      onError: (error) => {
        const errorMessage =
          error instanceof Error ? error.message : "Failed to retire field";
        pushToast("error", errorMessage);
        setRetireOpen(false);
        setRetireField(null);
      },
    });
  }

  // Get display name for selected object
  const selectedObjectLabel =
    OBJECT_CHIPS.find((chip) => chip.value === selectedObject)?.label ||
    selectedObject;

  return (
    <div className="min-h-screen bg-gf-page">
      <header className="flex items-center justify-between px-gf-lg py-gf-md border-b border-gf-subtle bg-gf-card">
        <div>
          <h1 className="text-gf-title font-semibold text-gf-primary">
            Custom Fields
          </h1>
          <p className="text-gf-body text-gf-secondary">
            Manage fields for deal, organization, contact, lead, and activity
            objects.
          </p>
        </div>
        {isAdmin && (
          <Button variant="primary" onClick={() => setNewCustomFieldOpen(true)}>
            + Add field
          </Button>
        )}
      </header>

      <main className="p-gf-lg space-y-gf-lg">
        {/* Table area */}
        {isLoading ? (
          <div data-testid="table-skeleton">
            <Skeleton height="200px" />
          </div>
        ) : isError ? (
          <InlineErrorFallback onReset={refetch} />
        ) : (
          <CustomFieldsTable
            fields={fields}
            members={members}
            selectedObject={selectedObject}
            role={role ?? ""}
            onEdit={handleRenameOpen}
            onRetire={handleRetireOpen}
            stagedRow={stagedRow}
            onObjectSelect={setSelectedObject}
          />
        )}

        {/* Object chip selector - embedded in table but shown here conceptually */}
        {/* Actually, CustomFieldsTable renders the chips internally */}

        {/* Audit card */}
        <CustomFieldAuditCard
          fields={fields}
          members={members}
          role={role ?? ""}
          isLoading={isLoading}
          isError={isError}
        />
      </main>

      {/* Modals and dialogs */}
      {newCustomFieldOpen && (
        <NewCustomFieldModal
          open={newCustomFieldOpen}
          object={selectedObject}
          onClose={() => setNewCustomFieldOpen(false)}
          onConfirm={handleNewFieldConfirm}
          isLoading={createPending}
          userId={userId}
        />
      )}

      {renameOpen && renameField && (
        <RenameCustomFieldModal
          open={renameOpen}
          field={renameField}
          onClose={() => {
            setRenameOpen(false);
            setRenameField(null);
          }}
          onSave={handleRenameSave}
          isLoading={renamePending}
        />
      )}

      {retireOpen && retireField && (
        <RetireCustomFieldDialog
          open={retireOpen}
          fieldLabel={retireField.label}
          objectDisplayName={selectedObjectLabel}
          onConfirm={handleRetireConfirm}
          onCancel={() => {
            setRetireOpen(false);
            setRetireField(null);
          }}
          isLoading={retirePending}
        />
      )}

      {/* Toast container */}
      <ToastContainer
        toasts={toasts}
        onDismiss={(id) => setToasts((t) => t.filter((x) => x.id !== id))}
      />
    </div>
  );
}
