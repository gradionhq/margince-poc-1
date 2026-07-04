import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { useStrengthSort } from "./useStrengthSort.js";

describe("useStrengthSort", () => {
  it("toggles undefined -> -strength -> strength -> undefined", () => {
    const { result } = renderHook(() => useStrengthSort());
    expect(result.current.sort).toBeUndefined();
    act(() => result.current.toggle());
    expect(result.current.sort).toBe("-strength");
    act(() => result.current.toggle());
    expect(result.current.sort).toBe("strength");
    act(() => result.current.toggle());
    expect(result.current.sort).toBeUndefined();
  });
});
