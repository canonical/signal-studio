import type {
  CollectorConfig,
  FilterAnalysis,
  Finding,
  MetricEntry,
  MetricsSnapshot,
  Signal,
  SpanEntry,
} from "../types/api";
import { componentType, componentQualifier } from "../types/api";
import { StatusIcon } from "./StatusIcon";

export interface CardFilter {
  pipeline: string;
  role: ColumnRole;
}

interface PipelineGraphProps {
  config: CollectorConfig;
  findings: Finding[];
  activeFilter: CardFilter | null;
  onFilterChange: (filter: CardFilter | null) => void;
  metricsSnapshot?: MetricsSnapshot | null;
  filterAnalyses?: FilterAnalysis[];
  catalogEntries?: MetricEntry[];
  spanEntries?: SpanEntry[];
}

const STATUS_COLORS: Record<string, string> = {
  ok: "#22c55e",
  info: "#3b82f6",
  warning: "#f59e0b",
  critical: "#e03c31",
};

export type ColumnRole = "receivers" | "processors" | "exporters";

const RECEIVER_RULES = [
  "receiver-endpoint-wildcard",
  "undefined-component-ref",
];
const PROCESSOR_RULES = [
  "missing-memory-limiter",
  "missing-batch",
  "no-trace-sampling",
  "memory-limiter-not-first",
  "batch-before-sampling",
  "batch-not-near-end",
  "no-log-severity-filter",
  "filter-error-mode-propagate",
  "memory-limiter-without-limits",
];
const EXPORTER_RULES = [
  "debug-exporter-in-pipeline",
  "exporter-no-sending-queue",
  "exporter-no-retry",
  "multiple-exporters-no-routing",
  "exporter-insecure-tls",
];
const PIPELINE_RULES = ["empty-pipeline", "undefined-component-ref"];

export function rulesForRole(role: ColumnRole): string[] {
  if (role === "receivers") return [...RECEIVER_RULES, ...PIPELINE_RULES];
  if (role === "processors") return [...PROCESSOR_RULES, ...PIPELINE_RULES];
  return [...EXPORTER_RULES, ...PIPELINE_RULES];
}

interface ComponentDetail {
  protocol?: string;
  endpoint?: string;
}

/** Extract detail lines for a component (e.g. endpoint, protocol sub-entries). */
function componentDetails(cfg?: Record<string, unknown>): ComponentDetail[] {
  if (!cfg) return [];
  const details: ComponentDetail[] = [];

  // Show endpoint at top level
  if (typeof cfg.endpoint === "string") {
    details.push({ endpoint: cfg.endpoint });
  }

  // Show protocol endpoints (e.g. otlp receiver with grpc/http)
  if (cfg.protocols && typeof cfg.protocols === "object") {
    const protocols = cfg.protocols as Record<
      string,
      Record<string, unknown> | null
    >;
    for (const [proto, protoCfg] of Object.entries(protocols)) {
      if (protoCfg && typeof protoCfg.endpoint === "string") {
        details.push({ protocol: proto, endpoint: protoCfg.endpoint });
      } else {
        details.push({ protocol: proto });
      }
    }
  }

  return details;
}

/** Count findings relevant to a column. */
function findingCount(
  findings: Finding[],
  pipelineName: string,
  role: ColumnRole,
): number {
  const ruleSet = rulesForRole(role);
  return findings.filter(
    (f) =>
      ruleSet.includes(f.ruleId) &&
      (f.scope === `pipeline:${pipelineName}` || !f.scope),
  ).length;
}

/** Worst severity for a column (for icon color). */
function worstSeverity(
  findings: Finding[],
  pipelineName: string,
  role: ColumnRole,
): "ok" | "info" | "warning" | "critical" {
  const ruleSet = rulesForRole(role);
  const relevant = findings.filter(
    (f) =>
      ruleSet.includes(f.ruleId) &&
      (f.scope === `pipeline:${pipelineName}` || !f.scope),
  );

  if (relevant.length === 0) return "ok";
  if (relevant.some((f) => f.severity === "critical")) return "critical";
  if (relevant.some((f) => f.severity === "warning")) return "warning";
  return "info";
}

