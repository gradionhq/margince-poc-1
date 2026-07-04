import type { Organization } from "../../../lib/api-client/generated/index.js";
import { OrgLogo } from "./OrgLogo.js";
import { OrgStrengthCell } from "./OrgStrengthCell.js";

export function CompanyRow({
  org,
  onClick,
}: {
  org: Organization;
  onClick?: () => void;
}) {
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
    </tr>
  );
}
