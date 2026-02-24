import { useState } from "react";
import type { Finding, Severity } from "../types/api";

interface FindingsPanelProps {
  findings: Finding[];
}

const SEVERITY_ORDER: Severity[] = ["critical", "warning", "info"];

const SEVERITY_COLORS: Record<Severity, string> = {
  critical: "#c7162b",
  warning: "#f99b11",
  info: "#24598f",
};

const SEVERITY_LABEL: Record<Severity, string> = {
  critical: "critical",
  warning: "warning",
  info: "info",
};

export function FindingsPanel({ findings }: FindingsPanelProps) {
  const sorted = [...findings].sort(
    (a, b) =>
      SEVERITY_ORDER.indexOf(a.severity) - SEVERITY_ORDER.indexOf(b.severity),
  );

  if (findings.length === 0) {
    return <p className="u-text--muted">No issues found.</p>;
  }

  return (
    <ul className="p-list--divided u-no-margin--bottom">
      {sorted.map((f, i) => (
        <FindingItem key={`${f.ruleId}-${f.pipeline}-${i}`} finding={f} />
      ))}
    </ul>
  );
}

function FindingItem({ finding }: { finding: Finding }) {
  const [expanded, setExpanded] = useState(false);
  const [copied, setCopied] = useState(false);

  function handleCopy(e: React.MouseEvent) {
    e.stopPropagation();
    navigator.clipboard.writeText(finding.snippet);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  const color = SEVERITY_COLORS[finding.severity];

  return (
    <li
      className="p-list__item"
      style={{ cursor: "pointer", borderLeft: `3px solid ${color}`, paddingLeft: "0.5rem" }}
      onClick={() => setExpanded(!expanded)}
    >
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "baseline", gap: "0.25rem" }}>
        <strong style={{ fontSize: "0.875rem" }}>{finding.title}</strong>
        <span
          className="p-chip is-inline is-dense"
          style={{ flexShrink: 0, borderColor: color, color }}
        >
          {SEVERITY_LABEL[finding.severity]}
        </span>
      </div>
      <p className="u-text--muted" style={{ fontSize: "0.8rem", margin: "0.25rem 0 0" }}>
        {finding.explanation}
      </p>

      {expanded && (
        <div
          style={{ marginTop: "0.5rem", fontSize: "0.8rem" }}
          onClick={(e) => e.stopPropagation()}
        >
          <dl style={{ margin: 0 }}>
            <dt style={{ fontWeight: 600 }}>Evidence</dt>
            <dd style={{ marginLeft: 0, marginBottom: "0.25rem" }}>{finding.evidence}</dd>
            <dt style={{ fontWeight: 600 }}>Why it matters</dt>
            <dd style={{ marginLeft: 0, marginBottom: "0.25rem" }}>{finding.whyItMatters}</dd>
            <dt style={{ fontWeight: 600 }}>Impact</dt>
            <dd style={{ marginLeft: 0, marginBottom: "0.25rem" }}>{finding.impact}</dd>
            <dt style={{ fontWeight: 600 }}>Placement</dt>
            <dd style={{ marginLeft: 0, marginBottom: "0.5rem" }}>{finding.placement}</dd>
          </dl>
          <div className="p-code-snippet is-bordered">
            <div className="p-code-snippet__header">
              <h5 className="p-code-snippet__title">Snippet</h5>
              <button
                className="p-button--base is-dense is-small"
                onClick={handleCopy}
              >
                {copied ? "Copied!" : "Copy"}
              </button>
            </div>
            <pre className="p-code-snippet__block is-wrapped">
              <code>{finding.snippet}</code>
            </pre>
          </div>
        </div>
      )}
    </li>
  );
}
