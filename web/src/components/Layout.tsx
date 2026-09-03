// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useCallback, createContext, useContext, useState, useEffect } from "react";
import { Outlet, NavLink, useNavigate, useLocation, Link } from "react-router-dom";
import { useLogout, useCurrentUser } from "@/lib/api";
import { useIdleTimeout } from "@/hooks/useIdleTimeout";

// ── Theme ─────────────────────────────────────────────────────────────────
export function useTheme() {
  const [theme, setTheme] = useState<"dark" | "light">(() => {
    return (localStorage.getItem("theme") as "dark" | "light") || "dark";
  });
  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    localStorage.setItem("theme", theme);
  }, [theme]);
  const toggle = () => setTheme(t => t === "dark" ? "light" : "dark");
  return { theme, toggle };
}

// ── Top-bar slot context ──────────────────────────────────────────────────
interface TopBarCtx {
  title: string;
  subtitle?: string;
  action?: React.ReactNode;
  setTopBar: (opts: { title: string; subtitle?: string; action?: React.ReactNode }) => void;
}
const TopBarContext = createContext<TopBarCtx>({
  title: "", setTopBar: () => {},
});
export function useTopBar() { return useContext(TopBarContext); }

// ── Icons — 16×16 viewBox, stroke 1.4, deliberately primitive ─────────────
const Icons = {
  dashboard:    `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><rect x="1.5" y="1.5" width="6" height="6" rx="1.2"/><rect x="8.5" y="1.5" width="6" height="3.5" rx="1.2"/><rect x="8.5" y="7" width="6" height="7.5" rx="1.2"/><rect x="1.5" y="9.5" width="6" height="5" rx="1.2"/></svg>`,
  applications: `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><rect x="1.5" y="1.5" width="13" height="5" rx="1.2"/><rect x="1.5" y="9.5" width="13" height="5" rx="1.2"/></svg>`,
  teams:        `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><circle cx="5.5" cy="5" r="2.2"/><circle cx="11" cy="5.5" r="1.8"/><path d="M1.5 14v-1a4 4 0 0 1 8 0v1"/><path d="M10.5 10.5a3.5 3.5 0 0 1 4 3.5"/></svg>`,
  credentials:  `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><rect x="3" y="7" width="10" height="7" rx="1.2"/><path d="M5.5 7V5a2.5 2.5 0 0 1 5 0v2"/><circle cx="8" cy="10.5" r="1" fill="currentColor" stroke="none"/></svg>`,
  audit:        `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M4 1.5h6l2.5 2.5V14a.5.5 0 0 1-.5.5H4a.5.5 0 0 1-.5-.5V2a.5.5 0 0 1 .5-.5Z"/><path d="M5.5 6.5h5M5.5 9h5M5.5 11.5h3"/></svg>`,
  users:        `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><circle cx="8" cy="5" r="2.5"/><path d="M2.5 14c0-2.2 2.5-4 5.5-4s5.5 1.8 5.5 4"/></svg>`,
  templates:    `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><rect x="1.5" y="1.5" width="13" height="13" rx="1.2"/><path d="M7 1.5v13M1.5 6h5.5"/></svg>`,
  platform:     `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><circle cx="8" cy="8" r="3"/><path d="M8 1v2M8 13v2M1 8h2M13 8h2M3.22 3.22l1.42 1.42M11.36 11.36l1.42 1.42M3.22 12.78l1.42-1.42M11.36 4.64l1.42-1.42"/></svg>`,
  signout:      `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M6 2H3a1 1 0 0 0-1 1v10a1 1 0 0 0 1 1h3"/><path d="M11 11l3-3-3-3M14 8H6"/></svg>`,
  sun:          `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><circle cx="8" cy="8" r="3"/><path d="M8 1v2M8 13v2M1 8h2M13 8h2M3.22 3.22l1.42 1.42M11.36 11.36l1.42 1.42M3.22 12.78l1.42-1.42M11.36 4.64l1.42-1.42"/></svg>`,
  moon:         `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.4"><path d="M13.5 10A6 6 0 0 1 6 2.5a6 6 0 1 0 7.5 7.5Z"/></svg>`,
};

function SvgIcon({ html, size = 15 }: { html: string; size?: number }) {
  return (
    <span
      style={{ width: size, height: size, display: "inline-flex", flexShrink: 0 }}
      dangerouslySetInnerHTML={{ __html: html }}
    />
  );
}

function initials(name: string) {
  return name.split(" ").map(w => w[0]).join("").toUpperCase().slice(0, 2);
}

