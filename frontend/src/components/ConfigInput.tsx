import { useState, useRef, useCallback, useEffect } from "react";
import Editor, { type OnMount, type BeforeMount } from "@monaco-editor/react";
import type { editor as monacoEditor } from "monaco-editor";
import type * as Monaco from "monaco-editor";
import jsYaml from "js-yaml";
import type { CoverageReport, AlertStatus } from "../types/api";

const VANILLA_DARK_BG = "#262626";

export const PLACEHOLDER = `# Paste your OTel Collector config here
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

processors:
  batch:
    timeout: 5s

exporters:
  debug:
    verbosity: detailed

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [debug]
`;

const ALERT_RULES_PLACEHOLDER = `# Paste alert rules YAML here
groups:
  - name: my_alerts
    rules:
      - alert: HighErrorRate
        expr: rate(http_errors_total[5m]) > 0.05
`;

type Tab = "config" | "alerts";

interface ConfigInputProps {
  yaml: string;
  onYamlChange: (yaml: string) => void;
  alertRulesText: string;
  onAlertRulesTextChange: (text: string) => void;
  alertCoverage: CoverageReport | null;
  alertApiConnected: boolean;
  onAnalyze: () => void;
  liveMode: boolean;
  onLiveModeChange: (enabled: boolean) => void;
  loading: boolean;
}

const LINE_CLASS: Record<AlertStatus, string> = {
  safe: "alert-line--safe",
  broken: "alert-line--broken",
  at_risk: "alert-line--at-risk",
  would_activate: "alert-line--would-activate",
  unknown: "alert-line--unknown",
};

/** Parse alert rules YAML to find the full line range of each `- alert:` entry. */
function findAlertRanges(text: string): { name: string; startLine: number; endLine: number }[] {
  const results: { name: string; startLine: number; endLine: number }[] = [];
  const lines = text.split("\n");
  for (let i = 0; i < lines.length; i++) {
    const match = lines[i]?.match(/^(\s*)-\s*alert:\s*(.+?)\s*$/);
    if (!match?.[2]) continue;
    const indent = match[1]?.length ?? 0;
    // Scan forward: entry continues while lines are empty or indented deeper.
    let end = i;
    for (let j = i + 1; j < lines.length; j++) {
      const ln = lines[j] ?? "";
      if (ln.trim() === "") { end = j; continue; }
      const leadingSpaces = ln.search(/\S/);
      if (leadingSpaces <= indent) break;
      end = j;
    }
    results.push({ name: match[2], startLine: i + 1, endLine: end + 1 }); // 1-based
  }
  return results;
}

function prettyPrintYaml(raw: string): string | null {
  try {
    const parsed = jsYaml.load(raw);
    if (parsed == null || typeof parsed !== "object") return null;
    return jsYaml.dump(parsed, {
      indent: 2,
      lineWidth: 120,
      noRefs: true,
      sortKeys: false,
    });
  } catch {
    return null;
  }
}

