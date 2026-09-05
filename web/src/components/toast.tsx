// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com

// Global toast stack. Two ways in:
//   - useToast() from inside a component, for one-off success/info messages.
//   - showToast() as a plain function, for callers outside React (queryClient.ts
//     uses this to surface query failures — see the QueryCache wiring there).
// Before this, a failed background data fetch was 100% invisible everywhere in
// the app: no page destructures `error` from its useQuery calls, so a down
// backend just showed an infinite spinner or a silently-empty list.
import { createContext, useCallback, useContext, useEffect, useRef, useState } from "react";

export type ToastTone = "ok" | "bad" | "warn" | "accent";

interface Toast {
  id: number;
  tone: ToastTone;
  message: string;
}

interface ShowToastOptions {
  tone?: ToastTone;
  message: string;
  durationMs?: number;
}

const TONE_COLOR: Record<ToastTone, string> = {
  ok: "var(--ok)", bad: "var(--bad)", warn: "var(--warn)", accent: "var(--accent)",
};

let dispatch: ((opts: ShowToastOptions) => void) | null = null;

// Callable from anywhere, including outside React (e.g. queryClient.ts). No-ops
// if ToastProvider hasn't mounted yet — that only happens during the earliest
// possible app bootstrap, before there'd be any query to fail.
export function showToast(opts: ShowToastOptions) {
  dispatch?.(opts);
}

interface ToastCtx {
  show: (opts: ShowToastOptions) => void;
}
const ToastContext = createContext<ToastCtx>({ show: () => {} });
export function useToast() { return useContext(ToastContext); }

const MAX_VISIBLE = 3;
const DEFAULT_DURATION = 5000;

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const nextId = useRef(0);

  const remove = useCallback((id: number) => {
    setToasts(prev => prev.filter(t => t.id !== id));
  }, []);

  const show = useCallback((opts: ShowToastOptions) => {
    const id = nextId.current++;
    const toast: Toast = { id, tone: opts.tone ?? "accent", message: opts.message };
    setToasts(prev => [...prev.slice(-(MAX_VISIBLE - 1)), toast]);
    window.setTimeout(() => remove(id), opts.durationMs ?? DEFAULT_DURATION);
  }, [remove]);

  useEffect(() => {
    dispatch = show;
    return () => { dispatch = null; };
  }, [show]);

  return (
    <ToastContext.Provider value={{ show }}>
      {children}
      <div
        style={{
          position: "fixed", bottom: 20, right: 20, zIndex: 2000,
          display: "flex", flexDirection: "column", gap: 8, width: 320,
          pointerEvents: "none",
        }}
      >
        {toasts.map(t => (
          <div
            key={t.id}
            role="status"
            style={{
              pointerEvents: "auto",
              background: "var(--panel)", border: "1px solid var(--line)",
              borderLeft: `3px solid ${TONE_COLOR[t.tone]}`,
              borderRadius: 8, padding: "11px 14px",
              fontSize: 13, color: "var(--text)", lineHeight: 1.4,
              boxShadow: "0 12px 24px rgba(0,0,0,0.4)",
              display: "flex", alignItems: "flex-start", gap: 10,
              animation: "dcrise 0.2s ease both",
            }}
          >
            <span style={{ flex: 1 }}>{t.message}</span>
            <button
              onClick={() => remove(t.id)}
              aria-label="Dismiss"
              style={{ background: "none", border: "none", color: "var(--faint)", cursor: "pointer", padding: 0, fontSize: 15, lineHeight: 1 }}
            >
              ×
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}
