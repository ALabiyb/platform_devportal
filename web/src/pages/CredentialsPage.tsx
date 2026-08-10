// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState } from "react";
import { useCredentials, useCreateCredential, useDeleteCredential } from "@/lib/api";

const CRED_TYPES = ["gitlab", "jenkins", "harbor", "defectdojo", "argocd"] as const;
type CredType = typeof CRED_TYPES[number];
const TYPE_LABELS: Record<CredType, string> = {
  gitlab: "GitLab", jenkins: "Jenkins", harbor: "Harbor",
  defectdojo: "DefectDojo", argocd: "ArgoCD",
};

interface CredRow {
  id: string;
  provider_type: string;
  label: string;
  created_at: string;
}

type PanelMode = "add" | "rotate";

interface Panel {
  mode: PanelMode;
  type: CredType;
  label: string;
  token: string;
  targetId?: string;
  targetLabel?: string;
}

export function CredentialsPage() {
  const { data: creds, isLoading } = useCredentials();
  const createCred = useCreateCredential();
  const deleteCred = useDeleteCredential();

  const [panel, setPanel] = useState<Panel | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);

  function openAdd() {
    setPanel({ mode: "add", type: "gitlab", label: "", token: "" });
  }
  function openRotate(c: CredRow) {
    setPanel({ mode: "rotate", type: c.provider_type as CredType, label: "", token: "", targetId: c.id, targetLabel: c.label });
  }

  async function handleSave() {
    if (!panel) return;
    if (!panel.token.trim()) return;
    try {
      await createCred.mutateAsync({
        provider_type: panel.type,
        label: panel.mode === "add" ? panel.label : (panel.targetLabel ?? ""),
        token: panel.token,
      });
      setPanel(null);
    } catch { /* errors shown below */ }
  }

  async function handleDelete(id: string) {
    try {
      await deleteCred.mutateAsync(id);
      setConfirmDelete(null);
    } catch { /* ignore */ }
  }

  return (
    <div className="p-8 max-w-[840px]">
      <div className="flex items-start justify-between mb-1.5">
        <div className="flex items-center gap-2">
          <svg viewBox="0 0 16 16" width="18" height="18" fill="none" stroke="#94a3b8" strokeWidth="1.5">
            <rect x="3" y="7" width="10" height="7" rx="1.2" />
            <path d="M5.2 7V4.8a2.8 2.8 0 0 1 5.6 0V7" />
          </svg>
          <h1 className="text-[24px] font-bold tracking-tight m-0">Credentials</h1>
        </div>
        <button
          onClick={openAdd}
          className="h-9 rounded-md border-none bg-primary text-white text-[13px] font-medium px-3.5 cursor-pointer"
        >
          + Add credential
        </button>
      </div>
      <p className="text-[13px] text-[#94a3b8] mb-6">
        Workspace tokens are encrypted at rest with AES-256-GCM. Raw values are never shown again after saving.
      </p>

      {/* Add / Rotate panel */}
      {panel && (
        <div className="border border-primary/30 bg-[#1e293b] rounded-[10px] p-5 mb-5">
          <h3 className="text-[14px] font-semibold m-0 mb-3.5">
            {panel.mode === "rotate" ? `Rotate token — ${panel.targetLabel}` : "Add credential"}
          </h3>

          {panel.mode === "add" && (
            <>
              {/* Type picker */}
              <div className="flex gap-2 mb-3.5 flex-wrap">
                {CRED_TYPES.map((t) => {
                  const active = panel.type === t;
                  return (
                    <button
                      key={t}
                      onClick={() => setPanel((p) => p ? { ...p, type: t } : p)}
                      className="text-[12px] px-3 py-1.5 rounded-md cursor-pointer border transition-colors"
                      style={{
                        background: active ? "rgba(56,189,248,0.1)" : "transparent",
                        borderColor: active ? "rgba(56,189,248,0.3)" : "#334155",
                        color: active ? "var(--brand-primary, #38bdf8)" : "#94a3b8",
                      }}
                    >
                      {TYPE_LABELS[t]}
                    </button>
                  );
                })}
              </div>
              <div className="flex flex-col gap-1.5 mb-3.5">
                <label className="text-[12px] text-[#94a3b8]">Label</label>
                <input
                  type="text"
                  placeholder="e.g. GitLab service account"
                  value={panel.label}
                  onChange={(e) => setPanel((p) => p ? { ...p, label: e.target.value } : p)}
                  className="h-[38px] rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-3 text-[13px] font-[inherit] focus:outline-none"
                />
              </div>
            </>
          )}

          <div className="flex flex-col gap-1.5 mb-4">
            <label className="text-[12px] text-[#94a3b8]">Token</label>
            <input
              type="password"
              placeholder="Paste token"
              value={panel.token}
              onChange={(e) => setPanel((p) => p ? { ...p, token: e.target.value } : p)}
              className="h-[38px] rounded-md border border-[#334155] bg-[#0f172a] text-[#f8fafc] px-3 text-[13px] font-mono focus:outline-none"
            />
          </div>
          <div className="flex gap-2">
            <button
              onClick={handleSave}
              disabled={createCred.isPending}
              className="h-9 rounded-md border-none bg-primary text-white text-[13px] font-medium px-4 cursor-pointer disabled:opacity-50"
            >
              {createCred.isPending ? "Saving…" : "Save"}
            </button>
            <button
              onClick={() => setPanel(null)}
              className="h-9 rounded-md border border-[#334155] bg-transparent text-[#94a3b8] text-[13px] px-3.5 cursor-pointer"
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {/* Credentials list */}
      {isLoading ? (
        <div className="text-[13px] text-[#64748b]">Loading credentials…</div>
      ) : !creds?.length ? (
        <div className="border border-dashed border-[#334155] rounded-[10px] py-12 text-center">
          <p className="text-[14px] font-medium m-0 mb-1">No credentials yet</p>
          <p className="text-[12px] text-[#94a3b8] m-0">Add workspace tokens to enable provisioning.</p>
        </div>
      ) : (
        <div className="flex flex-col gap-2.5">
          {(creds as CredRow[]).map((c) => {
            const typeLabel = TYPE_LABELS[c.provider_type as CredType] ?? c.provider_type;
            const confirming = confirmDelete === c.id;
            const updated = new Date(c.created_at).toLocaleDateString();
            return (
              <div
                key={c.id}
                className="border border-[#334155] bg-[#1e293b] rounded-[10px] px-[18px] py-4 flex items-center justify-between gap-3"
              >
                <div className="flex items-center gap-3.5 min-w-0">
                  <span className="text-[11px] font-semibold px-2.5 py-[3px] rounded-md bg-[#334155] text-[#cbd5e1] shrink-0">
                    {typeLabel}
                  </span>
                  <div className="min-w-0">
                    <div className="text-[14px] font-medium">{c.label}</div>
                    <div className="text-[12px] text-[#64748b] font-mono mt-0.5">
                      •••••••••••••••• &nbsp;·&nbsp; updated {updated}
                    </div>
                  </div>
                </div>
                {confirming ? (
                  <div className="flex gap-2 shrink-0">
                    <button
                      onClick={() => handleDelete(c.id)}
                      className="text-[12px] text-[#f87171] bg-none border-none cursor-pointer"
                    >
                      Confirm delete
                    </button>
                    <button
                      onClick={() => setConfirmDelete(null)}
                      className="text-[12px] text-[#94a3b8] bg-none border-none cursor-pointer"
                    >
                      Cancel
                    </button>
                  </div>
                ) : (
                  <div className="flex gap-2 shrink-0">
                    <button
                      onClick={() => openRotate(c)}
                      className="text-[12px] text-[#cbd5e1] bg-transparent border border-[#334155] rounded-md px-3 py-1.5 cursor-pointer"
                    >
                      Rotate
                    </button>
                    <button
                      onClick={() => setConfirmDelete(c.id)}
                      className="text-[12px] text-[#94a3b8] bg-transparent border border-[#334155] rounded-md px-3 py-1.5 cursor-pointer hover:text-[#f87171] hover:border-red-500/40 transition-colors"
                    >
                      Delete
                    </button>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
