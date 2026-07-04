// View-only staging: no Confirm/create call is wired here. The deals-create module
// (frontend/src/features/deals/) doesn't exist on main yet (Global Constraints gap #2), so there
// is nothing to POST to — this modal only stages context (org + known contacts) and closes on
// Cancel/X. Not a silent omission: called out explicitly here and in the PR description.
import type {
  Organization,
  Person,
} from "../../../lib/api-client/generated/index.js";
import { Modal } from "../../../shared/ui/forge.js";

export function NewDealModal({
  open,
  onClose,
  org,
  contacts,
}: {
  open: boolean;
  onClose: () => void;
  org: Organization;
  contacts: Person[];
}) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title="New deal"
      subtitle="Staged from this account"
    >
      <div className="px-gf-xl py-gf-lg flex flex-col gap-gf-md">
        <div>
          <p className="text-gf-caption text-gf-secondary">Company</p>
          <p className="text-gf-body text-gf-primary font-medium">
            {org.display_name}
          </p>
        </div>
        <div>
          <p className="text-gf-caption text-gf-secondary">Contacts</p>
          {contacts.length === 0 ? (
            <p className="text-gf-body text-gf-muted">
              No known contacts to pre-link.
            </p>
          ) : (
            <ul>
              {contacts.map((c) => (
                <li key={c.id} className="text-gf-body text-gf-primary">
                  {c.full_name}
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>
    </Modal>
  );
}
