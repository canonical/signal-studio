import { useCallback, useEffect, useRef, useState } from "react";
import type {
  MetricEntry,
  TapCatalogResponse,
  TapStatus,
  TapStatusResponse,
} from "../types/api";

const STATUS_POLL_MS = 3_000;
const CATALOG_POLL_MS = 5_000;

interface UseTapResult {
  status: TapStatus;
  entries: MetricEntry[];
  error: string | null;
  grpcAddr: string | null;
  httpAddr: string | null;
  rateChanged: boolean;
  resetCatalog: () => Promise<void>;
}

export function useTap(): UseTapResult {
  const [status, setStatus] = useState<TapStatus>("idle");
  const [entries, setEntries] = useState<MetricEntry[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [grpcAddr, setGrpcAddr] = useState<string | null>(null);
  const [httpAddr, setHttpAddr] = useState<string | null>(null);
  const [rateChanged, setRateChanged] = useState(false);
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
      setEntries(data.entries ?? []);
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
      fetchCatalog();
    } catch {
      // silently ignore
    }
  }, [fetchCatalog]);

  return { status, entries, error, grpcAddr, httpAddr, rateChanged, resetCatalog };
}