function capitalize(name: string): string {
  return name.replace(/\b\w/g, (c) => c.toUpperCase());
}

function SignalIcon({ signal }: { signal: Signal }) {
  const props = {
    className: "pipeline-section__signal-icon",
    width: 14,
    height: 14,
    viewBox: "0 0 24 24",
    fill: "none",
    stroke: "currentColor",
    strokeWidth: 1.5,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
  };
  switch (signal) {
    case "metrics":
      return (
        <svg {...props}>
          <polyline points="22 12 18 12 15 21 9 3 6 12 2 12" />
        </svg>
      );
    case "traces":
      return (
        <svg {...props}>
          <rect x="2" y="4" width="20" height="4" rx="1" />
          <rect x="6" y="11" width="14" height="4" rx="1" />
          <rect x="10" y="18" width="8" height="4" rx="1" />
        </svg>
      );
    case "logs":
      return (
        <svg {...props}>
          <line x1="3" y1="5" x2="21" y2="5" />
          <line x1="3" y1="10" x2="17" y2="10" />
          <line x1="3" y1="15" x2="19" y2="15" />
          <line x1="3" y1="20" x2="14" y2="20" />
        </svg>
      );
  }
}

/** Format a per-second rate, choosing /s or /min based on magnitude. */
function formatRateWithUnit(perSec: number): string {
  const perMin = perSec * 60;
  if (perMin < 10) {
    // Small rates: show per-second
    if (perSec >= 1) return `${perSec.toFixed(0)} pts/s`;
    if (perSec >= 0.1) return `${perSec.toFixed(1)} pts/s`;
    return `< 0.1 pts/s`;
  }
  // Larger rates: show per-minute
  if (perMin >= 1000) return `${(perMin / 1000).toFixed(1)}k pts/min`;
  return `${perMin.toFixed(0)} pts/min`;
}

interface ThroughputInfo {
  rate: string;
  queuePct?: number;
}

/** Get throughput info for a card based on the role and available metrics. */
function cardThroughput(
  snapshot: MetricsSnapshot | null | undefined,
  role: ColumnRole,
  items: string[],
  signal: Signal,
): ThroughputInfo | null {
  if (!snapshot) return null;

  if (role === "receivers") {
    let total = 0;
    for (const item of items) {
      const rm = snapshot.receivers[item];
      if (rm) {
        total += receiverRateForSignal(rm, signal);
      }
    }
    return total > 0 ? { rate: `${formatRateWithUnit(total)} in` } : null;
  }

  if (role === "exporters") {
    let totalSent = 0;
    let queuePct: number | undefined;
    for (const item of items) {
      const em = snapshot.exporters[item];
      if (em) {
        totalSent += exporterRateForSignal(em, signal);
        if (em.queueCapacity > 0) {
          queuePct = em.queueUtilizationPct;
        }
      }
    }
    if (totalSent === 0) return null;
    return { rate: `${formatRateWithUnit(totalSent)} out`, queuePct };
  }

  return null;
}

/** Pick the right receiver rate field for a signal type. */
function receiverRateForSignal(
  rm: {
    acceptedSpansRate: number;
    acceptedMetricPointsRate: number;
    acceptedLogRecordsRate: number;
  },
  signal: Signal,
): number {
  if (signal === "traces") return rm.acceptedSpansRate;
  if (signal === "metrics") return rm.acceptedMetricPointsRate;
  return rm.acceptedLogRecordsRate;
}

/** Pick the right exporter rate field for a signal type. */
function exporterRateForSignal(
  em: {
    sentSpansRate: number;
    sentMetricPointsRate: number;
    sentLogRecordsRate: number;
  },
  signal: Signal,
): number {
  if (signal === "traces") return em.sentSpansRate;
  if (signal === "metrics") return em.sentMetricPointsRate;
  return em.sentLogRecordsRate;
}


type PillIconType = "stack" | "timer" | "kept" | "dropped" | "memory" | "spike";

interface ProcessorPill {
  icon?: PillIconType;
  label: string;
}

