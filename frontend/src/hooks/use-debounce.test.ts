import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useDebounce } from "./use-debounce";

describe("useDebounce", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("returns the initial value immediately", () => {
    const { result } = renderHook(() => useDebounce("hello", 500));
    expect(result.current).toBe("hello");
  });

  it("debounces value changes", () => {
    const { result, rerender } = renderHook(
      ({ value, delay }) => useDebounce(value, delay),
      { initialProps: { value: "a", delay: 300 } },
    );

    // Change value
    rerender({ value: "b", delay: 300 });
    expect(result.current).toBe("a"); // not yet updated

    // Advance past the delay
    act(() => {
      vi.advanceTimersByTime(300);
    });
    expect(result.current).toBe("b");
  });

  it("resets the timer on rapid changes", () => {
    const { result, rerender } = renderHook(
      ({ value, delay }) => useDebounce(value, delay),
      { initialProps: { value: "a", delay: 200 } },
    );

    rerender({ value: "b", delay: 200 });
    act(() => {
      vi.advanceTimersByTime(100);
    });

    // Change again before debounce fires
    rerender({ value: "c", delay: 200 });
    act(() => {
      vi.advanceTimersByTime(100);
    });

    // "b" timer was cleared, still waiting for "c"
    expect(result.current).toBe("a");

    act(() => {
      vi.advanceTimersByTime(100);
    });
    expect(result.current).toBe("c");
  });
});
