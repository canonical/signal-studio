import type { MetricsSnapshot, Signal } from "../types/api";
import type { ColumnRole } from "../components/pipeline-graph";

export interface ThroughputInfo {
  rate: string;
  queuePct?: number;
}

/** Format a per-second rate, choosing /s or /min based on magnitude. */
export function formatRate(perSec: number): string {
  const perMin = perSec * 60;
  if (perMin < 10) {
    if (perSec >= 1) return `${perSec.toFixed(0)} pts/s`;
    if (perSec >= 0.1) return `${perSec.toFixed(1)} pts/s`;
    return `< 0.1 pts/s`;
  }
  if (perMin >= 1000) return `${(perMin / 1000).toFixed(1)}k pts/min`;
  return `${perMin.toFixed(0)} pts/min`;
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

/** Compute throughput info for a pipeline column from a metrics snapshot. */
export function pipelineThroughput(
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
    return total > 0 ? { rate: `${formatRate(total)} in` } : null;
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
    return { rate: `${formatRate(totalSent)} out`, queuePct };
  }

  return null;
}
