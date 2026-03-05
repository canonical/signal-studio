import { useCallback, useEffect, useRef, useState } from "react";
import type { AnalyzeResponse, CoverageReport, Finding } from "./types/api";
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
import { Toast } from "./components/Toast";
import { coverageToFindings } from "./utils/coverageToFindings";


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

  // Alert coverage state
  const [alertRulesText, setAlertRulesText] = useState(
    () => localStorage.getItem("signal-studio:alert-rules") || "",
  );
  const [alertCoverage, setAlertCoverage] = useState<CoverageReport | null>(null);
  const [alertError, setAlertError] = useState<string | null>(null);

  const metrics = useMetrics();
  const tap = useTap();

  useEffect(() => {
    localStorage.setItem("signal-studio:yaml", yaml);
  }, [yaml]);

  useEffect(() => {
    localStorage.setItem("signal-studio:alert-rules", alertRulesText);
  }, [alertRulesText]);

  const debouncedYaml = useDebounce(yaml, DEBOUNCE_MS);
  const debouncedAlertRules = useDebounce(alertRulesText, DEBOUNCE_MS);
  const abortRef = useRef<AbortController | null>(null);

  // Ref for alert rules text so analyze() can access current value without deps
  const alertRulesTextRef = useRef(alertRulesText);
  alertRulesTextRef.current = alertRulesText;

  // Helper: build alert coverage request body
  function buildAlertBody(configYaml: string, rulesYaml: string): Record<string, string> | null {
    if (!rulesYaml.trim()) return null;
    return { configYaml, rules: rulesYaml };
  }

  // Helper: run alert coverage analysis
  async function fetchAlertCoverage(body: Record<string, string>): Promise<CoverageReport> {
    const res = await fetch("/api/alert-coverage", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      const data = await res.json();
      throw new Error(data.error || `HTTP ${res.status}`);
    }
    return res.json();
  }

  // Config analysis
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

    // Also run alert coverage if rules are configured
    const alertBody = buildAlertBody(config, alertRulesTextRef.current);
    if (alertBody) {
      fetchAlertCoverage(alertBody)
        .then((data) => { setAlertCoverage(data); setAlertError(null); })
        .catch(() => {});
    }
  }, []);

  // Live mode: config analysis
  useEffect(() => {
    if (liveMode) {
      analyze(debouncedYaml);
    }
  }, [debouncedYaml, liveMode, analyze]);

  // Live mode: alert coverage re-analysis when alert rules change
  useEffect(() => {
    if (!liveMode || !debouncedYaml.trim()) return;
    const body = buildAlertBody(debouncedYaml, debouncedAlertRules);
    if (!body) return;

    fetchAlertCoverage(body)
      .then((data) => { setAlertCoverage(data); setAlertError(null); })
      .catch(() => {});
  }, [debouncedAlertRules, liveMode]); // eslint-disable-line react-hooks/exhaustive-deps

  const analysisFindings = result?.findings ?? [];
  const coverageFindings = alertCoverage ? coverageToFindings(alertCoverage) : [];
  const findings = [...analysisFindings, ...coverageFindings];

  const filteredFindings: Finding[] = cardFilter
    ? findings.filter((f) => {
        const ruleSet = rulesForRole(cardFilter.role);
        return (
          ruleSet.includes(f.ruleId) &&
          (f.scope === `pipeline:${cardFilter.pipeline}` || !f.scope)
        );
      })
    : findings;

  return (
    <div className="l-application">
      {metrics.error && (
        <Toast
          message={metrics.error}
          onDismiss={metrics.clearError}
        />
      )}
      {alertError && (
        <Toast
          message={alertError}
          onDismiss={() => setAlertError(null)}
        />
      )}
      <nav className="l-navigation is-dark">
        <div className="l-navigation__drawer">
          <ConfigInput
            yaml={yaml}
            onYamlChange={setYaml}
            alertRulesText={alertRulesText}
            onAlertRulesTextChange={setAlertRulesText}
            alertCoverage={alertCoverage}
            alertApiConnected={false}
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
                    entryCount={tap.entries.length + tap.spanEntries.length + tap.logEntries.length}
                    error={tap.error}
                    grpcAddr={tap.grpcAddr}
                    httpAddr={tap.httpAddr}
                    rateChanged={tap.rateChanged}
                    onReset={tap.resetCatalog}
                    onStart={tap.start}
                    onStop={tap.stop}
                  />
                  <MetricsConnection
                    status={metrics.status}
                    hasData={!!metrics.snapshot}
                    onConnect={metrics.connect}
                    onDisconnect={metrics.disconnect}
                    onReset={metrics.resetStore}
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
                    spanEntries={tap.spanEntries}
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
              <MetricCatalogPanel
                entries={tap.entries}
                spanEntries={tap.spanEntries}
                logEntries={tap.logEntries}
                filterAnalyses={result?.filterAnalyses}
                tapStatus={tap.status}
              />
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
