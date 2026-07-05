import type { Organization } from "../../../lib/api-client/generated/index.js";
import { ContextMenu } from "../../../shared/ui/ContextMenu.js";
import { IconButton } from "../../../shared/ui/forge.js";
import { OrgLogo } from "./OrgLogo.js";
import { OrgStrengthCell } from "./OrgStrengthCell.js";

export function CompanyRow({
  org,
  onClick,
  onArchive,
}: {
  org: Organization;
  onClick?: () => void;
  onArchive?: (id: string) => void;
}) {
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
  const strength = org.org_strength
    ? {
        score: org.org_strength.score,
        bucket: org.org_strength.bucket,
        top_person_id: org.org_strength.top_person_id,
        top_person_name: org.org_strength.top_person_name,
      }
    : null;

  return (
    <tr
      className="border-t border-gf-subtle hover:bg-gf-hover cursor-pointer"
      onClick={onClick}
    >
      <td className="p-gf-sm">
        <div className="flex items-center gap-gf-sm">
          <OrgLogo name={org.display_name} size="sm" />
          <div>
            <p className="text-gf-body font-medium text-gf-primary">
              {org.display_name}
            </p>
            {org.industry && (
              <p className="text-gf-caption text-gf-secondary">
                {org.industry}
              </p>
            )}
          </div>
        </div>
      </td>
      <td className="p-gf-sm text-gf-body text-gf-primary">
        {org.contact_count ?? 0}
      </td>
      <td className="p-gf-sm text-gf-body text-gf-primary">
        {org.open_deal_count ?? 0}
      </td>
      <td className="p-gf-sm">
        <OrgStrengthCell
          strength={strength}
          contactCount={org.contact_count ?? 0}
        />
      </td>
      <td className="p-gf-sm">
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
              id: "archive",
              label: "Archive",
              onSelect: () => onArchive?.(org.id),
            },
          ]}
        />
      </td>
    </tr>
  );
}
