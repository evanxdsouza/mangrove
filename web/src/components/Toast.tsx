import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from "react";
import { createSpring, prefersReducedMotion } from "../lib/spring";
import { MangroveIcon } from "../icons";

interface ToastEntry {
  id: number;
  message: string;
}

interface ToastState {
  showToast: (message: string) => void;
}

const ToastContext = createContext<ToastState | null>(null);

let nextId = 1;

/** Rare, on-brand milestone moments (first deploy, first rollback) surface
    here as a small toast -- kept deliberately rare by callers (see
    lib/milestones.ts) so this stays a delight, not a notification feed. */
export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastEntry[]>([]);

  const showToast = useCallback((message: string) => {
    const id = nextId++;
    setToasts((prev) => [...prev, { id, message }]);
  }, []);

  const dismiss = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  return (
    <ToastContext.Provider value={{ showToast }}>
      {children}
      <div className="toast-stack">
        {toasts.map((t) => (
          <ToastItem key={t.id} message={t.message} onDone={() => dismiss(t.id)} />
        ))}
      </div>
    </ToastContext.Provider>
  );
}

function ToastItem({ message, onDone }: { message: string; onDone: () => void }) {
  const [progress, setProgress] = useState(0);
  const onDoneRef = useRef(onDone);
  onDoneRef.current = onDone;

  useEffect(() => {
    if (prefersReducedMotion()) {
      setProgress(1);
      const timer = setTimeout(() => onDoneRef.current(), 5000);
      return () => clearTimeout(timer);
    }
    let closing = false;
    const spring = createSpring(0, {
      damping: 0.78,
      response: 0.4,
      onUpdate: setProgress,
      onComplete: () => {
        if (closing) onDoneRef.current();
      },
    });
    spring.set(1);
    const timer = setTimeout(() => {
      closing = true;
      spring.set(0, 0);
    }, 4200);
    return () => {
      clearTimeout(timer);
      spring.stop();
    };
  }, []);

  return (
    <div
      className="toast"
      style={{
        opacity: progress,
        transform: `translateY(${(1 - progress) * 14}px) scale(${0.94 + 0.06 * progress})`,
      }}
    >
      <MangroveIcon className="toast-icon" />
      <span>{message}</span>
    </div>
  );
}

export function useToast(): ToastState {
  const ctx = useContext(ToastContext);
  if (!ctx) throw new Error("useToast must be used within a ToastProvider");
  return ctx;
}
