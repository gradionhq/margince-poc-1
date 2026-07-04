import { useState } from "react";
import { apiClient } from "../../../lib/api-client/client.js";
import { Button, Modal } from "../../../shared/ui/forge.js";
import { useAuthStore } from "../../identity/store/authStore.js";
import { useOrganizations } from "../../organizations/api/organizations.js";
import {
  useCreateDeal,
  useOpenDealsForOrg,
  useOrgEmploymentRelationships,
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
  const { data: employmentRelationships } =
    useOrgEmploymentRelationships(organizationId);
  const createDeal = useCreateDeal();
  const hasDuplicate = (existingOpen?.data.length ?? 0) > 0;
  const stakeholderPersonIds = (employmentRelationships?.data ?? [])
    .map((r) => r.person_id)
    .filter((id): id is string => !!id);
  const [createError, setCreateError] = useState<string | null>(null);

  // Extracts an honest, specific cause from a failed create/pre-attach call — mirrors
  // PipelineBoard's advanceErrorMessage rather than surfacing a generic failure or
  // (worse) silently swallowing it.
  function errorMessage(error: unknown): string {
    if (error && typeof error === "object") {
      const problem = error as { detail?: unknown; code?: unknown };
      if (typeof problem.detail === "string" && problem.detail.length > 0) {
        return problem.detail;
      }
      if (typeof problem.code === "string" && problem.code.length > 0) {
        return `Create failed (${problem.code})`;
      }
    }
    return "Create failed — please try again.";
  }

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
                setCreateError(null);
                const capturedBy = `human:${user?.id ?? "unknown"}`;
                let deal: { id: string };
                try {
                  deal = await createDeal.mutateAsync({
                    name,
                    organization_id: organizationId,
                    pipeline_id: defaultPipelineId,
                    stage_id: defaultStageId,
                    source: "manual",
                    captured_by: capturedBy,
                  });
                } catch (err) {
                  setCreateError(errorMessage(err));
                  return;
                }
                // AC-pipeline-9/10: pre-attach the org's current employment relationships as
                // deal_stakeholder edges once the deal exists — one createRelationship POST per
                // person, batched, fired only after the deal create succeeds. The deal already
                // exists at this point, so a pre-attach failure is surfaced distinctly (not as a
                // generic create failure) and does NOT call onCreated — the modal stays open so
                // the rep sees the honest state rather than a silent no-op.
                try {
                  await Promise.all(
                    stakeholderPersonIds.map((personId) =>
                      apiClient.POST("/relationships", {
                        body: {
                          kind: "deal_stakeholder",
                          deal_id: deal.id,
                          person_id: personId,
                          source: "manual",
                          captured_by: capturedBy,
                          is_current_primary: false,
                        },
                      }),
                    ),
                  );
                } catch {
                  setCreateError(
                    "Deal was created, but pre-attaching stakeholders failed — check the deal record.",
                  );
                  return;
                }
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
            {createError && (
              <p className="text-gf-caption text-gf-status-danger">
                {createError}
              </p>
            )}
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
            <p className="text-gf-caption text-gf-secondary">
              {stakeholderPersonIds.length} stakeholder
              {stakeholderPersonIds.length === 1 ? "" : "s"} will be
              pre-attached.
            </p>
          </>
        )}
      </div>
    </Modal>
  );
}