/** Extract summary pills for a processor based on its type and config. */
function processorPills(
  processorName: string,
  cfg?: Record<string, unknown>,
): ProcessorPill[] {
  if (!cfg) return [];
  const typ = componentType(processorName);
  switch (typ) {
    case "memory_limiter": {
      const pills: ProcessorPill[] = [];
      const limit = cfg.limit_mib ?? cfg.limit_percentage;
      if (limit != null) {
        pills.push({
          icon: "memory",
          label: cfg.limit_mib ? `${limit} MiB` : `${limit}%`,
        });
      }
      const spike = cfg.spike_limit_mib ?? cfg.spike_limit_percentage;
      if (spike != null) {
        pills.push({
          icon: "spike",
          label: cfg.spike_limit_mib ? `${spike} MiB spike` : `${spike}% spike`,
        });
      }
      if (typeof cfg.check_interval === "string") {
        pills.push({ icon: "timer", label: cfg.check_interval });
      }
      return pills;
    }
    case "batch": {
      const pills: ProcessorPill[] = [];
      if (cfg.send_batch_size != null)
        pills.push({ icon: "stack", label: `${cfg.send_batch_size}` });
      if (typeof cfg.timeout === "string")
        pills.push({ icon: "timer", label: cfg.timeout });
      return pills;
    }
    default:
      return [];
  }
}

function PillIcon({ icon }: { icon: PillIconType }) {
  const props = {
    className: "pipeline-card__pill-icon",
    width: 12,
    height: 12,
    viewBox: "0 0 24 24",
    fill: "none",
    stroke: "currentColor",
    strokeWidth: 2.5,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
  };
  switch (icon) {
    case "stack":
      return (
        <svg {...props}>
          <path d="M12 2 2 7l10 5 10-5-10-5Z" />
          <path d="m2 17 10 5 10-5" />
          <path d="m2 12 10 5 10-5" />
        </svg>
      );
    case "timer":
      return (
        <svg {...props}>
          <circle cx="12" cy="13" r="8" />
          <path d="M12 9v4l2 2" />
          <path d="M5 3 2 6" />
          <path d="m22 6-3-3" />
        </svg>
      );
    case "kept":
      return (
        <svg {...props}>
          <path d="M20 6 9 17l-5-5" />
        </svg>
      );
    case "dropped":
      return (
        <svg {...props}>
          <path d="M18 6 6 18" />
          <path d="m6 6 12 12" />
        </svg>
      );
    case "memory":
      return (
        <svg {...props}>
          <rect x="2" y="6" width="20" height="12" rx="2" />
          <path d="M6 10v4" />
          <path d="M10 10v4" />
          <path d="M14 10v4" />
          <path d="M18 10v4" />
        </svg>
      );
    case "spike":
      return (
        <svg {...props}>
          <polyline points="2 18 8 12 12 16 22 4" />
          <polyline points="16 4 22 4 22 10" />
        </svg>
      );
  }
}

interface VolumeChangeInfo {
  changePct: number;
  totalPoints: number;
  windowMs: number;
}

type VolumeConfidence = "High" | "Medium" | "Low";

function volumeConfidence(totalPoints: number): VolumeConfidence {
  if (totalPoints >= 50_000) return "High";
  if (totalPoints >= 5_000) return "Medium";
  return "Low";
}

function formatDuration(ms: number): string {
  const totalSec = Math.floor(ms / 1000);
  const m = Math.floor(totalSec / 60);
  const s = totalSec % 60;
  if (m === 0) return `${s}s`;
  return `${m}m ${s}s`;
}

function formatCount(n: number): string {
  return n.toLocaleString("en-US");
}

function volumeTooltip(info: VolumeChangeInfo, signalLabel: string): string {
  const conf = volumeConfidence(info.totalPoints);
  return [
    `Observed reduction in ${signalLabel}`,
    `Window: ${formatDuration(info.windowMs)}`,
    `Evaluated: ${formatCount(info.totalPoints)} datapoints`,
    `Confidence: ${conf}`,
  ].join("\n");
}

