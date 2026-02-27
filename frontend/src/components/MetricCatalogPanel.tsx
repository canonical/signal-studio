import { Fragment, useMemo, useState } from "react";
import type {
  AttributeLevel,
  AttributeMeta,
  FilterAnalysis,
  LogEntry,
  MatchOutcome,
  MetricEntry,
  SpanEntry,
  TapStatus,
} from "../types/api";

type SignalTab = "metrics" | "spans" | "logs";

interface MetricCatalogPanelProps {
  entries: MetricEntry[];
  spanEntries: SpanEntry[];
  logEntries: LogEntry[];
  filterAnalyses?: FilterAnalysis[];
  tapStatus: TapStatus;
}

type SortKey = "outcome" | "name" | "type" | "rate" | "pointCount";

function outcomeForMetric(
  name: string,
  analyses: FilterAnalysis[],
): {
  outcome: MatchOutcome;
  processor: string;
  droppedRatio?: number;
} | null {
  // Priority: dropped > partial > unknown > kept.
  // Pick the most significant outcome across all filter analyses.
  const priority: Record<string, number> = {
    dropped: 3,
    partial: 2,
    unknown: 1,
    kept: 0,
  };
  let best: {
    outcome: MatchOutcome;
    processor: string;
    droppedRatio?: number;
  } | null = null;
  for (const fa of analyses) {
    for (const r of fa.results ?? []) {
      if (r.metricName === name) {
        const rp = priority[r.outcome] ?? 0;
        const bp = best ? (priority[best.outcome] ?? 0) : -1;
        if (rp > bp) {
          best = {
            outcome: r.outcome,
            processor: fa.processorName,
            droppedRatio: r.droppedRatio,
          };
        }
      }
    }
  }
  return best;
}

function pointsPerScrape(entry: MetricEntry): number {
  return entry.scrapeCount > 0 ? entry.pointCount / entry.scrapeCount : 0;
}

function formatPointsPerScrape(pps: number): string {
  if (pps >= 1000) return `${(pps / 1000).toFixed(1)}k`;
  if (pps >= 1) return `${pps.toFixed(0)}`;
  if (pps > 0) return `< 1`;
  return "0";
}

function OutcomeIndicator({ outcome }: { outcome: MatchOutcome }) {
  const colors: Record<MatchOutcome, string> = {
    kept: "#22c55e",
    dropped: "#e03c31",
    unknown: "#f59e0b",
    partial: "#f97316",
  };
  const icon =
    outcome === "kept" ? (
      <svg
        width="16"
        height="16"
        viewBox="0 0 16 16"
        fill="none"
        stroke={colors.kept}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M3 8.5l3.5 3.5 6.5-8" />
      </svg>
    ) : outcome === "dropped" ? (
      <svg
        width="16"
        height="16"
        viewBox="0 0 16 16"
        fill="none"
        stroke={colors.dropped}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <path d="M4 4l8 8M12 4l-8 8" />
      </svg>
    ) : outcome === "partial" ? (
      <svg
        width="16"
        height="16"
        viewBox="0 0 16 16"
        fill="none"
        stroke={colors.partial}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <circle cx="8" cy="8" r="5.5" />
        <path d="M5 8h6" />
      </svg>
    ) : (
      <svg
        width="16"
        height="16"
        viewBox="0 0 16 16"
        fill="none"
        stroke={colors.unknown}
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <circle cx="8" cy="8" r="5.5" />
        <path d="M6.5 6.5a1.75 1.75 0 0 1 3 1.25c0 1-1.5 1.25-1.5 1.25" />
        <circle cx="8" cy="11.5" r="0.5" fill={colors.unknown} />
      </svg>
    );
  return (
    <span className="catalog-outcome" title={outcome}>
      {icon}
    </span>
  );
}

interface PointSummary {
  keptMetrics: number;
  droppedMetrics: number;
  partialMetrics: number;
  keptPoints: number;
  droppedPoints: number;
}

function computeSummary(
  entries: MetricEntry[],
  analyses: FilterAnalysis[],
): PointSummary {
  const summary: PointSummary = {
    keptMetrics: 0,
    droppedMetrics: 0,
    partialMetrics: 0,
    keptPoints: 0,
    droppedPoints: 0,
  };
  for (const e of entries) {
    const result = outcomeForMetric(e.name, analyses);
    if (!result || result.outcome === "kept" || result.outcome === "unknown") {
      summary.keptMetrics++;
      summary.keptPoints += e.pointCount;
    } else if (result.outcome === "partial") {
      summary.partialMetrics++;
      const ratio = result.droppedRatio ?? 0;
      summary.droppedPoints += Math.round(e.pointCount * ratio);
      summary.keptPoints += Math.round(e.pointCount * (1 - ratio));
    } else {
      summary.droppedMetrics++;
      summary.droppedPoints += e.pointCount;
    }
  }
  return summary;
}

