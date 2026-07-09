import { useState, useEffect } from "react";
import {
  TextInput,
  Modal,
  Button,
} from "../../../shared/ui/forge.js";
import {
  buildApiKey,
  buildDdlPreview,
  detectStructuralWord,
  slugify,
  type ObjectKey,
} from "../lib/customFieldRules.js";
import type { CreateCustomFieldRequest } from "../../../lib/api-client/generated/index.js";

type FieldType = "text" | "number" | "date" | "currency" | "picklist" | "boolean";

const FIELD_TYPES: FieldType[] = ["text", "number", "date", "currency", "picklist", "boolean"];

export function NewCustomFieldModal({
  open,
  object,
  onClose,
  onConfirm,
  isLoading = false,
  userId,
  onGuardToast,
}: {
  open: boolean;
  object: ObjectKey;
  onClose: () => void;
  onConfirm: (req: CreateCustomFieldRequest) => void;
  isLoading?: boolean;
  userId?: string;
  onGuardToast?: (message: string) => void;
}) {
  const [label, setLabel] = useState("");
  const [type, setType] = useState<FieldType>("text");
  const [currencyCode, setCurrencyCode] = useState("");
  const [picklistOptions, setPicklistOptions] = useState<string[]>([""]);
  const [structuralWord, setStructuralWord] = useState<string | null>(null);

  const slug = slugify(label);
  const apiKey = buildApiKey(object, slug);
  const ddlPreview = buildDdlPreview(object, slug, type);

  // Detect structural words on label change
  useEffect(() => {
    if (label.trim()) {
      const detected = detectStructuralWord(label);
      setStructuralWord(detected);
    } else {
      setStructuralWord(null);
    }
  }, [label]);

  // Check if confirm should be disabled
  const isTrimmedLabelEmpty = label.trim() === "";
  const isCurrencyCodeMissing = type === "currency" && currencyCode.trim() === "";
  const hasStructuralWord = structuralWord !== null;

  const isConfirmDisabled =
    isTrimmedLabelEmpty || isCurrencyCodeMissing || hasStructuralWord || isLoading;

  const handleConfirm = async () => {
    // Defensive: early return on empty label
    if (isTrimmedLabelEmpty) {
      onGuardToast?.("Give the field a label first");
      return;
    }

    // Build the request
    const req: CreateCustomFieldRequest = {
      object,
      label: label.trim(),
      type,
      source: "manual",
      captured_by: `human:${userId ?? "unknown"}`,
    };

    // Include currency code if applicable
    if (type === "currency" && currencyCode.trim()) {
      req.currency = currencyCode.trim();
    }

    // Include picklist options if applicable
    if (type === "picklist" && picklistOptions.length > 0) {
      req.options = picklistOptions.filter((opt) => opt.trim() !== "");
    }

    onConfirm(req);
  };

  const handleAddOption = () => {
    setPicklistOptions([...picklistOptions, ""]);
  };

  const handleRemoveOption = (index: number) => {
    // Check if this is the last option
    if (picklistOptions.length === 1) {
      onGuardToast?.("A picklist needs at least one option");
      return;
    }

    const updated = picklistOptions.filter((_, i) => i !== index);
    setPicklistOptions(updated);
  };

  const handleOptionChange = (index: number, value: string) => {
    const updated = [...picklistOptions];
    updated[index] = value;
    setPicklistOptions(updated);
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="New custom field"
      subtitle={`Add a scalar attribute to ${object}s`}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button
            variant="primary"
            loading={isLoading}
            disabled={isConfirmDisabled}
            onClick={handleConfirm}
          >
            Confirm & create
          </Button>
        </>
      }
    >
      <div className="px-gf-xl py-gf-lg flex flex-col gap-gf-md">
        {/* Structural word refusal banner */}
        {structuralWord && (
          <div className="p-gf-md bg-gf-status-danger/10 border border-gf-status-danger rounded-md">
            <p className="text-gf-caption text-gf-status-danger">
              This looks like a new object, relationship, or logic — not a
              scalar attribute on an existing object. Runtime custom fields only
              add bounded scalar columns; a structural change ships as a
              reviewed source change instead.
            </p>
            <p className="text-gf-caption text-gf-secondary mt-gf-xs">
              This needs the development path, not this screen.
            </p>
          </div>
        )}

        {/* Label input */}
        <label className="flex flex-col gap-gf-xs">
          <span className="text-gf-caption text-gf-secondary">Field label</span>
          <TextInput
            value={label}
            onChange={(v) => setLabel(v)}
            placeholder="e.g., Renewal date"
          />
        </label>

        {/* API key display */}
        <label className="flex flex-col gap-gf-xs">
          <span className="text-gf-caption text-gf-secondary">API key</span>
          <TextInput
            value={apiKey}
            onChange={() => {}} // read-only
            disabled
          />
        </label>

        {/* DDL preview */}
        <div className="flex flex-col gap-gf-xs">
          <span className="text-gf-caption text-gf-secondary">DDL preview</span>
          <pre
            data-testid="ddl-preview"
            className="font-mono text-xs bg-gf-elevated border border-gf-subtle rounded-md px-gf-md py-gf-sm"
          >
            {ddlPreview}
          </pre>
        </div>

        {/* Type picker */}
        <label className="flex flex-col gap-gf-xs">
          <span className="text-gf-caption text-gf-secondary">Field type</span>
          <select
            value={type}
            onChange={(e) => setType(e.target.value as FieldType)}
            className="h-10 w-full rounded-md bg-gf-elevated border border-gf-subtle text-gf-body text-gf-primary px-gf-md"
          >
            {FIELD_TYPES.map((t) => (
              <option key={t} value={t}>
                {t}
              </option>
            ))}
          </select>
        </label>

        {/* Currency code field (conditional) */}
        {type === "currency" && (
          <label className="flex flex-col gap-gf-xs">
            <span className="text-gf-caption text-gf-secondary">
              ISO-4217 code
            </span>
            <TextInput
              value={currencyCode}
              onChange={(v) => setCurrencyCode(v)}
              placeholder="e.g., USD"
            />
            <span className="text-gf-caption text-gf-secondary">
              Stored as integer minor-units (e.g. cents).
            </span>
          </label>
        )}

        {/* Picklist options editor (conditional) */}
        {type === "picklist" && (
          <div className="flex flex-col gap-gf-md">
            <label className="flex flex-col gap-gf-xs">
              <span className="text-gf-caption text-gf-secondary">
                Picklist options
              </span>
              <div className="flex flex-col gap-gf-xs">
                {picklistOptions.map((option, index) => (
                  <div key={index} className="flex gap-gf-xs items-center">
                    <TextInput
                      value={option}
                      onChange={(v) => handleOptionChange(index, v)}
                      placeholder={`Option ${index + 1}`}
                      className="flex-1"
                    />
                    <button
                      type="button"
                      onClick={() => handleRemoveOption(index)}
                      className="px-gf-md py-gf-xs rounded-md bg-gf-elevated border border-gf-subtle text-gf-body text-gf-secondary hover:bg-gf-hover"
                    >
                      Remove
                    </button>
                  </div>
                ))}
              </div>
            </label>
            <button
              type="button"
              onClick={handleAddOption}
              className="px-gf-md py-gf-xs rounded-md bg-gf-elevated border border-gf-subtle text-gf-body text-gf-secondary hover:bg-gf-hover self-start"
            >
              Add option
            </button>
          </div>
        )}
      </div>
    </Modal>
  );
}
