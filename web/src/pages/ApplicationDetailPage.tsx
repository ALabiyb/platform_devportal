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
import { Modal, ConfirmDialog, FormField, Button, EmptyState, BackLink } from "@/components/kit";

function ServiceCard({ svc, appId, onNavigate }: { svc: Project; appId: string; onNavigate: (id: string) => void }) {
  const [hover, setHover] = useState(false);
  const [renaming, setRenaming] = useState(false);
  const [newName, setNewName] = useState(svc.name);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [deleteError, setDeleteError] = useState("");

  const renameService = useRenameService(appId, svc.id);
  const deleteService = useDeleteService(appId, svc.id);

  async function handleRename(e: React.FormEvent) {
    e.preventDefault();
    if (!newName.trim()) return;
    await renameService.mutateAsync({ name: newName.trim() });
    setRenaming(false);
  }

  async function handleDelete() {
    setDeleteError("");
    try {
      await deleteService.mutateAsync();
      setConfirmDelete(false);
    } catch (err) {
      setDeleteError(err instanceof ApiError ? err.message : "Failed to archive service.");
    }
  }

  return (
    <div
      className="border border-[var(--line)] bg-[var(--panel)] rounded-[10px] p-4 transition-colors hover:border-primary/40 relative"
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
    >
      {renaming ? (
        <form onSubmit={handleRename} className="flex gap-2 items-center">
          <input
            autoFocus
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            className="flex-1 h-8 rounded border border-[var(--line)] bg-[var(--bg)] text-[var(--text)] px-2 text-[13px] font-[inherit] focus:outline-none"
          />
          <button type="submit" disabled={renameService.isPending}
            className="h-8 px-3 rounded bg-primary border-none text-white text-[12px] cursor-pointer disabled:opacity-50">
            {renameService.isPending ? "…" : "Save"}
          </button>
          <button type="button" onClick={() => { setRenaming(false); setNewName(svc.name); }}
            className="h-8 px-2 rounded border border-[var(--line)] bg-transparent text-[var(--muted)] text-[12px] cursor-pointer">
            Cancel
          </button>
        </form>
      ) : (
        <div className="flex items-center justify-between">
          <div
            className="flex-1 cursor-pointer"
            onClick={() => onNavigate(svc.id)}
            role="link"
            tabIndex={0}
            onKeyDown={e => { if (e.key === "Enter") onNavigate(svc.id); }}
          >
            <div className="flex items-center gap-2 mb-0.5">
              <span className="text-[14px] font-semibold text-[var(--text)]">{svc.name}</span>
              <StatusBadge status={svc.status} />
            </div>
            <span className="text-[11px] text-[var(--faint)] font-mono">{svc.build_tool}</span>
          </div>
          <div className="flex items-center gap-2" style={{ opacity: hover ? 1 : 0.55, transition: "opacity .12s" }}>
            <button
              onClick={() => { setNewName(svc.name); setRenaming(true); }}
              title="Rename"
              aria-label={`Rename ${svc.name}`}
              className="h-7 w-7 flex items-center justify-center rounded border border-[var(--line)] bg-[var(--bg)] text-[var(--muted)] hover:text-[var(--accent)] hover:border-[var(--accent-soft)] text-[12px] cursor-pointer transition-colors"
            >✎</button>
            <button
              onClick={() => setConfirmDelete(true)}
              title="Archive service"
              aria-label={`Archive ${svc.name}`}
              className="h-7 w-7 flex items-center justify-center rounded border border-[var(--line)] bg-[var(--bg)] text-[var(--muted)] hover:text-[var(--bad)] hover:border-[var(--bad-soft)] text-[12px] cursor-pointer transition-colors"
            >×</button>
            <span className="text-[12px] text-[var(--accent)] cursor-pointer" onClick={() => onNavigate(svc.id)}>View →</span>
          </div>
        </div>
      )}
      {confirmDelete && (
        <ConfirmDialog
          title="Archive service?"
          message={<>Archive <strong className="text-[var(--text)]">{svc.name}</strong>?</>}
          confirmLabel="Archive"
          danger
          isPending={deleteService.isPending}
          error={deleteError}
          onConfirm={handleDelete}
          onCancel={() => { setConfirmDelete(false); setDeleteError(""); }}
        />
      )}
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const map: Record<string, { fg: string; bg: string }> = {
    active:       { fg: "var(--ok)",    bg: "var(--ok-soft)" },
    provisioning: { fg: "var(--warn)",  bg: "var(--warn-soft)" },
    failed:       { fg: "var(--bad)",   bg: "var(--bad-soft)" },
    archived:     { fg: "var(--muted)", bg: "var(--faint-soft)" },
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
    ? { fg: "var(--warn)", bg: "var(--warn-soft)" }
    : { fg: "var(--accent)", bg: "var(--accent-soft)" };
  return (
    <span style={{ color: s.fg, background: s.bg, borderRadius: 5, padding: "2px 8px", fontSize: 11, fontWeight: 600, fontFamily: "'JetBrains Mono','IBM Plex Mono',monospace" }}>
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
  const [removeError, setRemoveError] = useState("");

  // rename state
  const [showRename, setShowRename] = useState(false);
  const [renameName, setRenameName] = useState("");
  const [renameDesc, setRenameDesc] = useState("");
  const [renameError, setRenameError] = useState("");

  // delete confirm
  const [showDelete, setShowDelete] = useState(false);
  const [deleteError, setDeleteError] = useState("");

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
    setDeleteError("");
    try {
      await deleteApp.mutateAsync();
      navigate("/applications");
    } catch (err) {
      setDeleteError(err instanceof ApiError ? err.message : "Failed to archive application.");
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
    setRemoveError("");
    try {
      await removeMember.mutateAsync(userId);
      setRemoveTarget(null);
    } catch (err) {
      setRemoveError(err instanceof ApiError ? err.message : "Failed to remove member.");
    }
  }

  if (appLoading) return <div className="p-8 text-[13px] text-[var(--faint)]">Loading…</div>;
  if (!app) return (
    <div className="p-8">
      <p className="text-[14px] text-[var(--bad)]">Application not found.</p>
      <BackLink to="/applications" label="Back to Applications" />
    </div>
  );

  return (
    <div className="p-8 max-w-[1000px]">
      {/* Header */}
      <BackLink to="/applications" label="Applications" />
      <div className="flex items-start justify-between mb-7 mt-3">
        <div className="flex items-start gap-3">
          <div>
            <div className="flex items-center gap-2 mb-0.5">
              <h1 className="text-[22px] font-bold tracking-tight m-0">{app.name}</h1>
              <StatusBadge status={app.status} />
            </div>
            <p className="text-[12px] text-[var(--faint)] m-0 font-mono">/{app.git_namespace}</p>
            {app.description && <p className="text-[12px] text-[var(--muted)] mt-1 m-0">{app.description}</p>}
          </div>
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => { setRenameName(app.name); setRenameDesc(app.description ?? ""); setShowRename(true); }}
            className="h-9 px-3 rounded-md border border-[var(--line)] bg-transparent text-[var(--muted)] text-[13px] cursor-pointer hover:text-[var(--accent)] hover:border-[var(--accent-soft)] transition-colors"
            title="Rename"
          >
            ✎ Edit
          </button>
          <button
            onClick={() => setShowDelete(true)}
            className="h-9 px-3 rounded-md border border-[var(--line)] bg-transparent text-[var(--muted)] text-[13px] cursor-pointer hover:text-[var(--bad)] hover:border-[var(--bad-soft)] transition-colors"
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
        <Modal title="Edit application" onClose={() => setShowRename(false)}>
          <form onSubmit={handleRename} className="flex flex-col gap-3">
            <FormField label="Name">
              <input autoFocus value={renameName} onChange={(e) => setRenameName(e.target.value)} className="field" />
            </FormField>
            <FormField label="Description">
              <input value={renameDesc} onChange={(e) => setRenameDesc(e.target.value)} className="field" />
            </FormField>
            {renameError && <p className="text-[11px] text-[var(--bad)] m-0">{renameError}</p>}
            <div className="flex gap-2 mt-1">
              <Button type="submit" loading={updateApp.isPending} className="flex-1">Save</Button>
              <Button type="button" variant="secondary" onClick={() => setShowRename(false)} className="flex-1">Cancel</Button>
            </div>
          </form>
        </Modal>
      )}

      {/* Delete confirm modal */}
      {showDelete && (
        <ConfirmDialog
          title="Archive application?"
          message={<>
            <strong className="text-[var(--text)]">{app.name}</strong> will be archived and hidden from the
            active list. Existing services are not deleted.
          </>}
          confirmLabel="Archive"
          danger
          isPending={deleteApp.isPending}
          error={deleteError}
          onConfirm={handleDelete}
          onCancel={() => setShowDelete(false)}
        />
      )}

      <div className="grid gap-6" style={{ gridTemplateColumns: "1fr 340px" }}>

        {/* Services */}
        <div>
          <div className="flex items-center justify-between mb-3">
            <h2 className="text-[14px] font-semibold m-0">Services</h2>
            <span className="text-[12px] text-[var(--faint)]">{services?.length ?? 0} services</span>
          </div>

          {!services?.length ? (
            <div className="border border-dashed border-[var(--line)] rounded-[10px]">
              <EmptyState
                title="No services yet"
                description="Add your first microservice to start provisioning."
                action={
                  <Button onClick={() => navigate(`/applications/${appId}/services/new`)}>
                    + Add service
                  </Button>
                }
              />
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
              className="text-[12px] text-[var(--accent)] bg-transparent border-none cursor-pointer"
            >
              + Add
            </button>
          </div>

          {showAddMember && (
            <form onSubmit={handleAddMember} className="border border-primary/30 bg-[var(--bg)] rounded-[8px] p-3 mb-3 flex flex-col gap-2">
              <select
                value={selectedUser}
                onChange={(e) => setSelectedUser(e.target.value)}
                className="h-9 rounded border border-[var(--line)] bg-[var(--panel)] text-[var(--text)] px-2 text-[12px] font-[inherit] focus:outline-none"
              >
                <option value="">Select user…</option>
                {eligibleUsers.map((u) => (
                  <option key={u.id} value={u.id}>{u.display_name} ({u.email})</option>
                ))}
              </select>
              <select
                value={selectedRole}
                onChange={(e) => setSelectedRole(e.target.value)}
                className="h-9 rounded border border-[var(--line)] bg-[var(--panel)] text-[var(--text)] px-2 text-[12px] font-[inherit] focus:outline-none"
              >
                <option value="developer">Developer</option>
                <option value="lead">Lead</option>
              </select>
              {memberError && <p className="text-[11px] text-[var(--bad)] m-0">{memberError}</p>}
              <div className="flex gap-2">
                <button type="submit" disabled={!selectedUser || addMember.isPending}
                  className="flex-1 h-8 rounded bg-primary border-none text-white text-[12px] cursor-pointer disabled:opacity-50">
                  Add
                </button>
                <button type="button" onClick={() => setShowAddMember(false)}
                  className="flex-1 h-8 rounded border border-[var(--line)] bg-transparent text-[var(--muted)] text-[12px] cursor-pointer">
                  Cancel
                </button>
              </div>
            </form>
          )}

          <div className="border border-[var(--line)] rounded-[10px] overflow-hidden">
            {!members?.length ? (
              <div className="px-4 py-6 text-center text-[12px] text-[var(--faint)]">No members yet.</div>
            ) : (
              members.map((m) => (
                <div key={m.user_id} className="flex items-center justify-between px-4 py-3 border-b border-[var(--panel)] last:border-b-0">
                  <div>
                    <p className="text-[13px] font-medium m-0">{m.display_name}</p>
                    <p className="text-[11px] text-[var(--faint)] m-0 font-mono">{m.email}</p>
                  </div>
                  <div className="flex items-center gap-2">
                    <RoleChip role={m.role} />
                    <button onClick={() => setRemoveTarget(m.user_id)}
                      className="text-[11px] text-[var(--faint)] hover:text-[var(--bad)] bg-transparent border-none cursor-pointer transition-colors">
                      ×
                    </button>
                  </div>
                </div>
              ))
            )}
          </div>
          {removeTarget && (() => {
            const target = members?.find(m => m.user_id === removeTarget);
            return (
              <ConfirmDialog
                title="Remove member?"
                message={<>Remove <strong className="text-[var(--text)]">{target?.display_name ?? "this member"}</strong> from {app.name}?</>}
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
      </div>
    </div>
  );
}