// ── Nav item ──────────────────────────────────────────────────────────────
function NavItem({ to, icon, label, end }: { to: string; icon: string; label: string; end?: boolean }) {
  return (
    <NavLink
      to={to}
      end={end}
      className={({ isActive }) => [
        "flex items-center gap-[10px] h-8 px-[10px] rounded-[7px] text-[13px] no-underline transition-colors",
        isActive
          ? "font-semibold text-[var(--accent)] bg-[var(--accent-soft)]"
          : "font-medium text-[var(--muted)] hover:bg-[var(--card)] hover:text-[var(--text)]",
      ].join(" ")}
    >
      <SvgIcon html={icon} />
      {label}
    </NavLink>
  );
}

function NavGroup({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 2 }}>
      <p style={{
        fontSize: 10, fontWeight: 600, letterSpacing: ".1em",
        textTransform: "uppercase", color: "var(--faint)",
        padding: "6px 10px", margin: 0,
      }}>
        {label}
      </p>
      {children}
    </div>
  );
}

// ── Page-level top bar ────────────────────────────────────────────────────
export function TopBar({ theme, onToggleTheme }: { theme: "dark" | "light"; onToggleTheme: () => void }) {
  const location = useLocation();
  const { title, subtitle, action } = useTopBar();

  // Derive breadcrumb label from path
  const seg = location.pathname.split("/").filter(Boolean)[0] ?? "";
  const PAGE_NAMES: Record<string, string> = {
    "": "Dashboard", applications: "Applications", teams: "Teams",
    credentials: "Credentials", audit: "Audit Log", users: "Users",
    templates: "Templates", platform: "Platform Admin",
  };
  const pageLabel = title || PAGE_NAMES[seg] || seg;

  return (
    <div style={{
      position: "sticky", top: 0, zIndex: 5,
      height: 52, display: "flex", alignItems: "center",
      padding: "0 28px", gap: 10,
      background: "var(--bg)", borderBottom: "1px solid var(--line)",
    }}>
      {/* Breadcrumb */}
      <div style={{ flex: 1, display: "flex", alignItems: "center", gap: 8, fontSize: 13 }}>
        <Link to="/" style={{ color: "var(--faint)", textDecoration: "none" }}>NexBridge</Link>
        <span style={{ color: "var(--line2)" }}>/</span>
        <span style={{ color: "var(--text)", fontWeight: 600 }}>{pageLabel}</span>
        {subtitle && <span style={{ color: "var(--faint)", fontSize: 12 }}>{subtitle}</span>}
      </div>

      {/* Theme toggle */}
      <button
        onClick={onToggleTheme}
        title={theme === "dark" ? "Switch to light mode" : "Switch to dark mode"}
        style={{
          width: 30, height: 30, borderRadius: 7, border: "1px solid var(--line)",
          background: "transparent", color: "var(--faint)",
          display: "flex", alignItems: "center", justifyContent: "center",
          cursor: "pointer", transition: "border-color .12s, color .12s",
        }}
        onMouseEnter={e => { (e.currentTarget as HTMLElement).style.borderColor = "var(--accent)"; (e.currentTarget as HTMLElement).style.color = "var(--accent)"; }}
        onMouseLeave={e => { (e.currentTarget as HTMLElement).style.borderColor = "var(--line)"; (e.currentTarget as HTMLElement).style.color = "var(--faint)"; }}
      >
        <SvgIcon html={theme === "dark" ? Icons.sun : Icons.moon} size={14} />
      </button>

      {/* Page primary action */}
      {action}
    </div>
  );
}

