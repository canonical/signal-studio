import { describe, it, expect } from "vitest";
import { componentType, componentQualifier } from "./api";

describe("componentType", () => {
  it("returns the full name when there is no qualifier", () => {
    expect(componentType("otlp")).toBe("otlp");
  });

  it("extracts the base type before the slash", () => {
    expect(componentType("filter/info")).toBe("filter");
  });

  it("handles multiple slashes", () => {
    expect(componentType("a/b/c")).toBe("a");
  });
});

describe("componentQualifier", () => {
  it("returns null when there is no qualifier", () => {
    expect(componentQualifier("otlp")).toBeNull();
  });

  it("extracts the qualifier after the slash", () => {
    expect(componentQualifier("filter/info")).toBe("info");
  });

  it("includes everything after the first slash", () => {
    expect(componentQualifier("a/b/c")).toBe("b/c");
  });
});
