import { useRef } from "react";
import Editor, { type OnMount, type BeforeMount } from "@monaco-editor/react";
import type { editor as monacoEditor } from "monaco-editor";
import jsYaml from "js-yaml";

const VANILLA_DARK_BG = "#262626";

const PLACEHOLDER = `# Paste your OTel Collector config here
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

interface ConfigInputProps {
  yaml: string;
  onYamlChange: (yaml: string) => void;
  onAnalyze: () => void;
  liveMode: boolean;
  onLiveModeChange: (enabled: boolean) => void;
  loading: boolean;
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
  onAnalyze,
  liveMode,
  onLiveModeChange,
  loading,
}: ConfigInputProps) {
  const editorRef = useRef<monacoEditor.IStandaloneCodeEditor | null>(null);

  const handleBeforeMount: BeforeMount = (monaco) => {
    monaco.editor.defineTheme("vanilla-dark", {
      base: "vs-dark",
      inherit: true,
      rules: [],
      colors: {
        "editor.background": VANILLA_DARK_BG,
      },
    });
  };

  const handleMount: OnMount = (editor) => {
    editorRef.current = editor;

    editor.onDidPaste(() => {
      const content = editor.getValue();
      const pretty = prettyPrintYaml(content);
      if (pretty && pretty !== content) {
        onYamlChange(pretty);
      }
    });
  };

  return (
    <div className="p-panel">
      <div className="p-panel__header is-sticky">
        <h4 className="p-panel__title">
          <span className="app-logo">
            <svg viewBox="0 0 252.43 400" width="18" height="28" aria-hidden="true">
              <rect fill="#e95420" width="252.43" height="400"/>
              <circle fill="#fff" cx="57.72" cy="254.27" r="26.13"/>
              <circle fill="#fff" cx="166.56" cy="196.97" r="26.13"/>
              <path fill="#fff" d="m116.66,321.53c-18.82-4.03-34.56-16.06-43.4-33.1-6.97,3.17-14.8,4.14-22.34,2.75,10.7,26.28,33.4,45.37,61.25,51.34,6.11,1.31,12.35,1.95,18.56,1.91-4.8-6.31-7.47-13.92-7.65-21.85-2.15-.24-4.3-.59-6.41-1.04Z"/>
              <circle fill="#fff" cx="160.67" cy="321.68" r="26.13"/>
              <path fill="#fff" d="m197.03,312.08c8.13-10.24,13.85-22.35,16.61-35.23,4.81-22.44.32-45.99-12.32-65.09-3.01,7.09-8.12,13.09-14.7,17.21,7.05,13.29,9.2,28.63,6.04,43.39-1.55,7.22-4.28,14.03-8.13,20.25,6.15,5.04,10.49,11.83,12.51,19.47Z"/>
              <path fill="#fff" d="m55.7,216.8c.66-.04,1.33-.05,1.99-.05,2.64,0,5.28.28,7.89.84,4.26.91,8.27,2.53,11.93,4.8,11.77-16.92,30.75-27.08,51.32-27.45.11-1.97.37-3.95.79-5.88,1.2-5.6,3.65-10.83,7.14-15.31-32.86-2.6-64.8,14.36-81.06,43.05Z"/>
            </svg>
            OTel Signal Lens
          </span>
        </h4>
        <div className="p-panel__controls">
          <label className="p-checkbox--inline u-no-margin--bottom">
            <input
              type="checkbox"
              className="p-checkbox__input"
              checked={liveMode}
              onChange={(e) => onLiveModeChange(e.target.checked)}
            />
            <span className="p-checkbox__label">Live</span>
          </label>
        </div>
      </div>
      <div className="p-panel__content editor-content">
        <Editor
          height="100%"
          defaultLanguage="yaml"
          value={yaml || PLACEHOLDER}
          onChange={(v) => onYamlChange(v ?? "")}
          beforeMount={handleBeforeMount}
          onMount={handleMount}
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
      {!liveMode && (
        <div className="editor-actions">
          <button
            className="p-button--positive u-no-margin--bottom"
            onClick={onAnalyze}
            disabled={loading || !yaml.trim()}
            style={{ width: "100%" }}
          >
            {loading ? "Analyzing\u2026" : "Analyze"}
          </button>
        </div>
      )}
    </div>
  );
}
