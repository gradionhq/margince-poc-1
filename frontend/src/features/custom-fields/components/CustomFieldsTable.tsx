import { Archive } from "lucide-react";
import type { MouseEvent } from "react";
import type {
  CustomField,
  Member,
} from "../../../lib/api-client/generated/index.js";
import { ContextMenu } from "../../../shared/ui/ContextMenu.js";
import { DataTable } from "../../../shared/ui/DataTable.js";
import { FieldGuard } from "../../../shared/ui/FieldGuard.js";
import {
  Chip,
  EmptyState,
  IconButton,
  StatusBadge,
} from "../../../shared/ui/forge.js";
import type { ObjectKey } from "../lib/customFieldRules.js";
import {
  buildApiKey,
  OBJECT_CHIPS,
  resolveMemberName,
} from "../lib/customFieldRules.js";

export function CustomFieldsTable({
  fields,
  members,
  selectedObject,
  role,
  onEdit,
  onRetire,
  stagedRow,
  onObjectSelect,
}: {
  fields: CustomField[];
  members: Member[];
  selectedObject: ObjectKey;
  role: string;
  onEdit?: (f: CustomField) => void;
  onRetire?: (f: CustomField) => void;
  stagedRow?: { label: string; type: string } | null;
  onObjectSelect?: (obj: ObjectKey) => void;
}) {
  const shouldShowEmpty = fields.length === 0 && !stagedRow;
  const isAdmin = role === "admin";

  const countBadge = fields.length + (stagedRow ? 1 : 0);

  function stopTriggerClick(event?: MouseEvent<HTMLButtonElement>) {
    event?.stopPropagation();
  }

  type TableRow =
    | { type: "staged"; id: string; staged: { label: string; type: string } }
    | { type: "field"; id: string; field: CustomField };

  const tableRows: TableRow[] = [];

  if (stagedRow) {
    tableRows.push({
      type: "staged",
      id: "staged",
      staged: stagedRow,
    });
  }

  for (const field of fields) {
    tableRows.push({
      type: "field",
      id: field.id,
      field,
    });
  }

  const columns = [
    {
      key: "label",
      header: "Label",
      render: (row: TableRow) => {
        if (row.type === "staged") {
          return <span>writing…</span>;
        }
        return <span>{row.field.label}</span>;
      },
    },
    {
      key: "apiKey",
      header: "API Key",
      render: (row: TableRow) => {
        if (row.type === "staged") {
          return <span className="font-mono">—</span>;
        }
        const field = row.field;
        const slug = field.slug || "";
        const apiKey = buildApiKey(field.object, slug);
        return <span className="font-mono">{apiKey}</span>;
      },
    },
    {
      key: "type",
      header: "Type",
      render: (row: TableRow) => {
        if (row.type === "staged") {
          return <Chip>{row.staged.type}</Chip>;
        }
        const field = row.field;
        if (field.status === "retired") {
          return <StatusBadge label="Retired" variant="neutral" />;
        }
        return <Chip>{field.type}</Chip>;
      },
    },
    {
      key: "addedBy",
      header: "Added by",
      render: (row: TableRow) => {
        if (row.type === "staged") {
          return <span>—</span>;
        }
        const field = row.field;
        const memberName = resolveMemberName(members, field.created_by);
        const mode = isAdmin ? "visible" : "masked";
        return <FieldGuard mode={mode}>{memberName}</FieldGuard>;
      },
    },
    {
      key: "actions",
      header: "",
      render: (row: TableRow) => {
        if (row.type === "staged") {
          return null;
        }

        const field = row.field;

        if (field.status === "retired") {
          return null;
        }

        if (!isAdmin) {
          return null;
        }

        return (
          <ContextMenu
            trigger={
              <IconButton
                icon="MoreVertical"
                label="Row actions"
                onClick={stopTriggerClick}
              />
            }
            items={[
              {
                id: "edit",
                label: "Edit",
                onSelect: () => onEdit?.(field),
              },
              {
                id: "archive",
                label: "Archive",
                onSelect: () => onRetire?.(field),
              },
            ]}
          />
        );
      },
    },
  ];

  const getRowProps = (row: TableRow) => {
    if (row.type === "staged") {
      return { "data-staged": "true" };
    }
    if (row.field.status === "retired") {
      return { className: "opacity-60" };
    }
    return {};
  };

  return (
    <div className="space-y-gf-md">
      <div className="flex gap-gf-sm">
        {OBJECT_CHIPS.map((chip) => {
          const isSelected = chip.value === selectedObject;
          return (
            <button
              key={chip.value}
              type="button"
              data-selected={isSelected}
              data-testid={`chip-${chip.value}`}
              className={
                isSelected
                  ? "relative cursor-pointer bg-transparent border-0 p-0"
                  : "cursor-pointer bg-transparent border-0 p-0"
              }
              onClick={() => onObjectSelect?.(chip.value)}
            >
              <Chip>{chip.label}</Chip>
              {isSelected && (
                <span
                  data-testid="object-count"
                  className="absolute -top-2 -right-2 inline-flex items-center justify-center w-5 h-5 text-xs font-bold rounded-full bg-gf-status-info text-white"
                >
                  {countBadge}
                </span>
              )}
            </button>
          );
        })}
      </div>

      <div className="text-gf-body text-gf-secondary">
        Core fields are not shown — they aren't editable here.
      </div>

      {shouldShowEmpty ? (
        <EmptyState
          icon={Archive}
          title="No custom fields on this object yet"
        />
      ) : (
        <DataTable
          columns={columns}
          rows={tableRows}
          getRowKey={(row) => row.id}
          getRowProps={getRowProps}
          onRowClick={undefined}
        />
      )}
    </div>
  );
}
