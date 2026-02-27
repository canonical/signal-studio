import { useEffect, useRef, useState } from "react";
import type { CoverageReport } from "../types/api";
import { Building2, KeyRound, ShieldAlert, X } from "lucide-react";

interface AlertRulesConnectionProps {
  report: CoverageReport | null;
  apiConnected: boolean;
  onApiConnect: (url: string, token: string, orgId: string) => void;
  onApiDisconnect: () => void;
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
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect x="1" y="5" width="22" height="14" rx="7" ry="7" />
      <circle cx="8" cy="12" r="3" />
    </svg>
  );
}

function ConnectedBadge() {
  return (
    <svg className="metrics-icon__badge" width="10" height="10" viewBox="0 0 10 10">
      <circle cx="5" cy="5" r="5" fill="#22c55e" />
      <path d="M3 5.2l1.5 1.5 3-3" stroke="#fff" strokeWidth="1.2" fill="none" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}

export function AlertRulesConnection({
  report,
  apiConnected,
  onApiConnect,
  onApiDisconnect,
}: AlertRulesConnectionProps) {
  const [open, setOpen] = useState(false);
  const [url, setUrl] = useState(
    () => localStorage.getItem("signal-studio:rules-url") ?? "",
  );
  const [token, setToken] = useState("");
  const [orgId, setOrgId] = useState("");
  const [showToken, setShowToken] = useState(false);
  const [showOrgId, setShowOrgId] = useState(false);
  const popoutRef = useRef<HTMLDivElement>(null);
  const btnRef = useRef<HTMLButtonElement>(null);

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
    if (apiConnected) {
      onApiDisconnect();
    } else {
      if (!url.trim()) return;
      localStorage.setItem("signal-studio:rules-url", url.trim());
      onApiConnect(url.trim(), token.trim(), orgId.trim());
    }
  }

  let tooltip = "Alert rule coverage";
  if (apiConnected) tooltip = "Rules API connected";
  else if (report) tooltip = `Alert coverage: ${report.summary.total} rules analyzed`;

  return (
    <div className="metrics-icon-wrapper">
      <button
        ref={btnRef}
        className="metrics-icon__btn"
        onClick={() => setOpen(!open)}
        title={tooltip}
      >
        <ShieldAlert size={18} strokeWidth={1.5} />
        {apiConnected && <ConnectedBadge />}
      </button>

      {open && (
        <div ref={popoutRef} className="metrics-popout">
          <div className="tap-popout__status">
            <button
              className="tap-popout__toggle-btn"
              onClick={handleToggle}
              type="button"
              title={apiConnected ? "Disconnect from rules API" : "Connect to rules API"}
            >
              <ToggleIcon active={apiConnected} />
            </button>
            {apiConnected ? "Connected to rules API" : "Rules API"}
          </div>

          {!apiConnected && (
            <>
              <div className="metrics-popout__label-row">
                <label className="metrics-popout__label">
                  Prometheus / Mimir endpoint
                </label>
                <button
                  type="button"
                  className="metrics-popout__token-link"
                  onClick={() => setShowToken(true)}
                  disabled={showToken}
                  title="Add bearer token"
                >
                  <KeyRound size={14} />
                </button>
                <button
                  type="button"
                  className="metrics-popout__token-link"
                  onClick={() => setShowOrgId(true)}
                  disabled={showOrgId}
                  title="Add org ID (Mimir)"
                >
                  <Building2 size={14} />
                </button>
              </div>
              <input
                className="metrics-popout__input"
                type="text"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                placeholder="http://localhost:9090"
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
                  />
                </>
              )}
              {showOrgId && (
                <>
                  <div className="metrics-popout__label-row">
                    <label className="metrics-popout__label">X-Scope-OrgID</label>
                    <button
                      type="button"
                      className="metrics-popout__token-link"
                      onClick={() => { setShowOrgId(false); setOrgId(""); }}
                      title="Remove org ID"
                    >
                      <X size={14} />
                    </button>
                  </div>
                  <input
                    className="metrics-popout__input"
                    type="text"
                    value={orgId}
                    onChange={(e) => setOrgId(e.target.value)}
                    placeholder="Optional (Mimir multi-tenancy)"
                  />
                </>
              )}
            </>
          )}

        </div>
      )}
    </div>
  );
}
