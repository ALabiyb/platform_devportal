// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState } from "react";
import { useCredentials, useCreateCredential, useDeleteCredential, Credential } from "@/lib/api";

const PROVIDER_COLOR: Record<string, string> = {
  jenkins:       "#f87171",
  harbor:        "#38bdf8",
  gitea:         "#4ade80",
  argocd:        "#c084fc",
  vault:         "#0ea5e9",
  defectdojo:    "#fbbf24",
  dependencytrack:"#94a3b8",
  github:        "#e2e8f0",
  gitlab:        "#fb923c",
  docker:        "#38bdf8",
};

function providerColor(type: string) {
  return PROVIDER_COLOR[type.toLowerCase().replace(/[^a-z]/g, "")] ?? "#64748b";
}

function relativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "Just now";
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return new Date(iso).toLocaleDateString();
}

// ── Add Credential Modal ──────────────────────────────────────────────────────
function AddCredentialModal({ onClose }: { onClose: () => void }) {
  const createM = useCreateCredential();
  const [providerType, setProviderType] = useState("jenkins");
  const [customType, setCustomType] = useState("");
  const [label, setLabel] = useState("");
  const [token, setToken] = useState("");
  const [showToken, setShowToken] = useState(false);
  const [error, setError] = useState("");

  const KNOWN_PROVIDERS = ["jenkins","harbor","gitea","argocd","vault","defectdojo","dependencytrack","github","gitlab","docker"];
  const isCustom = providerType === "__custom__";
  const finalType = isCustom ? customType.trim() : providerType;

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!finalType || !label.trim() || !token.trim()) return;
    setError("");
    try {
      await createM.mutateAsync({ provider_type: finalType, label: label.trim(), token });
      onClose();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to add credential.");
    }
  }

  return (
    <div style={{
      position: "fixed", inset: 0, zIndex: 1000, display: "flex", alignItems: "center", justifyContent: "center",
      background: "rgba(2,8,23,0.7)", backdropFilter: "blur(4px)",
    }} onClick={e => e.target === e.currentTarget && onClose()}>
      <div style={{ background: "var(--panel)", border: "1px solid var(--line)", borderRadius: 12, width: "100%", maxWidth: 440, padding: 28, boxShadow: "0 24px 48px rgba(0,0,0,0.6)" }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 22 }}>
          <h2 style={{ margin: 0, fontSize: 16, fontWeight: 700 }}>Add credential</h2>
          <button onClick={onClose} style={{ background: "none", border: "none", color: "var(--faint)", fontSize: 20, cursor: "pointer", padding: 0 }}>×</button>
        </div>
        <form onSubmit={handleSubmit} style={{ display: "flex", flexDirection: "column", gap: 14 }}>
          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            <label style={{ fontSize: 12.5, fontWeight: 600, color: "var(--muted)" }}>Provider</label>
            <select className="field" value={providerType} onChange={e => setProviderType(e.target.value)}>
              {KNOWN_PROVIDERS.map(p => <option key={p} value={p}>{p}</option>)}
              <option value="__custom__">Other…</option>
            </select>
          </div>
          {isCustom && (
            <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
              <label style={{ fontSize: 12.5, fontWeight: 600, color: "var(--muted)" }}>Provider type</label>
              <input className="field field-mono" placeholder="sonarqube" value={customType} onChange={e => setCustomType(e.target.value)} autoFocus />
            </div>
          )}
          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            <label style={{ fontSize: 12.5, fontWeight: 600, color: "var(--muted)" }}>Label</label>
            <input className="field" placeholder="Production Jenkins" value={label} onChange={e => setLabel(e.target.value)} autoFocus={!isCustom} />
            <span style={{ fontSize: 11, color: "var(--faint)" }}>Human-readable name for this credential set.</span>
          </div>
          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            <label style={{ fontSize: 12.5, fontWeight: 600, color: "var(--muted)" }}>Token / secret</label>
            <div style={{ display: "flex", gap: 8 }}>
              <input className="field field-mono" style={{ flex: 1 }} type={showToken ? "text" : "password"}
                placeholder="API key, robot password, or token" value={token} onChange={e => setToken(e.target.value)} />
              <button type="button" className="btn btn-secondary btn-sm" onClick={() => setShowToken(v => !v)}>
                {showToken ? "Hide" : "Show"}
              </button>
            </div>
            <span style={{ fontSize: 11, color: "var(--faint)" }}>Stored in Vault — never logged or returned in API responses.</span>
          </div>
          {error && <p style={{ margin: 0, fontSize: 12, color: "var(--bad)" }}>{error}</p>}
          <div style={{ display: "flex", gap: 10, justifyContent: "flex-end", marginTop: 6 }}>
            <button type="button" className="btn btn-ghost" onClick={onClose}>Cancel</button>
            <button type="submit" className="btn btn-primary" disabled={!finalType || !label.trim() || !token.trim() || createM.isPending}>
              {createM.isPending ? "Adding…" : "Add credential"}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}

