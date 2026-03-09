import { useEffect } from "react";

interface ToastProps {
  message: string;
  onDismiss: () => void;
  duration?: number;
}

export function Toast({ message, onDismiss, duration = 5000 }: ToastProps) {
  useEffect(() => {
    const timer = setTimeout(onDismiss, duration);
    return () => clearTimeout(timer);
  }, [onDismiss, duration]);

  return (
    <div className="toast">
      <span className="toast__message">{message}</span>
      <button className="toast__dismiss" onClick={onDismiss} type="button">
        &times;
      </button>
    </div>
  );
}
