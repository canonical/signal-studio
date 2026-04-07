import { useEffect, useRef, useState } from "react";
import type { RemoteTapInfo, TapStatus } from "../types/api";
import { BrushCleaning } from "lucide-react";

interface TapConnectionProps {
  status: TapStatus;
  entryCount: number;
  error: string | null;
  grpcAddr: string | null;
  httpAddr: string | null;
  rateChanged: boolean;
  remotetap: RemoteTapInfo;
  onReset: () => void;
  onStart: () => void;
  onStop: () => void;
  onRemoteTapConnect: (addr: string) => void;
  onRemoteTapDisconnect: () => void;
}

function RadioTowerIcon() {
  return (
    <svg
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M4.9 16.1C1 12.2 1 5.8 4.9 1.9" />
      <path d="M7.8 13.2c-2.3-2.3-2.3-6.1 0-8.5" />
      <path d="M16.2 4.8c2.3 2.3 2.3 6.1 0 8.5" />
      <path d="M19.1 1.9C23 5.8 23 12.2 19.1 16.1" />
      <circle cx="12" cy="9" r="1" />
      <path d="M12 10v12" />
      <path d="M8 22h8" />
    </svg>
  );
}

function ToggleIcon({ active }: { active: boolean }) {
  if (active) {
    // toggle-right: green
    return (
      <svg
        width="18"
        height="18"
        viewBox="0 0 24 24"
        fill="none"
        stroke="#22c55e"
        strokeWidth="2"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <rect x="1" y="5" width="22" height="14" rx="7" ry="7" />
        <circle cx="16" cy="12" r="3" fill="#22c55e" />
      </svg>
    );
  }
  // toggle-left: black
  return (
    <svg
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <rect x="1" y="5" width="22" height="14" rx="7" ry="7" />
      <circle cx="8" cy="12" r="3" />
    </svg>
  );
}

function ListeningBadge() {
  return (
    <svg className="tap-icon__badge" width="12" height="12" viewBox="0 0 12 12">
      <circle cx="6" cy="6" r="6" fill="#22c55e" />
      <path
        d="M3.5 5.5C3.5 3.8 4.8 2.5 6 2.5s2.5 1.3 2.5 3"
        stroke="#fff"
        strokeWidth="1"
        fill="none"
        strokeLinecap="round"
      />
      <path
        d="M5 5.5C5 4.7 5.4 4 6 4s1 .7 1 1.5"
        stroke="#fff"
        strokeWidth="1"
        fill="none"
        strokeLinecap="round"
      />
      <circle cx="6" cy="6.5" r="0.7" fill="#fff" />
      <path d="M6 7.2v1.8" stroke="#fff" strokeWidth="1" strokeLinecap="round" />
    </svg>
  );
}

function WarningBadge() {
  return (
    <svg
      className="tap-icon__warning-badge"
      width="10"
      height="10"
      viewBox="0 0 10 10"
    >
      <circle cx="5" cy="5" r="5" fill="#f59e0b" />
      <text
        x="5"
        y="7.5"
        textAnchor="middle"
        fill="#fff"
        fontSize="8"
        fontWeight="bold"
      >
        !
      </text>
    </svg>
  );
}

function collectorSnippet(grpcAddr: string): string {
  // Extract host:port. If it's ":5317" (just port), use localhost.
  const endpoint = grpcAddr.startsWith(":") ? `localhost${grpcAddr}` : grpcAddr;

  return `exporters:
  otlp/signal-studio:
    endpoint: "${endpoint}"
    tls:
      insecure: true

service:
  pipelines:
    metrics:
      exporters: [..., otlp/signal-studio]
    traces:
      exporters: [..., otlp/signal-studio]
    logs:
      exporters: [..., otlp/signal-studio]`;
}

