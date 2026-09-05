// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com

// Shared UI primitives wrapping the app's existing design-system CSS
// classes (index.css: .btn/.badge/.field/.card/.page-h1/.spinner). These
// are not a new visual language — every style decision here already
// exists somewhere in the app; this file just stops each page from
// re-implementing the same button/modal/badge/empty-state markup by hand.
import { useEffect, useId, useRef } from "react";
import { useTopBar } from "@/components/Layout";

// ── Skeleton ────────────────────────────────────────────────────────────────
// Loading placeholder — replaces the "Loading…"/"Loading X…" text every page
// currently shows, with something that suggests the shape of what's coming
// instead of a blank flash. Respects prefers-reduced-motion (see index.css).
export function Skeleton({ width = "100%", height = 14, radius = 4, style }: {
  width?: number | string; height?: number | string; radius?: number; style?: React.CSSProperties;
}) {
  return <span className="skeleton" style={{ width, height, borderRadius: radius, ...style }} />;
}

// One skeleton .tbl-row matching a real table's own gridTemplateColumns, so
// the placeholder lines up with the columns it's about to be replaced by.
export function TableRowSkeleton({ gridTemplateColumns, minWidth }: {
  gridTemplateColumns: string; minWidth?: number;
}) {
  const count = gridTemplateColumns.trim().split(/\s+/).length;
  return (
    <div className="tbl-row" style={{ gridTemplateColumns, minWidth, alignItems: "center" }}>
      {Array.from({ length: count }).map((_, i) => (
        <Skeleton key={i} height={13} width={i === 0 ? "75%" : "45%"} />
      ))}
    </div>
  );
}

// ── Button ──────────────────────────────────────────────────────────────────
export type ButtonVariant = "primary" | "secondary" | "ghost" | "danger";
export type ButtonSize = "sm" | "md" | "lg";

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  loading?: boolean;
}

export function Button({
  variant = "primary", size = "md", loading = false,
  disabled, className, children, ...rest
}: ButtonProps) {
  const sizeClass = size === "sm" ? "btn-sm" : size === "lg" ? "btn-lg" : "";
  const cls = ["btn", `btn-${variant}`, sizeClass, className].filter(Boolean).join(" ");
  return (
    <button className={cls} disabled={disabled || loading} {...rest}>
      {loading && <span className="spinner" style={{ width: 13, height: 13 }} />}
      {children}
    </button>
  );
}

// ── Badge ───────────────────────────────────────────────────────────────────
export type BadgeTone = "ok" | "warn" | "bad" | "accent" | "pending";

const BADGE_CLASS: Record<BadgeTone, string> = {
  ok: "badge-success", warn: "badge-warn", bad: "badge-failed",
  accent: "badge-running", pending: "badge-pending",
};
const DOT_COLOR: Record<BadgeTone, string> = {
  ok: "var(--ok)", warn: "var(--warn)", bad: "var(--bad)",
  accent: "var(--accent)", pending: "var(--faint)",
};
const DOT_MODIFIER: Partial<Record<BadgeTone, string>> = {
  accent: "running", warn: "warn", bad: "failed",
};

export function Badge({ tone = "pending", dot = false, children }: {
  tone?: BadgeTone; dot?: boolean; children: React.ReactNode;
}) {
  return (
    <span className={`badge ${BADGE_CLASS[tone]}`}>
      {dot && (
        <span
          className={["status-dot", DOT_MODIFIER[tone]].filter(Boolean).join(" ")}
          style={{ background: DOT_COLOR[tone] }}
        />
      )}
      {children}
    </span>
  );
}

// ── FormField ───────────────────────────────────────────────────────────────
// Labels + hint/error text around a form control. Pass the control itself
// (an <input className="field">, <select className="field">, etc.) as
// children — this doesn't own the control, just its label/hint/error chrome.
export function FormField({ label, hint, error, required, children }: {
  label: string; hint?: string; error?: string; required?: boolean; children: React.ReactNode;
}) {
  const id = useId();
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
      <label htmlFor={id} style={{ fontSize: 12.5, fontWeight: 600, color: "var(--muted)" }}>
        {label}
        {required && <span style={{ color: "var(--bad)" }}> *</span>}
      </label>
      {children}
      {error ? (
        <div style={{ fontSize: 11, color: "var(--bad)" }}>{error}</div>
      ) : hint ? (
        <div style={{ fontSize: 11, color: "var(--faint)" }}>{hint}</div>
      ) : null}
    </div>
  );
}

