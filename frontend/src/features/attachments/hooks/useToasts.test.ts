import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { useToasts } from "./useToasts.js";

describe("useToasts", () => {
  it("starts empty", () => {
    const { result } = renderHook(() => useToasts());
    expect(result.current.toasts).toEqual([]);
  });

  it("pushes a toast with variant and message", () => {
    const { result } = renderHook(() => useToasts());
    act(() => result.current.pushToast("success", "Saved."));
    expect(result.current.toasts).toHaveLength(1);
    expect(result.current.toasts[0]).toMatchObject({
      variant: "success",
      message: "Saved.",
    });
  });

  it("assigns each pushed toast a unique id", () => {
    const { result } = renderHook(() => useToasts());
    act(() => {
      result.current.pushToast("info", "First");
      result.current.pushToast("error", "Second");
    });
    expect(result.current.toasts).toHaveLength(2);
    const [first, second] = result.current.toasts;
    expect(first.id).toBeTruthy();
    expect(second.id).toBeTruthy();
    expect(first.id).not.toBe(second.id);
  });

  it("preserves push order", () => {
    const { result } = renderHook(() => useToasts());
    act(() => {
      result.current.pushToast("info", "First");
      result.current.pushToast("error", "Second");
    });
    expect(result.current.toasts.map((toast) => toast.message)).toEqual([
      "First",
      "Second",
    ]);
  });

  it("dismisses a toast by id, leaving the rest", () => {
    const { result } = renderHook(() => useToasts());
    act(() => {
      result.current.pushToast("info", "First");
      result.current.pushToast("error", "Second");
    });
    const [first, second] = result.current.toasts;
    act(() => result.current.dismissToast(first.id));
    expect(result.current.toasts).toHaveLength(1);
    expect(result.current.toasts[0].id).toBe(second.id);
  });

  it("dismissing an unknown id is a no-op", () => {
    const { result } = renderHook(() => useToasts());
    act(() => result.current.pushToast("info", "First"));
    act(() => result.current.dismissToast("does-not-exist"));
    expect(result.current.toasts).toHaveLength(1);
  });
});
