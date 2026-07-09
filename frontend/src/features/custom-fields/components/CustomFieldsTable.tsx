import type { CustomField, Member } from "../../../lib/api-client/generated/index.js";
import type { ObjectKey } from "../lib/customFieldRules.js";
import { OBJECT_CHIPS, buildApiKey, resolveMemberName } from "../lib/customFieldRules.js";
import { Chip, EmptyState, StatusBadge, IconButton } from "../../../shared/ui/forge.js";
import { DataTable } from "../../../shared/ui/DataTable.js";
import { FieldGuard } from "../../../shared/ui/FieldGuard.js";
import { ContextMenu } from "../../../shared/ui/ContextMenu.js";

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

  // Calculate the count badge value for the selected chip
  const countBadge = fields.length + (stagedRow ? 1 : 0);

  function stopTriggerClick(...args: [unknown?]) {
    const event = args[0];
    if (
      event &&
      typeof event === "object" &&
      "stopPropagation" in event &&
      typeof event.stopPropagation === "function"
    ) {
      event.stopPropagation();
    }
  }

  // Build rows for the table: staged row first (if present), then real fields
  const tableRows: Array<{ type: "staged" | "field"; id: string; field?: CustomField; staged?: { label: string; type: string } }> = [];

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
      render: (row: (typeof tableRows)[0]) => {
        if (row.type === "staged") {
          return <span>writing…</span>;
        }
        return <span>{row.field!.label}</span>;
      },
    },
    {
      key: "apiKey",
      header: "API Key",
      render: (row: (typeof tableRows)[0]) => {
        if (row.type === "staged") {
          return <span className="font-mono">—</span>;
        }
        const field = row.field!;
        // For staged rows or when slug is empty, derive from label
        const slug = field.slug || "";
        const apiKey = buildApiKey(field.object, slug);
        return <span className="font-mono">{apiKey}</span>;
      },
    },
    {
      key: "type",
      header: "Type",
      render: (row: (typeof tableRows)[0]) => {
        if (row.type === "staged") {
          return <Chip>{row.staged!.type}</Chip>;
        }
        const field = row.field!;
        if (field.status === "retired") {
          return <StatusBadge>Retired</StatusBadge>;
        }
        return <Chip>{field.type}</Chip>;
      },
    },
    {
      key: "addedBy",
      header: "Added by",
      render: (row: (typeof tableRows)[0]) => {
        if (row.type === "staged") {
          return <span>—</span>;
        }
        const field = row.field!;
        const memberName = resolveMemberName(members, field.created_by);
        const mode = isAdmin ? "visible" : "masked";
        return <FieldGuard mode={mode}>{memberName}</FieldGuard>;
      },
    },
    {
      key: "actions",
      header: "",
      render: (row: (typeof tableRows)[0]) => {
        // No actions for staged rows
        if (row.type === "staged") {
          return null;
        }

        const field = row.field!;

        // No actions for retired fields
        if (field.status === "retired") {
          return null;
        }

        // No actions for non-admins
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

  const getRowProps = (row: (typeof tableRows)[0]) => {
    const props: Record<string, any> = {};

    if (row.type === "staged") {
      props["data-staged"] = "true";
    } else if (row.field!.status === "retired") {
      props.className = "opacity-60";
    }

    return props;
  };

  return (
    <div className="space-y-gf-md">
      {/* Object chips header */}
      <div className="flex gap-gf-sm">
        {OBJECT_CHIPS.map((chip) => {
          const isSelected = chip.value === selectedObject;
          return (
            <div
              key={chip.value}
              data-selected={isSelected}
              data-testid={`chip-${chip.value}`}
              className={isSelected ? "relative cursor-pointer" : "cursor-pointer"}
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
            </div>
          );
        })}
      </div>

      {/* Explanatory note */}
      <div className="text-gf-body text-gf-secondary">
        Core fields are not shown — they aren't editable here.
      </div>

      {/* Empty state or table */}
      {shouldShowEmpty ? (
        <EmptyState />
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
