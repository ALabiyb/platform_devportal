// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState, useRef, useEffect } from "react";
import { useUsers, useCurrentUser, useCreateUser, useUpdateUserRole, useDeactivateUser, User } from "@/lib/api";

type DisplayRole = "Platform admin" | "Developer" | "Read only";

function mapRole(r: string): DisplayRole {
  if (r === "admin") return "Platform admin";
  if (r === "developer") return "Developer";
  return "Read only";
}

function roleBadgeStyle(role: DisplayRole): { bg: string; color: string; border: string } {
  const m: Record<DisplayRole, { bg: string; color: string; border: string }> = {
    "Platform admin": { bg: "var(--accent-soft)", color: "var(--accent)", border: "var(--accent)" },
    "Developer":      { bg: "var(--card)",         color: "var(--muted)",  border: "var(--line2)" },
    "Read only":      { bg: "transparent",         color: "var(--faint)",  border: "var(--line)" },
  };
  return m[role];
}

function statusBadge(isActive: boolean) {
  if (isActive) return <span className="badge badge-success">Active</span>;
  return <span className="badge badge-pending">Suspended</span>;
}

function relativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "Just now";
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  const days = Math.floor(hrs / 24);
  return `${days}d ago`;
}

function Avatar({ name, size = 30, faded }: { name: string; size?: number; faded?: boolean }) {
  const initials = (name || "?").split(" ").map(w => w[0]).join("").slice(0, 2).toUpperCase();
  return (
    <span className="avatar" style={{ width: size, height: size, fontSize: size * 0.35, opacity: faded ? 0.5 : 1 }}>
      {initials}
    </span>
  );
}

