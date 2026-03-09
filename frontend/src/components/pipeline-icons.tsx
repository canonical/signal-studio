import type { Signal } from "../types/api";
import type { ColumnRole } from "./pipeline-graph";

export function SignalIcon({ signal }: { signal: Signal }) {
  const props = {
    className: "pipeline-section__signal-icon",
    width: 14,
    height: 14,
    viewBox: "0 0 24 24",
    fill: "none",
    stroke: "currentColor",
    strokeWidth: 1.5,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
  };
  switch (signal) {
    case "metrics":
      return (
        <svg {...props}>
          <polyline points="22 12 18 12 15 21 9 3 6 12 2 12" />
        </svg>
      );
    case "traces":
      return (
        <svg {...props}>
          <rect x="2" y="4" width="20" height="4" rx="1" />
          <rect x="6" y="11" width="14" height="4" rx="1" />
          <rect x="10" y="18" width="8" height="4" rx="1" />
        </svg>
      );
    case "logs":
      return (
        <svg {...props}>
          <line x1="3" y1="5" x2="21" y2="5" />
          <line x1="3" y1="10" x2="17" y2="10" />
          <line x1="3" y1="15" x2="19" y2="15" />
          <line x1="3" y1="20" x2="14" y2="20" />
        </svg>
      );
  }
}

export type PillIconType = "stack" | "timer" | "kept" | "dropped" | "memory" | "spike";

export function PillIcon({ icon }: { icon: PillIconType }) {
  const props = {
    className: "pipeline-card__pill-icon",
    width: 12,
    height: 12,
    viewBox: "0 0 24 24",
    fill: "none",
    stroke: "currentColor",
    strokeWidth: 2.5,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
  };
  switch (icon) {
    case "stack":
      return (
        <svg {...props}>
          <path d="M12 2 2 7l10 5 10-5-10-5Z" />
          <path d="m2 17 10 5 10-5" />
          <path d="m2 12 10 5 10-5" />
        </svg>
      );
    case "timer":
      return (
        <svg {...props}>
          <circle cx="12" cy="13" r="8" />
          <path d="M12 9v4l2 2" />
          <path d="M5 3 2 6" />
          <path d="m22 6-3-3" />
        </svg>
      );
    case "kept":
      return (
        <svg {...props}>
          <path d="M20 6 9 17l-5-5" />
        </svg>
      );
    case "dropped":
      return (
        <svg {...props}>
          <path d="M18 6 6 18" />
          <path d="m6 6 12 12" />
        </svg>
      );
    case "memory":
      return (
        <svg {...props}>
          <rect x="2" y="6" width="20" height="12" rx="2" />
          <path d="M6 10v4" />
          <path d="M10 10v4" />
          <path d="M14 10v4" />
          <path d="M18 10v4" />
        </svg>
      );
    case "spike":
      return (
        <svg {...props}>
          <polyline points="2 18 8 12 12 16 22 4" />
          <polyline points="16 4 22 4 22 10" />
        </svg>
      );
  }
}

const componentIconProps = {
  className: "pipeline-card__component-icon",
  width: 14,
  height: 14,
  viewBox: "0 0 24 24",
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 1.5,
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
};

export function ComponentRoleIcon({ role }: { role: ColumnRole }) {
  switch (role) {
    case "receivers":
      return (
        <svg {...componentIconProps}>
          <polyline points="7 10 12 15 17 10" />
          <line x1="12" y1="15" x2="12" y2="3" />
          <path d="M20 21H4" />
        </svg>
      );
    case "exporters":
      return (
        <svg {...componentIconProps}>
          <polyline points="7 10 12 5 17 10" />
          <line x1="12" y1="5" x2="12" y2="17" />
          <path d="M20 21H4" />
        </svg>
      );
    case "processors":
      return (
        <svg {...componentIconProps}>
          <rect x="6" y="6" width="12" height="12" rx="1" />
          <path d="M9 1v4M15 1v4M9 19v4M15 19v4M1 9h4M1 15h4M19 9h4M19 15h4" />
        </svg>
      );
  }
}

export function ThroughputIcon() {
  return (
    <svg
      className="pipeline-card__metrics-icon"
      width="12"
      height="12"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M12 22C6.5 22 2 17.5 2 12S6.5 2 12 2s10 4.5 10 10" />
      <path d="M12 12l4-4" />
      <circle cx="12" cy="12" r="1.5" fill="currentColor" stroke="none" />
    </svg>
  );
}

export function QueueIcon() {
  return (
    <svg
      width="12"
      height="12"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M12 2 2 7l10 5 10-5-10-5Z" />
      <path d="m2 17 10 5 10-5" />
      <path d="m2 12 10 5 10-5" />
    </svg>
  );
}

export function SpinnerIcon() {
  return (
    <svg
      className="pipeline-section__footer-spinner"
      width="12"
      height="12"
      viewBox="0 0 24 24"
      fill="none"
      stroke="#888"
      strokeWidth="2.5"
      strokeLinecap="round"
    >
      <path d="M12 2a10 10 0 0 1 10 10" />
    </svg>
  );
}

export function VolumeChangeIcon({ direction }: { direction: "down" | "up" }) {
  return (
    <svg
      className="pipeline-card__pill-icon"
      width="12"
      height="12"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.5"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      {direction === "down" ? (
        <>
          <polyline points="23 18 13.5 8.5 8.5 13.5 1 6" />
          <polyline points="17 18 23 18 23 12" />
        </>
      ) : (
        <>
          <polyline points="23 6 13.5 15.5 8.5 10.5 1 18" />
          <polyline points="17 6 23 6 23 12" />
        </>
      )}
    </svg>
  );
}
