import { useState } from "react";
import { Button, Modal } from "../../../shared/ui/forge.js";
import { useAuthStore } from "../../identity/store/authStore.js";
import { useOrganizations } from "../../organizations/api/organizations.js";
import {
  useCreateDeal,
  useOpenDealsForOrg,
  useRecentActivityCount,
} from "../api/deals.js";

export function NewDealModal({
  open,
  onClose,
  organizationId: presetOrgId,
  defaultPipelineId,
  defaultStageId,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  organizationId?: string;
  defaultPipelineId: string;
  defaultStageId: string;
  onCreated: () => void;
}) {
  const { user } = useAuthStore();
  const [pickedOrgId, setPickedOrgId] = useState<string | undefined>(undefined);
  const organizationId = presetOrgId ?? pickedOrgId;
  const { data: orgList } = useOrganizations({});
  const organization = orgList?.data.find((o) => o.id === organizationId);
  const [name, setName] = useState("");
  const { data: existingOpen } = useOpenDealsForOrg(organizationId);
  const { data: activityCount } = useRecentActivityCount(organizationId);
  const createDeal = useCreateDeal();
  const hasDuplicate = (existingOpen?.data.length ?? 0) > 0;

  // Once an org is picked/preset for the first time, seed the suggested deal name — but never
  // clobber a name the rep has already started editing.
  function pickOrg(id: string, displayName: string) {
    setPickedOrgId(id);
    if (name === "") setName(`${displayName} deal`);
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="New deal"
      subtitle={
        organization
          ? `For ${organization.display_name}`
          : "Pick a company to start"
      }
      footer={
        organizationId ? (
          <>
            <Button variant="secondary" onClick={onClose}>
              Cancel
            </Button>
            <Button
              variant="primary"
              loading={createDeal.isPending}
              onClick={async () => {
                await createDeal.mutateAsync({
                  name,
                  organization_id: organizationId,
                  pipeline_id: defaultPipelineId,
                  stage_id: defaultStageId,
                  source: "manual",
                  captured_by: `human:${user?.id ?? "unknown"}`,
                });
                onCreated();
              }}
            >
              Confirm & create
            </Button>
          </>
        ) : (
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
        )
      }
    >
      <div className="px-gf-xl py-gf-lg flex flex-col gap-gf-md">
        {!organizationId && (
          <ul className="flex flex-col gap-gf-xs">
            {(orgList?.data ?? []).map((org) => (
              <li key={org.id}>
                <button
                  type="button"
                  onClick={() => pickOrg(org.id, org.display_name)}
                  className="w-full text-left px-gf-md py-gf-sm rounded-md hover:bg-gf-hover text-gf-body text-gf-primary"
                >
                  {org.display_name}
                </button>
              </li>
            ))}
          </ul>
        )}
        {organizationId && (
          <>
            {hasDuplicate && (
              <p className="text-gf-caption text-gf-status-warning">
                {organization?.display_name ?? "This company"} already has an
                open deal.
              </p>
            )}
            <label className="flex flex-col gap-gf-xs">
              <span className="text-gf-caption text-gf-secondary">
                Deal name
              </span>
              <input
                type="text"
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="h-10 w-full rounded-md bg-gf-elevated border border-gf-subtle text-gf-body text-gf-primary px-gf-md"
              />
            </label>
            <p className="text-gf-caption text-gf-secondary">
              {activityCount ?? 0} recent activities will be linked.
            </p>
          </>
        )}
      </div>
    </Modal>
  );
}
