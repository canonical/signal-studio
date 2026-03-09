import { describe, it, expect } from "vitest";
import { rulesForRole } from "./pipeline-graph";

describe("rulesForRole", () => {
  it("returns receiver rules including pipeline rules", () => {
    const rules = rulesForRole("receivers");
    expect(rules).toContain("receiver-endpoint-wildcard");
    expect(rules).toContain("undefined-component-ref");
    expect(rules).toContain("empty-pipeline");
  });

  it("returns processor rules including pipeline rules", () => {
    const rules = rulesForRole("processors");
    expect(rules).toContain("missing-memory-limiter");
    expect(rules).toContain("missing-batch");
    expect(rules).toContain("empty-pipeline");
    expect(rules).toContain("undefined-component-ref");
  });

  it("returns exporter rules including pipeline rules", () => {
    const rules = rulesForRole("exporters");
    expect(rules).toContain("debug-exporter-in-pipeline");
    expect(rules).toContain("exporter-no-sending-queue");
    expect(rules).toContain("empty-pipeline");
  });

  it("does not leak rules across roles", () => {
    const receiverRules = rulesForRole("receivers");
    const exporterRules = rulesForRole("exporters");
    expect(receiverRules).not.toContain("debug-exporter-in-pipeline");
    expect(exporterRules).not.toContain("receiver-endpoint-wildcard");
  });
});
