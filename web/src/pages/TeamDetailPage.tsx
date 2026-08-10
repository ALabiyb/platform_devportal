// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useTeam, useTeamMembers, useAddTeamMember, useRemoveTeamMember, useUsers } from "@/lib/api";
import { ApiError } from "@/lib/queryClient";

function RoleChip({ label, variant }: { label: string; variant: "org" | "team" }) {
  const colors: Record<string, { fg: string; bg: string }> = {
    admin:     { fg: "#f87171", bg: "rgba(248,113,113,0.12)" },
    developer: { fg: "#60a5fa", bg: "rgba(96,165,250,0.12)" },
    viewer:    { fg: "#94a3b8", bg: "rgba(148,163,184,0.12)" },
    lead:      { fg: "#facc15", bg: "rgba(234,179,8,0.15)" },
    member:    { fg: "#60a5fa", bg: "rgba(96,165,250,0.12)" },
  };
  const s = colors[label] ?? colors.viewer;
  return (
    <span
      style={{
        color: s.fg,
        background: s.bg,
        borderRadius: 5,
        padding: "2px 8px",
        fontSize: 11,
        fontWeight: 600,
        fontFamily: variant === "team" ? "var(--font-mono,'IBM Plex Mono',monospace)" : undefined,
      }}
    >
      {label}
    </span>
  );
}

