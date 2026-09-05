// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState } from "react";
import { useTeams, useTeamMembers, useCreateTeam, useUpdateTeam, useDeleteTeam, useAddTeamMember, useRemoveTeamMember, useUsers, Team, TeamMember, User } from "@/lib/api";
import { PageHeader, Modal, FormField, Button, ConfirmDialog } from "@/components/kit";

const TEAM_PALETTE = [
  "#f87171","#38bdf8","#c084fc","#fbbf24","#4ade80","#94a3b8","#fb923c","#a78bfa",
];
function teamColor(idx: number) { return TEAM_PALETTE[idx % TEAM_PALETTE.length]; }

function roleBadge(role: string) {
  const isLead = role === "lead";
  return (
    <span style={{
      display: "inline-block", padding: "1px 8px", borderRadius: 6, fontSize: 10.5, fontWeight: 600,
      background: isLead ? "var(--accent-soft)" : "var(--card)",
      color: isLead ? "var(--accent)" : "var(--muted)",
      border: `1px solid ${isLead ? "var(--accent)" : "var(--line2)"}`,
    }}>{isLead ? "Lead" : "Member"}</span>
  );
}

function Avatar({ name, size = 24 }: { name: string; size?: number }) {
  const initials = (name || "?").split(" ").map(n => n[0]).join("").slice(0, 2).toUpperCase();
  return (
    <span className="avatar" style={{ width: size, height: size, fontSize: size * 0.37 }}>
      {initials}
    </span>
  );
}

function AvatarStack({ members, total }: { members: TeamMember[]; total: number }) {
  const shown = members.slice(0, 4);
  return (
    <div style={{ display: "flex", alignItems: "center" }}>
      {shown.map((m, i) => (
        <span key={m.user_id} style={{ marginLeft: i > 0 ? -7 : 0, border: "2px solid var(--panel)", borderRadius: "50%" }}>
          <Avatar name={m.display_name || m.email} size={22} />
        </span>
      ))}
      <span style={{ marginLeft: 8, fontSize: 11.5, color: "var(--faint)" }}>{total} member{total !== 1 ? "s" : ""}</span>
    </div>
  );
}

// ── Add member modal ──────────────────────────────────────────────────────────
function AddMemberModal({ teamId, existingMemberIds, onClose }: { teamId: string; existingMemberIds: string[]; onClose: () => void }) {
  const { data: allUsers = [] } = useUsers();
  const addM = useAddTeamMember(teamId);
  const [userId, setUserId] = useState("");
  const [role, setRole] = useState("member");
  const [error, setError] = useState("");

  const available = allUsers.filter((u: User) => !existingMemberIds.includes(u.id));

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!userId) return;
    setError("");
    try {
      await addM.mutateAsync({ user_id: userId, role });
      onClose();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to add member.");
    }
  }

  return (
    <Modal title="Add team member" onClose={onClose} maxWidth={400}>
      <form onSubmit={handleSubmit} style={{ display: "flex", flexDirection: "column", gap: 14 }}>
        <FormField label="User" hint={available.length === 0 ? "All platform users are already members." : undefined}>
          <select className="field" value={userId} onChange={e => setUserId(e.target.value)} autoFocus>
            <option value="">Select a user…</option>
            {available.map((u: User) => (
              <option key={u.id} value={u.id}>{u.display_name} ({u.email})</option>
            ))}
          </select>
        </FormField>
        <FormField label="Role">
          <select className="field" value={role} onChange={e => setRole(e.target.value)}>
            <option value="member">Member</option>
            <option value="lead">Lead</option>
          </select>
        </FormField>
        {error && <p style={{ margin: 0, fontSize: 12, color: "var(--bad)" }}>{error}</p>}
        <div style={{ display: "flex", gap: 10, justifyContent: "flex-end", marginTop: 6 }}>
          <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
          <Button type="submit" loading={addM.isPending} disabled={!userId}>Add member</Button>
        </div>
      </form>
    </Modal>
  );
}

