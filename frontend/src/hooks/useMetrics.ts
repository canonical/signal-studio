import { useCallback, useEffect, useRef, useState } from "react";
import type { MetricsSnapshot, MetricsStatus } from "../types/api";

const POLL_INTERVAL_MS = 10_000;

interface UseMetricsResult {
  status: MetricsStatus;
  snapshot: MetricsSnapshot | null;
  error: string | null;
  connect: (url: string, token?: string) => Promise<void>;
  disconnect: () => Promise<void>;
}

export function useMetrics(): UseMetricsResult {
  const [status, setStatus] = useState<MetricsStatus>("disconnected");
  const [snapshot, setSnapshot] = useState<MetricsSnapshot | null>(null);
  const [error, setError] = useState<string | null>(null);
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current);
      pollRef.current = null;
    }
  }, []);

  const fetchSnapshot = useCallback(async () => {
    try {
      const res = await fetch("/api/metrics/snapshot");
      if (!res.ok) return;
      const data: MetricsSnapshot = await res.json();
      setSnapshot(data);
      setStatus(data.status as MetricsStatus);
    } catch {
      // Silently ignore poll errors — status endpoint will surface issues
    }
  }, []);

  const startPolling = useCallback(() => {
    stopPolling();
    fetchSnapshot();
    pollRef.current = setInterval(fetchSnapshot, POLL_INTERVAL_MS);
  }, [fetchSnapshot, stopPolling]);

  const connect = useCallback(async (url: string, token?: string) => {
    setStatus("connecting");
    setError(null);
    try {
      const res = await fetch("/api/metrics/connect", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ url, token: token || undefined }),
      });
      if (!res.ok) {
        const body = await res.json();
        throw new Error(body.error || `HTTP ${res.status}`);
      }
      setStatus("connected");
      startPolling();
    } catch (e) {
      setStatus("error");
      setError(e instanceof Error ? e.message : "Connection failed");
    }
  }, [startPolling]);

  const disconnect = useCallback(async () => {
    stopPolling();
    try {
      await fetch("/api/metrics/disconnect", { method: "POST" });
    } catch {
      // Ignore disconnect errors
    }
    setStatus("disconnected");
    setSnapshot(null);
    setError(null);
  }, [stopPolling]);

  // Cleanup on unmount
  useEffect(() => stopPolling, [stopPolling]);

  return { status, snapshot, error, connect, disconnect };
}
