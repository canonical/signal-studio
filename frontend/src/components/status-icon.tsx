export type Status = "ok" | "info" | "warning" | "critical";

interface StatusIconProps {
  status: Status;
  size?: number;
}

export function StatusIcon({ status, size = 24 }: StatusIconProps) {
  if (status === "ok") {
    return (
      <svg width={size} height={size} viewBox="0 0 24 24" fill="none">
        <circle cx="12" cy="12" r="10" stroke="#22c55e" strokeWidth="1.5" fill="none" />
        <path d="M8 12.5l2.5 2.5 5.5-5.5" stroke="#22c55e" strokeWidth="1.5" fill="none" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    );
  }
  if (status === "critical") {
    return (
      <svg width={size} height={size} viewBox="0 0 24 24" fill="none">
        <circle cx="12" cy="12" r="10" stroke="#e03c31" strokeWidth="1.5" fill="none" />
        <path d="M12 8v5" stroke="#e03c31" strokeWidth="1.5" strokeLinecap="round" />
        <circle cx="12" cy="16" r="0.75" fill="#e03c31" />
      </svg>
    );
  }
  if (status === "warning") {
    return (
      <svg width={size} height={size} viewBox="0 0 24 24" fill="none">
        <path d="M12 3L2 21h20L12 3z" stroke="#f59e0b" strokeWidth="1.5" fill="none" strokeLinejoin="round" />
        <path d="M12 10v4" stroke="#f59e0b" strokeWidth="1.5" strokeLinecap="round" />
        <circle cx="12" cy="17" r="0.75" fill="#f59e0b" />
      </svg>
    );
  }
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none">
      <circle cx="12" cy="12" r="10" stroke="#3b82f6" strokeWidth="1.5" fill="none" />
      <path d="M12 11v5" stroke="#3b82f6" strokeWidth="1.5" strokeLinecap="round" />
      <circle cx="12" cy="8" r="0.75" fill="#3b82f6" />
    </svg>
  );
}
