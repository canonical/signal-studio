import { useMemo, useState } from "react";
import type { FilterAnalysis, MatchOutcome, MetricEntry } from "../types/api";

interface MetricCatalogPanelProps {
  entries: MetricEntry[];
  filterAnalyses?: FilterAnalysis[];
}

type SortKey = "outcome" | "name" | "type" | "rate" | "pointCount";

function outcomeForMetric(
  name: string,
  analyses: FilterAnalysis[],
): { outcome: MatchOutcome; processor: string } | null {
  for (const fa of analyses) {
    for (const r of fa.results ?? []) {
      if (r.metricName === name) {
        return { outcome: r.outcome, processor: fa.processorName };
      }
    }
  }
  return null;
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
    keptPoints: 0,
    droppedPoints: 0,
  };
  for (const e of entries) {
    const result = outcomeForMetric(e.name, analyses);
    if (!result || result.outcome === "kept" || result.outcome === "unknown") {
      summary.keptMetrics++;
      summary.keptPoints += e.pointCount;
    } else {
      summary.droppedMetrics++;
      summary.droppedPoints += e.pointCount;
    }
  }
  return summary;
}

export function MetricCatalogPanel({
  entries,
  filterAnalyses,
}: MetricCatalogPanelProps) {
  const [expanded, setExpanded] = useState(false);
  const [search, setSearch] = useState("");
  const [sortKey, setSortKey] = useState<SortKey>("name");
  const [sortAsc, setSortAsc] = useState(true);

  const analyses = filterAnalyses ?? [];

  const filtered = useMemo(() => {
    const outcomeOrder: Record<string, number> = {
      dropped: 0,
      unknown: 1,
      kept: 2,
    };
    const q = search.toLowerCase();
    let items = entries.filter((e) => !q || e.name.toLowerCase().includes(q));
    items.sort((a, b) => {
      let cmp = 0;
      if (sortKey === "outcome") {
        const oa = outcomeForMetric(a.name, analyses)?.outcome ?? "kept";
        const ob = outcomeForMetric(b.name, analyses)?.outcome ?? "kept";
        cmp = (outcomeOrder[oa] ?? 2) - (outcomeOrder[ob] ?? 2);
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

  const sortArrow = (key: SortKey) =>
    sortKey === key ? (sortAsc ? " \u25B2" : " \u25BC") : "";

  const hasAnalyses = analyses.length > 0;
  const summary = hasAnalyses ? computeSummary(entries, analyses) : null;

  if (entries.length === 0) return null;

  return (
    <div className={`catalog-panel${expanded ? " is-expanded" : ""}`}>
      <button
        className="catalog-panel__toggle"
        onClick={() => setExpanded(!expanded)}
      >
        <span className="catalog-panel__toggle-icon">
          {expanded ? "\u25BC" : "\u25B2"}
        </span>
        <span>Metric Catalog ({entries.length})</span>
        {summary && (
          <span className="catalog-panel__toggle-summary">
            {summary.keptMetrics} kept ({summary.keptPoints.toLocaleString()}{" "}
            pts)
            {" / "}
            {summary.droppedMetrics} dropped (
            {summary.droppedPoints.toLocaleString()} pts)
          </span>
        )}
      </button>

      {expanded && (
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
                  return (
                    <tr key={e.name}>
                      {hasAnalyses && (
                        <td className="catalog-panel__outcome-cell">
                          {analysis && (
                            <OutcomeIndicator outcome={analysis.outcome} />
                          )}
                        </td>
                      )}
                      <td className="catalog-panel__name">{e.name}</td>
                      <td className="catalog-panel__type">{e.type}</td>
                      <td className="catalog-panel__points">
                        {formatPointsPerScrape(pointsPerScrape(e))}
                      </td>
                      <td className="catalog-panel__points">
                        {e.pointCount.toLocaleString()}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}
