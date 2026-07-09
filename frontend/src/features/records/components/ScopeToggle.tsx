import { RadioGroup } from "../../../shared/ui/forge.js";

export function ScopeToggle({
  scope,
  onChange,
}: {
  scope: "tree" | "self";
  onChange: (scope: "tree" | "self") => void;
}) {
  return (
    <RadioGroup
      label="Scope"
      name="hierarchy-scope"
      value={scope}
      onChange={(v) => onChange(v as "tree" | "self")}
      options={[
        { value: "tree", label: "Whole tree (roll-up)" },
        { value: "self", label: "This account only (self)" },
      ]}
    />
  );
}
