import { useEffect, useRef, useState } from "react";
import type { TapStatus } from "../types/api";

interface TapConnectionProps {
  status: TapStatus;
  entryCount: number;
  error: string | null;
  grpcAddr: string | null;
  httpAddr: string | null;
  rateChanged: boolean;
  onReset: () => void;
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

function CheckBadge() {
  return (
    <svg className="tap-icon__badge" width="10" height="10" viewBox="0 0 10 10">
      <circle cx="5" cy="5" r="5" fill="#22c55e" />
      <path d="M3 5.2l1.5 1.5 3-3" stroke="#fff" strokeWidth="1.2" fill="none" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function WaitingBadge() {
  return (
    <svg className="tap-icon__badge tap-icon__badge--waiting" width="10" height="10" viewBox="0 0 10 10">
      <circle cx="5" cy="5" r="5" fill="#666" />
      <circle cx="5" cy="5" r="2.5" fill="none" stroke="#fff" strokeWidth="1" strokeDasharray="4 2" />
    </svg>
  );
}

function WarningBadge() {
  return (
    <svg className="tap-icon__warning-badge" width="10" height="10" viewBox="0 0 10 10">
      <circle cx="5" cy="5" r="5" fill="#f59e0b" />
      <text x="5" y="7.5" textAnchor="middle" fill="#fff" fontSize="8" fontWeight="bold">!</text>
    </svg>
  );
}

function collectorSnippet(grpcAddr: string): string {
  // Extract host:port. If it's ":4317" (just port), use localhost.
  const endpoint = grpcAddr.startsWith(":") ? `localhost${grpcAddr}` : grpcAddr;

  return `exporters:
  otlp/signal-studio:
    endpoint: "${endpoint}"
    tls:
      insecure: true

service:
  pipelines:
    metrics:
      # Add otlp/signal-studio alongside
      # your existing exporters
      exporters: [..., otlp/signal-studio]`;
}

export function TapConnection({
  status,
  entryCount,
  error,
  grpcAddr,
  httpAddr,
  rateChanged,
  onReset,
}: TapConnectionProps) {
  const [open, setOpen] = useState(false);
  const [copied, setCopied] = useState(false);
  const popoutRef = useRef<HTMLDivElement>(null);
  const btnRef = useRef<HTMLButtonElement>(null);

  const isListening = status === "listening";
  const isError = status === "error";
  const isIdle = status === "idle";

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
  if (isListening) tooltip = `Tap listening — ${entryCount} metrics discovered`;
  else if (isError) tooltip = "Tap error";
  else if (isIdle) tooltip = "Tap not enabled — click for setup instructions";

  return (
    <div className="tap-icon-wrapper">
      <button
        ref={btnRef}
        className="tap-icon__btn"
        onClick={() => setOpen(!open)}
        title={tooltip}
      >
        <RadioTowerIcon />
        {isListening && entryCount === 0 && <WaitingBadge />}
        {isListening && entryCount > 0 && !rateChanged && <CheckBadge />}
        {isListening && entryCount > 0 && rateChanged && <WarningBadge />}
        {isError && <span className="tap-icon__error-dot" />}
      </button>

      {open && (
        <div ref={popoutRef} className="tap-popout">
          <div className="tap-popout__status">
            <span
              className={`tap-popout__dot tap-popout__dot--${isListening ? "listening" : isError ? "error" : "idle"}`}
            />
            {isListening
              ? "Listening for OTLP metrics"
              : isError
                ? "Tap error"
                : "Tap not enabled"}
          </div>

          {isListening && (
            <div className="tap-popout__addrs">
              <span>gRPC: {grpcAddr}</span>
              <span>HTTP: {httpAddr}</span>
            </div>
          )}

          {rateChanged && (
            <div className="tap-popout__warning">
              <p className="tap-popout__warning-text">
                The telemetry ingestion rate has changed significantly. This
                likely means the scrape or collection interval was modified.
                Accumulated point counts may be inaccurate.
              </p>
              <button
                className="tap-popout__reset-btn"
                onClick={onReset}
                type="button"
              >
                Reset catalog
              </button>
            </div>
          )}

          {error && <p className="tap-popout__error">{error}</p>}

          {isIdle && (
            <p className="tap-popout__hint">
              Start the backend with <code>TAP_ENABLED=true</code> to enable the
              OTLP sampling tap.
            </p>
          )}

          {isListening && grpcAddr && (
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

          {entryCount > 0 && (
            <p className="tap-popout__count">
              {entryCount} metric{entryCount !== 1 ? "s" : ""} discovered
            </p>
          )}
        </div>
      )}
    </div>
  );
}
