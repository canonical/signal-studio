import { useEffect, useRef, useState } from "react";
import type { MetricsStatus } from "../types/api";

interface MetricsConnectionProps {
  status: MetricsStatus;
  error: string | null;
  onConnect: (url: string, token?: string) => Promise<void>;
  onDisconnect: () => Promise<void>;
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

function UnplugIcon() {
  return (
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 22v-5" />
      <path d="M9 8V3" />
      <path d="M15 8V1" />
      <path d="M18 8v2a6 6 0 0 1-12 0V8h12z" />
      <line x1="2" y1="2" x2="22" y2="22" />
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

export function MetricsConnection({
  status,
  error,
  onConnect,
  onDisconnect,
}: MetricsConnectionProps) {
  const [open, setOpen] = useState(false);
  const [url, setUrl] = useState(
    () => localStorage.getItem("signal-studio:metrics-url") ?? "http://localhost:8888/metrics",
  );
  const [token, setToken] = useState("");
  const [showToken, setShowToken] = useState(false);
  const [hovered, setHovered] = useState(false);
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

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!url.trim()) return;
    localStorage.setItem("signal-studio:metrics-url", url.trim());
    onConnect(url.trim(), token.trim() || undefined);
    setOpen(false);
  }

  function handleIconClick() {
    if (status === "connected" && hovered) {
      onDisconnect();
      return;
    }
    setOpen(!open);
  }

  const isConnected = status === "connected";
  const isConnecting = status === "connecting";
  const isError = status === "error";

  let tooltip = "Connect to live metrics";
  if (isConnected && hovered) tooltip = "Disconnect";
  else if (isConnected) tooltip = "Live metrics connected";
  else if (isConnecting) tooltip = "Connecting...";
  else if (isError) tooltip = "Connection error — click to configure";

  return (
    <div className="metrics-icon-wrapper">
      <button
        ref={btnRef}
        className={`metrics-icon__btn${isConnecting ? " metrics-icon__btn--pulse" : ""}`}
        onClick={handleIconClick}
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
        title={tooltip}
      >
        {isConnected && hovered ? <UnplugIcon /> : <PlugIcon />}
        {isConnected && !hovered && <CheckBadge />}
        {isError && <span className="metrics-icon__error-dot" />}
      </button>

      {open && (
        <div ref={popoutRef} className="metrics-popout">
          <form onSubmit={handleSubmit}>
            <label className="metrics-popout__label">Metrics endpoint</label>
            <input
              className="metrics-popout__input"
              type="text"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="http://localhost:8888/metrics"
              autoFocus
            />
            {showToken ? (
              <>
                <label className="metrics-popout__label">Bearer token</label>
                <input
                  className="metrics-popout__input"
                  type="password"
                  value={token}
                  onChange={(e) => setToken(e.target.value)}
                  placeholder="Optional"
                />
              </>
            ) : (
              <button
                type="button"
                className="metrics-popout__token-link"
                onClick={() => setShowToken(true)}
              >
                + Add bearer token
              </button>
            )}
            {error && <p className="metrics-popout__error">{error}</p>}
            <button className="metrics-popout__submit" type="submit">
              {isConnected ? "Reconnect" : "Connect"}
            </button>
          </form>
        </div>
      )}
    </div>
  );
}
