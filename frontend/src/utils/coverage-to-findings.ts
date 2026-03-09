import type { AlertStatus, CoverageReport, Finding, Severity } from "../types/api";

export const COVERAGE_SEVERITY: Partial<Record<AlertStatus, Severity>> = {
  broken: "critical",
  at_risk: "critical",
  would_activate: "critical",
  unknown: "info",
};

export const COVERAGE_TITLE: Record<string, (name: string) => string> = {
  broken: (n) => `Alert '${n}' broken by filter`,
  at_risk: (n) => `Alert '${n}' at risk from partial filtering`,
  would_activate: (n) => `Alert '${n}' would falsely activate`,
  unknown: (n) => `Alert '${n}' coverage unknown`,
};

export const COVERAGE_IMPLICATION: Record<string, string> = {
  broken: "This alert will never fire because one or more required metrics are being dropped by a filter processor.",
  at_risk: "Some series of the required metrics are partially filtered, which may cause inconsistent alert behaviour.",
  would_activate: "This alert uses absent() and the required metric is being dropped, which will cause the alert to fire unexpectedly.",
  unknown: "Unable to determine whether the metrics required by this alert are being collected.",
};

export const COVERAGE_RECOMMENDATION: Record<string, string> = {
  broken: "Review the filter processor configuration to ensure metrics required by this alert are preserved.",
  at_risk: "Check that attribute-level filters do not inadvertently remove series needed for this alert.",
  would_activate: "Either preserve the metric in the filter or disable the absent()-based alert to avoid false positives.",
  unknown: "Verify that the required metrics are being scraped and forwarded through the pipeline.",
};

export function coverageToFindings(report: CoverageReport): Finding[] {
  const out: Finding[] = [];
  for (const r of report.results) {
    const severity = COVERAGE_SEVERITY[r.status];
    if (!severity) continue; // safe → skip
    const metricsDetail = r.metrics
      .map((m) => `${m.metricName}: ${m.filterOutcome}`)
      .join(", ");
    out.push({
      ruleId: "alert-coverage",
      title: (COVERAGE_TITLE[r.status] ?? ((n: string) => n))(r.alertName),
      severity,
      confidence: r.status === "unknown" ? "low" : "high",
      evidence: `Metrics: ${metricsDetail}`,
      implication: COVERAGE_IMPLICATION[r.status] ?? "",
      recommendation: COVERAGE_RECOMMENDATION[r.status] ?? "",
      snippet: r.expr,
      scope: `alert-group:${r.alertGroup}`,
    });
  }
  return out;
}
