import { useState } from "react";
import type { Organization } from "../../../lib/api-client/generated/index.js";
import { Button, Modal, TextInput } from "../../../shared/ui/forge.js";
import { useUpdateOrganization } from "../api/organizations.js";

const SIZE_BANDS = [
  "1-10",
  "11-50",
  "51-200",
  "201-500",
  "501-1000",
  "1001-5000",
  "5000+",
] as const;

export function EditOrgModal({
  open,
  onClose,
  org,
  onSaved,
}: {
  open: boolean;
  onClose: () => void;
  org: Organization;
  onSaved: (changedFields: string[]) => void;
}) {
  const [industry, setIndustry] = useState(org.industry ?? "");
  const [sizeBand, setSizeBand] = useState(org.size_band ?? "");
  const [city, setCity] = useState(org.address?.city ?? "");
  const [country, setCountry] = useState(org.address?.country ?? "");
  const mutation = useUpdateOrganization(org.id);

  function handleSave() {
    const changed: string[] = [];
    if (industry !== (org.industry ?? "")) changed.push("industry");
    if (sizeBand !== (org.size_band ?? "")) changed.push("size_band");
    if (
      city !== (org.address?.city ?? "") ||
      country !== (org.address?.country ?? "")
    ) {
      changed.push("location");
    }
    mutation.mutate(
      {
        industry: industry || null,
        size_band: (sizeBand || null) as Organization["size_band"],
        address: {
          ...org.address,
          city: city || null,
          country: country || null,
        },
        version: org.version,
      },
      {
        onSuccess: () => {
          onSaved(changed);
          onClose();
        },
      },
    );
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Edit company"
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button
            variant="primary"
            onClick={handleSave}
            loading={mutation.isPending}
          >
            Save
          </Button>
        </>
      }
    >
      <div className="px-gf-xl py-gf-lg flex flex-col gap-gf-md">
        <div>
          <p className="text-gf-caption text-gf-secondary mb-gf-xs">Industry</p>
          <TextInput value={industry} onChange={setIndustry} />
        </div>
        <div>
          <p className="text-gf-caption text-gf-secondary mb-gf-xs">
            Staff size
          </p>
          <select
            value={sizeBand ?? ""}
            onChange={(e) =>
              setSizeBand(e.target.value as Organization["size_band"])
            }
            className="h-10 w-full rounded-md border border-gf-subtle bg-gf-elevated text-gf-body text-gf-primary px-gf-md"
          >
            <option value="">—</option>
            {SIZE_BANDS.map((b) => (
              <option key={b} value={b}>
                {b}
              </option>
            ))}
          </select>
        </div>
        <div>
          <p className="text-gf-caption text-gf-secondary mb-gf-xs">City</p>
          <TextInput value={city} onChange={setCity} />
        </div>
        <div>
          <p className="text-gf-caption text-gf-secondary mb-gf-xs">Country</p>
          <TextInput value={country} onChange={setCountry} />
        </div>
      </div>
    </Modal>
  );
}