// ── Team settings modal ───────────────────────────────────────────────────────
function TeamSettingsModal({ team, onClose }: { team: Team; onClose: () => void }) {
  const updateM = useUpdateTeam(team.id);
  const deleteM = useDeleteTeam(team.id);
  const [name, setName] = useState(team.name);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [error, setError] = useState("");

  async function handleRename(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim() || name.trim() === team.name) { onClose(); return; }
    setError("");
    try { await updateM.mutateAsync({ name: name.trim() }); onClose(); }
    catch (err: unknown) { setError(err instanceof Error ? err.message : "Failed to rename team."); }
  }

  async function handleDelete() {
    setError("");
    try { await deleteM.mutateAsync(); onClose(); }
    catch (err: unknown) { setError(err instanceof Error ? err.message : "Failed to delete team."); }
  }

  return (
    <>
      <Modal title="Team settings" onClose={onClose} maxWidth={400}>
        <form onSubmit={handleRename} style={{ display: "flex", flexDirection: "column", gap: 14 }}>
          <FormField label="Team name" hint={`Slug: ${team.slug}`}>
            <input className="field" value={name} onChange={e => setName(e.target.value)} autoFocus />
          </FormField>
          {error && !confirmDelete && <p style={{ margin: 0, fontSize: 12, color: "var(--bad)" }}>{error}</p>}
          <div style={{ display: "flex", gap: 10, justifyContent: "flex-end" }}>
            <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
            <Button type="submit" loading={updateM.isPending} disabled={!name.trim()}>Save</Button>
          </div>
        </form>
        <div style={{ marginTop: 24, paddingTop: 20, borderTop: "1px solid var(--line)" }}>
          <div style={{ fontSize: 13, fontWeight: 600, color: "var(--bad)", marginBottom: 8 }}>Danger zone</div>
          <Button variant="danger" size="sm" onClick={() => setConfirmDelete(true)}>Delete team</Button>
        </div>
      </Modal>
      {confirmDelete && (
        <ConfirmDialog
          title="Delete team?"
          message={<>Delete <strong className="text-[var(--text)]">{team.name}</strong>? This cannot be undone.</>}
          confirmLabel="Delete"
          danger
          isPending={deleteM.isPending}
          error={error}
          onConfirm={handleDelete}
          onCancel={() => { setConfirmDelete(false); setError(""); }}
        />
      )}
    </>
  );
}

// ── New team modal ────────────────────────────────────────────────────────────
function NewTeamModal({ onClose }: { onClose: () => void }) {
  const createM = useCreateTeam();
  const [name, setName] = useState("");
  const [error, setError] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    setError("");
    try { await createM.mutateAsync({ name: name.trim() }); onClose(); }
    catch (err: unknown) { setError(err instanceof Error ? err.message : "Failed to create team."); }
  }

  return (
    <Modal title="New team" onClose={onClose} maxWidth={380}>
      <form onSubmit={handleSubmit} style={{ display: "flex", flexDirection: "column", gap: 14 }}>
        <FormField label="Team name">
          <input className="field" placeholder="Backend Engineering" value={name} onChange={e => setName(e.target.value)} autoFocus />
        </FormField>
        {error && <p style={{ margin: 0, fontSize: 12, color: "var(--bad)" }}>{error}</p>}
        <div style={{ display: "flex", gap: 10, justifyContent: "flex-end", marginTop: 6 }}>
          <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
          <Button type="submit" loading={createM.isPending} disabled={!name.trim()}>Create team</Button>
        </div>
      </form>
    </Modal>
  );
}

