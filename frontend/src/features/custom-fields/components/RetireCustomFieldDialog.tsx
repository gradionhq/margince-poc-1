import { ConfirmDialog } from "../../../shared/ui/forge.js";

export function RetireCustomFieldDialog({
  open,
  fieldLabel,
  objectDisplayName,
  onConfirm,
  onCancel,
  isLoading = false,
}: {
  open: boolean;
  fieldLabel: string;
  objectDisplayName: string;
  onConfirm: () => void;
  onCancel: () => void;
  isLoading?: boolean;
}) {
  return (
    <ConfirmDialog
      open={open}
      onClose={onCancel}
      onConfirm={onConfirm}
      title="Retire this field?"
      description={`${fieldLabel} will be hidden from new ${objectDisplayName} records. Every existing value stays in place and the field remains in the audit trail.`}
      confirmLabel="Confirm"
      isLoading={isLoading}
    />
  );
}
