// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useCallback } from "react";
import { Outlet, NavLink, useNavigate } from "react-router-dom";
import { useLogout, useCurrentUser } from "@/lib/api";
import { useBrand } from "@/contexts/BrandContext";
import { useIdleTimeout } from "@/hooks/useIdleTimeout";

// Inline SVG icons matching the design exactly
const Icons = {
  dashboard: `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="1.5" y="1.5" width="6" height="6" rx="1"/><rect x="8.5" y="1.5" width="6" height="4" rx="1"/><rect x="8.5" y="7.5" width="6" height="7" rx="1"/><rect x="1.5" y="9.5" width="6" height="5" rx="1"/></svg>`,
  projects:  `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M1.5 3.5a1 1 0 0 1 1-1h3l1.2 1.5H13.5a1 1 0 0 1 1 1v8a1 1 0 0 1-1 1h-11a1 1 0 0 1-1-1v-9.5Z"/></svg>`,
  teams:     `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="5.5" cy="5" r="2"/><circle cx="11" cy="5.5" r="1.6"/><path d="M1.5 14v-1.2c0-1.7 1.6-3 3.9-3s3.9 1.3 3.9 3V14"/><path d="M10.3 10c1.9.2 3.2 1.4 3.2 2.9V14"/></svg>`,
  credentials:`<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="3" y="7" width="10" height="7" rx="1.2"/><path d="M5.2 7V4.8a2.8 2.8 0 0 1 5.6 0V7"/></svg>`,
  audit:     `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M4 1.5h6l2.5 2.5V14a.5.5 0 0 1-.5.5H4a.5.5 0 0 1-.5-.5v-12a.5.5 0 0 1 .5-.5Z"/><path d="M5.5 7h5M5.5 9.5h5M5.5 12h3"/></svg>`,
  users:     `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><circle cx="8" cy="5" r="2.6"/><path d="M2.5 14c0-2.4 2.4-4.2 5.5-4.2s5.5 1.8 5.5 4.2"/></svg>`,
  platform:  `<svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5"><rect x="1.5" y="2.5" width="13" height="4" rx="1"/><rect x="1.5" y="9.5" width="13" height="4" rx="1"/><circle cx="12.5" cy="4.5" r="0.8" fill="currentColor" stroke="none"/><circle cx="12.5" cy="11.5" r="0.8" fill="currentColor" stroke="none"/></svg>`,
};

function NavItem({ to, icon, label, end }: { to: string; icon: string; label: string; end?: boolean }) {
  return (
    <NavLink
      to={to}
      end={end}
      className={({ isActive }) =>
        `flex items-center gap-2.5 rounded-md px-3 py-2 text-[13px] transition-colors no-underline ${
          isActive
            ? "bg-primary/10 text-primary font-semibold"
            : "text-[#94a3b8] hover:bg-[#334155] hover:text-[#f8fafc]"
        }`
      }
    >
      <span
        className="w-4 h-4 shrink-0"
        dangerouslySetInnerHTML={{ __html: icon }}
      />
      {label}
    </NavLink>
  );
}

function initials(name: string) {
  return name
    .split(" ")
    .map((w) => w[0])
    .join("")
    .toUpperCase()
    .slice(0, 2);
}

