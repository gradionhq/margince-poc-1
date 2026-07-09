import { useState } from "react";
import { TextInput, Modal, Button } from "../../../shared/ui/forge.js";
import type { CustomField } from "../../../lib/api-client/generated/index.js";

export function RenameCustomFieldModal({
  open,
  field,
  onClose,
  onSave,
  isLoading = false,
}: {
  open: boolean;
  field: CustomField;
  onClose: () => void;
  onSave: (label: string) => void;
  isLoading?: boolean;
}) {
  const [newLabel, setNewLabel] = useState(field.label);

  const trimmedValue = newLabel.trim();
  const isUnchanged = trimmedValue === field.label;
  const isEmpty = trimmedValue === "";

  const isSaveDisabled = isEmpty || isUnchanged || isLoading;

  const handleSave = () => {
    onSave(trimmedValue);
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Rename field"
      subtitle={`Update the label for ${field.label}`}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button
            variant="primary"
            disabled={isSaveDisabled}
            loading={isLoading}
            onClick={handleSave}
          >
            Save
          </Button>
        </>
      }
    >
      <div className="px-gf-xl py-gf-lg flex flex-col gap-gf-md">
        <label className="flex flex-col gap-gf-xs">
          <span className="text-gf-caption text-gf-secondary">Field label</span>
          <TextInput
            value={newLabel}
            onChange={(v) => setNewLabel(v)}
            placeholder="e.g., Renewal date"
          />
        </label>
      </div>
    </Modal>
  );
}
