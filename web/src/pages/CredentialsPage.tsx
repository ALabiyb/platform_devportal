// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState } from "react";
import { useCredentials, useCreateCredential, useDeleteCredential, Credential } from "@/lib/api";
import { PageHeader, Modal, FormField, Button, ConfirmDialog, Skeleton } from "@/components/kit";
import { useToast } from "@/components/toast";

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
  return PROVIDER_COLOR[type.toLowerCase().replace(/[^a-z]/g, "")] ?? "var(--faint)";
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
  const { show } = useToast();

  const KNOWN_PROVIDERS = ["jenkins","harbor","gitea","argocd","vault","defectdojo","dependencytrack","github","gitlab","docker"];
  const isCustom = providerType === "__custom__";
  const finalType = isCustom ? customType.trim() : providerType;

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!finalType || !label.trim() || !token.trim()) return;
    setError("");
    try {
      await createM.mutateAsync({ provider_type: finalType, label: label.trim(), token });
      show({ tone: "ok", message: `${label.trim()} added.` });
      onClose();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to add credential.");
    }
  }

  return (
    <Modal title="Add credential" onClose={onClose} maxWidth={440}>
      <form onSubmit={handleSubmit} style={{ display: "flex", flexDirection: "column", gap: 14 }}>
        <FormField label="Provider">
          <select className="field" value={providerType} onChange={e => setProviderType(e.target.value)}>
            {KNOWN_PROVIDERS.map(p => <option key={p} value={p}>{p}</option>)}
            <option value="__custom__">Other…</option>
          </select>
        </FormField>
        {isCustom && (
          <FormField label="Provider type">
            <input className="field field-mono" placeholder="sonarqube" value={customType} onChange={e => setCustomType(e.target.value)} autoFocus />
          </FormField>
        )}
        <FormField label="Label" hint="Human-readable name for this credential set.">
          <input className="field" placeholder="Production Jenkins" value={label} onChange={e => setLabel(e.target.value)} autoFocus={!isCustom} />
        </FormField>
        <FormField label="Token / secret" hint="Stored in Vault — never logged or returned in API responses.">
          <div style={{ display: "flex", gap: 8 }}>
            <input className="field field-mono" style={{ flex: 1 }} type={showToken ? "text" : "password"}
              placeholder="API key, robot password, or token" value={token} onChange={e => setToken(e.target.value)} />
            <Button type="button" variant="secondary" size="sm" onClick={() => setShowToken(v => !v)}>
              {showToken ? "Hide" : "Show"}
            </Button>
          </div>
        </FormField>
        {error && <p style={{ margin: 0, fontSize: 12, color: "var(--bad)" }}>{error}</p>}
        <div style={{ display: "flex", gap: 10, justifyContent: "flex-end", marginTop: 6 }}>
          <Button type="button" variant="ghost" onClick={onClose}>Cancel</Button>
          <Button type="submit" loading={createM.isPending} disabled={!finalType || !label.trim() || !token.trim()}>
            Add credential
          </Button>
        </div>
      </form>
    </Modal>
  );
}

// ── Credential card ───────────────────────────────────────────────────────────
function CredCard({ cred }: { cred: Credential }) {
  const [confirming, setConfirming] = useState(false);
  const [error, setError] = useState("");
  const deleteM = useDeleteCredential();
  const color = providerColor(cred.provider_type);
  const { show } = useToast();

  async function handleDelete() {
    setError("");
    try {
      await deleteM.mutateAsync(cred.id);
      show({ tone: "ok", message: `${cred.label} revoked.` });
      setConfirming(false);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Failed to revoke credential.");
    }
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
        <Button variant="ghost" size="sm" style={{ color: "var(--bad)" }} onClick={() => setConfirming(true)}>
          Revoke
        </Button>
      </div>
      {confirming && (
        <ConfirmDialog
          title="Revoke credential?"
          message={<>Revoke <strong className="text-[var(--text)]">{cred.label}</strong>? Anything using it will start failing immediately.</>}
          confirmLabel="Revoke"
          danger
          isPending={deleteM.isPending}
          error={error}
          onConfirm={handleDelete}
          onCancel={() => { setConfirming(false); setError(""); }}
        />
      )}
    </div>
  );
}

// ── Main page ─────────────────────────────────────────────────────────────────
export function CredentialsPage() {
  const { data: creds = [], isLoading } = useCredentials();
  const [showAdd, setShowAdd] = useState(false);

  return (
    <div style={{ padding: 28, maxWidth: 1320, display: "flex", flexDirection: "column", gap: 22 }}>
      <PageHeader
        title="Platform credentials"
        subtitle={`${isLoading ? "Loading…" : `${creds.length} integration${creds.length !== 1 ? "s" : ""}`} · secrets stored in Vault, never rendered server-side`}
        actions={<button className="btn btn-primary" onClick={() => setShowAdd(true)}>+ Add credential</button>}
      />

      {isLoading ? (
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(320px,1fr))", gap: 14 }}>
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="card" style={{ padding: 0 }}>
              <div style={{ padding: "16px 18px", borderBottom: "1px solid var(--line)", display: "flex", alignItems: "center", gap: 12 }}>
                <Skeleton width={32} height={32} radius={8} />
                <div style={{ flex: 1 }}><Skeleton width="50%" height={14} /></div>
              </div>
              <div style={{ padding: "14px 18px" }}><Skeleton height={36} /></div>
            </div>
          ))}
        </div>
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