// ── Team row ──────────────────────────────────────────────────────────────────
function TeamRow({ team, color, open, onToggle }: {
  team: Team; color: string; open: boolean; onToggle: () => void;
}) {
  const { data: members = [], isLoading } = useTeamMembers(team.id);
  const removeM = useRemoveTeamMember(team.id);
  const lead = members.find(m => m.member_role === "lead");
  const [showAddMember, setShowAddMember] = useState(false);
  const [showSettings, setShowSettings] = useState(false);

  return (
    <>
      <div className="card" style={{ padding: 0, overflow: "hidden" }}>
        <div
          onClick={onToggle}
          style={{
            display: "grid",
            gridTemplateColumns: "34px minmax(0,1fr) 170px 130px 150px 20px",
            gap: 16, padding: "14px 18px", alignItems: "center",
            cursor: "pointer", transition: "background .1s",
          }}
          onMouseEnter={e => (e.currentTarget as HTMLElement).style.background = "var(--card)"}
          onMouseLeave={e => (e.currentTarget as HTMLElement).style.background = ""}
        >
          <span style={{
            width: 34, height: 34, borderRadius: 8, flexShrink: 0,
            background: color + "22", border: `1px solid ${color}44`, color,
            display: "inline-flex", alignItems: "center", justifyContent: "center",
            fontSize: 13, fontWeight: 700,
          }}>{(team.name[0] ?? "?").toUpperCase()}</span>
          <div>
            <div style={{ fontSize: 14, fontWeight: 600 }}>{team.name}</div>
            <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11, color: "var(--faint)", marginTop: 2 }}>{team.slug}</div>
          </div>
          <div>
            <div className="overline" style={{ marginBottom: 4 }}>Lead</div>
            {lead ? (
              <div style={{ display: "flex", alignItems: "center", gap: 5 }}>
                <span style={{ width: 6, height: 6, borderRadius: "50%", background: "var(--ok)", flexShrink: 0 }} />
                <span style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12, color: "var(--text)" }}>
                  {lead.email.split("@")[0]}
                </span>
              </div>
            ) : (
              <span style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12, color: "var(--faint)" }}>Unassigned</span>
            )}
          </div>
          <div>
            <div className="overline" style={{ marginBottom: 4 }}>Members</div>
            <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12, color: "var(--muted)" }}>
              {isLoading ? "…" : members.length}
            </div>
          </div>
          <AvatarStack members={members} total={members.length} />
          <span style={{ color: "var(--faint)", fontSize: 11, userSelect: "none" }}>{open ? "▾" : "▸"}</span>
        </div>

        {open && (
          <div style={{ background: "var(--bg)", borderTop: "1px solid var(--line)" }}>
            <div style={{
              display: "grid", gridTemplateColumns: "minmax(0,1fr) 150px 180px 80px",
              gap: 14, paddingLeft: 68, paddingRight: 18, paddingTop: 10, paddingBottom: 10,
              borderBottom: "1px solid var(--line)",
            }}>
              {["Member","Role","Email",""].map(h => (
                <div key={h} className="overline">{h}</div>
              ))}
            </div>
            {isLoading ? (
              <div style={{ padding: "20px 68px", fontSize: 13, color: "var(--faint)" }}>Loading members…</div>
            ) : members.length === 0 ? (
              <div style={{ padding: "20px 68px", fontSize: 13, color: "var(--faint)" }}>No members yet.</div>
            ) : members.map((m: TeamMember) => (
              <div key={m.user_id} style={{
                display: "grid", gridTemplateColumns: "minmax(0,1fr) 150px 180px 80px",
                gap: 14, paddingLeft: 68, paddingRight: 18,
                paddingTop: 11, paddingBottom: 11,
                borderBottom: "1px solid var(--line)", alignItems: "center",
              }}>
                <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                  <Avatar name={m.display_name || m.email} size={24} />
                  <div>
                    <div style={{ fontSize: 13, fontWeight: 500, color: "var(--text)" }}>{m.display_name}</div>
                    <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11, color: "var(--faint)" }}>{m.email}</div>
                  </div>
                </div>
                <div>{roleBadge(m.member_role)}</div>
                <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12, color: "var(--faint)" }}>{m.email}</div>
                <div>
                  <button
                    className="btn btn-ghost btn-sm"
                    style={{ fontSize: 11, color: "var(--bad)" }}
                    onClick={() => removeM.mutate(m.user_id)}
                    disabled={removeM.isPending}
                  >Remove</button>
                </div>
              </div>
            ))}
            <div style={{ padding: "12px 18px 12px 68px", display: "flex", alignItems: "center", gap: 10 }}>
              <button className="btn btn-secondary btn-sm" onClick={e => { e.stopPropagation(); setShowAddMember(true); }}>+ Add member</button>
              <button className="btn btn-ghost btn-sm" onClick={e => { e.stopPropagation(); setShowSettings(true); }}>Settings</button>
            </div>
          </div>
        )}
      </div>

      {showAddMember && (
        <AddMemberModal
          teamId={team.id}
          existingMemberIds={members.map(m => m.user_id)}
          onClose={() => setShowAddMember(false)}
        />
      )}
      {showSettings && <TeamSettingsModal team={team} onClose={() => setShowSettings(false)} />}
    </>
  );
}

// ── Main page ─────────────────────────────────────────────────────────────────
export function TeamsPage() {
  const { data: teams = [], isLoading } = useTeams();
  const [openId, setOpenId] = useState<string>("");
  const [showNewTeam, setShowNewTeam] = useState(false);

  return (
    <div style={{ padding: 28, maxWidth: 1320, display: "flex", flexDirection: "column", gap: 22 }}>
      <PageHeader
        title="Teams"
        subtitle="Teams own applications and carry on-call responsibilities."
        actions={<button className="btn btn-primary" onClick={() => setShowNewTeam(true)}>+ New team</button>}
      />
      {isLoading ? (
        <div style={{ padding: "48px 0", textAlign: "center", color: "var(--faint)", fontSize: 13 }}>Loading teams…</div>
      ) : teams.length === 0 ? (
        <div style={{ border: "1px dashed var(--line2)", borderRadius: 10, padding: 48, textAlign: "center" }}>
          <div style={{ fontSize: 14, fontWeight: 600, color: "var(--text)", marginBottom: 6 }}>No teams yet</div>
          <div style={{ fontSize: 13, color: "var(--muted)", marginBottom: 16 }}>Create a team to group developers and assign applications.</div>
          <button className="btn btn-primary" onClick={() => setShowNewTeam(true)}>Create first team</button>
        </div>
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
          {teams.map((t, idx) => (
            <TeamRow
              key={t.id}
              team={t}
              color={teamColor(idx)}
              open={openId === t.id}
              onToggle={() => setOpenId(openId === t.id ? "" : t.id)}
            />
          ))}
        </div>
      )}

      {showNewTeam && <NewTeamModal onClose={() => setShowNewTeam(false)} />}
    </div>
  );
}
