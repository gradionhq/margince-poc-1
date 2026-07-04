import { useRef, useState } from "react";
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
  // Mirrors of the four fields above, kept in sync on every input/change. React state updates
  // are batched/scheduled — a Save click that follows a keystroke in the same synchronous tick
  // (as happens when a native "input" event is dispatched without an intervening render, e.g. in
  // tests that drive the DOM directly rather than via userEvent) can otherwise read a stale
  // closure value. Refs are mutated synchronously and are what handleSave actually reads, so the
  // comparison below is never one render behind; the state above exists purely to keep the
  // controlled inputs' displayed value in sync with what was typed.
  const industryRef = useRef(industry);
  const sizeBandRef = useRef(sizeBand);
  const cityRef = useRef(city);
  const countryRef = useRef(country);
  const mutation = useUpdateOrganization(org.id);

  function updateIndustry(v: string) {
    industryRef.current = v;
    setIndustry(v);
  }
  function updateSizeBand(v: Organization["size_band"]) {
    sizeBandRef.current = v;
    setSizeBand(v);
  }
  function updateCity(v: string) {
    cityRef.current = v;
    setCity(v);
  }
  function updateCountry(v: string) {
    countryRef.current = v;
    setCountry(v);
  }

  function handleSave() {
    const changed: string[] = [];
    if (industryRef.current !== (org.industry ?? "")) changed.push("industry");
    if (sizeBandRef.current !== (org.size_band ?? ""))
      changed.push("size_band");
    if (
      cityRef.current !== (org.address?.city ?? "") ||
      countryRef.current !== (org.address?.country ?? "")
    ) {
      changed.push("location");
    }
    mutation.mutate(
      {
        industry: industryRef.current || null,
        size_band: (sizeBandRef.current || null) as Organization["size_band"],
        address: {
          ...org.address,
          city: cityRef.current || null,
          country: countryRef.current || null,
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
        {/* onInput (bubble-phase, native) mirrors TextInput's own onChange into the refs above.
            It is redundant with onChange in the browser but is what actually keeps the refs
            current when a caller dispatches a bare native "input" event directly on the DOM node
            (see the ref comment above) rather than going through userEvent/fireEvent. */}
        <div
          onInput={(e) => updateIndustry((e.target as HTMLInputElement).value)}
        >
          <p className="text-gf-caption text-gf-secondary mb-gf-xs">Industry</p>
          <TextInput value={industry} onChange={updateIndustry} />
        </div>
        <div
          onInput={(e) =>
            updateSizeBand(
              (e.target as HTMLSelectElement)
                .value as Organization["size_band"],
            )
          }
        >
          <p className="text-gf-caption text-gf-secondary mb-gf-xs">
            Staff size
          </p>
          <select
            value={sizeBand ?? ""}
            onChange={(e) =>
              updateSizeBand(e.target.value as Organization["size_band"])
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
        <div onInput={(e) => updateCity((e.target as HTMLInputElement).value)}>
          <p className="text-gf-caption text-gf-secondary mb-gf-xs">City</p>
          <TextInput value={city} onChange={updateCity} />
        </div>
        <div
          onInput={(e) => updateCountry((e.target as HTMLInputElement).value)}
        >
          <p className="text-gf-caption text-gf-secondary mb-gf-xs">Country</p>
          <TextInput value={country} onChange={updateCountry} />
        </div>
      </div>
    </Modal>
  );
}
