// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useCreateApplication, useUsers, User, apiFetch } from "@/lib/api";
import { ApiError } from "@/lib/queryClient";
import { BackLink, Button } from "@/components/kit";

export function CreateApplicationPage() {
  const navigate = useNavigate();
  const create = useCreateApplication();
  const { data: users = [] } = useUsers();

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [maintainerIds, setMaintainerIds] = useState<string[]>([]);
  const [error, setError] = useState("");
  const [partialSuccess, setPartialSuccess] = useState<{ appId: string } | null>(null);

  function slugPreview(n: string) {
    return n.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");
  }

  function toggleMaintainer(uid: string) {
    setMaintainerIds(prev =>
      prev.includes(uid) ? prev.filter(id => id !== uid) : [...prev, uid]
    );
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    setError("");
    try {
      const app = await create.mutateAsync({ name: name.trim(), description: description.trim() });
      // Add selected maintainers as members. The application already exists at
      // this point — a failure here shouldn't look like the whole thing failed,
      // but it also shouldn't be silent (plain fetch() never throws on HTTP
      // error status, so this used to succeed unconditionally either way).
      try {
        await Promise.all(
          maintainerIds.map(uid =>
            apiFetch(`/api/v1/applications/${app.id}/members`, {
              method: "POST",
              body: JSON.stringify({ user_id: uid, role: "maintainer" }),
            })
          )
        );
      } catch {
        setPartialSuccess({ appId: app.id });
        return;
      }
      navigate(`/applications/${app.id}`);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to create application.");
    }
  }

  const slug = slugPreview(name);

  if (partialSuccess) {
    return (
      <div className="p-8 max-w-[560px]">
        <div className="border border-[var(--warn)] bg-[var(--panel)] rounded-[12px] p-7">
          <h1 className="text-[18px] font-bold m-0 mb-2">Application created</h1>
          <p className="text-[13px] text-[var(--muted)] m-0 mb-5">
            {name.trim()} was created, but one or more selected members could not be added.
            You can add them from the application page.
          </p>
          <Button onClick={() => navigate(`/applications/${partialSuccess.appId}`)}>
            Go to application
          </Button>
        </div>
      </div>
    );
  }

  return (
    <div className="p-8 max-w-[560px]">
      <div className="mb-6"><BackLink to="/applications" label="Back to Applications" /></div>

      <div className="border border-[var(--line)] bg-[var(--panel)] rounded-[12px] p-7">
        <div className="flex items-center gap-3 mb-5">
          <div className="w-9 h-9 rounded-lg bg-primary/20 flex items-center justify-center text-primary text-[18px]">
            ▦
          </div>
          <div>
            <h1 className="text-[18px] font-bold m-0">New Application</h1>
            <p className="text-[12px] text-[var(--muted)] m-0">
              Creates a Gitea namespace and gives you control over who can add services.
            </p>
          </div>
        </div>

        <form onSubmit={handleSubmit} className="flex flex-col gap-5">
          <div className="flex flex-col gap-1.5">
            <label className="text-[13px] font-medium">Application name</label>
            <input
              type="text"
              placeholder="e.g. Restaurant POS System"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="h-10 rounded-md border border-[var(--line)] bg-[var(--bg)] text-[var(--text)] px-3 text-[13px] font-[inherit] focus:outline-none focus:border-primary/60"
              autoFocus
            />
            {slug && (
              <p className="text-[11px] text-[var(--faint)] m-0">
                Gitea namespace: <span className="text-[var(--accent)] font-mono">/{slug}</span>
              </p>
            )}
          </div>

          <div className="flex flex-col gap-1.5">
            <label className="text-[13px] font-medium">
              Description <span className="text-[var(--faint)] font-normal">(optional)</span>
            </label>
            <textarea
              placeholder="What is this application for?"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={3}
              className="rounded-md border border-[var(--line)] bg-[var(--bg)] text-[var(--text)] px-3 py-2 text-[13px] font-[inherit] focus:outline-none focus:border-primary/60 resize-none"
            />
          </div>

          {/* Maintainers */}
          <div className="flex flex-col gap-2">
            <label className="text-[13px] font-medium">
              Team members <span className="text-[var(--faint)] font-normal">(optional)</span>
            </label>
            <p className="text-[11px] text-[var(--faint)] m-0">Select users who will have access to add services to this application.</p>
            <div style={{ maxHeight: 180, overflowY: "auto", border: "1px solid var(--line)", borderRadius: 8, background: "var(--bg)" }}>
              {users.length === 0 && (
                <div style={{ padding: "12px", fontSize: 12, color: "var(--faint)" }}>No other users found.</div>
              )}
              {users.map((u: User) => {
                const selected = maintainerIds.includes(u.id);
                return (
                  <label key={u.id} style={{
                    display: "flex", alignItems: "center", gap: 10, padding: "9px 12px",
                    cursor: "pointer", borderBottom: "1px solid var(--panel)",
                    background: selected ? "var(--accent-soft)" : "transparent",
                  }}>
                    <input type="checkbox" checked={selected} onChange={() => toggleMaintainer(u.id)}
                      style={{ accentColor: "var(--accent)" }} />
                    <div>
                      <div style={{ fontSize: 13, fontWeight: 500, color: "var(--text)" }}>{u.display_name}</div>
                      <div style={{ fontSize: 11, color: "var(--faint)", fontFamily: "JetBrains Mono,monospace" }}>{u.email}</div>
                    </div>
                  </label>
                );
              })}
            </div>
            {maintainerIds.length > 0 && (
              <p className="text-[11px] text-[var(--accent)] m-0">{maintainerIds.length} member{maintainerIds.length !== 1 ? "s" : ""} selected</p>
            )}
          </div>

          {error && <p className="text-[12px] text-[var(--bad)] m-0">{error}</p>}

          <div className="flex gap-3 pt-1">
            <Button
              type="submit"
              size="lg"
              className="flex-1 justify-center"
              disabled={!name.trim()}
              loading={create.isPending}
            >
              {create.isPending ? "Creating…" : "Create application"}
            </Button>
            <button
              type="button"
              onClick={() => navigate("/applications")}
              className="h-10 px-5 rounded-md border border-[var(--line)] bg-transparent text-[var(--muted)] text-[13px] cursor-pointer"
            >
              Cancel
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