// ── Modal ───────────────────────────────────────────────────────────────────
// Esc-to-close, backdrop-click-to-close, focus moved to the panel on open —
// none of the hand-rolled overlays scattered across pages had any of these.
export function Modal({ title, onClose, children, maxWidth = 480 }: {
  title: string; onClose: () => void; children: React.ReactNode; maxWidth?: number;
}) {
  const panelRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function onKey(e: KeyboardEvent) { if (e.key === "Escape") onClose(); }
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);

  useEffect(() => { panelRef.current?.focus(); }, []);

  return (
    <div
      style={{
        position: "fixed", inset: 0, zIndex: 1000, display: "flex",
        alignItems: "center", justifyContent: "center",
        background: "rgba(2,8,23,0.7)", backdropFilter: "blur(4px)",
      }}
      onClick={e => e.target === e.currentTarget && onClose()}
    >
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-label={title}
        tabIndex={-1}
        style={{
          background: "var(--panel)", border: "1px solid var(--line)", borderRadius: 12,
          width: "100%", maxWidth, padding: 28, boxShadow: "0 24px 48px rgba(0,0,0,0.6)",
          outline: "none",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 22 }}>
          <h2 style={{ margin: 0, fontSize: 16, fontWeight: 700, color: "var(--text)" }}>{title}</h2>
          <button
            onClick={onClose}
            aria-label="Close"
            style={{ background: "none", border: "none", color: "var(--faint)", fontSize: 20, cursor: "pointer", padding: 0, lineHeight: 1 }}
          >
            ×
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}

// ── ConfirmDialog ───────────────────────────────────────────────────────────
// One confirm surface for every "are you sure?" in the app, replacing the
// mix of inline ✓/✗ glyphs, card-content swaps, and window.confirm-style
// patterns each page invented separately.
export function ConfirmDialog({
  title, message, confirmLabel = "Confirm", cancelLabel = "Cancel",
  danger = false, isPending = false, error, onConfirm, onCancel,
}: {
  title: string;
  message: React.ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  danger?: boolean;
  isPending?: boolean;
  error?: string;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  return (
    <Modal title={title} onClose={onCancel} maxWidth={380}>
      <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
        <div style={{ fontSize: 13, color: "var(--muted)", lineHeight: 1.5 }}>{message}</div>
        {error && <div style={{ fontSize: 12, color: "var(--bad)" }}>{error}</div>}
        <div style={{ display: "flex", gap: 10, justifyContent: "flex-end" }}>
          <Button variant="ghost" onClick={onCancel} disabled={isPending}>{cancelLabel}</Button>
          <Button variant={danger ? "danger" : "primary"} onClick={onConfirm} loading={isPending}>
            {confirmLabel}
          </Button>
        </div>
      </div>
    </Modal>
  );
}

// ── EmptyState ──────────────────────────────────────────────────────────────
export function EmptyState({ title, description, action }: {
  title: string; description?: string; action?: React.ReactNode;
}) {
  return (
    <div style={{ padding: "48px 24px", textAlign: "center" }}>
      <div style={{ fontSize: 14, fontWeight: 600, color: "var(--text)", marginBottom: description ? 6 : action ? 18 : 0 }}>
        {title}
      </div>
      {description && (
        <div style={{ fontSize: 13, color: "var(--muted)", marginBottom: action ? 18 : 0 }}>
          {description}
        </div>
      )}
      {action}
    </div>
  );
}

// ── PageHeader ──────────────────────────────────────────────────────────────
// Renders the standard .page-head/.page-h1/.page-sub block every page
// currently hand-writes, and syncs title/action into the sticky TopBar
// (Layout.tsx) so the primary action stays reachable when the page scrolls.
// TopBar already falls back to a route-derived label when nothing calls
// this, so pages that don't use PageHeader are unaffected.
export function PageHeader({ title, subtitle, actions }: {
  title: string; subtitle?: string; actions?: React.ReactNode;
}) {
  const { setTopBar } = useTopBar();

  useEffect(() => {
    setTopBar({ title, subtitle, action: actions });
    return () => setTopBar({ title: "" });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [title, subtitle]);

  return (
    <div className="page-head" style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 16, flexWrap: "wrap" }}>
      <div>
        <h1 className="page-h1">{title}</h1>
        {subtitle && <p className="page-sub">{subtitle}</p>}
      </div>
      {actions && <div style={{ display: "flex", gap: 10, alignItems: "center" }}>{actions}</div>}
    </div>
  );
}
