// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState } from "react";
import { useParams, useNavigate } from "react-router-dom";
import {
  useApplication, useApplicationMembers, useApplicationServices,
  useAddApplicationMember, useRemoveApplicationMember, useUsers,
  useUpdateApplication, useDeleteApplication,
  useDeleteService, useRenameService,
  type Project,
} from "@/lib/api";
import { ApiError } from "@/lib/queryClient";

function ServiceCard({ svc, appId, onNavigate }: { svc: Project; appId: string; onNavigate: (id: string) => void }) {
  const [hover, setHover] = useState(false);
  const [renaming, setRenaming] = useState(false);
  const [newName, setNewName] = useState(svc.name);
  const [confirmDelete, setConfirmDelete] = useState(false);

  const renameService = useRenameService(appId, svc.id);
  const deleteService = useDeleteService(appId, svc.id);

  async function handleRename(e: React.FormEvent) {
    e.preventDefault();
    if (!newName.trim()) return;
    await renameService.mutateAsync({ name: newName.trim() });
    setRenaming(false);
  }

  async function handleDelete() {
    await deleteService.mutateAsync();
    setConfirmDelete(false);
  }

  return (
    <div
      className="border border-[#334155] bg-[#1e293b] rounded-[10px] p-4 transition-colors hover:border-primary/40 relative"
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
    >
      {renaming ? (
        <form onSubmit={handleRename} className="flex gap-2 items-center">
          <input
            autoFocus
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            className="flex-1 h-8 rounded border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-2 text-[13px] font-[inherit] focus:outline-none"
          />
          <button type="submit" disabled={renameService.isPending}
            className="h-8 px-3 rounded bg-primary border-none text-white text-[12px] cursor-pointer disabled:opacity-50">
            {renameService.isPending ? "…" : "Save"}
          </button>
          <button type="button" onClick={() => { setRenaming(false); setNewName(svc.name); }}
            className="h-8 px-2 rounded border border-[#334155] bg-transparent text-[#94a3b8] text-[12px] cursor-pointer">
            Cancel
          </button>
        </form>
      ) : confirmDelete ? (
        <div className="flex items-center gap-3">
          <p className="text-[13px] text-[#f8fafc] m-0 flex-1">
            Archive <strong>{svc.name}</strong>?
          </p>
          <button onClick={handleDelete} disabled={deleteService.isPending}
            className="h-8 px-3 rounded bg-[#f87171] border-none text-white text-[12px] cursor-pointer disabled:opacity-50">
            {deleteService.isPending ? "…" : "Archive"}
          </button>
          <button onClick={() => setConfirmDelete(false)}
            className="h-8 px-2 rounded border border-[#334155] bg-transparent text-[#94a3b8] text-[12px] cursor-pointer">
            Cancel
          </button>
        </div>
      ) : (
        <div className="flex items-center justify-between">
          <div
            className="flex-1 cursor-pointer"
            onClick={() => onNavigate(svc.id)}
          >
            <div className="flex items-center gap-2 mb-0.5">
              <span className="text-[14px] font-semibold text-[#f8fafc]">{svc.name}</span>
              <StatusBadge status={svc.status} />
            </div>
            <span className="text-[11px] text-[#64748b] font-mono">{svc.build_tool}</span>
          </div>
          <div className="flex items-center gap-2">
            {hover && (
              <>
                <button
                  onClick={() => { setNewName(svc.name); setRenaming(true); }}
                  title="Rename"
                  className="h-7 w-7 flex items-center justify-center rounded border border-[#334155] bg-[#0f172a] text-[#94a3b8] hover:text-[#60a5fa] hover:border-[#60a5fa]/40 text-[12px] cursor-pointer transition-colors"
                >✎</button>
                <button
                  onClick={() => setConfirmDelete(true)}
                  title="Archive service"
                  className="h-7 w-7 flex items-center justify-center rounded border border-[#334155] bg-[#0f172a] text-[#94a3b8] hover:text-[#f87171] hover:border-[#f87171]/40 text-[12px] cursor-pointer transition-colors"
                >×</button>
              </>
            )}
            <span className="text-[12px] text-[#60a5fa] cursor-pointer" onClick={() => onNavigate(svc.id)}>View →</span>
          </div>
        </div>
      )}
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const map: Record<string, { fg: string; bg: string }> = {
    active:       { fg: "#4ade80", bg: "rgba(34,197,94,0.15)" },
    provisioning: { fg: "#facc15", bg: "rgba(234,179,8,0.15)" },
    failed:       { fg: "#f87171", bg: "rgba(248,113,113,0.15)" },
    archived:     { fg: "#94a3b8", bg: "rgba(148,163,184,0.12)" },
  };
  const s = map[status] ?? map.archived;
  return (
    <span style={{ color: s.fg, background: s.bg, borderRadius: 5, padding: "2px 9px", fontSize: 11, fontWeight: 600 }}>
      {status}
    </span>
  );
}

function RoleChip({ role }: { role: string }) {
  const s = role === "lead"
    ? { fg: "#facc15", bg: "rgba(234,179,8,0.15)" }
    : { fg: "#60a5fa", bg: "rgba(96,165,250,0.12)" };
  return (
    <span style={{ color: s.fg, background: s.bg, borderRadius: 5, padding: "2px 8px", fontSize: 11, fontWeight: 600, fontFamily: "var(--font-mono,'IBM Plex Mono',monospace)" }}>
      {role}
    </span>
  );
}

export function ApplicationDetailPage() {
  const { appId } = useParams<{ appId: string }>();
  const navigate = useNavigate();

  const { data: app, isLoading: appLoading } = useApplication(appId ?? "");
  const { data: members } = useApplicationMembers(appId ?? "");
  const { data: services } = useApplicationServices(appId ?? "");
  const { data: allUsers } = useUsers();
  const addMember = useAddApplicationMember(appId ?? "");
  const removeMember = useRemoveApplicationMember(appId ?? "");
  const updateApp = useUpdateApplication(appId ?? "");
  const deleteApp = useDeleteApplication(appId ?? "");

  const [showAddMember, setShowAddMember] = useState(false);
  const [selectedUser, setSelectedUser] = useState("");
  const [selectedRole, setSelectedRole] = useState("developer");
  const [removeTarget, setRemoveTarget] = useState<string | null>(null);
  const [memberError, setMemberError] = useState("");

  // rename state
  const [showRename, setShowRename] = useState(false);
  const [renameName, setRenameName] = useState("");
  const [renameDesc, setRenameDesc] = useState("");
  const [renameError, setRenameError] = useState("");

  // delete confirm
  const [showDelete, setShowDelete] = useState(false);

  async function handleRename(e: React.FormEvent) {
    e.preventDefault();
    if (!renameName.trim()) return;
    try {
      await updateApp.mutateAsync({ name: renameName.trim(), description: renameDesc });
      setShowRename(false);
      setRenameError("");
    } catch (err) {
      setRenameError(err instanceof ApiError ? err.message : "Failed to update.");
    }
  }

  async function handleDelete() {
    try {
      await deleteApp.mutateAsync();
      navigate("/applications");
    } catch (err) {
      alert(err instanceof ApiError ? err.message : "Failed to delete application.");
    }
  }

  const memberUserIds = new Set(members?.map((m) => m.user_id) ?? []);
  const eligibleUsers = allUsers?.filter((u) => !memberUserIds.has(u.id)) ?? [];

  async function handleAddMember(e: React.FormEvent) {
    e.preventDefault();
    if (!selectedUser) return;
    setMemberError("");
    try {
      await addMember.mutateAsync({ user_id: selectedUser, role: selectedRole });
      setShowAddMember(false);
      setSelectedUser("");
      setSelectedRole("developer");
    } catch (err) {
      setMemberError(err instanceof ApiError ? err.message : "Failed to add member.");
    }
  }

  async function handleRemove(userId: string) {
    try {
      await removeMember.mutateAsync(userId);
      setRemoveTarget(null);
    } catch {}
  }

  if (appLoading) return <div className="p-8 text-[13px] text-[#64748b]">Loading…</div>;
  if (!app) return (
    <div className="p-8">
      <p className="text-[14px] text-[#f87171]">Application not found.</p>
      <button onClick={() => navigate("/applications")} className="mt-2 text-[13px] text-[#60a5fa] underline bg-transparent border-none cursor-pointer">
        ← Back to Applications
      </button>
    </div>
  );

  return (
    <div className="p-8 max-w-[1000px]">
      {/* Header */}
      <div className="flex items-start justify-between mb-7">
        <div className="flex items-start gap-3">
          <button onClick={() => navigate("/applications")}
            className="mt-0.5 h-7 w-7 flex items-center justify-center rounded border border-[#334155] bg-transparent text-[#94a3b8] cursor-pointer hover:border-primary/50 text-[13px]">
            ←
          </button>
          <div>
            <div className="flex items-center gap-2 mb-0.5">
              <h1 className="text-[22px] font-bold tracking-tight m-0">{app.name}</h1>
              <StatusBadge status={app.status} />
            </div>
            <p className="text-[12px] text-[#64748b] m-0 font-mono">/{app.git_namespace}</p>
            {app.description && <p className="text-[12px] text-[#94a3b8] mt-1 m-0">{app.description}</p>}
          </div>
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => { setRenameName(app.name); setRenameDesc(app.description ?? ""); setShowRename(true); }}
            className="h-9 px-3 rounded-md border border-[#334155] bg-transparent text-[#94a3b8] text-[13px] cursor-pointer hover:text-[#60a5fa] hover:border-[#60a5fa]/40 transition-colors"
            title="Rename"
          >
            ✎ Edit
          </button>
          <button
            onClick={() => setShowDelete(true)}
            className="h-9 px-3 rounded-md border border-[#334155] bg-transparent text-[#94a3b8] text-[13px] cursor-pointer hover:text-[#f87171] hover:border-[#f87171]/40 transition-colors"
            title="Archive application"
          >
            Archive
          </button>
          <button
            onClick={() => navigate(`/applications/${appId}/services/new`)}
            className="h-9 px-4 rounded-md bg-primary border-none text-white text-[13px] font-medium cursor-pointer"
          >
            + Add service
          </button>
        </div>
      </div>

      {/* Rename modal */}
      {showRename && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
          <div className="bg-[#1e293b] border border-[#334155] rounded-[12px] p-6 w-[400px]">
            <h3 className="text-[15px] font-semibold m-0 mb-4">Edit application</h3>
            <form onSubmit={handleRename} className="flex flex-col gap-3">
              <div className="flex flex-col gap-1.5">
                <label className="text-[12px] text-[#94a3b8]">Name</label>
                <input autoFocus value={renameName} onChange={(e) => setRenameName(e.target.value)}
                  className="h-9 rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-3 text-[13px] font-[inherit] focus:outline-none" />
              </div>
              <div className="flex flex-col gap-1.5">
                <label className="text-[12px] text-[#94a3b8]">Description</label>
                <input value={renameDesc} onChange={(e) => setRenameDesc(e.target.value)}
                  className="h-9 rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-3 text-[13px] font-[inherit] focus:outline-none" />
              </div>
              {renameError && <p className="text-[11px] text-[#f87171] m-0">{renameError}</p>}
              <div className="flex gap-2 mt-1">
                <button type="submit" disabled={updateApp.isPending}
                  className="flex-1 h-9 rounded-md bg-primary border-none text-white text-[13px] cursor-pointer disabled:opacity-50">
                  {updateApp.isPending ? "Saving…" : "Save"}
                </button>
                <button type="button" onClick={() => setShowRename(false)}
                  className="flex-1 h-9 rounded-md border border-[#334155] bg-transparent text-[#94a3b8] text-[13px] cursor-pointer">
                  Cancel
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Delete confirm modal */}
      {showDelete && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
          <div className="bg-[#1e293b] border border-[#334155] rounded-[12px] p-6 w-[380px]">
            <h3 className="text-[15px] font-semibold m-0 mb-2">Archive application?</h3>
            <p className="text-[13px] text-[#94a3b8] m-0 mb-5">
              <strong className="text-[#f8fafc]">{app.name}</strong> will be archived and hidden from the
              active list. Existing services are not deleted.
            </p>
            <div className="flex gap-2">
              <button onClick={handleDelete} disabled={deleteApp.isPending}
                className="flex-1 h-9 rounded-md bg-[#f87171] border-none text-white text-[13px] cursor-pointer disabled:opacity-50">
                {deleteApp.isPending ? "Archiving…" : "Archive"}
              </button>
              <button onClick={() => setShowDelete(false)}
                className="flex-1 h-9 rounded-md border border-[#334155] bg-transparent text-[#94a3b8] text-[13px] cursor-pointer">
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}

      <div className="grid gap-6" style={{ gridTemplateColumns: "1fr 340px" }}>

        {/* Services */}
        <div>
          <div className="flex items-center justify-between mb-3">
            <h2 className="text-[14px] font-semibold m-0">Services</h2>
            <span className="text-[12px] text-[#64748b]">{services?.length ?? 0} services</span>
          </div>

          {!services?.length ? (
            <div className="border border-dashed border-[#334155] rounded-[10px] py-12 text-center">
              <p className="text-[14px] font-medium m-0 mb-1">No services yet</p>
              <p className="text-[12px] text-[#94a3b8] m-0 mb-4">Add your first microservice to start provisioning.</p>
              <button
                onClick={() => navigate(`/applications/${appId}/services/new`)}
                className="h-9 px-4 rounded-md bg-primary border-none text-white text-[13px] cursor-pointer"
              >
                + Add service
              </button>
            </div>
          ) : (
            <div className="flex flex-col gap-3">
              {services.map((svc) => (
                <ServiceCard
                  key={svc.id}
                  svc={svc}
                  appId={appId ?? ""}
                  onNavigate={(id) => navigate(`/projects/${id}`)}
                />
              ))}
            </div>
          )}
        </div>

        {/* Members panel */}
        <div>
          <div className="flex items-center justify-between mb-3">
            <h2 className="text-[14px] font-semibold m-0">Members</h2>
            <button
              onClick={() => setShowAddMember(!showAddMember)}
              className="text-[12px] text-[#60a5fa] bg-transparent border-none cursor-pointer"
            >
              + Add
            </button>
          </div>

          {showAddMember && (
            <form onSubmit={handleAddMember} className="border border-primary/30 bg-[#0f172a] rounded-[8px] p-3 mb-3 flex flex-col gap-2">
              <select
                value={selectedUser}
                onChange={(e) => setSelectedUser(e.target.value)}
                className="h-9 rounded border border-[#334155] bg-[#1e293b] text-[#f8fafc] px-2 text-[12px] font-[inherit] focus:outline-none"
              >
                <option value="">Select user…</option>
                {eligibleUsers.map((u) => (
                  <option key={u.id} value={u.id}>{u.display_name} ({u.email})</option>
                ))}
              </select>
              <select
                value={selectedRole}
                onChange={(e) => setSelectedRole(e.target.value)}
                className="h-9 rounded border border-[#334155] bg-[#1e293b] text-[#f8fafc] px-2 text-[12px] font-[inherit] focus:outline-none"
              >
                <option value="developer">Developer</option>
                <option value="lead">Lead</option>
              </select>
              {memberError && <p className="text-[11px] text-[#f87171] m-0">{memberError}</p>}
              <div className="flex gap-2">
                <button type="submit" disabled={!selectedUser || addMember.isPending}
                  className="flex-1 h-8 rounded bg-primary border-none text-white text-[12px] cursor-pointer disabled:opacity-50">
                  Add
                </button>
                <button type="button" onClick={() => setShowAddMember(false)}
                  className="flex-1 h-8 rounded border border-[#334155] bg-transparent text-[#94a3b8] text-[12px] cursor-pointer">
                  Cancel
                </button>
              </div>
            </form>
          )}

          <div className="border border-[#334155] rounded-[10px] overflow-hidden">
            {!members?.length ? (
              <div className="px-4 py-6 text-center text-[12px] text-[#64748b]">No members yet.</div>
            ) : (
              members.map((m) => (
                <div key={m.user_id} className="flex items-center justify-between px-4 py-3 border-b border-[#1e293b] last:border-b-0">
                  <div>
                    <p className="text-[13px] font-medium m-0">{m.display_name}</p>
                    <p className="text-[11px] text-[#64748b] m-0 font-mono">{m.email}</p>
                  </div>
                  <div className="flex items-center gap-2">
                    <RoleChip role={m.role} />
                    {removeTarget === m.user_id ? (
                      <div className="flex gap-1">
                        <button onClick={() => handleRemove(m.user_id)}
                          className="text-[11px] text-[#f87171] bg-transparent border-none cursor-pointer">✓</button>
                        <button onClick={() => setRemoveTarget(null)}
                          className="text-[11px] text-[#64748b] bg-transparent border-none cursor-pointer">✗</button>
                      </div>
                    ) : (
                      <button onClick={() => setRemoveTarget(m.user_id)}
                        className="text-[11px] text-[#64748b] hover:text-[#f87171] bg-transparent border-none cursor-pointer transition-colors">
                        ×
                      </button>
                    )}
                  </div>
                </div>
              ))
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