// ── Layout ────────────────────────────────────────────────────────────────
export function Layout() {
  const navigate = useNavigate();
  const { data: user } = useCurrentUser();
  const logout = useLogout();
  const { theme, toggle: toggleTheme } = useTheme();

  const [topBarState, setTopBarState] = useState<{
    title: string; subtitle?: string; action?: React.ReactNode;
  }>({ title: "" });

  const handleLogout = async () => {
    await logout.mutateAsync();
    navigate("/signin");
  };
  const handleIdleLogout = useCallback(async () => {
    try { await logout.mutateAsync(); } catch {}
    navigate("/signin");
  }, [logout, navigate]);

  const { warningVisible, secondsLeft, stayLoggedIn } = useIdleTimeout(handleIdleLogout);

  const userInitials = user?.display_name ? initials(user.display_name) : (user?.email ? user.email.slice(0,2).toUpperCase() : "??");
  const displayName  = user?.display_name || user?.email || "";

  return (
    <TopBarContext.Provider value={{
      title: topBarState.title,
      subtitle: topBarState.subtitle,
      action: topBarState.action,
      setTopBar: opts => setTopBarState(opts),
    }}>
      <div style={{
        display: "flex", height: "100vh", minWidth: 1024,
        background: "var(--bg)", color: "var(--text)",
      }}>
        {/* ── Sidebar 220px ── */}
        <aside style={{
          width: 220, flexShrink: 0, display: "flex", flexDirection: "column",
          background: "var(--panel)", borderRight: "1px solid var(--line)",
          position: "sticky", top: 0, height: "100vh",
        }}>
          {/* Brand block */}
          <div style={{ padding: "16px 16px 14px", display: "flex", alignItems: "center", gap: 10 }}>
            <div style={{
              width: 26, height: 26, borderRadius: 7,
              background: "var(--accent)", color: "var(--accent-ink)",
              display: "flex", alignItems: "center", justifyContent: "center",
              fontSize: 13, fontWeight: 700, flexShrink: 0,
            }}>N</div>
            <div>
              <div style={{ fontSize: 13, fontWeight: 600, color: "var(--text)", lineHeight: 1.2 }}>DevPortal</div>
              <div style={{ fontSize: 10, fontWeight: 500, letterSpacing: ".04em", color: "var(--faint)", lineHeight: 1 }}>NEXBRIDGE</div>
            </div>
          </div>

          {/* Nav */}
          <nav style={{ flex: 1, padding: "6px 10px", display: "flex", flexDirection: "column", gap: 18, overflowY: "auto" }}>
            <NavGroup label="Build">
              <NavItem to="/" icon={Icons.dashboard} label="Dashboard" end />
              <NavItem to="/applications" icon={Icons.applications} label="Applications" />
              <NavItem to="/teams" icon={Icons.teams} label="Teams" />
            </NavGroup>
            <NavGroup label="Governance">
              <NavItem to="/credentials" icon={Icons.credentials} label="Credentials" />
              <NavItem to="/audit" icon={Icons.audit} label="Audit Log" />
              <NavItem to="/users" icon={Icons.users} label="Users" />
            </NavGroup>
            <NavGroup label="Platform">
              <NavItem to="/templates" icon={Icons.templates} label="Templates" />
              <NavItem to="/platform" icon={Icons.platform} label="Platform" />
            </NavGroup>
          </nav>

          {/* Footer */}
          <div style={{ borderTop: "1px solid var(--line)", padding: 10, display: "flex", alignItems: "center", gap: 10 }}>
            <div style={{
              width: 28, height: 28, borderRadius: "50%",
              background: "var(--card)", border: "1px solid var(--line2)",
              color: "var(--muted)", fontSize: 9, fontWeight: 600, flexShrink: 0,
              display: "flex", alignItems: "center", justifyContent: "center",
            }}>
              {userInitials}
            </div>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontSize: 12, fontWeight: 600, color: "var(--text)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                {displayName}
              </div>
              <div style={{ fontSize: 10.5, color: "var(--faint)", textTransform: "capitalize" }}>
                {user?.role ?? "engineer"}
              </div>
            </div>
            <button
              onClick={handleLogout}
              title="Sign out"
              style={{
                width: 24, height: 24, borderRadius: 6, border: "none",
                background: "transparent", color: "var(--faint)",
                display: "flex", alignItems: "center", justifyContent: "center",
                cursor: "pointer", flexShrink: 0,
                transition: "color .12s",
              }}
              onMouseEnter={e => (e.currentTarget as HTMLElement).style.color = "var(--bad)"}
              onMouseLeave={e => (e.currentTarget as HTMLElement).style.color = "var(--faint)"}
            >
              <SvgIcon html={Icons.signout} size={14} />
            </button>
          </div>
        </aside>

        {/* ── Main ── */}
        <div style={{ flex: 1, display: "flex", flexDirection: "column", overflow: "hidden" }}>
          <TopBar theme={theme} onToggleTheme={toggleTheme} />
          <main style={{ flex: 1, overflowY: "auto" }}>
            <Outlet />
          </main>
        </div>

        {/* ── Idle timeout modal ── */}
        {warningVisible && (
          <div style={{
            position: "fixed", inset: 0, zIndex: 9999,
            background: "rgba(0,0,0,.65)", backdropFilter: "blur(4px)",
            display: "flex", alignItems: "center", justifyContent: "center",
          }}>
            <div style={{
              background: "var(--panel)", border: "1px solid var(--line2)",
              borderRadius: 12, padding: "32px 36px", maxWidth: 400, width: "100%",
              textAlign: "center",
            }}>
              <p style={{ fontSize: 16, fontWeight: 700, margin: "0 0 8px", color: "var(--text)" }}>
                Session expiring soon
              </p>
              <p style={{ fontSize: 13, color: "var(--muted)", margin: "0 0 20px" }}>
                You'll be signed out in{" "}
                <span style={{ color: "var(--warn)", fontWeight: 700, fontVariantNumeric: "tabular-nums" }}>
                  {Math.floor(secondsLeft / 60)}:{String(secondsLeft % 60).padStart(2, "0")}
                </span>{" "}
                due to inactivity.
              </p>
              <div style={{ display: "flex", gap: 10 }}>
                <button onClick={stayLoggedIn} className="btn btn-primary" style={{ flex: 1 }}>
                  Stay logged in
                </button>
                <button onClick={handleLogout} className="btn btn-secondary" style={{ flex: 1 }}>
                  Sign out now
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    </TopBarContext.Provider>
  );
}
