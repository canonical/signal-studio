import { describe, it, expect } from "vitest";
import { coverageToFindings } from "./coverageToFindings";
import type { CoverageReport } from "../types/api";

function makeReport(
  overrides: Partial<CoverageReport["results"][number]>,
): CoverageReport {
  return {
    results: [
      {
        alertName: "TestAlert",
        alertGroup: "test",
        expr: "rate(x[5m]) > 0.1",
        metrics: [{ metricName: "x", filterOutcome: "dropped" }],
        status: "safe",
        ...overrides,
      },
    ],
    summary: { total: 1, safe: 1, atRisk: 0, broken: 0, wouldActivate: 0, unknown: 0 },
  };
}

describe("coverageToFindings", () => {
  it("skips safe alerts", () => {
    const findings = coverageToFindings(makeReport({ status: "safe" }));
    expect(findings).toHaveLength(0);
  });

  it("maps broken to critical severity", () => {
    const findings = coverageToFindings(makeReport({ status: "broken" }));
    expect(findings).toHaveLength(1);
    expect(findings[0]?.severity).toBe("critical");
    expect(findings[0]?.confidence).toBe("high");
    expect(findings[0]?.title).toContain("broken by filter");
  });

  it("maps at_risk to critical", () => {
    const findings = coverageToFindings(makeReport({ status: "at_risk" }));
    expect(findings[0]?.severity).toBe("critical");
    expect(findings[0]?.title).toContain("at risk");
  });

  it("maps would_activate to critical", () => {
    const findings = coverageToFindings(
      makeReport({ status: "would_activate" }),
    );
    expect(findings[0]?.severity).toBe("critical");
    expect(findings[0]?.title).toContain("falsely activate");
  });

  it("maps unknown to info with low confidence", () => {
    const findings = coverageToFindings(makeReport({ status: "unknown" }));
    expect(findings[0]?.severity).toBe("info");
    expect(findings[0]?.confidence).toBe("low");
  });

  it("includes scope with alert group", () => {
    const findings = coverageToFindings(
      makeReport({ status: "broken", alertGroup: "my-group" }),
    );
    expect(findings[0]?.scope).toBe("alert-group:my-group");
  });

  it("includes metrics evidence", () => {
    const findings = coverageToFindings(
      makeReport({
        status: "broken",
        metrics: [
          { metricName: "a", filterOutcome: "dropped" },
          { metricName: "b", filterOutcome: "kept" },
        ],
      }),
    );
    expect(findings[0]?.evidence).toContain("a: dropped");
    expect(findings[0]?.evidence).toContain("b: kept");
  });
});
