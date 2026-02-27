import { useCallback, useEffect, useRef, useState } from "react";
import type { MetricsSnapshot, MetricsStatus } from "../types/api";

const POLL_INTERVAL_MS = 10_000;

interface UseMetricsResult {
  status: MetricsStatus;
  snapshot: MetricsSnapshot | null;
  error: string | null;
  clearError: () => void;
  connect: (url: string, token?: string) => Promise<void>;
  disconnect: () => Promise<void>;
  resetStore: () => Promise<void>;
}

const LS_CONNECTED = "signal-studio:metrics-connected";

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
      localStorage.setItem(LS_CONNECTED, "1");
      startPolling();
    } catch (e) {
      setStatus("error");
      localStorage.removeItem(LS_CONNECTED);
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
    localStorage.removeItem(LS_CONNECTED);
  }, [stopPolling]);

  const resetStore = useCallback(async () => {
    try {
      await fetch("/api/metrics/reset", { method: "POST" });
    } catch {
      // Ignore reset errors
    }
    setSnapshot(null);
    fetchSnapshot();
  }, [fetchSnapshot]);

  // Restore connection on mount if previously connected
  useEffect(() => {
    if (localStorage.getItem(LS_CONNECTED) !== "1") return;
    (async () => {
      try {
        const res = await fetch("/api/metrics/snapshot");
        if (!res.ok) throw new Error();
        const data: MetricsSnapshot = await res.json();
        if (data.status === "connected") {
          setStatus("connected");
          setSnapshot(data);
          startPolling();
          return;
        }
      } catch {
        // Backend not reachable or lost state — try reconnecting
      }
      // Backend lost the connection — reconnect with saved URL
      const url = localStorage.getItem("signal-studio:metrics-url");
      if (url) {
        connect(url);
      } else {
        localStorage.removeItem(LS_CONNECTED);
      }
    })();
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // Cleanup on unmount
  useEffect(() => stopPolling, [stopPolling]);

  const clearError = useCallback(() => setError(null), []);

  return { status, snapshot, error, clearError, connect, disconnect, resetStore };
}
