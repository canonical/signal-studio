import type {
  CollectorConfig,
  FilterAnalysis,
  Finding,
  MetricEntry,
  MetricsSnapshot,
  SpanEntry,
} from "../types/api";
import { componentType, componentQualifier } from "../types/api";
import { StatusIcon } from "./status-icon";
import {
  SignalIcon,
  PillIcon,
  ComponentRoleIcon,
  ThroughputIcon,
  QueueIcon,
  SpinnerIcon,
  VolumeChangeIcon,
} from "./pipeline-icons";
import type { PillIconType } from "./pipeline-icons";
import { pipelineThroughput } from "../utils/throughput";
import { filterVolumeChange, findFilterAnalysis, volumeTooltip } from "../utils/filter-volume";

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
                                ? findFilterAnalysis(item, filterAnalyses)
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
                                  <ComponentRoleIcon role={col.role} />
                                  {displayName}
                                  {hasPills && (
                                    <span className="pipeline-card__filter-stats">
                                      {volChange !== null && (
                                        <span
                                          className={`pipeline-card__filter-stat ${volChange.changePct < 0 ? "pipeline-card__filter-stat--kept" : volChange.changePct > 0 ? "pipeline-card__filter-stat--dropped" : "pipeline-card__filter-stat--neutral"}`}
                                          title={volumeTooltip(volChange, pipeline.signal === "traces" ? "span datapoints" : "metric datapoints")}
                                        >
                                          <VolumeChangeIcon direction={volChange.changePct < 0 ? "down" : "up"} />
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
                  const throughput = pipelineThroughput(
                    metricsSnapshot,
                    col.role,
                    col.items,
                    pipeline.signal,
                  );
                  return (
                    <div key={col.role} className="pipeline-section__footer-cell">
                      {throughput ? (
                        <>
                          <ThroughputIcon />
                          {throughput.rate}
                          {throughput.queuePct != null && (
                            <span className="pipeline-card__queue">
                              <QueueIcon />
                              {throughput.queuePct.toFixed(0)}%
                            </span>
                          )}
                        </>
                      ) : col.role !== "processors" ? (
                        <span className="pipeline-section__footer-pending">
                          <SpinnerIcon />
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
