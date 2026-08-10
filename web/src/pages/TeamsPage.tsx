// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTeams, useCreateTeam, useUpdateTeam, useDeleteTeam } from "@/lib/api";
import { ApiError } from "@/lib/queryClient";

function TeamCard({
  id, name, slug,
  onRename, onDelete, onClick,
}: {
  id: string; name: string; slug: string;
  onRename: (id: string, name: string) => void;
  onDelete: (id: string) => void;
  onClick: (id: string) => void;
}) {
  const [hover, setHover] = useState(false);

  return (
    <div
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
      className="border border-[#334155] bg-[#1e293b] rounded-[10px] p-[18px] transition-colors hover:border-primary/40 relative"
    >
      <div
        className="cursor-pointer"
        onClick={() => onClick(id)}
      >
        <div className="text-[15px] font-semibold mb-1.5 pr-14">{name}</div>
        <p className="text-[12px] text-[#94a3b8] m-0 font-mono">{slug}</p>
      </div>

      {hover && (
        <div className="absolute top-3 right-3 flex gap-1">
          <button
            onClick={(e) => { e.stopPropagation(); onRename(id, name); }}
            title="Rename"
            className="h-7 w-7 flex items-center justify-center rounded border border-[#334155] bg-[#0f172a] text-[#94a3b8] hover:text-[#60a5fa] hover:border-[#60a5fa]/40 text-[12px] cursor-pointer transition-colors"
          >
            ✎
          </button>
          <button
            onClick={(e) => { e.stopPropagation(); onDelete(id); }}
            title="Delete"
            className="h-7 w-7 flex items-center justify-center rounded border border-[#334155] bg-[#0f172a] text-[#94a3b8] hover:text-[#f87171] hover:border-[#f87171]/40 text-[12px] cursor-pointer transition-colors"
          >
            ×
          </button>
        </div>
      )}
    </div>
  );
}