/** Compute per-filter volume change percentage from catalog data. */
function filterVolumeChange(
  fa: FilterAnalysis,
  catalogEntries: MetricEntry[] | undefined,
  spanEntries: SpanEntry[] | undefined,
  pipelineSignal: Signal,
): VolumeChangeInfo | null {
  if (fa.droppedCount === 0 && fa.partialCount === 0 && fa.keptCount === 0)
    return null;

  const droppedNames = new Set<string>();
  const partialRatios = new Map<string, number>();
  for (const r of fa.results ?? []) {
    if (r.outcome === "dropped") droppedNames.add(r.metricName);
    if (r.outcome === "partial" && r.droppedRatio != null) {
      partialRatios.set(r.metricName, r.droppedRatio);
    }
  }

  let keptPoints = 0;
  let totalPoints = 0;
  let earliest = Infinity;
  let latest = -Infinity;

  if (pipelineSignal === "traces") {
    if (!spanEntries || spanEntries.length === 0) return null;
    for (const entry of spanEntries) {
      totalPoints += entry.spanCount;
      const first = new Date(entry.firstSeenAt).getTime();
      const last = new Date(entry.lastSeenAt).getTime();
      if (first < earliest) earliest = first;
      if (last > latest) latest = last;
      if (droppedNames.has(entry.spanName)) continue;
      const ratio = partialRatios.get(entry.spanName);
      if (ratio != null) {
        keptPoints += entry.spanCount * (1 - ratio);
      } else {
        keptPoints += entry.spanCount;
      }
    }
  } else {
    if (!catalogEntries || catalogEntries.length === 0) return null;
    for (const entry of catalogEntries) {
      totalPoints += entry.pointCount;
      const first = new Date(entry.firstSeenAt).getTime();
      const last = new Date(entry.lastSeenAt).getTime();
      if (first < earliest) earliest = first;
      if (last > latest) latest = last;
      if (droppedNames.has(entry.name)) continue;
      const ratio = partialRatios.get(entry.name);
      if (ratio != null) {
        keptPoints += entry.pointCount * (1 - ratio);
      } else {
        keptPoints += entry.pointCount;
      }
    }
  }

  if (totalPoints === 0) return null;
  const windowMs = latest > earliest ? latest - earliest : 0;
  return {
    changePct: ((keptPoints - totalPoints) / totalPoints) * 100,
    totalPoints,
    windowMs,
  };
}

/** Get filter analysis for a processor, if available. */
function filterAnalysis(
  processorName: string,
  filterAnalyses?: FilterAnalysis[],
): FilterAnalysis | null {
  if (!filterAnalyses) return null;
  for (const fa of filterAnalyses) {
    if (fa.processorName === processorName) {
      return fa;
    }
  }
  return null;
}