export function Layout() {
  const navigate = useNavigate();
  const { data: user } = useCurrentUser();
  const logout = useLogout();
  const brand = useBrand();

  const handleLogout = async () => {
    await logout.mutateAsync();
    navigate("/login");
  };

  const handleIdleLogout = useCallback(async () => {
    try { await logout.mutateAsync(); } catch {}
    navigate("/login");
  }, [logout, navigate]);

  const { warningVisible, secondsLeft, stayLoggedIn } = useIdleTimeout(handleIdleLogout);

  const userInitials = user?.display_name ? initials(user.display_name) : "??";

  return (
    <div className="flex h-screen bg-[#0f172a] text-[#f8fafc]" style={{ minWidth: 1024 }}>
      {/* Sidebar */}
      <aside className="w-60 shrink-0 flex flex-col border-r border-[#334155] bg-[#1e293b]">
        {/* Brand */}
        <div className="h-14 flex items-center px-4 border-b border-[#334155] gap-2">
          {brand.logo_url ? (
            <img src={brand.logo_url} alt={brand.app_name} className="h-7 w-auto" />
          ) : (
            <span className="text-[17px] font-bold tracking-tight text-primary">
              {brand.app_name}
            </span>
          )}
        </div>

        {/* Nav */}
        <nav className="flex-1 p-2 pt-4 flex flex-col gap-0.5">
          <NavItem to="/" icon={Icons.dashboard} label="Dashboard" end />
          <NavItem to="/applications" icon={Icons.projects} label="Applications" />
          <NavItem to="/teams" icon={Icons.teams} label="Teams" />

          <div className="mx-2 mt-3.5 mb-1.5 text-[10px] tracking-widest text-[#64748b] font-semibold">
            ADMIN
          </div>
          <NavItem to="/credentials" icon={Icons.credentials} label="Credentials" />
          <NavItem to="/audit" icon={Icons.audit} label="Audit log" />
          <NavItem to="/users" icon={Icons.users} label="Users" />
          <NavItem to="/templates" icon={Icons.projects} label="Templates" />
          <NavItem to="/platform" icon={Icons.platform} label="Platform" />
        </nav>

        {/* User footer */}
        <div className="border-t border-[#334155] p-3">
          <div className="flex items-center gap-2 px-1 pb-2.5">
            <div className="w-[26px] h-[26px] rounded-full bg-primary/20 text-primary flex items-center justify-center text-[11px] font-semibold shrink-0">
              {userInitials}
            </div>
            <div className="min-w-0">
              <div className="text-[12px] text-[#f8fafc] truncate">{user?.email}</div>
              <div className="text-[11px] text-[#64748b] capitalize">{user?.role}</div>
            </div>
          </div>
          <button
            onClick={handleLogout}
            className="w-full flex items-center gap-2 bg-transparent border-none text-[#94a3b8] text-[13px] px-2 py-2 rounded-md cursor-pointer text-left transition-colors hover:bg-[#334155] hover:text-[#f8fafc]"
          >
            <span className="text-sm">←</span> Sign out
          </button>
        </div>
      </aside>

      {/* Main */}
      <main className="flex-1 overflow-y-auto">
        <Outlet />
      </main>

      {/* Idle timeout warning */}
      {warningVisible && (
        <div
          style={{
            position: "fixed", inset: 0, zIndex: 9999,
            background: "rgba(0,0,0,0.65)", backdropFilter: "blur(4px)",
            display: "flex", alignItems: "center", justifyContent: "center",
          }}
        >
          <div
            style={{
              background: "#1e293b", border: "1px solid #334155",
              borderRadius: 12, padding: "32px 36px", maxWidth: 400, width: "100%",
              textAlign: "center",
            }}
          >
            <div style={{ fontSize: 36, marginBottom: 12 }}>⏱</div>
            <p style={{ fontSize: 16, fontWeight: 700, margin: "0 0 8px", color: "#f8fafc" }}>
              Session expiring soon
            </p>
            <p style={{ fontSize: 13, color: "#94a3b8", margin: "0 0 20px" }}>
              You'll be signed out in{" "}
              <span style={{ color: "#facc15", fontWeight: 700, fontVariantNumeric: "tabular-nums" }}>
                {Math.floor(secondsLeft / 60)}:{String(secondsLeft % 60).padStart(2, "0")}
              </span>{" "}
              due to inactivity.
            </p>
            <div style={{ display: "flex", gap: 10 }}>
              <button
                onClick={stayLoggedIn}
                style={{
                  flex: 1, height: 38, borderRadius: 8, border: "none",
                  background: "var(--color-primary, #3b82f6)", color: "#fff",
                  fontSize: 13, fontWeight: 600, cursor: "pointer",
                }}
              >
                Stay logged in
              </button>
              <button
                onClick={handleLogout}
                style={{
                  flex: 1, height: 38, borderRadius: 8,
                  border: "1px solid #334155", background: "transparent",
                  color: "#94a3b8", fontSize: 13, cursor: "pointer",
                }}
              >
                Sign out now
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