export function TeamsPage() {
  const { data: teams, isLoading } = useTeams();
  const createTeam = useCreateTeam();
  const navigate = useNavigate();

  const [showCreate, setShowCreate] = useState(false);
  const [createName, setCreateName] = useState("");
  const [createError, setCreateError] = useState("");

  // rename state
  const [renameId, setRenameId] = useState<string | null>(null);
  const [renameName, setRenameName] = useState("");
  const [renameError, setRenameError] = useState("");
  const updateTeam = useUpdateTeam(renameId ?? "");

  // delete confirm state
  const [deleteId, setDeleteId] = useState<string | null>(null);
  const deleteTeam = useDeleteTeam(deleteId ?? "");

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    if (!createName.trim()) return;
    try {
      await createTeam.mutateAsync({ name: createName.trim() });
      setCreateName("");
      setShowCreate(false);
      setCreateError("");
    } catch (err) {
      setCreateError(err instanceof ApiError ? err.message : "Failed to create team.");
    }
  }

  async function handleRename(e: React.FormEvent) {
    e.preventDefault();
    if (!renameName.trim() || !renameId) return;
    try {
      await updateTeam.mutateAsync({ name: renameName.trim() });
      setRenameId(null);
      setRenameName("");
      setRenameError("");
    } catch (err) {
      setRenameError(err instanceof ApiError ? err.message : "Failed to rename team.");
    }
  }

  async function handleDelete() {
    if (!deleteId) return;
    try {
      await deleteTeam.mutateAsync();
      setDeleteId(null);
    } catch (err) {
      alert(err instanceof ApiError ? err.message : "Failed to delete team. Make sure it has no projects assigned.");
      setDeleteId(null);
    }
  }

  const deleteName = teams?.find((t) => t.id === deleteId)?.name;

  return (
    <div className="p-8 max-w-[1000px]">
      <div className="flex items-start justify-between mb-6">
        <div>
          <h1 className="text-[24px] font-bold tracking-tight m-0 mb-1">Teams</h1>
          <p className="text-[13px] text-[#94a3b8] m-0">
            Manage team membership and project ownership.
          </p>
        </div>
        <button
          onClick={() => setShowCreate(true)}
          className="h-[36px] rounded-md border-none bg-primary text-white text-[13px] font-medium px-4 cursor-pointer"
        >
          + New team
        </button>
      </div>

      {/* Create form */}
      {showCreate && (
        <div className="border border-primary/30 bg-[#1e293b] rounded-[10px] p-5 mb-5">
          <h3 className="text-[14px] font-semibold m-0 mb-3">Create team</h3>
          <form onSubmit={handleCreate} className="flex gap-3 items-end">
            <div className="flex flex-col gap-1.5">
              <label className="text-[12px] text-[#94a3b8]">Team name</label>
              <input
                type="text"
                placeholder="e.g. Platform Engineering"
                value={createName}
                autoFocus
                onChange={(e) => setCreateName(e.target.value)}
                className="h-9 w-52 rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-3 text-[13px] font-[inherit] focus:outline-none"
              />
            </div>
            <button type="submit" disabled={createTeam.isPending}
              className="h-9 rounded-md border-none bg-primary text-white text-[13px] font-medium px-4 cursor-pointer disabled:opacity-50">
              {createTeam.isPending ? "Creating…" : "Create"}
            </button>
            <button type="button" onClick={() => { setShowCreate(false); setCreateName(""); }}
              className="h-9 rounded-md border border-[#334155] bg-transparent text-[#94a3b8] text-[13px] px-4 cursor-pointer">
              Cancel
            </button>
          </form>
          {createError && <p className="text-[12px] text-[#f87171] mt-2">{createError}</p>}
        </div>
      )}

      {/* Rename modal */}
      {renameId && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
          <div className="bg-[#1e293b] border border-[#334155] rounded-[12px] p-6 w-[360px]">
            <h3 className="text-[15px] font-semibold m-0 mb-4">Rename team</h3>
            <form onSubmit={handleRename} className="flex flex-col gap-3">
              <input
                type="text"
                autoFocus
                value={renameName}
                onChange={(e) => setRenameName(e.target.value)}
                className="h-9 rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-3 text-[13px] font-[inherit] focus:outline-none"
              />
              {renameError && <p className="text-[11px] text-[#f87171] m-0">{renameError}</p>}
              <div className="flex gap-2 mt-1">
                <button type="submit" disabled={updateTeam.isPending}
                  className="flex-1 h-9 rounded-md bg-primary border-none text-white text-[13px] cursor-pointer disabled:opacity-50">
                  {updateTeam.isPending ? "Saving…" : "Save"}
                </button>
                <button type="button" onClick={() => { setRenameId(null); setRenameError(""); }}
                  className="flex-1 h-9 rounded-md border border-[#334155] bg-transparent text-[#94a3b8] text-[13px] cursor-pointer">
                  Cancel
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Delete confirm modal */}
      {deleteId && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
          <div className="bg-[#1e293b] border border-[#334155] rounded-[12px] p-6 w-[360px]">
            <h3 className="text-[15px] font-semibold m-0 mb-2">Delete team?</h3>
            <p className="text-[13px] text-[#94a3b8] m-0 mb-5">
              <strong className="text-[#f8fafc]">{deleteName}</strong> will be permanently deleted.
              This will fail if the team still owns projects.
            </p>
            <div className="flex gap-2">
              <button onClick={handleDelete} disabled={deleteTeam.isPending}
                className="flex-1 h-9 rounded-md bg-[#f87171] border-none text-white text-[13px] cursor-pointer disabled:opacity-50">
                {deleteTeam.isPending ? "Deleting…" : "Delete"}
              </button>
              <button onClick={() => setDeleteId(null)}
                className="flex-1 h-9 rounded-md border border-[#334155] bg-transparent text-[#94a3b8] text-[13px] cursor-pointer">
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}

      {isLoading ? (
        <div className="text-[13px] text-[#64748b]">Loading teams…</div>
      ) : !teams?.length ? (
        <div className="border border-dashed border-[#334155] rounded-[10px] py-14 text-center">
          <p className="text-[14px] font-medium m-0 mb-1">No teams yet</p>
          <p className="text-[12px] text-[#94a3b8] m-0">Create a team to assign projects to it.</p>
        </div>
      ) : (
        <div className="grid gap-4" style={{ gridTemplateColumns: "repeat(auto-fill, minmax(240px, 1fr))" }}>
          {teams.map((t) => (
            <TeamCard
              key={t.id}
              id={t.id}
              name={t.name}
              slug={t.slug}
              onClick={(id) => navigate(`/teams/${id}`)}
              onRename={(id, name) => { setRenameId(id); setRenameName(name); setRenameError(""); }}
              onDelete={(id) => setDeleteId(id)}
            />
          ))}
        </div>
      )}
    </div>
  );
}
