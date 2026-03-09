import type { FilterAnalysis, MetricEntry, Signal, SpanEntry } from "../types/api";

export interface VolumeChangeInfo {
  changePct: number;
  totalPoints: number;
  windowMs: number;
}

export type VolumeConfidence = "High" | "Medium" | "Low";

export function volumeConfidence(totalPoints: number): VolumeConfidence {
  if (totalPoints >= 50_000) return "High";
  if (totalPoints >= 5_000) return "Medium";
  return "Low";
}

export function formatDuration(ms: number): string {
  const totalSec = Math.floor(ms / 1000);
  const m = Math.floor(totalSec / 60);
  const s = totalSec % 60;
  if (m === 0) return `${s}s`;
  return `${m}m ${s}s`;
}

export function formatCount(n: number): string {
  return n.toLocaleString("en-US");
}

export function volumeTooltip(info: VolumeChangeInfo, signalLabel: string): string {
  const conf = volumeConfidence(info.totalPoints);
  return [
    `Observed reduction in ${signalLabel}`,
    `Window: ${formatDuration(info.windowMs)}`,
    `Evaluated: ${formatCount(info.totalPoints)} datapoints`,
    `Confidence: ${conf}`,
  ].join("\n");
}

/** Compute per-filter volume change percentage from catalog data. */
export function filterVolumeChange(
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
export function findFilterAnalysis(
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
