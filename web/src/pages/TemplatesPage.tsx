// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState } from "react";
import {
  useTemplates,
  useUpdateTemplate,
  type PipelineTemplate,
} from "@/lib/api";
import { ApiError } from "@/lib/queryClient";

const TAB_LABELS: Record<string, string> = {
  maven:          "Java — Maven",
  gradle:         "Java — Gradle",
  go:             "Go",
  "nodejs-express": "Node.js",
  nextjs:         "Next.js",
  "python-fastapi": "Python",
  dotnet:         ".NET",
  "flutter-web":  "Flutter Web",
  auto:           "Auto-detect",
};

export function TemplatesPage() {
  const { data: templates, isLoading } = useTemplates();
  const [selected, setSelected] = useState<PipelineTemplate | null>(null);
  const [tab, setTab] = useState<"jenkinsfile" | "dockerfile">("jenkinsfile");
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState("");
  const [saveError, setSaveError] = useState("");
  const [saveOk, setSaveOk] = useState(false);
  const update = useUpdateTemplate(selected?.build_tool ?? "");

  function openTemplate(t: PipelineTemplate) {
    setSelected(t);
    setTab("jenkinsfile");
    setEditing(false);
    setDraft("");
    setSaveError("");
    setSaveOk(false);
  }

  function startEdit() {
    setDraft(tab === "jenkinsfile" ? selected!.jenkinsfile : selected!.dockerfile);
    setEditing(true);
    setSaveError("");
    setSaveOk(false);
  }

  async function save() {
    if (!selected) return;
    setSaveError("");
    setSaveOk(false);
    try {
      await update.mutateAsync(
        tab === "jenkinsfile"
          ? { jenkinsfile: draft }
          : { dockerfile: draft },
      );
      setSaveOk(true);
      setEditing(false);
      // patch local copy so textarea reflects saved content
      setSelected((prev) =>
        prev
          ? tab === "jenkinsfile"
            ? { ...prev, jenkinsfile: draft }
            : { ...prev, dockerfile: draft }
          : prev,
      );
    } catch (err) {
      setSaveError(err instanceof ApiError ? err.message : "Save failed.");
    }
  }

  if (isLoading) return <div className="p-8 text-[#64748b] text-[13px]">Loading templates…</div>;

  return (
    <div className="p-8 flex gap-6 h-[calc(100vh-64px)]">
      {/* Sidebar */}
      <aside className="w-52 flex-shrink-0 flex flex-col gap-1">
        <p className="text-[11px] text-[#475569] font-semibold uppercase tracking-widest mb-2">
          Build tools
        </p>
        {(templates ?? []).map((t) => (
          <button
            key={t.build_tool}
            onClick={() => openTemplate(t)}
            className={`text-left px-3 py-2 rounded-lg text-[13px] border-none cursor-pointer transition-colors ${
              selected?.build_tool === t.build_tool
                ? "bg-primary/20 text-primary"
                : "bg-transparent text-[#94a3b8] hover:bg-[#1e293b] hover:text-[#f8fafc]"
            }`}
          >
            {TAB_LABELS[t.build_tool] ?? t.build_tool}
          </button>
        ))}
      </aside>

      {/* Editor panel */}
      {selected ? (
        <div className="flex-1 flex flex-col min-w-0">
          <div className="flex items-center gap-4 mb-4">
            <h2 className="text-[16px] font-bold m-0">
              {TAB_LABELS[selected.build_tool] ?? selected.build_tool}
            </h2>
            <div className="flex gap-1 bg-[#0f172a] rounded-md p-0.5 border border-[#334155]">
              {(["jenkinsfile", "dockerfile"] as const).map((t) => (
                <button
                  key={t}
                  onClick={() => {
                    setTab(t);
                    setEditing(false);
                    setDraft("");
                    setSaveError("");
                    setSaveOk(false);
                  }}
                  className={`px-3 py-1 rounded text-[12px] font-medium border-none cursor-pointer transition-colors ${
                    tab === t
                      ? "bg-primary text-white"
                      : "bg-transparent text-[#64748b] hover:text-[#f8fafc]"
                  }`}
                >
                  {t === "jenkinsfile" ? "Jenkinsfile" : "Dockerfile"}
                </button>
              ))}
            </div>
            <div className="ml-auto flex items-center gap-2">
              {selected.updated_at && (
                <span className="text-[11px] text-[#475569]">
                  Updated {new Date(selected.updated_at).toLocaleDateString()}
                </span>
              )}
              {editing ? (
                <>
                  <button
                    onClick={() => { setEditing(false); setSaveError(""); }}
                    className="h-8 px-3 rounded border border-[#334155] bg-transparent text-[#94a3b8] text-[12px] cursor-pointer"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={save}
                    disabled={update.isPending}
                    className="h-8 px-4 rounded bg-primary border-none text-white text-[12px] font-medium cursor-pointer disabled:opacity-50"
                  >
                    {update.isPending ? "Saving…" : "Save changes"}
                  </button>
                </>
              ) : (
                <button
                  onClick={startEdit}
                  className="h-8 px-4 rounded bg-[#1e3a5f] border border-[#334155] text-[#93c5fd] text-[12px] cursor-pointer hover:bg-[#1e293b]"
                >
                  Edit
                </button>
              )}
            </div>
          </div>

          {saveError && (
            <p className="text-[12px] text-[#f87171] mb-2 m-0">{saveError}</p>
          )}
          {saveOk && (
            <p className="text-[12px] text-[#4ade80] mb-2 m-0">Template saved. New services will use this template.</p>
          )}

          <textarea
            readOnly={!editing}
            value={editing ? draft : (tab === "jenkinsfile" ? selected.jenkinsfile : selected.dockerfile)}
            onChange={(e) => setDraft(e.target.value)}
            spellCheck={false}
            className="flex-1 font-mono text-[12px] text-[#e2e8f0] bg-[#0f172a] border border-[#334155] rounded-lg p-4 resize-none focus:outline-none focus:border-primary/60"
            style={{ lineHeight: "1.6" }}
          />
        </div>
      ) : (
        <div className="flex-1 flex items-center justify-center text-[#475569] text-[13px]">
          Select a build tool to view its Jenkinsfile and Dockerfile templates.
        </div>
      )}
    </div>
  );
}
