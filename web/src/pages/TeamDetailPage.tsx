// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import { useTeam, useTeamMembers, useAddTeamMember, useRemoveTeamMember, useUsers } from "@/lib/api";
import { ApiError } from "@/lib/queryClient";
import { ConfirmDialog } from "@/components/kit";

function RoleChip({ label, variant }: { label: string; variant: "org" | "team" }) {
  const colors: Record<string, { fg: string; bg: string }> = {
    admin:     { fg: "var(--bad)",   bg: "var(--bad-soft)" },
    developer: { fg: "var(--accent)", bg: "var(--accent-soft)" },
    viewer:    { fg: "var(--muted)", bg: "var(--faint-soft)" },
    lead:      { fg: "var(--warn)",  bg: "var(--warn-soft)" },
    member:    { fg: "var(--accent)", bg: "var(--accent-soft)" },
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
        fontFamily: variant === "team" ? "'JetBrains Mono','IBM Plex Mono',monospace" : undefined,
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

  const [removeError, setRemoveError] = useState("");

  async function handleRemove(userId: string) {
    setRemoveError("");
    try {
      await removeMember.mutateAsync(userId);
      setRemoveTarget(null);
    } catch (err) {
      setRemoveError(err instanceof ApiError ? err.message : "Failed to remove member.");
    }
  }

  if (teamLoading) {
    return <div className="p-8 text-[13px] text-[var(--faint)]">Loading team…</div>;
  }
  if (!team) {
    return (
      <div className="p-8">
        <p className="text-[14px] text-[var(--bad)]">Team not found.</p>
        <button
          onClick={() => navigate("/teams")}
          className="mt-3 text-[13px] text-[var(--accent)] underline bg-transparent border-none cursor-pointer"
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
            className="mt-0.5 h-7 w-7 flex items-center justify-center rounded border border-[var(--line)] bg-transparent text-[var(--muted)] cursor-pointer hover:border-primary/50"
          >
            ←
          </button>
          <div>
            <h1 className="text-[22px] font-bold tracking-tight m-0 mb-0.5">{team.name}</h1>
            <p className="text-[12px] text-[var(--muted)] m-0 font-mono">{team.slug}</p>
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
          className="border border-primary/30 bg-[var(--bg)] rounded-[10px] p-4 mb-5 flex flex-col gap-3"
        >
          <p className="text-[13px] font-semibold m-0">Add team member</p>
          <div className="flex gap-3">
            <select
              value={selectedUser}
              onChange={(e) => setSelectedUser(e.target.value)}
              className="flex-1 h-9 rounded border border-[var(--line)] bg-[var(--panel)] text-[var(--text)] px-2 text-[12px] font-[inherit] focus:outline-none"
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
              className="w-32 h-9 rounded border border-[var(--line)] bg-[var(--panel)] text-[var(--text)] px-2 text-[12px] font-[inherit] focus:outline-none"
            >
              <option value="member">Member</option>
              <option value="lead">Lead</option>
            </select>
          </div>
          {addError && <p className="text-[11px] text-[var(--bad)] m-0">{addError}</p>}
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
              className="h-8 px-5 rounded border border-[var(--line)] bg-transparent text-[var(--muted)] text-[12px] cursor-pointer"
            >
              Cancel
            </button>
          </div>
        </form>
      )}

      {/* Members table */}
      <div className="border border-[var(--line)] rounded-[10px] overflow-hidden">
        <div className="flex items-center justify-between px-5 py-3 border-b border-[var(--line)] bg-[var(--panel)]">
          <span className="text-[13px] font-semibold">Members</span>
          <span className="text-[12px] text-[var(--faint)]">
            {membersLoading ? "…" : `${members?.length ?? 0} members`}
          </span>
        </div>

        {membersLoading ? (
          <div className="px-5 py-8 text-[13px] text-[var(--faint)]">Loading…</div>
        ) : !members?.length ? (
          <div className="px-5 py-10 text-center">
            <p className="text-[14px] font-medium m-0 mb-1">No members yet</p>
            <p className="text-[12px] text-[var(--muted)] m-0">
              Click <strong>+ Add member</strong> to assign users to this team.
            </p>
          </div>
        ) : (
          <table className="w-full">
            <thead>
              <tr className="border-b border-[var(--line)]">
                {["Name", "Email", "Org role", "Team role", ""].map((h) => (
                  <th
                    key={h}
                    className="text-left px-5 py-2.5 text-[11px] font-semibold text-[var(--faint)] uppercase tracking-wider"
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
                  className="border-b border-[var(--panel)] last:border-b-0 hover:bg-[var(--panel)] transition-colors"
                >
                  <td className="px-5 py-3 text-[13px] font-medium">{m.display_name}</td>
                  <td className="px-5 py-3 text-[12px] text-[var(--muted)] font-mono">{m.email}</td>
                  <td className="px-5 py-3"><RoleChip label={m.role} variant="org" /></td>
                  <td className="px-5 py-3"><RoleChip label={m.member_role} variant="team" /></td>
                  <td className="px-5 py-3 text-right">
                    <button
                      onClick={() => setRemoveTarget(m.user_id)}
                      className="text-[12px] text-[var(--faint)] hover:text-[var(--bad)] bg-transparent border-none cursor-pointer transition-colors"
                    >
                      Remove
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
      {removeTarget && (() => {
        const target = members?.find(m => m.user_id === removeTarget);
        return (
          <ConfirmDialog
            title="Remove member?"
            message={<>Remove <strong className="text-[var(--text)]">{target?.display_name ?? "this member"}</strong> from {team.name}?</>}
            confirmLabel="Remove"
            danger
            isPending={removeMember.isPending}
            error={removeError}
            onConfirm={() => handleRemove(removeTarget)}
            onCancel={() => { setRemoveTarget(null); setRemoveError(""); }}
          />
        );
      })()}
    </div>
  );
}
