export type Signal = "traces" | "metrics" | "logs";
export type Severity = "info" | "warning" | "critical";

export interface Pipeline {
  signal: Signal;
  receivers: string[];
  processors: string[];
  exporters: string[];
}

export interface ComponentConfig {
  type: string;
  name: string;
  config?: Record<string, unknown>;
}

export interface CollectorConfig {
  receivers: Record<string, ComponentConfig>;
  processors: Record<string, ComponentConfig>;
  exporters: Record<string, ComponentConfig>;
  pipelines: Record<string, Pipeline>;
}

export interface Finding {
  ruleId: string;
  title: string;
  severity: Severity;
  evidence: string;
  explanation: string;
  whyItMatters: string;
  impact: string;
  snippet: string;
  placement: string;
  pipeline?: string;
}

export interface AnalyzeResponse {
  config: CollectorConfig;
  findings: Finding[];
}

export interface ErrorResponse {
  error: string;
}

// Metrics types

export type MetricsStatus = "disconnected" | "connecting" | "connected" | "error";

export interface SignalMetrics {
  receiverAcceptedRate: number;
  exporterSentRate: number;
  exporterFailedRate: number;
  dropRatePct: number;
}

export interface ExporterMetrics {
  queueSize: number;
  queueCapacity: number;
  queueUtilizationPct: number;
  sentSpansRate: number;
  sentMetricPointsRate: number;
  sentLogRecordsRate: number;
  failedSpansRate: number;
}

export interface ReceiverMetrics {
  acceptedSpansRate: number;
  acceptedMetricPointsRate: number;
  acceptedLogRecordsRate: number;
}

export interface MetricsSnapshot {
  status: MetricsStatus;
  collectedAt: string;
  signals: Record<string, SignalMetrics>;
  exporters: Record<string, ExporterMetrics>;
  receivers: Record<string, ReceiverMetrics>;
}

/** Extract the base type from a component name (e.g. "filter/info" → "filter"). */
export function componentType(name: string): string {
  const idx = name.indexOf("/");
  return idx === -1 ? name : name.slice(0, idx);
}

/** Extract the instance qualifier from a component name (e.g. "filter/info" → "info"). */
export function componentQualifier(name: string): string | null {
  const idx = name.indexOf("/");
  return idx === -1 ? null : name.slice(idx + 1);
}
