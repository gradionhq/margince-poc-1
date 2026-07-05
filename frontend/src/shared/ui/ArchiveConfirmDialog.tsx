import { ConfirmDialog } from "./forge.js";

export function ArchiveConfirmDialog({
  open,
  entityLabel,
  onConfirm,
  onCancel,
  isLoading = false,
}: {
  open: boolean;
  entityLabel: string;
  onConfirm: () => void;
  onCancel: () => void;
  isLoading?: boolean;
}) {
  return (
    <ConfirmDialog
      open={open}
      onClose={onCancel}
      onConfirm={onConfirm}
      title="Archive this record?"
      description={`${entityLabel} will be removed from the default list. It stays retrievable by id and can be restored later.`}
      confirmLabel="Archive"
      isLoading={isLoading}
    />
  );
}