// ── Credential card ───────────────────────────────────────────────────────────
function CredCard({ cred }: { cred: Credential }) {
  const [confirming, setConfirming] = useState(false);
  const deleteM = useDeleteCredential();
  const color = providerColor(cred.provider_type);

  function handleDelete() {
    if (!confirming) { setConfirming(true); return; }
    deleteM.mutate(cred.id, { onSettled: () => setConfirming(false) });
  }

  return (
    <div className="card" style={{ padding: 0 }}>
      <div style={{ padding: "16px 18px", borderBottom: "1px solid var(--line)", display: "flex", alignItems: "center", gap: 12 }}>
        <span style={{
          width: 32, height: 32, borderRadius: 8, flexShrink: 0,
          background: "var(--card)", border: "1px solid var(--line2)",
          display: "inline-flex", alignItems: "center", justifyContent: "center",
          color, fontSize: 13, fontWeight: 700,
        }}>{(cred.label?.[0] ?? cred.provider_type[0]).toUpperCase()}</span>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontSize: 14, fontWeight: 600, color: "var(--text)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{cred.label}</div>
          <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11.5, color, marginTop: 2 }}>{cred.provider_type}</div>
        </div>
        <span style={{
          padding: "2px 8px", borderRadius: 5, fontSize: 10.5, fontWeight: 600,
          background: "var(--ok-soft)", color: "var(--ok)", border: "1px solid var(--ok)",
        }}>Active</span>
      </div>

      <div style={{ padding: "14px 18px", display: "flex", flexDirection: "column", gap: 10 }}>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 14 }}>
          <div>
            <div className="overline" style={{ marginBottom: 4 }}>Provider</div>
            <div style={{ fontSize: 12.5, color: "var(--text)", fontFamily: "JetBrains Mono,monospace" }}>{cred.provider_type}</div>
          </div>
          <div>
            <div className="overline" style={{ marginBottom: 4 }}>Added</div>
            <div style={{ fontSize: 12.5, color: "var(--muted)" }}>{relativeTime(cred.created_at)}</div>
          </div>
        </div>
        <div>
          <div className="overline" style={{ marginBottom: 4 }}>Secret</div>
          <input
            readOnly value="••••••••••••••••••••"
            className="field field-mono"
            style={{ background: "var(--card)", border: "1px solid var(--line2)", cursor: "default" }}
          />
          <p style={{ fontSize: 11, color: "var(--faint)", margin: "6px 0 0" }}>
            Stored in Vault, never rendered server-side.
          </p>
        </div>
      </div>

      <div style={{ padding: "12px 18px", borderTop: "1px solid var(--line)", display: "flex", alignItems: "center", gap: 8 }}>
        <button
          className={`btn btn-sm ${confirming ? "btn-primary" : "btn-ghost"}`}
          style={confirming ? { background: "var(--bad)", borderColor: "var(--bad)" } : { color: "var(--bad)" }}
          onClick={handleDelete}
          disabled={deleteM.isPending}
        >
          {deleteM.isPending ? "Revoking…" : confirming ? "Confirm revoke" : "Revoke"}
        </button>
        {confirming && (
          <button className="btn btn-ghost btn-sm" onClick={() => setConfirming(false)}>Cancel</button>
        )}
      </div>
    </div>
  );
}

// ── Main page ─────────────────────────────────────────────────────────────────
export function CredentialsPage() {
  const { data: creds = [], isLoading } = useCredentials();
  const [showAdd, setShowAdd] = useState(false);

  return (
    <div style={{ padding: 28, maxWidth: 1320, display: "flex", flexDirection: "column", gap: 22 }}>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", flexWrap: "wrap", gap: 12 }}>
        <div>
          <h1 className="page-h1">Platform credentials</h1>
          <p className="page-sub">
            {isLoading ? "Loading…" : `${creds.length} integration${creds.length !== 1 ? "s" : ""}`} · secrets stored in Vault, never rendered server-side
          </p>
        </div>
        <button className="btn btn-primary" onClick={() => setShowAdd(true)}>+ Add credential</button>
      </div>

      {isLoading ? (
        <div style={{ padding: "48px 0", textAlign: "center", color: "var(--faint)", fontSize: 13 }}>Loading credentials…</div>
      ) : creds.length === 0 ? (
        <div style={{ border: "1px dashed var(--line2)", borderRadius: 10, padding: 48, textAlign: "center" }}>
          <div style={{ fontSize: 14, fontWeight: 600, color: "var(--text)", marginBottom: 6 }}>No credentials yet</div>
          <div style={{ fontSize: 13, color: "var(--muted)", marginBottom: 16 }}>Add a credential to connect Jenkins, Harbor, Gitea and other integrations.</div>
          <button className="btn btn-primary" onClick={() => setShowAdd(true)}>+ Add first credential</button>
        </div>
      ) : (
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(320px,1fr))", gap: 14 }}>
          {creds.map((c: Credential) => <CredCard key={c.id} cred={c} />)}
        </div>
      )}

      {showAdd && <AddCredentialModal onClose={() => setShowAdd(false)} />}
    </div>
  );
}
