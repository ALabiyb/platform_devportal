// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com

// Global Cmd+K / Ctrl+K command palette — jump to any page, application,
// service, or team without clicking through the sidebar. Mounted once in
// Layout.tsx. openCommandPalette() lets any component (e.g. a visible
// "Search" affordance in the top bar) open it without prop drilling, the
// same module-level-dispatcher pattern toast.tsx already uses.
import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useApplications, useProjects, useTeams } from "@/lib/api";

interface PaletteItem {
  id: string;
  section: string;
  label: string;
  sub?: string;
  to: string;
}

const STATIC_ITEMS: PaletteItem[] = [
  { id: "nav-dashboard",    section: "Go to", label: "Dashboard",     to: "/" },
  { id: "nav-applications", section: "Go to", label: "Applications",  to: "/applications" },
  { id: "nav-teams",        section: "Go to", label: "Teams",         to: "/teams" },
  { id: "nav-credentials",  section: "Go to", label: "Credentials",   to: "/credentials" },
  { id: "nav-audit",        section: "Go to", label: "Audit Log",     to: "/audit" },
  { id: "nav-users",        section: "Go to", label: "Users",         to: "/users" },
  { id: "nav-templates",    section: "Go to", label: "Templates",     to: "/templates" },
  { id: "nav-platform",     section: "Go to", label: "Platform",      to: "/platform" },
  { id: "action-new-app",   section: "Actions", label: "New application", to: "/applications/new" },
];

export const isMac = typeof navigator !== "undefined" && /Mac|iPod|iPhone|iPad/.test(navigator.platform);

let dispatch: (() => void) | null = null;
export function openCommandPalette() { dispatch?.(); }

export function CommandPalette() {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [activeIndex, setActiveIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const navigate = useNavigate();

  const { data: apps = [] } = useApplications();
  const { data: projects = [] } = useProjects();
  const { data: teams = [] } = useTeams();

  useEffect(() => {
    dispatch = () => setOpen(true);
    return () => { dispatch = null; };
  }, []);

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen(o => !o);
      }
    }
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, []);

  useEffect(() => {
    if (!open) return;
    setQuery("");
    setActiveIndex(0);
    document.body.style.overflow = "hidden";
    const raf = requestAnimationFrame(() => inputRef.current?.focus());
    return () => {
      document.body.style.overflow = "";
      cancelAnimationFrame(raf);
    };
  }, [open]);

  const appNameById = useMemo(() => new Map(apps.map(a => [a.id, a.name])), [apps]);

  const items = useMemo<PaletteItem[]>(() => [
    ...STATIC_ITEMS,
    ...apps.map(a => ({ id: `app-${a.id}`, section: "Applications", label: a.name, sub: a.git_namespace, to: `/applications/${a.id}` })),
    ...projects.map(p => ({ id: `svc-${p.id}`, section: "Services", label: p.name, sub: appNameById.get(p.application_id) ?? p.build_tool, to: `/projects/${p.id}` })),
    ...teams.map(t => ({ id: `team-${t.id}`, section: "Teams", label: t.name, sub: t.slug, to: `/teams/${t.id}` })),
  ], [apps, projects, teams, appNameById]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return items.filter(i => i.section === "Go to" || i.section === "Actions");
    return items
      .filter(i => i.label.toLowerCase().includes(q) || (i.sub ?? "").toLowerCase().includes(q))
      .slice(0, 50);
  }, [items, query]);

  const grouped = useMemo(() => {
    const map = new Map<string, PaletteItem[]>();
    for (const it of filtered) {
      if (!map.has(it.section)) map.set(it.section, []);
      map.get(it.section)!.push(it);
    }
    return [...map.entries()];
  }, [filtered]);

  useEffect(() => {
    const el = listRef.current?.querySelector(`[data-idx="${activeIndex}"]`);
    el?.scrollIntoView({ block: "nearest" });
  }, [activeIndex]);

  function select(item: PaletteItem) {
    setOpen(false);
    navigate(item.to);
  }

  function onInputKeyDown(e: React.KeyboardEvent) {
    if (e.key === "ArrowDown") { e.preventDefault(); setActiveIndex(i => Math.min(i + 1, filtered.length - 1)); }
    else if (e.key === "ArrowUp") { e.preventDefault(); setActiveIndex(i => Math.max(i - 1, 0)); }
    else if (e.key === "Enter") { e.preventDefault(); const item = filtered[activeIndex]; if (item) select(item); }
    else if (e.key === "Escape") { e.preventDefault(); setOpen(false); }
  }

  if (!open) return null;

  return (
    <div
      style={{
        position: "fixed", inset: 0, zIndex: 3000, display: "flex",
        alignItems: "flex-start", justifyContent: "center", paddingTop: "12vh",
        background: "rgba(2,8,23,0.7)", backdropFilter: "blur(4px)",
      }}
      onClick={e => e.target === e.currentTarget && setOpen(false)}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Command palette"
        style={{
          width: "100%", maxWidth: 560, maxHeight: "70vh", display: "flex", flexDirection: "column",
          background: "var(--panel)", border: "1px solid var(--line)", borderRadius: 12,
          boxShadow: "0 24px 48px rgba(0,0,0,0.6)", overflow: "hidden",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 10, padding: "14px 16px", borderBottom: "1px solid var(--line)", flexShrink: 0 }}>
          <span style={{ color: "var(--faint)", fontSize: 14 }} aria-hidden="true">⌘</span>
          <input
            ref={inputRef}
            role="combobox"
            aria-expanded="true"
            aria-controls="command-palette-list"
            aria-activedescendant={filtered[activeIndex]?.id}
            value={query}
            onChange={e => { setQuery(e.target.value); setActiveIndex(0); }}
            onKeyDown={onInputKeyDown}
            placeholder="Jump to an application, service, team, or page…"
            style={{ flex: 1, background: "none", border: "none", outline: "none", color: "var(--text)", fontSize: 14 }}
          />
          <kbd style={{ fontSize: 11, color: "var(--faint)", border: "1px solid var(--line)", borderRadius: 4, padding: "2px 5px", fontFamily: "inherit" }}>Esc</kbd>
        </div>
        <div ref={listRef} id="command-palette-list" role="listbox" style={{ overflowY: "auto", padding: "6px 0" }}>
          {filtered.length === 0 ? (
            <div style={{ padding: "24px 16px", textAlign: "center", color: "var(--faint)", fontSize: 13 }}>No matches.</div>
          ) : grouped.map(([section, sectionItems]) => (
            <div key={section}>
              <div className="overline" style={{ padding: "8px 16px 4px" }}>{section}</div>
              {sectionItems.map(item => {
                const idx = filtered.indexOf(item);
                const active = idx === activeIndex;
                return (
                  <div
                    key={item.id}
                    id={item.id}
                    data-idx={idx}
                    role="option"
                    aria-selected={active}
                    onMouseEnter={() => setActiveIndex(idx)}
                    onClick={() => select(item)}
                    style={{
                      display: "flex", justifyContent: "space-between", alignItems: "center", gap: 12,
                      padding: "9px 16px", cursor: "pointer",
                      background: active ? "var(--accent-soft)" : "transparent",
                    }}
                  >
                    <span style={{ fontSize: 13.5, color: active ? "var(--accent)" : "var(--text)", fontWeight: active ? 600 : 400, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                      {item.label}
                    </span>
                    {item.sub && (
                      <span style={{ fontSize: 11.5, color: "var(--faint)", fontFamily: "JetBrains Mono,monospace", flexShrink: 0 }}>
                        {item.sub}
                      </span>
                    )}
                  </div>
                );
              })}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
