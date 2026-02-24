import type { CollectorConfig, Finding, MetricsSnapshot, Pipeline, Signal } from "../types/api";
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
];
const PIPELINE_RULES = ["empty-pipeline", "undefined-component-ref"];

export function rulesForRole(role: ColumnRole): string[] {
  if (role === "receivers") return [...RECEIVER_RULES, ...PIPELINE_RULES];
  if (role === "processors") return [...PROCESSOR_RULES, ...PIPELINE_RULES];
  return [...EXPORTER_RULES, ...PIPELINE_RULES];
}

/** Extract detail lines for a component (e.g. endpoint, protocol sub-entries). */
function componentDetails(
  cfg?: Record<string, unknown>,
): string[] {
  if (!cfg) return [];
  const details: string[] = [];

  // Show endpoint at top level
  if (typeof cfg.endpoint === "string") {
    details.push(cfg.endpoint);
  }

  // Show protocol endpoints (e.g. otlp receiver with grpc/http)
  if (cfg.protocols && typeof cfg.protocols === "object") {
    const protocols = cfg.protocols as Record<
      string,
      Record<string, unknown> | null
    >;
    for (const [proto, protoCfg] of Object.entries(protocols)) {
      if (protoCfg && typeof protoCfg.endpoint === "string") {
        details.push(`${proto}  ${protoCfg.endpoint}`);
      } else {
        details.push(proto);
      }
    }
  }

  return details;
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
      (f.pipeline === pipelineName || !f.pipeline),
  );

  if (relevant.length === 0) return "ok";
  if (relevant.some((f) => f.severity === "critical")) return "critical";
  if (relevant.some((f) => f.severity === "warning")) return "warning";
  return "info";
}



function capitalize(name: string): string {
  return name.replace(/\b\w/g, (c) => c.toUpperCase());
}

function formatRate(rate: number): string {
  if (rate >= 10000) return `${(rate / 1000).toFixed(1)}k`;
  if (rate >= 1000) return `${(rate / 1000).toFixed(1)}k`;
  return rate.toFixed(0);
}

/** Get throughput label for a card based on the role and available metrics. */
function cardThroughput(
  snapshot: MetricsSnapshot | null | undefined,
  role: ColumnRole,
  items: string[],
): string | null {
  if (!snapshot) return null;

  if (role === "receivers") {
    let total = 0;
    for (const item of items) {
      const rm = snapshot.receivers[item];
      if (rm) {
        total += rm.acceptedSpansRate + rm.acceptedMetricPointsRate + rm.acceptedLogRecordsRate;
      }
    }
    return total > 0 ? `${formatRate(total)}/s in` : null;
  }

  if (role === "exporters") {
    const parts: string[] = [];
    for (const item of items) {
      const em = snapshot.exporters[item];
      if (em) {
        const sentTotal = em.sentSpansRate + em.sentMetricPointsRate + em.sentLogRecordsRate;
        if (sentTotal > 0) {
          let label = `${formatRate(sentTotal)}/s`;
          if (em.queueCapacity > 0) {
            label += ` (queue ${em.queueUtilizationPct.toFixed(0)}%)`;
          }
          parts.push(label);
        }
      }
    }
    return parts.length > 0 ? parts.join(", ") : null;
  }

  return null;
}

/** Pick the right receiver rate field for a signal type. */
function receiverRateForSignal(
  rm: { acceptedSpansRate: number; acceptedMetricPointsRate: number; acceptedLogRecordsRate: number },
  signal: Signal,
): number {
  if (signal === "traces") return rm.acceptedSpansRate;
  if (signal === "metrics") return rm.acceptedMetricPointsRate;
  return rm.acceptedLogRecordsRate;
}

/** Pick the right exporter rate field for a signal type. */
function exporterRateForSignal(
  em: { sentSpansRate: number; sentMetricPointsRate: number; sentLogRecordsRate: number },
  signal: Signal,
): number {
  if (signal === "traces") return em.sentSpansRate;
  if (signal === "metrics") return em.sentMetricPointsRate;
  return em.sentLogRecordsRate;
}

/** Compute aggregate in/out throughput label for a pipeline. */
function pipelineThroughput(
  snapshot: MetricsSnapshot | null | undefined,
  pipeline: Pipeline,
): string | null {
  if (!snapshot) return null;

  let inRate = 0;
  for (const recv of pipeline.receivers ?? []) {
    const rm = snapshot.receivers[recv];
    if (rm) inRate += receiverRateForSignal(rm, pipeline.signal);
  }

  let outRate = 0;
  for (const exp of pipeline.exporters ?? []) {
    const em = snapshot.exporters[exp];
    if (em) outRate += exporterRateForSignal(em, pipeline.signal);
  }

  if (inRate === 0 && outRate === 0) return null;
  return `${formatRate(inRate)}/s in \u2192 ${formatRate(outRate)}/s out`;
}

export function PipelineGraph({ config, findings, activeFilter, onFilterChange, metricsSnapshot }: PipelineGraphProps) {
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

        const throughputLabel = pipelineThroughput(metricsSnapshot, pipeline);
        return (
          <div key={name} className="pipeline-section">
            <h3 className="pipeline-section__title">
              {capitalize(name)}
              {throughputLabel && (
                <span className="pipeline-section__throughput">({throughputLabel})</span>
              )}
            </h3>
            <div className="pipeline-section__cards">
              {columns.map((col) => {
                const throughput = cardThroughput(metricsSnapshot, col.role, col.items);
                const status = worstSeverity(findings, name, col.role);
                const isActive =
                  activeFilter?.pipeline === name &&
                  activeFilter?.role === col.role;
                const isMuted =
                  activeFilter != null && !isActive;
                return (
                  <div key={col.role} className="pipeline-card-wrapper">
                    <div
                      className={`pipeline-card${isMuted ? " is-muted" : ""}`}
                      style={{ "--card-accent": STATUS_COLORS[status] ?? STATUS_COLORS.ok } as React.CSSProperties}
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
                              onFilterChange({ pipeline: name, role: col.role });
                            }
                          }}
                        >
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
                            const details = componentDetails(compCfg as Record<string, unknown> | undefined);

                            return (
                              <div key={item} className="pipeline-card__component">
                                <div className="pipeline-card__component-name">
                                  {col.role === "processors" && (
                                    <svg className="pipeline-card__component-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                                      <rect x="6" y="6" width="12" height="12" rx="1" />
                                      <path d="M9 1v4M15 1v4M9 19v4M15 19v4M1 9h4M1 15h4M19 9h4M19 15h4" />
                                    </svg>
                                  )}
                                  {displayName}
                                </div>
                                {details.map((d, j) => (
                                  <div
                                    key={j}
                                    className="pipeline-card__component-detail"
                                  >
                                    <span className="pipeline-card__detail-arrow">
                                      &#x21B3;
                                    </span>{" "}
                                    {d}
                                  </div>
                                ))}
                              </div>
                            );
                          })
                        )}
                      </div>
                      {throughput && (
                        <div className="pipeline-card__metrics">
                          <svg className="pipeline-card__metrics-icon" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <path d="M12 22C6.5 22 2 17.5 2 12S6.5 2 12 2s10 4.5 10 10" />
                            <path d="M12 12l4-4" />
                            <circle cx="12" cy="12" r="1.5" fill="currentColor" stroke="none" />
                          </svg>
                          {throughput}
                        </div>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        );
      })}
    </div>
  );
}
