// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useCreateApplication } from "@/lib/api";
import { ApiError } from "@/lib/queryClient";

export function CreateApplicationPage() {
  const navigate = useNavigate();
  const create = useCreateApplication();

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [error, setError] = useState("");

  function slugPreview(n: string) {
    return n.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "");
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    setError("");
    try {
      const app = await create.mutateAsync({ name: name.trim(), description: description.trim() });
      navigate(`/applications/${app.id}`);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to create application.");
    }
  }

  const slug = slugPreview(name);

  return (
    <div className="p-8 max-w-[560px]">
      <button
        onClick={() => navigate("/applications")}
        className="text-[12px] text-[#94a3b8] no-underline hover:text-[#f8fafc] bg-transparent border-none cursor-pointer mb-6 block"
      >
        ← Back to Applications
      </button>

      <div className="border border-[#334155] bg-[#1e293b] rounded-[12px] p-7">
        <div className="flex items-center gap-3 mb-5">
          <div className="w-9 h-9 rounded-lg bg-primary/20 flex items-center justify-center text-primary text-[18px]">
            ▦
          </div>
          <div>
            <h1 className="text-[18px] font-bold m-0">New Application</h1>
            <p className="text-[12px] text-[#94a3b8] m-0">
              Creates a GitLab group and gives you control over who can add services.
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
              className="h-10 rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-3 text-[13px] font-[inherit] focus:outline-none focus:border-primary/60"
              autoFocus
            />
            {slug && (
              <p className="text-[11px] text-[#64748b] m-0">
                GitLab group: <span className="text-[#93c5fd] font-mono">/{slug}</span>
              </p>
            )}
          </div>

          <div className="flex flex-col gap-1.5">
            <label className="text-[13px] font-medium">
              Description <span className="text-[#64748b] font-normal">(optional)</span>
            </label>
            <textarea
              placeholder="What is this application for?"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={3}
              className="rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-3 py-2 text-[13px] font-[inherit] focus:outline-none focus:border-primary/60 resize-none"
            />
          </div>

          {error && <p className="text-[12px] text-[#f87171] m-0">{error}</p>}

          <div className="flex gap-3 pt-1">
            <button
              type="submit"
              disabled={!name.trim() || create.isPending}
              className="h-10 flex-1 rounded-md bg-primary border-none text-white text-[13px] font-medium cursor-pointer disabled:opacity-50"
            >
              {create.isPending ? "Creating…" : "Create application"}
            </button>
            <button
              type="button"
              onClick={() => navigate("/applications")}
              className="h-10 px-5 rounded-md border border-[#334155] bg-transparent text-[#94a3b8] text-[13px] cursor-pointer"
            >
              Cancel
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
