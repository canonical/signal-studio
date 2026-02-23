import { useCallback, useEffect, useRef, useState } from "react";
import type { AnalyzeResponse } from "./types/api";
import { useDebounce } from "./hooks/useDebounce";
import { ConfigInput } from "./components/ConfigInput";
import { FindingsPanel } from "./components/FindingsPanel";
import { PipelineGraph } from "./components/PipelineGraph";

const DEBOUNCE_MS = 500;

export default function App() {
  const [yaml, setYaml] = useState(
    () => localStorage.getItem("otel-signal-lens:yaml") ?? "",
  );
  const [result, setResult] = useState<AnalyzeResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [liveMode, setLiveMode] = useState(true);

  useEffect(() => {
    localStorage.setItem("otel-signal-lens:yaml", yaml);
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
        <div className="p-panel">
          <div className="p-panel__header is-sticky">
            <h4 className="p-panel__title">Pipelines</h4>
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
              <PipelineGraph config={result.config} />
            ) : (
              <p className="u-text--muted">
                Paste a collector configuration to visualize pipelines.
              </p>
            )}
          </div>
        </div>
      </main>

      <aside className="l-aside is-pinned">
        <div className="p-panel">
          <div className="p-panel__header is-sticky">
            <h4 className="p-panel__title">
              Recommendations{findings.length > 0 && ` (${findings.length})`}
            </h4>
          </div>
          <div className="p-panel__content">
            {result ? (
              <FindingsPanel findings={findings} />
            ) : (
              <p className="u-text--muted">
                Recommendations will appear here.
              </p>
            )}
          </div>
        </div>
      </aside>
    </div>
  );
}
