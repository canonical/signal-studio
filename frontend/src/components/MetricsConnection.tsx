import { useEffect, useRef, useState } from "react";
import type { MetricsStatus } from "../types/api";
import { BrushCleaning, KeyRound, X } from "lucide-react";

interface MetricsConnectionProps {
  status: MetricsStatus;
  hasData: boolean;
  onConnect: (url: string, token?: string) => Promise<void>;
  onDisconnect: () => Promise<void>;
  onReset: () => Promise<void>;
}

function PlugIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 22v-5" />
      <path d="M9 8V2" />
      <path d="M15 8V2" />
      <path d="M18 8v2a6 6 0 0 1-12 0V8h12z" />
    </svg>
  );
}

function CheckBadge() {
  return (
    <svg className="metrics-icon__badge" width="10" height="10" viewBox="0 0 10 10">
      <circle cx="5" cy="5" r="5" fill="#22c55e" />
      <path d="M3 5.2l1.5 1.5 3-3" stroke="#fff" strokeWidth="1.2" fill="none" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

function ToggleIcon({ active }: { active: boolean }) {
  if (active) {
    return (
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#22c55e" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <rect x="1" y="5" width="22" height="14" rx="7" ry="7" />
        <circle cx="16" cy="12" r="3" fill="#22c55e" />
      </svg>
    );
  }
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="1" y="5" width="22" height="14" rx="7" ry="7" />
      <circle cx="8" cy="12" r="3" />
    </svg>
  );
}

export function MetricsConnection({
  status,
  hasData,
  onConnect,
  onDisconnect,
  onReset,
}: MetricsConnectionProps) {
  const [open, setOpen] = useState(false);
  const [url, setUrl] = useState(
    () => localStorage.getItem("signal-studio:metrics-url") ?? "http://localhost:8888/metrics",
  );
  const [token, setToken] = useState("");
  const [showToken, setShowToken] = useState(false);
  const popoutRef = useRef<HTMLDivElement>(null);
  const btnRef = useRef<HTMLButtonElement>(null);

  // Close popout on outside click
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

  function handleToggle() {
    if (isConnected) {
      onDisconnect();
    } else {
      if (!url.trim()) return;
      localStorage.setItem("signal-studio:metrics-url", url.trim());
      onConnect(url.trim(), token.trim() || undefined);
    }
  }

  const isConnected = status === "connected";
  const isConnecting = status === "connecting";
  const isError = status === "error";
  const isDisabled = isConnected || isConnecting;

  let tooltip = "Connect to live metrics";
  if (isConnected) tooltip = "Live metrics connected";
  else if (isConnecting) tooltip = "Connecting...";
  else if (isError) tooltip = "Connection error — click to configure";

  return (
    <div className="metrics-icon-wrapper">
      <button
        ref={btnRef}
        className={`metrics-icon__btn${isConnecting ? " metrics-icon__btn--pulse" : ""}`}
        onClick={() => setOpen(!open)}
        title={tooltip}
      >
        <PlugIcon />
        {isConnected && <CheckBadge />}
        {isError && <span className="metrics-icon__error-dot" />}
      </button>

      {open && (
        <div ref={popoutRef} className="metrics-popout">
          <div className="tap-popout__status">
            <button
              className="tap-popout__toggle-btn"
              onClick={handleToggle}
              type="button"
              title={isConnected ? "Disconnect" : "Connect"}
            >
              <ToggleIcon active={isConnected} />
            </button>
            {isConnected ? "Connected" : isConnecting ? "Connecting..." : "Metrics endpoint"}
            <button
              className="tap-popout__reset-btn"
              onClick={() => onReset()}
              type="button"
              disabled={!hasData}
              title="Reset metrics data"
            >
              <BrushCleaning size={18} strokeWidth={1.5} />
            </button>
          </div>

          <div className="metrics-popout__label-row">
            <label className="metrics-popout__label">Endpoint URL</label>
            <button
              type="button"
              className="metrics-popout__token-link"
              onClick={() => setShowToken(true)}
              disabled={showToken || isDisabled}
              title="Add bearer token"
            >
              <KeyRound size={14} />
            </button>
          </div>
          <input
            className="metrics-popout__input"
            type="text"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder="http://localhost:8888/metrics"
            disabled={isDisabled}
          />
          {showToken && (
            <>
              <div className="metrics-popout__label-row">
                <label className="metrics-popout__label">Bearer token</label>
                <button
                  type="button"
                  className="metrics-popout__token-link"
                  onClick={() => { setShowToken(false); setToken(""); }}
                  title="Remove bearer token"
                >
                  <X size={14} />
                </button>
              </div>
              <input
                className="metrics-popout__input"
                type="password"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                placeholder="Optional"
                disabled={isDisabled}
              />
            </>
          )}

        </div>
      )}
    </div>
  );
}