export function TeamDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();

  const { data: team, isLoading: teamLoading } = useTeam(id ?? "");
  const { data: members, isLoading: membersLoading } = useTeamMembers(id ?? "");
  const { data: allUsers } = useUsers();
  const addMember = useAddTeamMember(id ?? "");
  const removeMember = useRemoveTeamMember(id ?? "");

  const [showAdd, setShowAdd] = useState(false);
  const [selectedUser, setSelectedUser] = useState("");
  const [selectedRole, setSelectedRole] = useState("member");
  const [addError, setAddError] = useState("");
  const [removeTarget, setRemoveTarget] = useState<string | null>(null);

  const memberIds = new Set(members?.map((m) => m.user_id) ?? []);
  const eligible = allUsers?.filter((u) => !memberIds.has(u.id)) ?? [];

  async function handleAdd(e: React.FormEvent) {
    e.preventDefault();
    if (!selectedUser) return;
    setAddError("");
    try {
      await addMember.mutateAsync({ user_id: selectedUser, role: selectedRole });
      setShowAdd(false);
      setSelectedUser("");
      setSelectedRole("member");
    } catch (err) {
      setAddError(err instanceof ApiError ? err.message : "Failed to add member.");
    }
  }

  async function handleRemove(userId: string) {
    try {
      await removeMember.mutateAsync(userId);
      setRemoveTarget(null);
    } catch {}
  }

  if (teamLoading) {
    return <div className="p-8 text-[13px] text-[#64748b]">Loading team…</div>;
  }
  if (!team) {
    return (
      <div className="p-8">
        <p className="text-[14px] text-[#f87171]">Team not found.</p>
        <button
          onClick={() => navigate("/teams")}
          className="mt-3 text-[13px] text-[#60a5fa] underline bg-transparent border-none cursor-pointer"
        >
          ← Back to Teams
        </button>
      </div>
    );
  }

  return (
    <div className="p-8 max-w-[860px]">
      {/* Header */}
      <div className="flex items-start justify-between mb-7">
        <div className="flex items-start gap-3">
          <button
            onClick={() => navigate("/teams")}
            className="mt-0.5 h-7 w-7 flex items-center justify-center rounded border border-[#334155] bg-transparent text-[#94a3b8] cursor-pointer hover:border-primary/50"
          >
            ←
          </button>
          <div>
            <h1 className="text-[22px] font-bold tracking-tight m-0 mb-0.5">{team.name}</h1>
            <p className="text-[12px] text-[#94a3b8] m-0 font-mono">{team.slug}</p>
          </div>
        </div>
        <button
          onClick={() => setShowAdd(!showAdd)}
          className="h-9 px-4 rounded-md bg-primary border-none text-white text-[13px] font-medium cursor-pointer"
        >
          + Add member
        </button>
      </div>

      {/* Add member form */}
      {showAdd && (
        <form
          onSubmit={handleAdd}
          className="border border-primary/30 bg-[#0f172a] rounded-[10px] p-4 mb-5 flex flex-col gap-3"
        >
          <p className="text-[13px] font-semibold m-0">Add team member</p>
          <div className="flex gap-3">
            <select
              value={selectedUser}
              onChange={(e) => setSelectedUser(e.target.value)}
              className="flex-1 h-9 rounded border border-[#334155] bg-[#1e293b] text-[#f8fafc] px-2 text-[12px] font-[inherit] focus:outline-none"
            >
              <option value="">Select user…</option>
              {eligible.map((u) => (
                <option key={u.id} value={u.id}>
                  {u.display_name} ({u.email})
                </option>
              ))}
            </select>
            <select
              value={selectedRole}
              onChange={(e) => setSelectedRole(e.target.value)}
              className="w-32 h-9 rounded border border-[#334155] bg-[#1e293b] text-[#f8fafc] px-2 text-[12px] font-[inherit] focus:outline-none"
            >
              <option value="member">Member</option>
              <option value="lead">Lead</option>
            </select>
          </div>
          {addError && <p className="text-[11px] text-[#f87171] m-0">{addError}</p>}
          <div className="flex gap-2">
            <button
              type="submit"
              disabled={!selectedUser || addMember.isPending}
              className="h-8 px-5 rounded bg-primary border-none text-white text-[12px] cursor-pointer disabled:opacity-50"
            >
              {addMember.isPending ? "Adding…" : "Add"}
            </button>
            <button
              type="button"
              onClick={() => { setShowAdd(false); setAddError(""); }}
              className="h-8 px-5 rounded border border-[#334155] bg-transparent text-[#94a3b8] text-[12px] cursor-pointer"
            >
              Cancel
            </button>
          </div>
        </form>
      )}

      {/* Members table */}
      <div className="border border-[#334155] rounded-[10px] overflow-hidden">
        <div className="flex items-center justify-between px-5 py-3 border-b border-[#334155] bg-[#1e293b]">
          <span className="text-[13px] font-semibold">Members</span>
          <span className="text-[12px] text-[#64748b]">
            {membersLoading ? "…" : `${members?.length ?? 0} members`}
          </span>
        </div>

        {membersLoading ? (
          <div className="px-5 py-8 text-[13px] text-[#64748b]">Loading…</div>
        ) : !members?.length ? (
          <div className="px-5 py-10 text-center">
            <p className="text-[14px] font-medium m-0 mb-1">No members yet</p>
            <p className="text-[12px] text-[#94a3b8] m-0">
              Click <strong>+ Add member</strong> to assign users to this team.
            </p>
          </div>
        ) : (
          <table className="w-full">
            <thead>
              <tr className="border-b border-[#334155]">
                {["Name", "Email", "Org role", "Team role", ""].map((h) => (
                  <th
                    key={h}
                    className="text-left px-5 py-2.5 text-[11px] font-semibold text-[#64748b] uppercase tracking-wider"
                  >
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {members.map((m) => (
                <tr
                  key={m.user_id}
                  className="border-b border-[#1e293b] last:border-b-0 hover:bg-[#1e293b] transition-colors"
                >
                  <td className="px-5 py-3 text-[13px] font-medium">{m.display_name}</td>
                  <td className="px-5 py-3 text-[12px] text-[#94a3b8] font-mono">{m.email}</td>
                  <td className="px-5 py-3"><RoleChip label={m.role} variant="org" /></td>
                  <td className="px-5 py-3"><RoleChip label={m.member_role} variant="team" /></td>
                  <td className="px-5 py-3 text-right">
                    {removeTarget === m.user_id ? (
                      <div className="flex items-center justify-end gap-2">
                        <span className="text-[12px] text-[#94a3b8]">Remove?</span>
                        <button
                          onClick={() => handleRemove(m.user_id)}
                          className="text-[12px] text-[#f87171] bg-transparent border-none cursor-pointer underline"
                        >
                          Confirm
                        </button>
                        <button
                          onClick={() => setRemoveTarget(null)}
                          className="text-[12px] text-[#64748b] bg-transparent border-none cursor-pointer underline"
                        >
                          Cancel
                        </button>
                      </div>
                    ) : (
                      <button
                        onClick={() => setRemoveTarget(m.user_id)}
                        className="text-[12px] text-[#64748b] hover:text-[#f87171] bg-transparent border-none cursor-pointer transition-colors"
                      >
                        Remove
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
