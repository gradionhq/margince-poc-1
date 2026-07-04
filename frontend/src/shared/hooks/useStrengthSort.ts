import { useState } from "react";

type StrengthSort = "-strength" | "strength" | undefined;

const CYCLE: StrengthSort[] = [undefined, "-strength", "strength"];

export function useStrengthSort() {
  const [index, setIndex] = useState(0);
  return {
    sort: CYCLE[index],
    toggle() {
      setIndex((i) => (i + 1) % CYCLE.length);
    },
  };
}