export function ConfigInput({
  yaml,
  onYamlChange,
  alertRulesText,
  onAlertRulesTextChange,
  alertCoverage,
  alertApiConnected,
  onAnalyze,
  liveMode,
  onLiveModeChange,
  loading,
}: ConfigInputProps) {
  const [activeTab, setActiveTab] = useState<Tab>("config");
  const configEditorRef = useRef<monacoEditor.IStandaloneCodeEditor | null>(
    null,
  );
  const [alertsEditor, setAlertsEditor] = useState<monacoEditor.IStandaloneCodeEditor | null>(null);
  const monacoRef = useRef<typeof Monaco | null>(null);
  const decorationsRef = useRef<monacoEditor.IEditorDecorationsCollection | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Apply gutter decorations to the alerts editor when coverage data changes.
  // Coverage results arrive in the same order as the YAML alert definitions,
  // so we match by (name, index) to handle duplicate alert names correctly.
  useEffect(() => {
    const monaco = monacoRef.current;
    if (!alertsEditor || !monaco) return;

    const alertRanges = findAlertRanges(alertRulesText);
    if (!alertCoverage || alertRanges.length === 0) {
      decorationsRef.current?.clear();
      decorationsRef.current = null;
      return;
    }

    // Build a queue per alert name so duplicate names are consumed in order.
    const statusQueues = new Map<string, AlertStatus[]>();
    for (const r of alertCoverage.results) {
      const q = statusQueues.get(r.alertName);
      if (q) {
        q.push(r.status);
      } else {
        statusQueues.set(r.alertName, [r.status]);
      }
    }

    const newDecorations: monacoEditor.IModelDeltaDecoration[] = [];
    for (const { name, startLine, endLine } of alertRanges) {
      const q = statusQueues.get(name);
      const status = q?.shift();
      if (!status) continue;
      const lineClass = LINE_CLASS[status];
      if (!lineClass) continue;
      newDecorations.push({
        range: new monaco.Range(startLine, 1, endLine, 1),
        options: {
          isWholeLine: true,
          className: lineClass,
        },
      });
    }

    if (decorationsRef.current) {
      decorationsRef.current.clear();
    }
    decorationsRef.current = alertsEditor.createDecorationsCollection(newDecorations);
  }, [alertRulesText, alertCoverage, alertsEditor]);

  const handleUpload = useCallback(() => {
    fileInputRef.current?.click();
  }, []);

  const handleFileChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (!file) return;
      const reader = new FileReader();
      reader.onload = () => {
        const text = reader.result as string;
        const pretty = prettyPrintYaml(text);
        if (activeTab === "config") {
          onYamlChange(pretty ?? text);
        } else {
          onAlertRulesTextChange(pretty ?? text);
        }
      };
      reader.readAsText(file);
      e.target.value = "";
    },
    [activeTab, onYamlChange, onAlertRulesTextChange],
  );

  const handleDownload = useCallback(() => {
    const content = activeTab === "config" ? yaml : alertRulesText;
    const filename =
      activeTab === "config" ? "collector-config.yaml" : "alert-rules.yaml";
    const blob = new Blob([content], { type: "text/yaml" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = filename;
    a.click();
    URL.revokeObjectURL(url);
  }, [activeTab, yaml, alertRulesText]);

  const handleBeforeMount: BeforeMount = (monaco) => {
    monacoRef.current = monaco;
    monaco.editor.defineTheme("vanilla-dark", {
      base: "vs-dark",
      inherit: true,
      rules: [],
      colors: {
        "editor.background": VANILLA_DARK_BG,
      },
    });
  };

  const handleConfigMount: OnMount = (editor) => {
    configEditorRef.current = editor;

    editor.onDidPaste(() => {
      const content = editor.getValue();
      const pretty = prettyPrintYaml(content);
      if (pretty && pretty !== content) {
        onYamlChange(pretty);
      }
    });
  };

  const handleAlertsMount: OnMount = (editor) => {
    setAlertsEditor(editor);

    editor.onDidPaste(() => {
      const content = editor.getValue();
      const pretty = prettyPrintYaml(content);
      if (pretty && pretty !== content) {
        onAlertRulesTextChange(pretty);
      }
    });
  };

  return (
    <div className="p-panel">
      <div className="p-panel__header is-sticky">
        <h4 className="p-panel__title">
          <span className="app-logo">
            <svg
              viewBox="0 0 252.43 400"
              width="18"
              height="28"
              aria-hidden="true"
            >
              <rect fill="#e95420" width="252.43" height="400" />
              <circle fill="#fff" cx="57.72" cy="254.27" r="26.13" />
              <circle fill="#fff" cx="166.56" cy="196.97" r="26.13" />
              <path
                fill="#fff"
                d="m116.66,321.53c-18.82-4.03-34.56-16.06-43.4-33.1-6.97,3.17-14.8,4.14-22.34,2.75,10.7,26.28,33.4,45.37,61.25,51.34,6.11,1.31,12.35,1.95,18.56,1.91-4.8-6.31-7.47-13.92-7.65-21.85-2.15-.24-4.3-.59-6.41-1.04Z"
              />
              <circle fill="#fff" cx="160.67" cy="321.68" r="26.13" />
              <path
                fill="#fff"
                d="m197.03,312.08c8.13-10.24,13.85-22.35,16.61-35.23,4.81-22.44.32-45.99-12.32-65.09-3.01,7.09-8.12,13.09-14.7,17.21,7.05,13.29,9.2,28.63,6.04,43.39-1.55,7.22-4.28,14.03-8.13,20.25,6.15,5.04,10.49,11.83,12.51,19.47Z"
              />
              <path
                fill="#fff"
                d="m55.7,216.8c.66-.04,1.33-.05,1.99-.05,2.64,0,5.28.28,7.89.84,4.26.91,8.27,2.53,11.93,4.8,11.77-16.92,30.75-27.08,51.32-27.45.11-1.97.37-3.95.79-5.88,1.2-5.6,3.65-10.83,7.14-15.31-32.86-2.6-64.8,14.36-81.06,43.05Z"
              />
            </svg>
            Signal Studio
          </span>
        </h4>
        <div className="p-panel__controls" />
      </div>
      <div className="editor-tabs">
        <button
          className={`editor-tabs__tab${activeTab === "config" ? " is-active" : ""}`}
          onClick={() => setActiveTab("config")}
          type="button"
        >
          Configuration
        </button>
        <button
          className={`editor-tabs__tab${activeTab === "alerts" ? " is-active" : ""}`}
          onClick={() => setActiveTab("alerts")}
          type="button"
        >
          Alert Rules
        </button>
        <div className="editor-tabs__actions">
          <input
            ref={fileInputRef}
            type="file"
            accept=".yaml,.yml"
            style={{ display: "none" }}
            onChange={handleFileChange}
          />
          <button
            className="editor-tabs__action-btn"
            onClick={handleUpload}
            title="Open YAML file"
            type="button"
          >
            <svg
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="m6 14 1.5-2.9A2 2 0 0 1 9.24 10H20a2 2 0 0 1 1.94 2.5l-1.54 6a2 2 0 0 1-1.95 1.5H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h3.9a2 2 0 0 1 1.69.9l.81 1.2a2 2 0 0 0 1.67.9H18a2 2 0 0 1 2 2v2" />
            </svg>
          </button>
          <button
            className="editor-tabs__action-btn"
            onClick={handleDownload}
            title="Save YAML file"
            type="button"
          >
            <svg
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="M15.2 3a2 2 0 0 1 1.4.6l3.8 3.8a2 2 0 0 1 .6 1.4V19a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2z" />
              <path d="M17 21v-7a1 1 0 0 0-1-1H8a1 1 0 0 0-1 1v7" />
              <path d="M7 3v4a1 1 0 0 0 1 1h7" />
            </svg>
          </button>
          <span className="editor-tabs__separator" />
          <button
            className="editor-tabs__action-btn"
            onClick={() => onLiveModeChange(!liveMode)}
            title={liveMode ? "Pause live analysis" : "Resume live analysis"}
            type="button"
          >
            {liveMode ? (
              <svg
                width="14"
                height="14"
                viewBox="0 0 24 24"
                fill="none"
                stroke="#22c55e"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <path d="M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8" />
                <path d="M21 3v5h-5" />
              </svg>
            ) : (
              <svg
                width="14"
                height="14"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              >
                <path d="M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8" />
                <path d="M21 3v5h-5" />
                <line x1="2" y1="2" x2="22" y2="22" />
              </svg>
            )}
          </button>
          <button
            className="editor-tabs__action-btn"
            onClick={onAnalyze}
            disabled={liveMode || loading || !yaml.trim()}
            title="Run analysis"
            type="button"
          >
            <svg
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill={liveMode ? "currentColor" : "#22c55e"}
              stroke="none"
            >
              <path d="M6 4l15 8-15 8V4z" />
            </svg>
          </button>
        </div>
      </div>
      <div className="p-panel__content editor-content">
        <div
          style={{
            display: activeTab === "config" ? "block" : "none",
            height: "100%",
          }}
        >
          <Editor
            height="100%"
            defaultLanguage="yaml"
            value={yaml || PLACEHOLDER}
            onChange={(v) => onYamlChange(v ?? "")}
            beforeMount={handleBeforeMount}
            onMount={handleConfigMount}
            theme="vanilla-dark"
            options={{
              minimap: { enabled: false },
              fontSize: 13,
              lineNumbers: "on",
              scrollBeyondLastLine: false,
              wordWrap: "on",
              tabSize: 2,
              automaticLayout: true,
            }}
          />
        </div>
        <div
          style={{
            display: activeTab === "alerts" ? "block" : "none",
            height: "100%",
          }}
        >
          <Editor
            height="100%"
            defaultLanguage="yaml"
            value={alertRulesText || ALERT_RULES_PLACEHOLDER}
            onChange={(v) => onAlertRulesTextChange(v ?? "")}
            beforeMount={handleBeforeMount}
            onMount={handleAlertsMount}
            theme="vanilla-dark"
            options={{
              minimap: { enabled: false },
              fontSize: 13,
              lineNumbers: "on",
              scrollBeyondLastLine: false,
              wordWrap: "on",
              tabSize: 2,
              automaticLayout: true,
              readOnly: alertApiConnected,
            }}
          />
        </div>
      </div>
    </div>
  );
}
