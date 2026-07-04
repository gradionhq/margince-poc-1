import { useState } from "react";

export function useStrengthSort(): {
  sort: "-strength" | "strength" | undefined;
  toggle: () => void;
} {
  const [sort, setSort] = useState<"-strength" | "strength" | undefined>(
    undefined,
  );

  function toggle() {
    setSort((prev) => {
      if (prev === undefined) return "-strength";
      if (prev === "-strength") return "strength";
      return undefined;
    });
  }

  return { sort, toggle };
}
