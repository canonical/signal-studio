import { useCallback, useEffect, useRef, useState } from "react";
import type { AnalyzeResponse, Finding } from "./types/api";
import { useDebounce } from "./hooks/useDebounce";
import { useMetrics } from "./hooks/useMetrics";
import { useTap } from "./hooks/useTap";
import { ConfigInput, PLACEHOLDER } from "./components/ConfigInput";
import { FindingsPanel } from "./components/FindingsPanel";
import { MetricCatalogPanel } from "./components/MetricCatalogPanel";
import { MetricsConnection } from "./components/MetricsConnection";
import {
  PipelineGraph,
  rulesForRole,
  type CardFilter,
} from "./components/PipelineGraph";
import { TapConnection } from "./components/TapConnection";

const DEBOUNCE_MS = 500;

export default function App() {
  const [yaml, setYaml] = useState(
    () => localStorage.getItem("signal-studio:yaml") || PLACEHOLDER,
  );
  const [result, setResult] = useState<AnalyzeResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [liveMode, setLiveMode] = useState(true);
  const [cardFilter, setCardFilter] = useState<CardFilter | null>(null);
  const [asideOpen, setAsideOpen] = useState(true);
  const metrics = useMetrics();
  const tap = useTap();

  useEffect(() => {
    localStorage.setItem("signal-studio:yaml", yaml);
  }, [yaml]);

  const debouncedYaml = useDebounce(yaml, DEBOUNCE_MS);
  const abortRef = useRef<AbortController | null>(null);

  const analyze = useCallback(async (config: string) => {
    if (!config.trim()) return;

    abortRef.current?.abort();
    const controller = new AbortController();
    abortRef.current = controller;

    setError(null);
    setLoading(true);
    try {
      const res = await fetch("/api/config/analyze", {
        method: "POST",
        headers: { "Content-Type": "text/yaml" },
        body: config,
        signal: controller.signal,
      });
      if (!res.ok) {
        const body = await res.json();
        throw new Error(body.error || `HTTP ${res.status}`);
      }
      setResult(await res.json());
    } catch (e) {
      if (e instanceof DOMException && e.name === "AbortError") return;
      setError(e instanceof Error ? e.message : "Unknown error");
      setResult(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (liveMode) {
      analyze(debouncedYaml);
    }
  }, [debouncedYaml, liveMode, analyze]);

  const findings = result?.findings ?? [];

  const filteredFindings: Finding[] = cardFilter
    ? findings.filter((f) => {
        const ruleSet = rulesForRole(cardFilter.role);
        return (
          ruleSet.includes(f.ruleId) &&
          (f.pipeline === cardFilter.pipeline || !f.pipeline)
        );
      })
    : findings;

  return (
    <div className="l-application">
      <nav className="l-navigation is-dark">
        <div className="l-navigation__drawer">
          <ConfigInput
            yaml={yaml}
            onYamlChange={setYaml}
            onAnalyze={() => analyze(yaml)}
            liveMode={liveMode}
            onLiveModeChange={setLiveMode}
            loading={loading}
          />
        </div>
      </nav>

      <main className="l-main">
        <div className="main-split">
          <div className="main-split__pipelines">
            <div className="p-panel">
              <div className="p-panel__header is-sticky">
                <h4 className="p-panel__title">Pipelines</h4>
                <div className="p-panel__controls">
                  <TapConnection
                    status={tap.status}
                    entryCount={tap.entries.length}
                    error={tap.error}
                    grpcAddr={tap.grpcAddr}
                    httpAddr={tap.httpAddr}
                    rateChanged={tap.rateChanged}
                    onReset={tap.resetCatalog}
                  />
                  <MetricsConnection
                    status={metrics.status}
                    error={metrics.error}
                    onConnect={metrics.connect}
                    onDisconnect={metrics.disconnect}
                  />
                </div>
              </div>
              <div className="p-panel__content pipeline-panel-content">
                {error && (
                  <div className="p-notification--negative">
                    <div className="p-notification__content">
                      <p className="p-notification__message">{error}</p>
                    </div>
                  </div>
                )}
                {result ? (
                  <PipelineGraph
                    config={result.config}
                    findings={findings}
                    activeFilter={cardFilter}
                    onFilterChange={setCardFilter}
                    metricsSnapshot={metrics.snapshot}
                    filterAnalyses={result.filterAnalyses}
                    catalogEntries={tap.entries}
                  />
                ) : (
                  <p
                    className="u-text--muted"
                    style={{ paddingLeft: "1.5rem" }}
                  >
                    Paste a collector configuration to visualize pipelines.
                  </p>
                )}
              </div>
              {result && (
                <MetricCatalogPanel
                  entries={tap.entries}
                  filterAnalyses={result.filterAnalyses}
                />
              )}
            </div>
          </div>

          <div className={`aside-drawer${asideOpen ? " is-open" : ""}`}>
            <button
              className="aside-drawer__toggle"
              onClick={() => setAsideOpen(!asideOpen)}
              title={
                asideOpen
                  ? "Collapse recommendations"
                  : "Expand recommendations"
              }
            >
              <span className="aside-drawer__chevron">
                {asideOpen ? "\u203A" : "\u2039"}
              </span>
              {!asideOpen && (
                <span className="aside-drawer__label">
                  Recommendations
                  {filteredFindings.length > 0 &&
                    ` (${filteredFindings.length})`}
                </span>
              )}
            </button>
            <div className="aside-drawer__content">
              <div className="p-panel">
                <div className="p-panel__header is-sticky">
                  <h4 className="p-panel__title">
                    Recommendations
                    {filteredFindings.length > 0 &&
                      ` (${filteredFindings.length})`}
                  </h4>
                </div>
                <div className="p-panel__content">
                  {cardFilter && (
                    <button
                      className="p-button--base is-small u-no-margin--bottom"
                      style={{ marginBottom: "0.5rem" }}
                      onClick={() => setCardFilter(null)}
                    >
                      Remove filter
                    </button>
                  )}
                  {result ? (
                    <FindingsPanel findings={filteredFindings} />
                  ) : (
                    <p className="u-text--muted">
                      Recommendations will appear here.
                    </p>
                  )}
                </div>
              </div>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
