import { Link } from "react-router-dom";
import type { Organization } from "../../../lib/api-client/generated/index.js";
import { SectionHeader } from "../../../shared/ui/forge.js";
import { stalledDeal } from "../api/orgSelectors.js";

export function AccountSignalCard({ org }: { org: Organization }) {
  const deal = stalledDeal(org.deals ?? []);
  return (
    <div className="p-gf-lg rounded-lg border border-gf-subtle bg-gf-card">
      <SectionHeader label="Account signal" />
      {deal ? (
        <p className="mt-gf-sm text-gf-body text-gf-secondary">
          Single-threaded on the top contact —{" "}
          <Link to={`/deals/${deal.id}`} className="text-gf-accent underline">
            open the deal
          </Link>
        </p>
      ) : (
        <p className="mt-gf-sm text-gf-body text-gf-muted">No account signal to flag right now.</p>
      )}
    </div>
  );
}
