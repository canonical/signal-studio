import { useState } from "react";
import type { Finding, Severity } from "../types/api";
import { StatusIcon } from "./StatusIcon";

interface FindingsPanelProps {
  findings: Finding[];
}

const SEVERITY_ORDER: Severity[] = ["critical", "warning", "info"];

export function FindingsPanel({ findings }: FindingsPanelProps) {
  const sorted = [...findings].sort(
    (a, b) =>
      SEVERITY_ORDER.indexOf(a.severity) - SEVERITY_ORDER.indexOf(b.severity),
  );

  if (findings.length === 0) {
    return <p className="u-text--muted none-found">No issues found.</p>;
  }

  return (
    <div className="findings-list">
      {sorted.map((f, i) => (
        <FindingItem key={`${f.ruleId}-${f.pipeline}-${i}`} finding={f} />
      ))}
    </div>
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

  return (
    <div
      className={`finding-item${expanded ? " is-expanded" : ""}`}
      onClick={() => setExpanded(!expanded)}
    >
      <div className="finding-item__icon">
        <StatusIcon status={finding.severity} size={18} />
      </div>
      <div className="finding-item__content">
        <span className="finding-item__title">{finding.title}</span>
        <p className="finding-item__explanation">{finding.explanation}</p>

        {expanded && (
          <div
            className="finding-item__details"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="finding-item__detail-row">
              <span className="finding-item__detail-label">Evidence</span>
              <span>{finding.evidence}</span>
            </div>
            <div className="finding-item__detail-row">
              <span className="finding-item__detail-label">Why it matters</span>
              <span>{finding.whyItMatters}</span>
            </div>
            <div className="finding-item__detail-row">
              <span className="finding-item__detail-label">Impact</span>
              <span>{finding.impact}</span>
            </div>
            <div className="finding-item__detail-row">
              <span className="finding-item__detail-label">Suggestion</span>
              <span>{finding.placement}</span>
            </div>
            <div className="finding-item__snippet">
              <div className="finding-item__snippet-header">
                <span className="finding-item__snippet-title">Snippet</span>
                <button className="finding-item__copy-btn" onClick={handleCopy}>
                  {copied ? "Copied!" : "Copy"}
                </button>
              </div>
              <pre className="finding-item__snippet-code">
                <code>{finding.snippet}</code>
              </pre>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
