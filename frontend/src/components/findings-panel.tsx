import { useState } from "react";
import type { Confidence, Finding, Severity } from "../types/api";
import { StatusIcon } from "./status-icon";

interface FindingsPanelProps {
  findings: Finding[];
}

const SEVERITY_ORDER: Severity[] = ["critical", "warning", "info"];

const CONFIDENCE_LABEL: Record<Confidence, string> = {
  high: "High",
  medium: "Medium",
  low: "Low",
};

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
        <FindingItem key={`${f.ruleId}-${f.scope}-${i}`} finding={f} />
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
      <span className={`finding-item__chevron${expanded ? " is-open" : ""}`}>
        &#x203A;
      </span>
      <div className="finding-item__icon">
        <StatusIcon status={finding.severity} size={18} />
      </div>
      <div className="finding-item__content">
        <div className="finding-item__header">
          <span className="finding-item__title">{finding.title}</span>
        </div>
        <div className="finding-item__pills">
          {finding.scope && (
            <span className="finding-item__scope">{finding.scope}</span>
          )}
          <span
            className={`finding-item__confidence finding-item__confidence--${finding.confidence}`}
          >
            Confidence: {CONFIDENCE_LABEL[finding.confidence]}
          </span>
        </div>

        {expanded && (
          <div
            className="finding-item__details"
            onClick={(e) => e.stopPropagation()}
          >
            {finding.evidence && (
              <div className="finding-item__detail-row finding-item__detail-row--observed">
                <span className="finding-item__detail-label">Evidence</span>
                <span>{finding.evidence}</span>
              </div>
            )}
            {finding.implication && (
              <>
                <span className="finding-item__detail-label">Implication</span>
                <div className="finding-item__detail-row">
                  <span>
                    {finding.implication.split("\n").map((para, j, arr) => (
                      <span key={j}>
                        {para}
                        {j < arr.length - 1 && (
                          <>
                            <br />
                            <br />
                          </>
                        )}
                      </span>
                    ))}
                  </span>
                </div>
              </>
            )}
            <div className="finding-item__detail-row">
              <span className="finding-item__detail-label">Recommendation</span>
              <span>{finding.recommendation}</span>
            </div>
            {finding.snippet && (
              <div className="finding-item__snippet">
                <div className="finding-item__snippet-header">
                  <span className="finding-item__snippet-title">Snippet</span>
                  <button
                    className="finding-item__copy-btn"
                    onClick={handleCopy}
                  >
                    {copied ? "Copied!" : "Copy"}
                  </button>
                </div>
                <pre className="finding-item__snippet-code">
                  <code>{finding.snippet}</code>
                </pre>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
