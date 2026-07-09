import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useParams } from "react-router-dom";
import type { Organization } from "../../../lib/api-client/generated/index.js";
import { ToastContainer } from "../../../shared/ui/ToastContainer.js";
import {
  useOrganization,
  useUpdateOrganization,
} from "../../organizations/api/organizations.js";
import {
  buildAccountTree,
  findSuggestedEdgeCandidates,
  flattenTree,
} from "../api/accountTree.js";
import {
  recordsKeys,
  useAccountTreeOrgs,
  useOrganizationHierarchyRollup,
} from "../api/records.js";
import { AccountTree } from "../components/AccountTree.js";
import { RollupExplainBox } from "../components/RollupExplainBox.js";
import { RollupTilesBand } from "../components/RollupTilesBand.js";
import { ScopeToggle } from "../components/ScopeToggle.js";
import { SuggestedEdgeCard } from "../components/SuggestedEdgeCard.js";

// CandidateCard is an internal component that holds the mutation hook per candidate,
// keeping useUpdateOrganization out of a loop (hooks can't be called conditionally).
function CandidateCard({
  candidate,
  parentId,
  onDismiss,
  onSuccess,
  onError,
}: {
  candidate: Organization;
  parentId: string;
  onDismiss: () => void;
  onSuccess: () => void;
  onError: () => void;
}) {
  const qc = useQueryClient();
  const [status, setStatus] = useState<"staged" | "accepted">("staged");
  const updateOrg = useUpdateOrganization(candidate.id);

  function handleAccept() {
    updateOrg.mutate(
      { parent_org_id: parentId, version: candidate.version },
      {
        onSuccess: () => {
          setStatus("accepted");
          void qc.invalidateQueries({ queryKey: recordsKeys.treeOrgs() });
          void qc.invalidateQueries({
            queryKey: recordsKeys.rollup(parentId, "tree"),
          });
          void qc.invalidateQueries({
            queryKey: recordsKeys.rollup(parentId, "self"),
          });
          onSuccess();
        },
        onError: () => onError(),
      },
    );
  }

  return (
    <SuggestedEdgeCard
      candidate={candidate}
      parentId={parentId}
      status={status}
      onAccept={handleAccept}
      onDismiss={onDismiss}
    />
  );
}

export function AccountHierarchyPage() {
  const { id: rootId } = useParams<{ id: string }>();

  const [scope, setScope] = useState<"tree" | "self">("tree");
  const [expandedIds, setExpandedIds] = useState<Set<string>>(
    new Set(rootId ? [rootId] : []),
  );
  const [dismissedIds, setDismissedIds] = useState<Set<string>>(new Set());
  const [toasts, setToasts] = useState<
    Array<{ id: string; variant: "success" | "error"; message: string }>
  >([]);

  const { data: rootOrg } = useOrganization(rootId);
  const {
    data: rollup,
    isLoading: rollupLoading,
    isError: rollupError,
  } = useOrganizationHierarchyRollup(rootId, scope);
  const { data: selfRollup } = useOrganizationHierarchyRollup(rootId, "self");
  const { data: treeRollup } = useOrganizationHierarchyRollup(rootId, "tree");
  const { data: treeOrgs = [] } = useAccountTreeOrgs();

  function pushToast(variant: "success" | "error", message: string) {
    setToasts((t) => [...t, { id: String(Math.random()), variant, message }]);
  }

  function handleToggleExpand(id: string) {
    setExpandedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }

  const tree = rootId ? buildAccountTree(treeOrgs, rootId) : null;
  const rows = tree ? flattenTree(tree, expandedIds) : [];
  const treeIds = new Set(rows.map((r) => r.node.org.id));

  const candidates =
    rootOrg && treeOrgs.length > 0
      ? findSuggestedEdgeCandidates(treeOrgs, rootOrg, treeIds).filter(
          (c) => !dismissedIds.has(c.id),
        )
      : [];

  const nodeCount = rows.length;
  const depth = rows.reduce((max, r) => Math.max(max, r.depth), 0);

  // Derive explain-box figures from server rollups (never recomputed client-side).
  const selfFigure = selfRollup
    ? {
        amount_minor: selfRollup.weighted_pipeline.amount_minor ?? 0,
        currency: selfRollup.weighted_pipeline.currency ?? "EUR",
      }
    : {
        amount_minor: 0,
        currency: rollup?.weighted_pipeline.currency ?? "EUR",
      };
  const childrenSumFigure =
    treeRollup && selfRollup
      ? {
          amount_minor:
            (treeRollup.weighted_pipeline.amount_minor ?? 0) -
            (selfRollup.weighted_pipeline.amount_minor ?? 0),
          currency: treeRollup.weighted_pipeline.currency ?? "EUR",
        }
      : {
          amount_minor: 0,
          currency: rollup?.weighted_pipeline.currency ?? "EUR",
        };

  const isEmpty =
    !rollupLoading &&
    !rollupError &&
    tree &&
    rows.length <= 1 &&
    (rollup?.aggregated_account_count ?? 0) <= 1;

  return (
    <div className="min-h-screen bg-gf-page">
      <header className="flex items-center justify-between px-gf-lg py-gf-md border-b border-gf-subtle bg-gf-card">
        <h2 className="text-gf-title font-semibold text-gf-primary">
          Account Hierarchy
          {rootOrg ? ` — ${rootOrg.display_name}` : ""}
        </h2>
        <ScopeToggle scope={scope} onChange={setScope} />
      </header>
      <main className="p-gf-lg">
        <RollupTilesBand
          rollup={rollup}
          isLoading={rollupLoading}
          isError={rollupError}
          depth={depth}
          nodeCount={nodeCount}
        />
        {!rollupLoading && !rollupError && rollup && (
          <RollupExplainBox
            rollup={rollup}
            selfFigure={selfFigure}
            childrenSumFigure={childrenSumFigure}
          />
        )}
        {isEmpty ? (
          <p className="mt-gf-md text-gf-body text-gf-secondary">
            No sub-accounts in this hierarchy yet.
          </p>
        ) : (
          <div className="mt-gf-md">
            <AccountTree
              rows={rows}
              expandedIds={expandedIds}
              onToggleExpand={handleToggleExpand}
              restrictedNodes={rollup?.restricted_excluded ?? []}
            />
          </div>
        )}
        {candidates.length > 0 && (
          <div className="mt-gf-lg">
            <h3 className="mb-gf-sm text-gf-body font-medium text-gf-secondary">
              Suggested connections
            </h3>
            <div className="flex flex-col gap-gf-sm">
              {candidates.map((candidate) => (
                <CandidateCard
                  key={candidate.id}
                  candidate={candidate}
                  parentId={rootId ?? ""}
                  onDismiss={() =>
                    setDismissedIds((prev) => new Set([...prev, candidate.id]))
                  }
                  onSuccess={() =>
                    pushToast(
                      "success",
                      `${candidate.display_name} added as a child account.`,
                    )
                  }
                  onError={() =>
                    pushToast(
                      "error",
                      `Failed to link ${candidate.display_name}. Please try again.`,
                    )
                  }
                />
              ))}
            </div>
          </div>
        )}
      </main>
      <ToastContainer
        toasts={toasts}
        onDismiss={(id) =>
          setToasts((t) => t.filter((toast) => toast.id !== id))
        }
      />
    </div>
  );
}