export function TapConnection({
  status,
  entryCount,
  error,
  grpcAddr,
  httpAddr,
  rateChanged,
  remotetap,
  onReset,
  onStart,
  onStop,
  onRemoteTapConnect,
  onRemoteTapDisconnect,
}: TapConnectionProps) {
  const [open, setOpen] = useState(false);
  const [copied, setCopied] = useState(false);
  const [remoteTapAddr, setRemoteTapAddr] = useState("");
  const popoutRef = useRef<HTMLDivElement>(null);
  const btnRef = useRef<HTMLButtonElement>(null);

  const isListening = status === "listening";
  const isError = status === "error";
  const isIdle = status === "idle";
  const isDisabled = status === "disabled";

  useEffect(() => {
    if (!open) return;
    function handleClick(e: MouseEvent) {
      if (
        popoutRef.current &&
        !popoutRef.current.contains(e.target as Node) &&
        btnRef.current &&
        !btnRef.current.contains(e.target as Node)
      ) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [open]);

  function handleCopy() {
    if (!grpcAddr) return;
    navigator.clipboard.writeText(collectorSnippet(grpcAddr));
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  let tooltip = "OTLP sampling tap";
  if (isListening) tooltip = `Tap listening — ${entryCount} signals discovered`;
  else if (isError) tooltip = "Tap error";
  else if (isDisabled) tooltip = "Tap disabled by server configuration";
  else if (isIdle) tooltip = "Tap idle — click to start";

  return (
    <div className="tap-icon-wrapper">
      <button
        ref={btnRef}
        className="tap-icon__btn"
        onClick={() => setOpen(!open)}
        title={tooltip}
      >
        <RadioTowerIcon />
        {isListening && !rateChanged && <ListeningBadge />}
        {isListening && rateChanged && <WarningBadge />}
        {isError && <span className="tap-icon__error-dot" />}
      </button>

      {open && (
        <div ref={popoutRef} className="tap-popout">
          <div className="tap-popout__status">
            {isDisabled ? (
              <>
                <span className="tap-popout__dot tap-popout__dot--disabled" />
                Tap disabled
              </>
            ) : (
              <>
                <button
                  className="tap-popout__toggle-btn"
                  onClick={isListening ? onStop : onStart}
                  type="button"
                  title={isListening ? "Stop tap" : "Start tap"}
                >
                  <ToggleIcon active={isListening} />
                </button>
                {isListening
                  ? "Listening for OTLP signals"
                  : isError
                    ? "Tap error"
                    : "Tap idle"}
                <button
                  className="tap-popout__reset-btn"
                  onClick={onReset}
                  type="button"
                  disabled={entryCount === 0}
                  title="Reset catalog"
                >
                  <BrushCleaning size={18} strokeWidth={1.5} />
                </button>
              </>
            )}
          </div>

          {isListening && (
            <div className="tap-popout__addrs">
              <span className="pipeline-card__filter-stat pipeline-card__filter-stat--neutral">
                <span className="pipeline-card__detail-proto">gRPC</span>
                {grpcAddr}
              </span>
              <span className="pipeline-card__filter-stat pipeline-card__filter-stat--neutral">
                <span className="pipeline-card__detail-proto">HTTP</span>
                {httpAddr}
              </span>
            </div>
          )}

          {rateChanged && (
            <div className="tap-popout__warning">
              <p className="tap-popout__warning-text">
                The telemetry ingestion rate has changed significantly. This
                likely means the scrape or collection interval was modified.
                Accumulated point counts may be inaccurate.
              </p>
            </div>
          )}

          {error && <p className="tap-popout__error">{error}</p>}



          {isDisabled && (
            <p className="tap-popout__hint">
              The OTLP sampling tap has been disabled by the server
              configuration (<code>TAP_DISABLED=true</code>).
            </p>
          )}

          {isIdle && (
            <p className="tap-popout__hint">
              Toggle the tap to start discovering metrics from your Collector.
            </p>
          )}

          {isListening && grpcAddr && entryCount === 0 && (
            <>
              <p className="tap-popout__hint">
                Add a fan-out exporter to your Collector config. To see which
                metrics a filter processor would drop, temporarily remove it
                from the pipeline so all metrics reach Signal Studio.
              </p>
              <div className="tap-popout__snippet">
                <div className="tap-popout__snippet-header">
                  <span className="tap-popout__snippet-title">
                    Collector Config
                  </span>
                  <button
                    className="tap-popout__copy-btn"
                    onClick={handleCopy}
                    type="button"
                  >
                    {copied ? "Copied!" : "Copy"}
                  </button>
                </div>
                <pre className="tap-popout__snippet-code">
                  {collectorSnippet(grpcAddr)}
                </pre>
              </div>
            </>
          )}

          <hr className="tap-popout__divider" />

          <div className="tap-popout__section-label">Remote tap</div>

          {remotetap.status === "idle" || remotetap.status === "error" ? (
            <div className="tap-popout__remotetap-form">
              <input
                className="tap-popout__remotetap-input"
                type="text"
                placeholder="host:port (e.g. localhost:12001)"
                value={remoteTapAddr}
                onChange={(e) => setRemoteTapAddr(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && remoteTapAddr.trim()) {
                    onRemoteTapConnect(remoteTapAddr.trim());
                  }
                }}
              />
              <button
                className="tap-popout__remotetap-btn"
                type="button"
                disabled={!remoteTapAddr.trim()}
                onClick={() => onRemoteTapConnect(remoteTapAddr.trim())}
              >
                Connect
              </button>
            </div>
          ) : (
            <div className="tap-popout__remotetap-status">
              <span className={`tap-popout__dot tap-popout__dot--${remotetap.status === "connected" ? "listening" : "connecting"}`} />
              <span className="tap-popout__remotetap-addr">{remotetap.addr}</span>
              <button
                className="tap-popout__remotetap-disconnect"
                type="button"
                onClick={onRemoteTapDisconnect}
              >
                Disconnect
              </button>
            </div>
          )}

          {remotetap.error && (
            <p className="tap-popout__error">{remotetap.error}</p>
          )}
        </div>
      )}
    </div>
  );
}