/** Parse a matchedBy OTTL expression to extract the targeted attr key, level, and match logic. */
function parseMatchedBy(
  expr: string,
): {
  attrKey: string;
  level: AttributeLevel;
  test: (v: string) => boolean;
} | null {
  let m: RegExpMatchArray | null;

  // IsMatch(attributes["key"], "pattern")
  m = expr.match(/^IsMatch\(attributes\["([^"]+)"\],\s*"(.+)"\)$/);
  if (m && m[1] && m[2]) {
    try {
      const pat = m[2].replace(/\\\\/g, "\\");
      const re = new RegExp("^(?:" + pat + ")$");
      return { attrKey: m[1], level: "datapoint", test: (v) => re.test(v) };
    } catch {
      return null;
    }
  }

  // IsMatch(resource.attributes["key"], "pattern")
  m = expr.match(
    /^IsMatch\(resource\.attributes\["([^"]+)"\],\s*"(.+)"\)$/,
  );
  if (m && m[1] && m[2]) {
    try {
      const pat = m[2].replace(/\\\\/g, "\\");
      const re = new RegExp("^(?:" + pat + ")$");
      return { attrKey: m[1], level: "resource", test: (v) => re.test(v) };
    } catch {
      return null;
    }
  }

  // attributes["key"] == "value"
  m = expr.match(/^attributes\["([^"]+)"\]\s*==\s*"([^"]*)"$/);
  if (m && m[1] && m[2] !== undefined) {
    const val = m[2];
    return { attrKey: m[1], level: "datapoint", test: (v) => v === val };
  }

  // resource.attributes["key"] == "value"
  m = expr.match(/^resource\.attributes\["([^"]+)"\]\s*==\s*"([^"]*)"$/);
  if (m && m[1] && m[2] !== undefined) {
    const val = m[2];
    return { attrKey: m[1], level: "resource", test: (v) => v === val };
  }

  // HasAttrOnDatapoint("key", "value")
  m = expr.match(/^HasAttrOnDatapoint\("([^"]+)",\s*"([^"]*)"\)$/);
  if (m && m[1] && m[2] !== undefined) {
    const val = m[2];
    return { attrKey: m[1], level: "datapoint", test: (v) => v === val };
  }

  // HasAttrKeyOnDatapoint("key") — all values are affected
  m = expr.match(/^HasAttrKeyOnDatapoint\("([^"]+)"\)$/);
  if (m && m[1]) {
    return { attrKey: m[1], level: "datapoint", test: () => true };
  }

  return null;
}

/**
 * Build a set of filtered sample values for a metric, keyed by "level:attrKey".
 * Only populated when the metric has a partial or dropped outcome from an
 * attribute-based filter rule.
 */
function computeFilteredValues(
  metricName: string,
  attributes: AttributeMeta[],
  analyses: FilterAnalysis[],
): Map<string, Set<string>> {
  const result = new Map<string, Set<string>>();
  for (const fa of analyses) {
    for (const r of fa.results ?? []) {
      if (r.metricName !== metricName) continue;
      if (r.outcome !== "partial" && r.outcome !== "dropped") continue;
      if (!r.matchedBy) continue;

      const parsed = parseMatchedBy(r.matchedBy);
      if (!parsed) continue;

      for (const attr of attributes) {
        if (attr.key !== parsed.attrKey || attr.level !== parsed.level)
          continue;
        const key = `${attr.level}:${attr.key}`;
        const set = result.get(key) ?? new Set<string>();
        for (const v of attr.sampleValues) {
          if (parsed.test(v)) set.add(v);
        }
        if (set.size > 0) result.set(key, set);
      }
    }
  }
  return result;
}

const levelLabels: Record<AttributeLevel, string> = {
  resource: "Resource",
  scope: "Scope",
  datapoint: "Datapoint",
};

const levelOrder: Record<AttributeLevel, number> = {
  resource: 0,
  scope: 1,
  datapoint: 2,
};

function AttributeDetailPanel({
  attributes,
  filteredValues,
}: {
  attributes: AttributeMeta[];
  filteredValues: Map<string, Set<string>>;
}) {
  const grouped = useMemo(() => {
    const groups = new Map<AttributeLevel, AttributeMeta[]>();
    for (const a of attributes) {
      const list = groups.get(a.level) ?? [];
      list.push(a);
      groups.set(a.level, list);
    }
    return [...groups.entries()].sort(
      ([a], [b]) => (levelOrder[a] ?? 3) - (levelOrder[b] ?? 3),
    );
  }, [attributes]);

  return (
    <div className="catalog-panel__attributes">
      {grouped.map(([level, attrs]) => (
        <div key={level} className="catalog-panel__attr-group">
          <div className="catalog-panel__attr-level">
            {levelLabels[level] ?? level}
          </div>
          {attrs.map((a) => {
            const fv = filteredValues.get(`${a.level}:${a.key}`);
            return (
              <div key={a.key} className="catalog-panel__attr-row">
                <span className="catalog-panel__attr-key">{a.key}</span>
                <span className="catalog-panel__attr-count">
                  {a.uniqueCount} unique
                  {a.capped && (
                    <span className="catalog-panel__attr-capped" title="High cardinality — not all values tracked">
                      {" "}(capped)
                    </span>
                  )}
                </span>
                <span className="catalog-panel__attr-values">
                  {a.sampleValues.map((v) => (
                    <span
                      key={v}
                      className={`catalog-panel__attr-chip${fv?.has(v) ? " is-filtered" : ""}`}
                      title={fv?.has(v) ? "Dropped by filter" : undefined}
                    >
                      {v}
                    </span>
                  ))}
                  {a.capped && (
                    <span className="catalog-panel__attr-chip is-more">...</span>
                  )}
                </span>
              </div>
            );
          })}
        </div>
      ))}
    </div>
  );
}

function SpanTable({ entries }: { entries: SpanEntry[] }) {
  const [search, setSearch] = useState("");
  const filtered = useMemo(() => {
    const q = search.toLowerCase();
    const items = entries.filter(
      (e) =>
        !q ||
        e.serviceName.toLowerCase().includes(q) ||
        e.spanName.toLowerCase().includes(q),
    );
    items.sort((a, b) => {
      if (a.serviceName !== b.serviceName)
        return a.serviceName.localeCompare(b.serviceName);
      return a.spanName.localeCompare(b.spanName);
    });
    return items;
  }, [entries, search]);

  return (
    <div className="catalog-panel__body">
      <input
        className="catalog-panel__search"
        type="text"
        placeholder="Search spans..."
        value={search}
        onChange={(e) => setSearch(e.target.value)}
      />
      <div className="catalog-panel__table-wrap">
        <table className="catalog-panel__table">
          <thead>
            <tr>
              <th>Service</th>
              <th>Span Name</th>
              <th className="catalog-panel__type">Kind</th>
              <th className="catalog-panel__type">Status</th>
              <th className="catalog-panel__points">Total</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((e) => (
              <tr key={`${e.serviceName}\0${e.spanName}`}>
                <td className="catalog-panel__name">{e.serviceName}</td>
                <td className="catalog-panel__name">{e.spanName}</td>
                <td className="catalog-panel__type">{e.spanKind}</td>
                <td className="catalog-panel__type">
                  <span
                    style={{
                      color: e.statusCode === "error" ? "#e03c31" : undefined,
                    }}
                  >
                    {e.statusCode}
                  </span>
                </td>
                <td className="catalog-panel__points">
                  {e.spanCount.toLocaleString()}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function SeverityIcon({ severity }: { severity: string }) {
  const s = 12;
  const props = {
    width: s,
    height: s,
    viewBox: "0 0 16 16",
    fill: "none",
    stroke: "currentColor",
    strokeWidth: 1,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
  };

  switch (severity) {
    case "fatal":
      // skull
      return (
        <svg {...props}>
          <circle cx="8" cy="7" r="5" />
          <path d="M5.5 6.5v1M10.5 6.5v1" />
          <path d="M5 14v-2.5a3 3 0 0 1 6 0V14" />
        </svg>
      );
    case "error":
      // x-circle
      return (
        <svg {...props}>
          <circle cx="8" cy="8" r="6" />
          <path d="M6 6l4 4M10 6l-4 4" />
        </svg>
      );
    case "warn":
      // triangle-alert
      return (
        <svg {...props}>
          <path d="M8 2L1 14h14L8 2z" />
          <path d="M8 6v4" />
          <circle cx="8" cy="12" r="0.5" fill="currentColor" stroke="none" />
        </svg>
      );
    case "info":
      // info circle
      return (
        <svg {...props}>
          <circle cx="8" cy="8" r="6" />
          <path d="M8 7v4" />
          <circle cx="8" cy="5" r="0.5" fill="currentColor" stroke="none" />
        </svg>
      );
    case "debug":
      // bug
      return (
        <svg {...props}>
          <path d="M4 8h8M4 5.5l-2-1M12 5.5l2-1M4 10.5l-2 1M12 10.5l2 1" />
          <rect x="5" y="4" width="6" height="8" rx="3" />
        </svg>
      );
    case "trace":
      // footprints / route
      return (
        <svg {...props}>
          <circle cx="4" cy="4" r="1.5" />
          <circle cx="12" cy="12" r="1.5" />
          <path d="M5.5 5.5l5 5" />
        </svg>
      );
    default:
      // question mark
      return (
        <svg {...props}>
          <circle cx="8" cy="8" r="6" />
          <path d="M6 6a2 2 0 0 1 3.5 1.5c0 1.5-1.5 1.5-1.5 1.5" />
          <circle cx="8" cy="12" r="0.5" fill="currentColor" stroke="none" />
        </svg>
      );
  }
}

function SeverityPills({ counts }: { counts: LogEntry["severityCounts"] }) {
  if (!counts || counts.length === 0) return null;
  return (
    <span className="catalog-panel__severity-pills">
      {counts.map((sc) => (
        <span
          key={sc.severity}
          className={`catalog-panel__severity-pill catalog-panel__severity-pill--${sc.severity}`}
          title={`${sc.severity}: ${sc.count.toLocaleString()}`}
        >
          <SeverityIcon severity={sc.severity} />
          {sc.count.toLocaleString()}
        </span>
      ))}
    </span>
  );
}

function LogKindIcon({ kind }: { kind: string }) {
  const props = {
    width: 14,
    height: 14,
    viewBox: "0 0 16 16",
    fill: "none",
    stroke: "currentColor",
    strokeWidth: 1,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
  };

  if (kind === "event") {
    return (
      <span title="Event">
        <svg {...props}>
          <path d="M9 1L3 9h5l-1 6 6-8H8l1-6z" />
        </svg>
      </span>
    );
  }

  return (
    <span title="Log">
      <svg {...props}>
        <path d="M3 4h10M3 8h7M3 12h10" />
      </svg>
    </span>
  );
}

function LogTable({ entries }: { entries: LogEntry[] }) {
  const [search, setSearch] = useState("");
  const filtered = useMemo(() => {
    const q = search.toLowerCase();
    const items = entries.filter(
      (e) =>
        !q ||
        e.serviceName.toLowerCase().includes(q) ||
        e.scopeName.toLowerCase().includes(q) ||
        (e.eventName?.toLowerCase().includes(q) ?? false),
    );
    items.sort((a, b) => {
      if (a.serviceName !== b.serviceName)
        return a.serviceName.localeCompare(b.serviceName);
      if (a.scopeName !== b.scopeName)
        return a.scopeName.localeCompare(b.scopeName);
      return (a.eventName ?? "").localeCompare(b.eventName ?? "");
    });
    return items;
  }, [entries, search]);

  return (
    <div className="catalog-panel__body">
      <input
        className="catalog-panel__search"
        type="text"
        placeholder="Search logs..."
        value={search}
        onChange={(e) => setSearch(e.target.value)}
      />
      <div className="catalog-panel__table-wrap">
        <table className="catalog-panel__table">
          <thead>
            <tr>
              <th>Service</th>
              <th>Source</th>
              <th className="catalog-panel__type">Kind</th>
              <th>Severity</th>
              <th className="catalog-panel__points">Records</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((e) => (
              <tr key={`${e.serviceName}\0${e.scopeName}\0${e.eventName ?? ""}`}>
                <td className="catalog-panel__name">{e.serviceName}</td>
                <td className="catalog-panel__name">
                  {e.eventName || e.scopeName}
                </td>
                <td className="catalog-panel__type"><LogKindIcon kind={e.logKind} /></td>
                <td><SeverityPills counts={e.severityCounts} /></td>
                <td className="catalog-panel__points">
                  {e.recordCount.toLocaleString()}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

export function MetricCatalogPanel({
  entries,
  spanEntries,
  logEntries,
  filterAnalyses,
  tapStatus,
}: MetricCatalogPanelProps) {
  const [expanded, setExpanded] = useState(false);
  const [activeTab, setActiveTab] = useState<SignalTab>("metrics");
  const [search, setSearch] = useState("");
  const [sortKey, setSortKey] = useState<SortKey>("name");
  const [sortAsc, setSortAsc] = useState(true);
  const [expandedRows, setExpandedRows] = useState<Set<string>>(new Set());

  const analyses = filterAnalyses ?? [];

  const filtered = useMemo(() => {
    const outcomeOrd: Record<string, number> = {
      dropped: 0,
      partial: 1,
      unknown: 2,
      kept: 3,
    };
    const q = search.toLowerCase();
    let items = entries.filter((e) => !q || e.name.toLowerCase().includes(q));
    items.sort((a, b) => {
      let cmp = 0;
      if (sortKey === "outcome") {
        const oa = outcomeForMetric(a.name, analyses)?.outcome ?? "kept";
        const ob = outcomeForMetric(b.name, analyses)?.outcome ?? "kept";
        cmp = (outcomeOrd[oa] ?? 3) - (outcomeOrd[ob] ?? 3);
      } else if (sortKey === "name") cmp = a.name.localeCompare(b.name);
      else if (sortKey === "type") cmp = a.type.localeCompare(b.type);
      else if (sortKey === "rate")
        cmp = pointsPerScrape(a) - pointsPerScrape(b);
      else cmp = a.pointCount - b.pointCount;
      return sortAsc ? cmp : -cmp;
    });
    return items;
  }, [entries, analyses, search, sortKey, sortAsc]);

  function toggleSort(key: SortKey) {
    if (sortKey === key) {
      setSortAsc(!sortAsc);
    } else {
      setSortKey(key);
      setSortAsc(true);
    }
  }

  function toggleRow(name: string) {
    setExpandedRows((prev) => {
      const next = new Set(prev);
      if (next.has(name)) {
        next.delete(name);
      } else {
        next.add(name);
      }
      return next;
    });
  }

  const sortArrow = (key: SortKey) =>
    sortKey === key ? (sortAsc ? " \u25B2" : " \u25BC") : "";

  const hasAnalyses = analyses.length > 0;
  const summary = hasAnalyses ? computeSummary(entries, analyses) : null;
  const colCount = hasAnalyses ? 5 : 4;

  const totalCount = entries.length + spanEntries.length + logEntries.length;
  const isEmpty = totalCount === 0;
  const tapEnabled = tapStatus === "listening";

  return (
    <div className={`catalog-panel${expanded ? " is-expanded" : ""}`}>
      <button
        className="catalog-panel__toggle"
        onClick={() => setExpanded(!expanded)}
      >
        <span className="catalog-panel__toggle-icon">
          {expanded ? "\u25BC" : "\u25B2"}
        </span>
        <span>Signal Catalog ({totalCount})</span>
        {summary && (
          <span className="catalog-panel__toggle-summary">
            {summary.keptMetrics} kept ({summary.keptPoints.toLocaleString()}{" "}
            pts)
            {summary.partialMetrics > 0 && (
              <>
                {" / "}
                {summary.partialMetrics} partial
              </>
            )}
            {" / "}
            {summary.droppedMetrics} dropped (
            {summary.droppedPoints.toLocaleString()} pts)
          </span>
        )}
      </button>

      {expanded && isEmpty && (
        <div className="catalog-panel__body">
          <p className="u-text--muted" style={{ padding: "1rem" }}>
            {tapEnabled
              ? "No data yet"
              : "The catalog only works with the tap enabled"}
          </p>
        </div>
      )}

      {expanded && !isEmpty && (
        <>
          <div className="catalog-panel__tabs">
            <button
              className={`catalog-panel__tab${activeTab === "metrics" ? " is-active" : ""}`}
              onClick={() => setActiveTab("metrics")}
            >
              Metrics ({entries.length})
            </button>
            <button
              className={`catalog-panel__tab${activeTab === "spans" ? " is-active" : ""}`}
              onClick={() => setActiveTab("spans")}
            >
              Spans ({spanEntries.length})
            </button>
            <button
              className={`catalog-panel__tab${activeTab === "logs" ? " is-active" : ""}`}
              onClick={() => setActiveTab("logs")}
            >
              Logs ({logEntries.length})
            </button>
          </div>

          {activeTab === "metrics" && entries.length > 0 && (
            <div className="catalog-panel__body">
              <input
                className="catalog-panel__search"
                type="text"
                placeholder="Search metrics..."
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
              <div className="catalog-panel__table-wrap">
                <table className="catalog-panel__table">
                  <thead>
                    <tr>
                      {hasAnalyses && (
                        <th
                          className="catalog-panel__outcome-cell"
                          title="Filter outcome"
                          onClick={() => toggleSort("outcome")}
                        >
                          <svg
                            width="14"
                            height="14"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            strokeWidth="2"
                            strokeLinecap="round"
                            strokeLinejoin="round"
                          >
                            <path d="M12 3v18" />
                            <path d="m19 8 3 8a5 5 0 0 1-6 0z" />
                            <path d="M3 7h1a17 17 0 0 0 8-2 17 17 0 0 0 8 2h1" />
                            <path d="m5 8 3 8a5 5 0 0 1-6 0z" />
                            <path d="M7 21h10" />
                          </svg>
                          {sortArrow("outcome")}
                        </th>
                      )}
                      <th onClick={() => toggleSort("name")}>
                        Name{sortArrow("name")}
                      </th>
                      <th
                        className="catalog-panel__type"
                        onClick={() => toggleSort("type")}
                      >
                        Type{sortArrow("type")}
                      </th>
                      <th
                        className="catalog-panel__points"
                        onClick={() => toggleSort("rate")}
                      >
                        Points / Scrape{sortArrow("rate")}
                      </th>
                      <th
                        className="catalog-panel__points"
                        onClick={() => toggleSort("pointCount")}
                      >
                        Total {sortArrow("pointCount")}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {filtered.map((e) => {
                      const analysis = hasAnalyses
                        ? outcomeForMetric(e.name, analyses)
                        : null;
                      const hasAttrs = (e.attributes?.length ?? 0) > 0;
                      const isRowExpanded = expandedRows.has(e.name);
                      return (
                        <Fragment key={e.name}>
                          <tr
                            onClick={hasAttrs ? () => toggleRow(e.name) : undefined}
                            style={hasAttrs ? { cursor: "pointer" } : undefined}
                          >
                            {hasAnalyses && (
                              <td className="catalog-panel__outcome-cell">
                                {analysis && (
                                  <OutcomeIndicator outcome={analysis.outcome} />
                                )}
                              </td>
                            )}
                            <td className="catalog-panel__name">
                              {hasAttrs && (
                                <span className="catalog-panel__expand-icon">
                                  {isRowExpanded ? "\u25BC" : "\u25B6"}{" "}
                                </span>
                              )}
                              {e.name}
                            </td>
                            <td className="catalog-panel__type">{e.type}</td>
                            <td className="catalog-panel__points">
                              {formatPointsPerScrape(pointsPerScrape(e))}
                            </td>
                            <td className="catalog-panel__points">
                              {e.pointCount.toLocaleString()}
                            </td>
                          </tr>
                          {isRowExpanded && e.attributes && (
                            <tr className="catalog-panel__detail-row">
                              <td colSpan={colCount}>
                                <AttributeDetailPanel
                                  attributes={e.attributes}
                                  filteredValues={computeFilteredValues(
                                    e.name,
                                    e.attributes,
                                    analyses,
                                  )}
                                />
                              </td>
                            </tr>
                          )}
                        </Fragment>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {activeTab === "metrics" && entries.length === 0 && (
            <div className="catalog-panel__body">
              <p className="u-text--muted" style={{ padding: "1rem" }}>
                No metrics received yet
              </p>
            </div>
          )}

          {activeTab === "spans" && spanEntries.length > 0 && (
            <SpanTable entries={spanEntries} />
          )}
          {activeTab === "spans" && spanEntries.length === 0 && (
            <div className="catalog-panel__body">
              <p className="u-text--muted" style={{ padding: "1rem" }}>
                No spans received yet
              </p>
            </div>
          )}

          {activeTab === "logs" && logEntries.length > 0 && (
            <LogTable entries={logEntries} />
          )}
          {activeTab === "logs" && logEntries.length === 0 && (
            <div className="catalog-panel__body">
              <p className="u-text--muted" style={{ padding: "1rem" }}>
                No logs received yet
              </p>
            </div>
          )}
        </>
      )}
    </div>
  );
}