// ── Inline ⋯ dropdown ─────────────────────────────────────────────────────────
function UserActionsMenu({ user, onClose }: { user: User; onClose: () => void }) {
  const ref = useRef<HTMLDivElement>(null);
  const roleM = useUpdateUserRole();
  const deactivateM = useDeactivateUser();

  useEffect(() => {
    function handle(e: MouseEvent) { if (ref.current && !ref.current.contains(e.target as Node)) onClose(); }
    document.addEventListener("mousedown", handle);
    return () => document.removeEventListener("mousedown", handle);
  }, [onClose]);

  async function changeRole(role: string) {
    await roleM.mutateAsync({ userId: user.id, role });
    onClose();
  }

  async function handleSuspend() {
    await deactivateM.mutateAsync(user.id);
    onClose();
  }

  const currentApiRole = user.role;

  return (
    <div ref={ref} style={{
      position: "absolute", top: "100%", right: 0, zIndex: 200, minWidth: 180,
      background: "var(--panel)", border: "1px solid var(--line)", borderRadius: 8,
      boxShadow: "0 8px 24px rgba(0,0,0,0.4)", padding: "4px 0", marginTop: 4,
    }}>
      <div style={{ padding: "6px 12px 4px", fontSize: 11, color: "var(--faint)", fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.05em" }}>Change role to</div>
      {[["admin","Platform admin"],["developer","Developer"],["viewer","Read only"]].map(([r, label]) => (
        <button key={r} disabled={currentApiRole === r || roleM.isPending}
          onClick={() => changeRole(r)}
          style={{
            display: "block", width: "100%", textAlign: "left", padding: "8px 12px",
            background: currentApiRole === r ? "var(--accent-soft)" : "none",
            border: "none", color: currentApiRole === r ? "var(--accent)" : "var(--text)",
            fontSize: 13, cursor: currentApiRole === r ? "default" : "pointer",
          }}
          onMouseEnter={e => { if (currentApiRole !== r) (e.currentTarget as HTMLElement).style.background = "var(--card)"; }}
          onMouseLeave={e => { (e.currentTarget as HTMLElement).style.background = currentApiRole === r ? "var(--accent-soft)" : "none"; }}
        >{label}{currentApiRole === r && " ✓"}</button>
      ))}
      <div style={{ height: 1, background: "var(--line)", margin: "4px 0" }} />
      <button
        onClick={handleSuspend}
        disabled={deactivateM.isPending}
        style={{
          display: "block", width: "100%", textAlign: "left", padding: "8px 12px",
          background: "none", border: "none", color: user.is_active ? "var(--bad)" : "var(--ok)",
          fontSize: 13, cursor: "pointer",
        }}
        onMouseEnter={e => (e.currentTarget as HTMLElement).style.background = "var(--card)"}
        onMouseLeave={e => (e.currentTarget as HTMLElement).style.background = "none"}
      >
        {user.is_active ? "Suspend user" : "Reactivate user"}
      </button>
    </div>
  );
}

// ── Invite user modal ─────────────────────────────────────────────────────────
function InviteModal({ onClose }: { onClose: () => void }) {
  const createM = useCreateUser();
  const [displayName, setDisplayName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [role, setRole] = useState("developer");
  const [error, setError] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!email.trim() || !password.trim()) return;
    setError("");
    try {
      await createM.mutateAsync({ display_name: displayName.trim() || email.split("@")[0], email: email.trim(), password, role });
      onClose();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to invite user.");
    }
  }

  return (
    <div style={{
      position: "fixed", inset: 0, zIndex: 1000, display: "flex", alignItems: "center", justifyContent: "center",
      background: "rgba(2,8,23,0.7)", backdropFilter: "blur(4px)",
    }} onClick={e => e.target === e.currentTarget && onClose()}>
      <div style={{ background: "var(--panel)", border: "1px solid var(--line)", borderRadius: 12, width: "100%", maxWidth: 440, padding: 28, boxShadow: "0 24px 48px rgba(0,0,0,0.6)" }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 22 }}>
          <h2 style={{ margin: 0, fontSize: 16, fontWeight: 700 }}>Invite user</h2>
          <button onClick={onClose} style={{ background: "none", border: "none", color: "var(--faint)", fontSize: 20, cursor: "pointer", padding: 0 }}>×</button>
        </div>
        <form onSubmit={handleSubmit} style={{ display: "flex", flexDirection: "column", gap: 14 }}>
          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            <label style={{ fontSize: 12.5, fontWeight: 600, color: "var(--muted)" }}>Display name</label>
            <input className="field" placeholder="Jane Mwangi" value={displayName} onChange={e => setDisplayName(e.target.value)} autoFocus />
          </div>
          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            <label style={{ fontSize: 12.5, fontWeight: 600, color: "var(--muted)" }}>Email</label>
            <input className="field" type="email" placeholder="jane@nexbridge.dev" value={email} onChange={e => setEmail(e.target.value)} />
          </div>
          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            <label style={{ fontSize: 12.5, fontWeight: 600, color: "var(--muted)" }}>Temporary password</label>
            <input className="field" type="password" placeholder="Min 8 characters" value={password} onChange={e => setPassword(e.target.value)} />
          </div>
          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            <label style={{ fontSize: 12.5, fontWeight: 600, color: "var(--muted)" }}>Role</label>
            <select className="field" value={role} onChange={e => setRole(e.target.value)}>
              <option value="developer">Developer</option>
              <option value="admin">Platform admin</option>
              <option value="viewer">Read only</option>
            </select>
          </div>
          {error && <p style={{ margin: 0, fontSize: 12, color: "var(--bad)" }}>{error}</p>}
          <div style={{ display: "flex", gap: 10, justifyContent: "flex-end", marginTop: 6 }}>
            <button type="button" className="btn btn-ghost" onClick={onClose}>Cancel</button>
            <button type="submit" className="btn btn-primary" disabled={!email.trim() || !password.trim() || createM.isPending}>
              {createM.isPending ? "Inviting…" : "Create account"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// ── Main page ─────────────────────────────────────────────────────────────────
export function UsersPage() {
  const { data: users = [], isLoading } = useUsers();
  const { data: currentUser } = useCurrentUser();
  const [search, setSearch] = useState("");
  const [roleFilter, setRoleFilter] = useState("All roles");
  const [statusSeg, setStatusSeg] = useState<"All" | "Active" | "Suspended">("All");
  const [showInvite, setShowInvite] = useState(false);
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);

  const filtered = users.filter((u: User) => {
    const q = search.toLowerCase();
    const matchSearch = !q || u.email.toLowerCase().includes(q) || (u.display_name ?? "").toLowerCase().includes(q);
    const displayRole = mapRole(u.role);
    const matchRole = roleFilter === "All roles" || displayRole === roleFilter;
    const matchStatus = statusSeg === "All"
      || (statusSeg === "Active" && u.is_active)
      || (statusSeg === "Suspended" && !u.is_active);
    return matchSearch && matchRole && matchStatus;
  });

  const stats = [
    { label: "Total users",     value: users.length,                                      sub: "across all teams" },
    { label: "Platform admins", value: users.filter(u => u.role === "admin").length,      sub: "break-glass capable" },
    { label: "Developers",      value: users.filter(u => u.role === "developer").length,  sub: "active contributors" },
    { label: "Suspended",       value: users.filter(u => !u.is_active).length,            sub: "no active sessions" },
  ];

  return (
    <div style={{ padding: 28, maxWidth: 1400, display: "flex", flexDirection: "column", gap: 22 }}>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between" }}>
        <div>
          <h1 className="page-h1">Users</h1>
          <p className="page-sub">Manage access and roles for everyone on the platform.</p>
        </div>
        {currentUser?.role === "admin" && (
          <button className="btn btn-primary" onClick={() => setShowInvite(true)}>+ Invite user</button>
        )}
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(180px,1fr))", gap: 12 }}>
        {stats.map(s => (
          <div key={s.label} className="card" style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            <p className="overline" style={{ margin: 0 }}>{s.label}</p>
            <span style={{ fontSize: 26, fontWeight: 600, color: "var(--text)", letterSpacing: "-0.03em" }}>{s.value}</span>
            <p style={{ margin: 0, fontSize: 12, color: "var(--faint)" }}>{s.sub}</p>
          </div>
        ))}
      </div>

      <div style={{ display: "flex", gap: 10, alignItems: "center", flexWrap: "wrap" }}>
        <input className="field" style={{ maxWidth: 250 }} placeholder="Search name or email"
          value={search} onChange={e => setSearch(e.target.value)} />
        <select className="field" style={{ width: "auto", padding: "0 12px" }}
          value={roleFilter} onChange={e => setRoleFilter(e.target.value)}>
          <option>All roles</option>
          <option>Platform admin</option>
          <option>Developer</option>
          <option>Read only</option>
        </select>
        <div className="segmented" style={{ marginLeft: "auto" }}>
          {(["All", "Active", "Suspended"] as const).map(s => (
            <button key={s} className={statusSeg === s ? "active" : ""} onClick={() => setStatusSeg(s)}>{s}</button>
          ))}
        </div>
      </div>

      {isLoading ? (
        <div style={{ padding: "48px 0", textAlign: "center", color: "var(--faint)", fontSize: 13 }}>Loading users…</div>
      ) : (
        <div className="tbl-wrap">
          <div className="tbl-head" style={{
            gridTemplateColumns: "minmax(220px,1.3fr) 140px minmax(180px,1fr) 150px 110px",
            minWidth: 900,
          }}>
            <div>User</div><div>Role</div><div>Email</div>
            <div>Provider</div><div>Status</div>
          </div>
          {filtered.length === 0 ? (
            <div style={{ padding: "24px 18px", fontSize: 13, color: "var(--faint)" }}>No users match these filters</div>
          ) : filtered.map((u: User) => {
            const faded = !u.is_active;
            const displayRole = mapRole(u.role);
            const roleStyle = roleBadgeStyle(displayRole);
            const isSelf = u.id === currentUser?.id;
            const menuOpen = openMenuId === u.id;
            return (
              <div key={u.id} className="tbl-row" style={{
                gridTemplateColumns: "minmax(220px,1.3fr) 140px minmax(180px,1fr) 150px 110px",
                minWidth: 900, opacity: faded ? 0.7 : 1, position: "relative",
              }}>
                <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                  <Avatar name={u.display_name || u.email} size={30} faded={faded} />
                  <div>
                    <div style={{ fontSize: 13, fontWeight: 500, color: "var(--text)" }}>
                      {u.display_name}
                      {isSelf && <span style={{ fontSize: 10, color: "var(--accent)", marginLeft: 6, fontWeight: 600 }}>YOU</span>}
                    </div>
                    <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11.5, color: "var(--faint)" }}>
                      {relativeTime(u.created_at)}
                    </div>
                  </div>
                </div>
                <div>
                  <span style={{
                    display: "inline-block", padding: "2px 9px", borderRadius: 6, fontSize: 11, fontWeight: 600,
                    background: roleStyle.bg, color: roleStyle.color, border: `1px solid ${roleStyle.border}`,
                  }}>{displayRole}</span>
                </div>
                <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12, color: "var(--faint)" }}>{u.email}</div>
                <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12, color: "var(--faint)" }}>local</div>
                <div style={{ display: "flex", alignItems: "center", gap: 6, position: "relative" }}>
                  {statusBadge(u.is_active)}
                  {currentUser?.role === "admin" && !isSelf && (
                    <>
                      <button
                        className="btn btn-ghost btn-sm"
                        style={{ padding: "0 6px", minWidth: 26, fontWeight: 700, letterSpacing: "0.05em" }}
                        onClick={() => setOpenMenuId(menuOpen ? null : u.id)}
                      >⋯</button>
                      {menuOpen && <UserActionsMenu user={u} onClose={() => setOpenMenuId(null)} />}
                    </>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}

      {showInvite && <InviteModal onClose={() => setShowInvite(false)} />}
    </div>
  );
}
