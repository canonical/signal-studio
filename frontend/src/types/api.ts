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

export type Confidence = "high" | "medium" | "low";

export interface Finding {
  ruleId: string;
  title: string;
  severity: Severity;
  confidence: Confidence;
  evidence: string;
  implication: string;
  recommendation: string;
  snippet: string;
  scope?: string;
}

export interface AnalyzeResponse {
  config: CollectorConfig;
  findings: Finding[];
  filterAnalyses?: FilterAnalysis[];
}

// Tap types

export type TapStatus = "idle" | "listening" | "stopping" | "error" | "disabled";

export type MetricType =
  | "gauge"
  | "sum"
  | "histogram"
  | "summary"
  | "exponential_histogram";

export type AttributeLevel = "resource" | "scope" | "datapoint";

export interface AttributeMeta {
  key: string;
  level: AttributeLevel;
  sampleValues: string[];
  uniqueCount: number;
  capped: boolean;
}

export interface MetricEntry {
  name: string;
  type: MetricType;
  attributeKeys: string[];
  attributes?: AttributeMeta[];
  pointCount: number;
  scrapeCount: number;
  lastSeenAt: string;
  firstSeenAt: string;
}

export type RemoteTapStatus = "idle" | "connecting" | "connected" | "error";

export interface RemoteTapInfo {
  status: RemoteTapStatus;
  addr?: string;
  error?: string;
}

export interface TapStatusResponse {
  status: TapStatus;
  error?: string;
  startedAt?: string;
  grpcAddr?: string;
  httpAddr?: string;
  remotetap?: RemoteTapInfo;
}

export type SpanKind =
  | "client"
  | "server"
  | "internal"
  | "producer"
  | "consumer"
  | "unset";

export type SpanStatusCode = "unset" | "ok" | "error";

export type SeverityRange =
  | "trace"
  | "debug"
  | "info"
  | "warn"
  | "error"
  | "fatal"
  | "unset";

export interface SpanEntry {
  serviceName: string;
  spanName: string;
  spanKind: SpanKind;
  statusCode: SpanStatusCode;
  attributes?: AttributeMeta[];
  spanCount: number;
  scrapeCount: number;
  lastSeenAt: string;
  firstSeenAt: string;
}

export type LogKind = "event" | "log";

export interface SeverityCount {
  severity: SeverityRange;
  count: number;
}

export interface LogEntry {
  serviceName: string;
  scopeName: string;
  eventName?: string;
  logKind: LogKind;
  severityCounts: SeverityCount[];
  attributes?: AttributeMeta[];
  recordCount: number;
  scrapeCount: number;
  lastSeenAt: string;
  firstSeenAt: string;
}

export interface TapCatalogResponse {
  metrics: MetricEntry[];
  spans: SpanEntry[];
  logs: LogEntry[];
  count: number;
  spanCount: number;
  logCount: number;
  rateChanged: boolean;
}

// Filter analysis types

export type MatchOutcome = "kept" | "dropped" | "unknown" | "partial";

export interface MatchResult {
  metricName: string;
  outcome: MatchOutcome;
  matchedBy?: string;
  droppedRatio?: number;
}

export interface FilterAnalysis {
  processorName: string;
  pipeline: string;
  style: string;
  results: MatchResult[];
  keptCount: number;
  droppedCount: number;
  unknownCount: number;
  partialCount: number;
  hasUnsupported: boolean;
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

// Alert coverage types

export type AlertStatus =
  | "safe"
  | "at_risk"
  | "broken"
  | "would_activate"
  | "unknown";

export interface AlertMetricResult {
  metricName: string;
  filterOutcome: MatchOutcome;
}

export interface AlertCoverageResult {
  alertName: string;
  alertGroup: string;
  expr: string;
  metrics: AlertMetricResult[];
  status: AlertStatus;
}

export interface CoverageSummary {
  total: number;
  safe: number;
  atRisk: number;
  broken: number;
  wouldActivate: number;
  unknown: number;
}

export interface CoverageReport {
  results: AlertCoverageResult[];
  summary: CoverageSummary;
  rulesYaml?: string;
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
