import { useCallback, useEffect, useRef, useState } from "react";
import type {
  LogEntry,
  MetricEntry,
  RemoteTapInfo,
  SpanEntry,
  TapCatalogResponse,
  TapStatus,
  TapStatusResponse,
} from "../types/api";

const STATUS_POLL_MS = 3_000;
const CATALOG_POLL_MS = 5_000;

interface UseTapResult {
  status: TapStatus;
  entries: MetricEntry[];
  spanEntries: SpanEntry[];
  logEntries: LogEntry[];
  error: string | null;
  grpcAddr: string | null;
  httpAddr: string | null;
  rateChanged: boolean;
  remotetap: RemoteTapInfo;
  resetCatalog: () => Promise<void>;
  start: () => Promise<void>;
  stop: () => Promise<void>;
  connectRemoteTap: (addr: string) => Promise<void>;
  disconnectRemoteTap: () => Promise<void>;
}

const defaultRemoteTapInfo: RemoteTapInfo = { status: "idle" };

export function useTap(): UseTapResult {
  const [status, setStatus] = useState<TapStatus>("idle");
  const [entries, setEntries] = useState<MetricEntry[]>([]);
  const [spanEntries, setSpanEntries] = useState<SpanEntry[]>([]);
  const [logEntries, setLogEntries] = useState<LogEntry[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [grpcAddr, setGrpcAddr] = useState<string | null>(null);
  const [httpAddr, setHttpAddr] = useState<string | null>(null);
  const [rateChanged, setRateChanged] = useState(false);
  const [remotetap, setRemotetap] = useState<RemoteTapInfo>(defaultRemoteTapInfo);
  const statusPollRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const catalogPollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchStatus = useCallback(async () => {
    try {
      const res = await fetch("/api/tap/status");
      if (!res.ok) return;
      const data: TapStatusResponse = await res.json();
      setStatus(data.status);
      setGrpcAddr(data.grpcAddr ?? null);
      setHttpAddr(data.httpAddr ?? null);
      setRemotetap(data.remotetap ?? defaultRemoteTapInfo);
      if (data.error) setError(data.error);
      else setError(null);
    } catch {
      // silently ignore poll errors
    }
  }, []);

  const fetchCatalog = useCallback(async () => {
    try {
      const res = await fetch("/api/tap/catalog");
      if (!res.ok) return;
      const data: TapCatalogResponse = await res.json();
      setEntries(data.metrics ?? []);
      setSpanEntries(data.spans ?? []);
      setLogEntries(data.logs ?? []);
      setRateChanged(data.rateChanged ?? false);
    } catch {
      // silently ignore
    }
  }, []);

  useEffect(() => {
    // Initial fetch
    fetchStatus();
    fetchCatalog();

    // Start polling
    statusPollRef.current = setInterval(fetchStatus, STATUS_POLL_MS);
    catalogPollRef.current = setInterval(fetchCatalog, CATALOG_POLL_MS);

    return () => {
      if (statusPollRef.current) clearInterval(statusPollRef.current);
      if (catalogPollRef.current) clearInterval(catalogPollRef.current);
    };
  }, [fetchStatus, fetchCatalog]);

  const resetCatalog = useCallback(async () => {
    try {
      await fetch("/api/tap/reset", { method: "POST" });
      setRateChanged(false);
      setEntries([]);
      setSpanEntries([]);
      setLogEntries([]);
      fetchCatalog();
    } catch {
      // silently ignore
    }
  }, [fetchCatalog]);

  const start = useCallback(async () => {
    try {
      const res = await fetch("/api/tap/start", { method: "POST" });
      if (!res.ok) {
        const data = await res.json();
        setError(data.error ?? "Failed to start tap");
      }
      await fetchStatus();
    } catch {
      setError("Failed to start tap");
    }
  }, [fetchStatus]);

  const stop = useCallback(async () => {
    try {
      await fetch("/api/tap/stop", { method: "POST" });
      setEntries([]);
      setSpanEntries([]);
      setLogEntries([]);
      await fetchStatus();
    } catch {
      // silently ignore
    }
  }, [fetchStatus]);

  const connectRemoteTap = useCallback(async (addr: string) => {
    try {
      const res = await fetch("/api/tap/remotetap/connect", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ addr }),
      });
      if (!res.ok) {
        const data = await res.json();
        setError(data.error ?? "Failed to connect to remotetap");
      }
      await fetchStatus();
    } catch {
      setError("Failed to connect to remotetap");
    }
  }, [fetchStatus]);

  const disconnectRemoteTap = useCallback(async () => {
    try {
      await fetch("/api/tap/remotetap/disconnect", { method: "POST" });
      await fetchStatus();
    } catch {
      // silently ignore
    }
  }, [fetchStatus]);

  return { status, entries, spanEntries, logEntries, error, grpcAddr, httpAddr, rateChanged, remotetap, resetCatalog, start, stop, connectRemoteTap, disconnectRemoteTap };
}
