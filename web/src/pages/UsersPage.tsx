// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState } from "react";
import { useUsers, useCreateUser, useUpdateUserRole, useDeactivateUser } from "@/lib/api";

const ROLES = ["admin", "developer", "viewer"] as const;

interface AppUser {
  id: string;
  email: string;
  display_name: string;
  role: string;
  is_active: boolean;
  created_at: string;
}

function statusStyle(active: boolean) {
  return active
    ? { fg: "#4ade80", bg: "rgba(34,197,94,0.15)", border: "rgba(34,197,94,0.3)" }
    : { fg: "#94a3b8", bg: "rgba(148,163,184,0.12)", border: "rgba(148,163,184,0.25)" };
}

export function UsersPage() {
  const { data: users, isLoading } = useUsers();
  const createUser = useCreateUser();
  const updateRole = useUpdateUserRole();
  const deactivate = useDeactivateUser();

  const [inviteOpen, setInviteOpen] = useState(false);
  const [inviteForm, setInviteForm] = useState({ name: "", email: "", password: "", role: "developer" });
  const [confirmToggle, setConfirmToggle] = useState<string | null>(null);
  const [inviteError, setInviteError] = useState("");

  async function handleInvite(e: React.FormEvent) {
    e.preventDefault();
    if (!inviteForm.name.trim() || !inviteForm.email.trim() || !inviteForm.password.trim()) return;
    try {
      await createUser.mutateAsync({
        display_name: inviteForm.name.trim(),
        email: inviteForm.email.trim(),
        password: inviteForm.password,
        role: inviteForm.role,
      });
      setInviteOpen(false);
      setInviteForm({ name: "", email: "", password: "", role: "developer" });
      setInviteError("");
    } catch (err: unknown) {
      setInviteError(err instanceof Error ? err.message : "Failed to create user.");
    }
  }

  async function handleRoleChange(userId: string, role: string) {
    try {
      await updateRole.mutateAsync({ userId, role });
    } catch { /* ignore */ }
  }

  async function handleDeactivate(userId: string) {
    try {
      await deactivate.mutateAsync(userId);
      setConfirmToggle(null);
    } catch { /* ignore */ }
  }

  const rows = users as AppUser[] | undefined;

  return (
    <div className="p-8 max-w-[1000px]">
      <div className="flex items-start justify-between mb-5">
        <div>
          <h1 className="text-[24px] font-bold tracking-tight m-0 mb-1">Users</h1>
          <p className="text-[13px] text-[#94a3b8] m-0">Manage who can access this workspace.</p>
        </div>
        <button
          onClick={() => setInviteOpen(true)}
          className="h-9 rounded-md border-none bg-primary text-white text-[13px] font-medium px-3.5 cursor-pointer"
        >
          + Invite user
        </button>
      </div>

      {/* Invite panel */}
      {inviteOpen && (
        <div className="border border-primary/30 bg-[#1e293b] rounded-[10px] p-5 mb-5">
          <form onSubmit={handleInvite} className="flex gap-3 flex-wrap items-end">
            <div className="flex flex-col gap-1.5">
              <label className="text-[12px] text-[#94a3b8]">Full name</label>
              <input
                type="text"
                value={inviteForm.name}
                onChange={(e) => setInviteForm((p) => ({ ...p, name: e.target.value }))}
                className="h-9 w-44 rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-2.5 text-[13px] font-[inherit] focus:outline-none"
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <label className="text-[12px] text-[#94a3b8]">Email</label>
              <input
                type="email"
                value={inviteForm.email}
                onChange={(e) => setInviteForm((p) => ({ ...p, email: e.target.value }))}
                className="h-9 w-52 rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-2.5 text-[13px] font-[inherit] focus:outline-none"
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <label className="text-[12px] text-[#94a3b8]">Password</label>
              <input
                type="password"
                value={inviteForm.password}
                onChange={(e) => setInviteForm((p) => ({ ...p, password: e.target.value }))}
                className="h-9 w-36 rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-2.5 text-[13px] font-[inherit] focus:outline-none"
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <label className="text-[12px] text-[#94a3b8]">Role</label>
              <select
                value={inviteForm.role}
                onChange={(e) => setInviteForm((p) => ({ ...p, role: e.target.value }))}
                className="h-9 rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-2.5 text-[13px] font-[inherit] focus:outline-none"
              >
                {ROLES.map((r) => <option key={r} value={r}>{r}</option>)}
              </select>
            </div>
            <button
              type="submit"
              disabled={createUser.isPending}
              className="h-9 rounded-md border-none bg-primary text-white text-[13px] font-medium px-4 cursor-pointer disabled:opacity-50"
            >
              {createUser.isPending ? "Creating…" : "Create account"}
            </button>
            <button
              type="button"
              onClick={() => setInviteOpen(false)}
              className="h-9 rounded-md border border-[#334155] bg-transparent text-[#94a3b8] text-[13px] px-3.5 cursor-pointer"
            >
              Cancel
            </button>
          </form>
          {inviteError && <p className="text-[12px] text-[#f87171] mt-2">{inviteError}</p>}
        </div>
      )}

      {/* Users table */}
      {isLoading ? (
        <div className="text-[13px] text-[#64748b]">Loading users…</div>
      ) : (
        <div className="border border-[#334155] bg-[#1e293b] rounded-[10px] overflow-hidden">
          <div
            className="grid px-4 py-2.5 text-[11px] text-[#64748b] uppercase tracking-[0.04em] border-b border-[#334155]"
            style={{ gridTemplateColumns: "1fr 1.3fr 130px 100px 90px" }}
          >
            <div>Name</div>
            <div>Email</div>
            <div>Role</div>
            <div>Status</div>
            <div />
          </div>

          {!rows?.length ? (
            <div className="px-4 py-6 text-[13px] text-[#64748b]">No users found.</div>
          ) : (
            rows.map((u) => {
              const sc = statusStyle(u.is_active);
              const confirming = confirmToggle === u.id;
              return (
                <div
                  key={u.id}
                  className="grid px-4 py-[11px] items-center text-[13px] border-b border-[#334155] last:border-0"
                  style={{ gridTemplateColumns: "1fr 1.3fr 130px 100px 90px" }}
                >
                  <div>{u.display_name}</div>
                  <div className="text-[#94a3b8] text-[12px]">{u.email}</div>
                  <div>
                    <select
                      value={u.role}
                      onChange={(e) => handleRoleChange(u.id, e.target.value)}
                      className="h-7 rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] text-[12px] px-1.5 font-[inherit] focus:outline-none"
                    >
                      {ROLES.map((r) => <option key={r} value={r}>{r}</option>)}
                    </select>
                  </div>
                  <div>
                    <span
                      className="text-[11px] font-semibold px-2 py-0.5 rounded-full border"
                      style={{ color: sc.fg, background: sc.bg, borderColor: sc.border }}
                    >
                      {u.is_active ? "active" : "deactivated"}
                    </span>
                  </div>
                  <div>
                    {u.is_active && (
                      confirming ? (
                        <div className="flex gap-2">
                          <button
                            onClick={() => handleDeactivate(u.id)}
                            className="text-[11px] text-[#f87171] bg-none border-none cursor-pointer p-0"
                          >
                            Confirm
                          </button>
                          <button
                            onClick={() => setConfirmToggle(null)}
                            className="text-[11px] text-[#94a3b8] bg-none border-none cursor-pointer p-0"
                          >
                            Cancel
                          </button>
                        </div>
                      ) : (
                        <button
                          onClick={() => setConfirmToggle(u.id)}
                          className="text-[12px] text-[#94a3b8] bg-none border-none cursor-pointer p-0 text-left hover:text-[#f87171] transition-colors"
                        >
                          Deactivate
                        </button>
                      )
                    )}
                  </div>
                </div>
              );
            })
          )}
        </div>
      )}
    </div>
  );
}