export function PipelineGraph({
  config,
  findings,
  activeFilter,
  onFilterChange,
  metricsSnapshot,
  filterAnalyses,
  catalogEntries,
  spanEntries,
}: PipelineGraphProps) {
  const pipelines = Object.entries(config.pipelines);

  if (pipelines.length === 0) {
    return (
      <p className="u-text--muted" style={{ padding: "1rem" }}>
        No pipelines found in this configuration.
      </p>
    );
  }

  return (
    <div className="pipeline-cards-container">
      {pipelines.map(([name, pipeline]) => {
        const columns: { role: ColumnRole; items: string[] }[] = [
          { role: "receivers", items: pipeline.receivers ?? [] },
          { role: "processors", items: pipeline.processors ?? [] },
          { role: "exporters", items: pipeline.exporters ?? [] },
        ];

        const metricsConnected = metricsSnapshot?.status === "connected";
        return (
          <div key={name} className="pipeline-section">
            <h3 className="pipeline-section__title">
              <SignalIcon signal={pipeline.signal} />
              {capitalize(name)}
            </h3>
            <div className="pipeline-section__cards">
              {columns.map((col) => {
                const issueCount = findingCount(findings, name, col.role);
                const status = worstSeverity(findings, name, col.role);
                const isActive =
                  activeFilter?.pipeline === name &&
                  activeFilter?.role === col.role;
                const isMuted = activeFilter != null && !isActive;
                return (
                  <div key={col.role} className="pipeline-card-wrapper">
                    <div
                      className={`pipeline-card${isMuted ? " is-muted" : ""}`}
                      style={
                        {
                          "--card-accent":
                            STATUS_COLORS[status] ?? STATUS_COLORS.ok,
                        } as React.CSSProperties
                      }
                    >
                      <div className="pipeline-card__header">
                        <span className="pipeline-card__title">
                          {capitalize(col.role)}
                        </span>
                        <button
                          className="pipeline-card__status-btn"
                          aria-label={`Filter by ${col.role} in ${name}`}
                          onClick={() => {
                            if (
                              activeFilter?.pipeline === name &&
                              activeFilter?.role === col.role
                            ) {
                              onFilterChange(null);
                            } else {
                              onFilterChange({
                                pipeline: name,
                                role: col.role,
                              });
                            }
                          }}
                        >
                          {issueCount > 0 && (
                            <span
                              className="pipeline-card__issue-count"
                              style={{ color: STATUS_COLORS[status] }}
                            >
                              {issueCount}
                            </span>
                          )}
                          <StatusIcon status={status} />
                        </button>
                      </div>
                      <div className="pipeline-card__body">
                        {col.items.length === 0 ? (
                          <span className="pipeline-card__empty">None</span>
                        ) : (
                          col.items.map((item) => {
                            const type = componentType(item);
                            const qualifier = componentQualifier(item);
                            const displayName = qualifier
                              ? `${type}/${qualifier}`
                              : type;

                            // Look up component config for details
                            const compMap =
                              col.role === "receivers"
                                ? config.receivers
                                : col.role === "processors"
                                  ? config.processors
                                  : config.exporters;
                            const compCfg = compMap[item]?.config;
                            const details = componentDetails(
                              compCfg as Record<string, unknown> | undefined,
                            );

                            const fa =
                              col.role === "processors"
                                ? filterAnalysis(item, filterAnalyses)
                                : null;
                            const volChange = fa
                              ? filterVolumeChange(fa, catalogEntries, spanEntries, pipeline.signal)
                              : null;
                            const pills =
                              col.role === "processors"
                                ? processorPills(
                                    item,
                                    compCfg as
                                      | Record<string, unknown>
                                      | undefined,
                                  )
                                : [];
                            const hasPills =
                              volChange !== null || pills.length > 0;

                            return (
                              <div
                                key={item}
                                className="pipeline-card__component"
                              >
                                <div className="pipeline-card__component-name">
                                  {col.role === "receivers" && (
                                    <svg
                                      className="pipeline-card__component-icon"
                                      width="14"
                                      height="14"
                                      viewBox="0 0 24 24"
                                      fill="none"
                                      stroke="currentColor"
                                      strokeWidth="1.5"
                                      strokeLinecap="round"
                                      strokeLinejoin="round"
                                    >
                                      <polyline points="7 10 12 15 17 10" />
                                      <line x1="12" y1="15" x2="12" y2="3" />
                                      <path d="M20 21H4" />
                                    </svg>
                                  )}
                                  {col.role === "exporters" && (
                                    <svg
                                      className="pipeline-card__component-icon"
                                      width="14"
                                      height="14"
                                      viewBox="0 0 24 24"
                                      fill="none"
                                      stroke="currentColor"
                                      strokeWidth="1.5"
                                      strokeLinecap="round"
                                      strokeLinejoin="round"
                                    >
                                      <polyline points="7 10 12 5 17 10" />
                                      <line x1="12" y1="5" x2="12" y2="17" />
                                      <path d="M20 21H4" />
                                    </svg>
                                  )}
                                  {col.role === "processors" && (
                                    <svg
                                      className="pipeline-card__component-icon"
                                      width="14"
                                      height="14"
                                      viewBox="0 0 24 24"
                                      fill="none"
                                      stroke="currentColor"
                                      strokeWidth="1.5"
                                      strokeLinecap="round"
                                      strokeLinejoin="round"
                                    >
                                      <rect
                                        x="6"
                                        y="6"
                                        width="12"
                                        height="12"
                                        rx="1"
                                      />
                                      <path d="M9 1v4M15 1v4M9 19v4M15 19v4M1 9h4M1 15h4M19 9h4M19 15h4" />
                                    </svg>
                                  )}
                                  {displayName}
                                  {hasPills && (
                                    <span className="pipeline-card__filter-stats">
                                      {volChange !== null && (
                                        <span
                                          className={`pipeline-card__filter-stat ${volChange.changePct < 0 ? "pipeline-card__filter-stat--kept" : volChange.changePct > 0 ? "pipeline-card__filter-stat--dropped" : "pipeline-card__filter-stat--neutral"}`}
                                          title={volumeTooltip(volChange, pipeline.signal === "traces" ? "span datapoints" : "metric datapoints")}
                                        >
                                          <svg
                                            className="pipeline-card__pill-icon"
                                            width="12"
                                            height="12"
                                            viewBox="0 0 24 24"
                                            fill="none"
                                            stroke="currentColor"
                                            strokeWidth="2.5"
                                            strokeLinecap="round"
                                            strokeLinejoin="round"
                                          >
                                            {volChange.changePct < 0 ? (
                                              <>
                                                <polyline points="23 18 13.5 8.5 8.5 13.5 1 6" />
                                                <polyline points="17 18 23 18 23 12" />
                                              </>
                                            ) : (
                                              <>
                                                <polyline points="23 6 13.5 15.5 8.5 10.5 1 18" />
                                                <polyline points="17 6 23 6 23 12" />
                                              </>
                                            )}
                                          </svg>
                                          {volChange.changePct > 0 ? "+" : ""}
                                          {volChange.changePct.toFixed(0)}%
                                        </span>
                                      )}
                                      {pills.map((p) => (
                                        <span
                                          key={p.label}
                                          className="pipeline-card__filter-stat pipeline-card__filter-stat--neutral"
                                        >
                                          {p.icon && <PillIcon icon={p.icon} />}
                                          {p.label}
                                        </span>
                                      ))}
                                    </span>
                                  )}
                                  {details.length > 0 && (
                                    <span className="pipeline-card__filter-stats">
                                      {details.map((d, j) => (
                                        <span
                                          key={j}
                                          className="pipeline-card__filter-stat pipeline-card__filter-stat--neutral"
                                        >
                                          {d.protocol && (
                                            <span className="pipeline-card__detail-proto">
                                              {d.protocol}
                                            </span>
                                          )}
                                          {d.endpoint && (
                                            <span>{d.endpoint}</span>
                                          )}
                                        </span>
                                      ))}
                                    </span>
                                  )}
                                </div>
                              </div>
                            );
                          })
                        )}
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
            {metricsConnected && (
              <div className="pipeline-section__footer">
                {columns.map((col) => {
                  const throughput = cardThroughput(
                    metricsSnapshot,
                    col.role,
                    col.items,
                    pipeline.signal,
                  );
                  return (
                    <div key={col.role} className="pipeline-section__footer-cell">
                      {throughput ? (
                        <>
                          <svg
                            className="pipeline-card__metrics-icon"
                            width="12"
                            height="12"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            strokeWidth="2"
                            strokeLinecap="round"
                            strokeLinejoin="round"
                          >
                            <path d="M12 22C6.5 22 2 17.5 2 12S6.5 2 12 2s10 4.5 10 10" />
                            <path d="M12 12l4-4" />
                            <circle
                              cx="12"
                              cy="12"
                              r="1.5"
                              fill="currentColor"
                              stroke="none"
                            />
                          </svg>
                          {throughput.rate}
                          {throughput.queuePct != null && (
                            <span className="pipeline-card__queue">
                              <svg
                                width="12"
                                height="12"
                                viewBox="0 0 24 24"
                                fill="none"
                                stroke="currentColor"
                                strokeWidth="2"
                                strokeLinecap="round"
                                strokeLinejoin="round"
                              >
                                <path d="M12 2 2 7l10 5 10-5-10-5Z" />
                                <path d="m2 17 10 5 10-5" />
                                <path d="m2 12 10 5 10-5" />
                              </svg>
                              {throughput.queuePct.toFixed(0)}%
                            </span>
                          )}
                        </>
                      ) : col.role !== "processors" ? (
                        <span className="pipeline-section__footer-pending">
                          <svg
                            className="pipeline-section__footer-spinner"
                            width="12"
                            height="12"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="#888"
                            strokeWidth="2.5"
                            strokeLinecap="round"
                          >
                            <path d="M12 2a10 10 0 0 1 10 10" />
                          </svg>
                          Waiting&hellip;
                        </span>
                      ) : null}
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
