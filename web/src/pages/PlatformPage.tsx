// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import {
  useClusters, useCreateCluster, useEnvironmentProfiles, useUpdateEnvironmentProfile,
  useLanguageProfiles, useUpsertLanguageProfile,
  useManifestTemplates, useUpsertManifestTemplate,
  useClusterServices, useUpsertClusterService,
  Cluster, EnvironmentProfile, LanguageProfile, ManifestTemplate, ClusterPlatformService,
} from "@/lib/api";
import { Modal, FormField as Field, PageHeader, TableRowSkeleton } from "@/components/kit";

// ── Register Cluster Modal ─────────────────────────────────────────────────────
function RegisterClusterModal({ onClose }: { onClose: () => void }) {
  const createM = useCreateCluster();
  const [name, setName] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [env, setEnv] = useState("dev");
  const [endpoint, setEndpoint] = useState("");
  const [error, setError] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim() || !endpoint.trim()) return;
    setError("");
    try {
      await createM.mutateAsync({ name: name.trim(), display_name: displayName.trim() || name.trim(), environment: env as "dev" | "uat" | "prod", api_endpoint: endpoint.trim(), status: "active" });
      onClose();
    } catch { setError("Failed to register cluster."); }
  }

  return (
    <Modal title="Register cluster" onClose={onClose}>
      <form onSubmit={handleSubmit} style={{ display: "flex", flexDirection: "column", gap: 16 }}>
        <Field label="Cluster name (slug)">
          <input className="field field-mono" placeholder="eu-prod-01" value={name} onChange={e => setName(e.target.value)} autoFocus />
        </Field>
        <Field label="Display name">
          <input className="field" placeholder="EU Production 01" value={displayName} onChange={e => setDisplayName(e.target.value)} />
        </Field>
        <Field label="Environment">
          <select className="field" value={env} onChange={e => setEnv(e.target.value)}>
            <option value="dev">dev</option>
            <option value="uat">uat</option>
            <option value="prod">prod</option>
          </select>
        </Field>
        <Field label="API endpoint">
          <input className="field field-mono" placeholder="https://k8s.example.com:6443" value={endpoint} onChange={e => setEndpoint(e.target.value)} />
        </Field>
        {error && <p style={{ margin: 0, fontSize: 12, color: "var(--bad)" }}>{error}</p>}
        <div style={{ display: "flex", gap: 10, justifyContent: "flex-end", marginTop: 8 }}>
          <button type="button" className="btn btn-ghost" onClick={onClose}>Cancel</button>
          <button type="submit" className="btn btn-primary" disabled={!name.trim() || !endpoint.trim() || createM.isPending}>
            {createM.isPending ? "Registering…" : "Register cluster"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

// ── Language Profile Modal ────────────────────────────────────────────────────
function LangProfileModal({ profile, onClose }: { profile?: LanguageProfile; onClose: () => void }) {
  const isEdit = !!profile;
  const [buildTool, setBuildTool] = useState(profile?.build_tool ?? "");
  const [displayName, setDisplayName] = useState(profile?.display_name ?? "");
  const [liveness, setLiveness] = useState(String(profile?.liveness_delay ?? 30));
  const [readiness, setReadiness] = useState(String(profile?.readiness_delay ?? 10));
  const [extraEnvJson, setExtraEnvJson] = useState(
    profile?.extra_env ? JSON.stringify(profile.extra_env, null, 2) : ""
  );
  const [jsonError, setJsonError] = useState("");
  const [error, setError] = useState("");

  const upsert = useUpsertLanguageProfile(isEdit ? profile!.build_tool : buildTool);

  function validateJson(v: string) {
    if (!v.trim()) { setJsonError(""); return true; }
    try { const p = JSON.parse(v); if (typeof p !== "object" || Array.isArray(p)) throw new Error(); setJsonError(""); return true; }
    catch { setJsonError("Must be a JSON object { \"KEY\": \"value\" }"); return false; }
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!buildTool.trim()) return;
    if (!validateJson(extraEnvJson)) return;
    setError("");
    let extra_env: Record<string, string> = {};
    if (extraEnvJson.trim()) extra_env = JSON.parse(extraEnvJson);
    try {
      await upsert.mutateAsync({ display_name: displayName.trim() || buildTool, liveness_delay: Number(liveness), readiness_delay: Number(readiness), extra_env });
      onClose();
    } catch { setError("Failed to save language profile."); }
  }

  return (
    <Modal title={isEdit ? `Edit — ${profile!.build_tool}` : "New language profile"} onClose={onClose}>
      <form onSubmit={handleSubmit} style={{ display: "flex", flexDirection: "column", gap: 16 }}>
        {!isEdit && (
          <Field label="Build tool (slug)">
            <input className="field field-mono" placeholder="go" value={buildTool} onChange={e => setBuildTool(e.target.value)} autoFocus />
          </Field>
        )}
        <Field label="Display name">
          <input className="field" placeholder="Go" value={displayName} onChange={e => setDisplayName(e.target.value)} autoFocus={isEdit} />
        </Field>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
          <Field label="Liveness delay (s)">
            <input className="field field-mono" type="number" min={0} value={liveness} onChange={e => setLiveness(e.target.value)} />
          </Field>
          <Field label="Readiness delay (s)">
            <input className="field field-mono" type="number" min={0} value={readiness} onChange={e => setReadiness(e.target.value)} />
          </Field>
        </div>
        <Field label="Extra env vars (JSON)">
          <textarea className="field field-mono" rows={4} style={{ resize: "vertical", padding: "10px 12px", lineHeight: 1.5 }}
            placeholder={'{"JAVA_TOOL_OPTIONS": "-Xms256m"}'} value={extraEnvJson}
            onChange={e => { setExtraEnvJson(e.target.value); validateJson(e.target.value); }} />
          {jsonError && <span style={{ fontSize: 11.5, color: "var(--bad)" }}>{jsonError}</span>}
        </Field>
        {error && <p style={{ margin: 0, fontSize: 12, color: "var(--bad)" }}>{error}</p>}
        <div style={{ display: "flex", gap: 10, justifyContent: "flex-end", marginTop: 8 }}>
          <button type="button" className="btn btn-ghost" onClick={onClose}>Cancel</button>
          <button type="submit" className="btn btn-primary" disabled={!buildTool.trim() || upsert.isPending}>
            {upsert.isPending ? "Saving…" : "Save profile"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

// ── Manifest Template Modal ───────────────────────────────────────────────────
function ManifestTemplateModal({ template, onClose }: { template?: ManifestTemplate; onClose: () => void }) {
  const isEdit = !!template;
  const [name, setName] = useState(template?.name ?? "");
  const [displayName, setDisplayName] = useState(template?.display_name ?? "");
  const [conditional, setConditional] = useState(template?.conditional ?? "");
  const [content, setContent] = useState(template?.content ?? "");
  const [error, setError] = useState("");

  const upsert = useUpsertManifestTemplate(isEdit ? template!.name : name);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim() || !content.trim()) return;
    setError("");
    try {
      await upsert.mutateAsync({ display_name: displayName.trim() || name, conditional: conditional.trim(), content });
      onClose();
    } catch { setError("Failed to save template."); }
  }

  return (
    <Modal title={isEdit ? `Edit — ${template!.display_name || template!.name}` : "New manifest template"} onClose={onClose}>
      <form onSubmit={handleSubmit} style={{ display: "flex", flexDirection: "column", gap: 16 }}>
        {!isEdit && (
          <Field label="Name (slug)">
            <input className="field field-mono" placeholder="hpa" value={name} onChange={e => setName(e.target.value)} autoFocus />
          </Field>
        )}
        <Field label="Display name">
          <input className="field" placeholder="Horizontal Pod Autoscaler" value={displayName} onChange={e => setDisplayName(e.target.value)} autoFocus={isEdit} />
        </Field>
        <Field label="Conditional (optional)">
          <input className="field field-mono" placeholder=".hpa_enabled" value={conditional} onChange={e => setConditional(e.target.value)} />
        </Field>
        <Field label="Content (YAML / Go template)">
          <textarea className="field field-mono" rows={10} style={{ resize: "vertical", padding: "10px 12px", lineHeight: 1.5, fontSize: 12 }}
            placeholder={"apiVersion: autoscaling/v2\nkind: HorizontalPodAutoscaler\n..."}
            value={content} onChange={e => setContent(e.target.value)} />
        </Field>
        {error && <p style={{ margin: 0, fontSize: 12, color: "var(--bad)" }}>{error}</p>}
        <div style={{ display: "flex", gap: 10, justifyContent: "flex-end", marginTop: 8 }}>
          <button type="button" className="btn btn-ghost" onClick={onClose}>Cancel</button>
          <button type="submit" className="btn btn-primary" disabled={!name.trim() || !content.trim() || upsert.isPending}>
            {upsert.isPending ? "Saving…" : "Save template"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

// ── Env Profile Edit Modal ────────────────────────────────────────────────────
function EnvProfileEditModal({ profile, onClose }: { profile: EnvironmentProfile; onClose: () => void }) {
  const update = useUpdateEnvironmentProfile(profile.name);
  const [cpuReq, setCpuReq] = useState(profile.cpu_request);
  const [memReq, setMemReq] = useState(profile.mem_request);
  const [cpuLim, setCpuLim] = useState(profile.cpu_limit);
  const [memLim, setMemLim] = useState(profile.mem_limit);
  const [replicas, setReplicas] = useState(String(profile.replicas));
  const [storageClass, setStorageClass] = useState(profile.storage_class ?? "");
  const [hpaEnabled, setHpaEnabled] = useState(profile.hpa_enabled);
  const [hpaMin, setHpaMin] = useState(String(profile.hpa_min));
  const [hpaMax, setHpaMax] = useState(String(profile.hpa_max));
  const [cpuThreshold, setCpuThreshold] = useState(String(profile.cpu_threshold));
  const [memThreshold, setMemThreshold] = useState(String(profile.mem_threshold));
  const [error, setError] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    try {
      await update.mutateAsync({
        cpu_request: cpuReq.trim(), mem_request: memReq.trim(),
        cpu_limit: cpuLim.trim(), mem_limit: memLim.trim(),
        replicas: Number(replicas), storage_class: storageClass.trim(),
        hpa_enabled: hpaEnabled,
        hpa_min: Number(hpaMin), hpa_max: Number(hpaMax),
        cpu_threshold: Number(cpuThreshold), mem_threshold: Number(memThreshold),
      });
      onClose();
    } catch { setError("Failed to update profile."); }
  }

  const envBadgeColor = profile.name === "prod" ? "var(--bad)" : profile.name === "uat" ? "var(--warn)" : "var(--ok)";

  return (
    <Modal title={`Edit profile — ${profile.name}`} onClose={onClose}>
      <form onSubmit={handleSubmit} style={{ display: "flex", flexDirection: "column", gap: 14 }}>
        <div style={{ padding: "6px 10px", background: "var(--bg)", borderRadius: 6, fontSize: 12, color: "var(--muted)", borderLeft: `3px solid ${envBadgeColor}` }}>
          Resource requests are guaranteed capacity. Limits are the hard ceiling. HPA scales replicas automatically based on CPU/memory thresholds.
        </div>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
          <Field label="CPU request"><input className="field field-mono" placeholder="250m" value={cpuReq} onChange={e => setCpuReq(e.target.value)} /></Field>
          <Field label="CPU limit"><input className="field field-mono" placeholder="1000m" value={cpuLim} onChange={e => setCpuLim(e.target.value)} /></Field>
          <Field label="Mem request"><input className="field field-mono" placeholder="256Mi" value={memReq} onChange={e => setMemReq(e.target.value)} /></Field>
          <Field label="Mem limit"><input className="field field-mono" placeholder="1Gi" value={memLim} onChange={e => setMemLim(e.target.value)} /></Field>
        </div>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
          <Field label="Replicas (base)"><input className="field field-mono" type="number" min={1} value={replicas} onChange={e => setReplicas(e.target.value)} /></Field>
          <Field label="Storage class"><input className="field field-mono" placeholder="standard" value={storageClass} onChange={e => setStorageClass(e.target.value)} /></Field>
        </div>
        <Field label="HPA (autoscaling)">
          <label style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 13, cursor: "pointer" }}>
            <input type="checkbox" checked={hpaEnabled} onChange={e => setHpaEnabled(e.target.checked)} style={{ accentColor: "var(--accent)", width: 15, height: 15 }} />
            Enable Horizontal Pod Autoscaler
          </label>
        </Field>
        {hpaEnabled && (
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr 1fr", gap: 12 }}>
            <Field label="Min replicas"><input className="field field-mono" type="number" min={1} value={hpaMin} onChange={e => setHpaMin(e.target.value)} /></Field>
            <Field label="Max replicas"><input className="field field-mono" type="number" min={1} value={hpaMax} onChange={e => setHpaMax(e.target.value)} /></Field>
            <Field label="CPU threshold %"><input className="field field-mono" type="number" min={1} max={100} value={cpuThreshold} onChange={e => setCpuThreshold(e.target.value)} /></Field>
            <Field label="Mem threshold %"><input className="field field-mono" type="number" min={1} max={100} value={memThreshold} onChange={e => setMemThreshold(e.target.value)} /></Field>
          </div>
        )}
        {error && <p style={{ margin: 0, fontSize: 12, color: "var(--bad)" }}>{error}</p>}
        <div style={{ display: "flex", gap: 10, justifyContent: "flex-end", marginTop: 8 }}>
          <button type="button" className="btn btn-ghost" onClick={onClose}>Cancel</button>
          <button type="submit" className="btn btn-primary" disabled={update.isPending}>
            {update.isPending ? "Saving…" : "Save changes"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

// ── Clusters tab ──────────────────────────────────────────────────────────────
// ── Cluster platform services ──────────────────────────────────────────────────
const ALL_SERVICE_TYPES = ["cnpg", "kafka", "minio", "redis", "rabbitmq", "vault", "gateway"];

const SVC_LABELS: Record<string, string> = {
  cnpg:     "CloudNativePG (Postgres)",
  kafka:    "Kafka",
  minio:    "MinIO (Object Storage)",
  redis:    "Redis",
  rabbitmq: "RabbitMQ",
  vault:    "HashiCorp Vault",
  gateway:  "Gateway API",
};

// Which config fields to show per service type.
const SVC_FIELDS: Record<string, string[]> = {
  cnpg:     ["cluster_name", "namespace", "superuser_secret"],
  kafka:    ["brokers", "admin_secret_ref", "admin_secret_namespace"],
  minio:    ["endpoint", "admin_secret_ref", "admin_secret_namespace"],
  redis:    ["host", "port", "secret_ref", "secret_namespace"],
  rabbitmq: ["host", "port", "secret_ref", "secret_namespace"],
  vault:    ["addr", "mount", "auth_mount", "namespace"],
  gateway:  ["name", "namespace", "section_name", "tls_secret", "domain"],
};

function ClusterServicesModal({ cluster, onClose }: { cluster: Cluster; onClose: () => void }) {
  const { data: svcs = [], isLoading } = useClusterServices(cluster.id);
  const [selected, setSelected] = useState(ALL_SERVICE_TYPES[0]);
  const [enabled, setEnabled] = useState(false);
  const [fields, setFields] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  const upsert = useUpsertClusterService(cluster.id, selected);

  function selectService(type: string) {
    setSelected(type);
    setError("");
    setSaved(false);
    const existing = svcs.find((s: ClusterPlatformService) => s.service_type === type);
    setEnabled(existing?.enabled ?? false);
    setFields(existing?.config ?? {});
  }

  async function handleSave() {
    setError("");
    setSaved(false);
    try {
      await upsert.mutateAsync({ enabled, config: fields });
      setSaved(true);
    } catch {
      setError("Failed to save service config.");
    }
  }

  const fieldNames = SVC_FIELDS[selected] ?? [];

  return (
    <Modal title={`Platform services — ${cluster.display_name}`} onClose={onClose} maxWidth={680}>
      {isLoading ? (
        <p style={{ fontSize: 13, color: "var(--faint)" }}>Loading…</p>
      ) : (
        <div style={{ display: "flex", gap: 16 }}>
          <div style={{ width: 168, display: "flex", flexDirection: "column", gap: 2 }}>
            {ALL_SERVICE_TYPES.map((type) => {
              const existing = svcs.find((s: ClusterPlatformService) => s.service_type === type);
              const isSelected = selected === type;
              return (
                <button
                  key={type}
                  onClick={() => selectService(type)}
                  className="btn btn-ghost btn-sm"
                  style={{
                    justifyContent: "flex-start", gap: 8, textAlign: "left",
                    background: isSelected ? "var(--accent-soft)" : "transparent",
                    color: isSelected ? "var(--accent)" : "var(--muted)",
                  }}
                >
                  <span style={{ flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{type}</span>
                  {existing?.enabled && <span style={{ width: 6, height: 6, borderRadius: "50%", background: "var(--ok)", flexShrink: 0 }} />}
                </button>
              );
            })}
          </div>

          <div style={{ flex: 1, display: "flex", flexDirection: "column", gap: 12 }}>
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
              <div>
                <p style={{ margin: 0, fontSize: 14, fontWeight: 600, color: "var(--text)" }}>{SVC_LABELS[selected]}</p>
                <p style={{ margin: "2px 0 0", fontSize: 11.5, color: "var(--faint)" }}>
                  Connection details for {cluster.display_name}.
                </p>
              </div>
              <label style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 12.5, cursor: "pointer" }}>
                <input type="checkbox" checked={enabled} onChange={e => setEnabled(e.target.checked)} style={{ accentColor: "var(--accent)", width: 15, height: 15 }} />
                Enabled
              </label>
            </div>

            {fieldNames.map((field) => (
              <Field key={field} label={field.replace(/_/g, " ")}>
                <input
                  className="field field-mono"
                  value={fields[field] ?? ""}
                  onChange={e => setFields(prev => ({ ...prev, [field]: e.target.value }))}
                  placeholder={field}
                />
              </Field>
            ))}

            {error && <p style={{ margin: 0, fontSize: 12, color: "var(--bad)" }}>{error}</p>}
            {saved && <p style={{ margin: 0, fontSize: 12, color: "var(--ok)" }}>Saved.</p>}

            <div style={{ display: "flex", justifyContent: "flex-end", marginTop: 4 }}>
              <button className="btn btn-primary btn-sm" onClick={handleSave} disabled={upsert.isPending}>
                {upsert.isPending ? "Saving…" : "Save service config"}
              </button>
            </div>
          </div>
        </div>
      )}
    </Modal>
  );
}

function clusterStatusBadge(s: string) {
  if (s === "active" || s === "healthy") return <span className="badge badge-success">Healthy</span>;
  if (s === "degraded") return <span className="badge badge-warn">Degraded</span>;
  if (s === "standby")  return <span className="badge badge-pending">Standby</span>;
  return <span className="badge badge-pending">{s}</span>;
}

function ClustersTab({ onRegister }: { onRegister: () => void }) {
  const { data: clusters = [], isLoading } = useClusters();
  const [configCluster, setConfigCluster] = useState<Cluster | null>(null);
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      <div className="tbl-wrap">
        <div className="tbl-head" style={{ gridTemplateColumns: "minmax(0,1.3fr) 100px minmax(200px,1fr) 130px 110px" }}>
          <div>Cluster</div><div>Environment</div><div>API Endpoint</div><div>Status</div><div></div>
        </div>
        {isLoading ? (
          Array.from({ length: 3 }).map((_, i) => (
            <TableRowSkeleton key={i} gridTemplateColumns="minmax(0,1.3fr) 100px minmax(200px,1fr) 130px 110px" />
          ))
        ) : clusters.length === 0 ? (
          <div style={{ padding: "32px 18px", textAlign: "center" }}>
            <div style={{ fontSize: 13, color: "var(--faint)", marginBottom: 10 }}>No clusters registered.</div>
            <button className="btn btn-primary btn-sm" onClick={onRegister}>Register your first cluster</button>
          </div>
        ) : clusters.map((c: Cluster) => (
          <div key={c.id} className="tbl-row" style={{ gridTemplateColumns: "minmax(0,1.3fr) 100px minmax(200px,1fr) 130px 110px" }}>
            <div>
              <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12.5, fontWeight: 500, color: "var(--text)" }}>
                {c.display_name || c.name}
              </div>
              <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11, color: "var(--faint)", marginTop: 2 }}>{c.name}</div>
            </div>
            <div>
              <span style={{
                padding: "1px 7px", borderRadius: 5, fontSize: 10.5, fontWeight: 600,
                background: c.environment === "prod" ? "var(--bad-soft)" : c.environment === "uat" ? "var(--warn-soft)" : "var(--ok-soft)",
                color: c.environment === "prod" ? "var(--bad)" : c.environment === "uat" ? "var(--warn)" : "var(--ok)",
              }}>{c.environment}</span>
            </div>
            <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11, color: "var(--faint)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
              {c.api_endpoint}
            </div>
            <div>{clusterStatusBadge(c.status)}</div>
            <div>
              <button className="btn btn-secondary btn-sm" onClick={() => setConfigCluster(c)}>Configure</button>
            </div>
          </div>
        ))}
      </div>
      {configCluster && <ClusterServicesModal cluster={configCluster} onClose={() => setConfigCluster(null)} />}
    </div>
  );
}

// ── Environment Profiles tab ──────────────────────────────────────────────────
function EnvProfilesTab() {
  const { data: profiles = [], isLoading } = useEnvironmentProfiles();
  const [editProfile, setEditProfile] = useState<EnvironmentProfile | null>(null);

  return (
    <div>
      <p style={{ fontSize: 12.5, color: "var(--muted)", marginBottom: 14, lineHeight: 1.6 }}>
        Environment profiles set the CPU / memory and scaling defaults that services inherit at deploy time.
        There is one profile per environment (<strong>dev</strong>, <strong>uat</strong>, <strong>prod</strong>).
        Click <strong>Edit</strong> on a row to adjust resource limits or enable HPA.
      </p>
      <div className="tbl-wrap">
        <div className="tbl-head" style={{ gridTemplateColumns: "minmax(0,1fr) 100px 100px 100px 100px 120px 80px 80px" }}>
          <div>Profile</div><div>CPU req</div><div>CPU lim</div><div>Mem req</div><div>Mem lim</div><div>Replicas</div><div>HPA</div><div></div>
        </div>
        {isLoading ? (
          <div style={{ padding: "20px 18px", fontSize: 13, color: "var(--faint)" }}>Loading profiles…</div>
        ) : profiles.length === 0 ? (
          <div style={{ padding: "20px 18px", fontSize: 13, color: "var(--faint)" }}>No environment profiles configured.</div>
        ) : profiles.map((p: EnvironmentProfile) => (
          <div key={p.name} className="tbl-row" style={{ gridTemplateColumns: "minmax(0,1fr) 100px 100px 100px 100px 120px 80px 80px" }}>
            <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
              <span style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12.5, fontWeight: 500, color: "var(--text)" }}>{p.name}</span>
              <span style={{
                padding: "1px 7px", borderRadius: 5, fontSize: 10.5, fontWeight: 600,
                background: p.name === "prod" ? "var(--bad-soft)" : p.name === "uat" ? "var(--warn-soft)" : "var(--ok-soft)",
                color: p.name === "prod" ? "var(--bad)" : p.name === "uat" ? "var(--warn)" : "var(--ok)",
              }}>{p.name}</span>
            </div>
            {[p.cpu_request, p.cpu_limit, p.mem_request, p.mem_limit].map((v, i) => (
              <div key={i} style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12, color: "var(--muted)" }}>{v || "—"}</div>
            ))}
            <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12, color: "var(--muted)" }}>
              {p.hpa_enabled ? `${p.hpa_min}–${p.hpa_max}` : p.replicas}
            </div>
            <div style={{ fontSize: 12, color: "var(--muted)" }}>
              {p.hpa_enabled ? <span style={{ color: "var(--ok)", fontFamily: "JetBrains Mono,monospace", fontSize: 11.5 }}>{p.cpu_threshold}% cpu</span> : <span style={{ color: "var(--faint)" }}>off</span>}
            </div>
            <div>
              <button className="btn btn-secondary btn-sm" onClick={() => setEditProfile(p)}>Edit</button>
            </div>
          </div>
        ))}
      </div>
      {editProfile && <EnvProfileEditModal profile={editProfile} onClose={() => setEditProfile(null)} />}
    </div>
  );
}

// ── Language Profiles tab ─────────────────────────────────────────────────────
const LANG_COLOR: Record<string, string> = {
  maven: "#f87171", gradle: "#4ade80", go: "#38bdf8",
  node: "#fbbf24", npm: "#fbbf24", python: "#c084fc", pip: "#c084fc",
};

function LangProfilesTab({ onNew }: { onNew: () => void }) {
  const { data: profiles = [], isLoading } = useLanguageProfiles();
  const [editProfile, setEditProfile] = useState<LanguageProfile | null>(null);

  return (
    <>
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(320px,1fr))", gap: 14 }}>
        {isLoading && (
          <div style={{ gridColumn: "1/-1", padding: "48px 0", textAlign: "center", color: "var(--faint)", fontSize: 13 }}>
            Loading language profiles…
          </div>
        )}
        {profiles.length === 0 && !isLoading && (
          <div style={{ gridColumn: "1/-1", padding: "48px 0", textAlign: "center", color: "var(--faint)", fontSize: 13 }}>
            No language profiles configured yet.
            <br />
            <button className="btn btn-primary btn-sm" style={{ marginTop: 12 }} onClick={onNew}>Create first profile</button>
          </div>
        )}
        {profiles.map((p: LanguageProfile) => {
          const color = LANG_COLOR[p.build_tool] ?? "var(--faint)";
          const envEntries = Object.entries(p.extra_env ?? {});
          return (
            <div key={p.build_tool} className="card" style={{ padding: 0 }}>
              <div style={{ padding: "14px 16px", borderBottom: "1px solid var(--line)", display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                <div>
                  <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12, fontWeight: 600, color, marginBottom: 4 }}>{p.build_tool}</div>
                  <div style={{ fontSize: 14, fontWeight: 600, color: "var(--text)" }}>{p.display_name || p.build_tool}</div>
                </div>
                <button className="btn btn-secondary btn-sm" onClick={() => setEditProfile(p)}>Edit</button>
              </div>
              <div style={{ padding: "12px 16px", borderBottom: "1px solid var(--line)" }}>
                <div className="overline" style={{ marginBottom: 8 }}>Probe timing</div>
                <div style={{ display: "grid", gridTemplateColumns: "repeat(2,1fr)", gap: 8 }}>
                  {[["Liveness delay", `${p.liveness_delay}s`], ["Readiness delay", `${p.readiness_delay}s`]].map(([k, v]) => (
                    <div key={k}>
                      <div style={{ fontSize: 11, color: "var(--faint)", marginBottom: 3 }}>{k}</div>
                      <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 13, color: "var(--text)" }}>{v}</div>
                    </div>
                  ))}
                </div>
              </div>
              <div style={{ padding: "12px 16px", borderBottom: "1px solid var(--line)", minHeight: 52 }}>
                <div className="overline" style={{ marginBottom: 8 }}>Default environment</div>
                {envEntries.length === 0 ? (
                  <div style={{ fontSize: 12, color: "var(--faint)" }}>No extra env vars.</div>
                ) : envEntries.map(([k, v]) => (
                  <div key={k} style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11.5, lineHeight: "20px", display: "flex", gap: 8 }}>
                    <span style={{ color: "var(--accent)" }}>{k}</span>
                    <span style={{ color: "var(--line2)" }}>=</span>
                    <span style={{ color: "var(--muted)" }}>{v}</span>
                  </div>
                ))}
              </div>
              <div style={{ padding: "10px 16px", background: "var(--card)", display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11.5, color: "var(--faint)" }}>
                  {new Date(p.updated_at).toLocaleDateString()}
                </div>
                <Link to="/templates" style={{ fontSize: 12, color: "var(--accent)", textDecoration: "none" }}>Pipeline →</Link>
              </div>
            </div>
          );
        })}
      </div>
      {editProfile && <LangProfileModal profile={editProfile} onClose={() => setEditProfile(null)} />}
    </>
  );
}

// ── Manifest Templates tab ────────────────────────────────────────────────────
function ManifestTemplatesTab({ onNew }: { onNew: () => void }) {
  const { data: templates = [], isLoading } = useManifestTemplates();
  const [expandedName, setExpandedName] = useState<string | null>(null);
  const [editTemplate, setEditTemplate] = useState<ManifestTemplate | null>(null);

  return (
    <div>
      <div className="tbl-wrap">
        <div className="tbl-head" style={{ gridTemplateColumns: "minmax(0,1fr) minmax(150px,1fr) 100px 110px" }}>
          <div>Template</div><div>Conditional</div><div>Updated</div><div></div>
        </div>
        {isLoading ? (
          <div style={{ padding: "20px 18px", fontSize: 13, color: "var(--faint)" }}>Loading templates…</div>
        ) : templates.length === 0 ? (
          <div style={{ padding: "32px 18px", textAlign: "center" }}>
            <div style={{ fontSize: 13, color: "var(--faint)", marginBottom: 10 }}>No manifest templates yet.</div>
            <button className="btn btn-primary btn-sm" onClick={onNew}>Add first template</button>
          </div>
        ) : templates.map((t: ManifestTemplate) => (
          <div key={t.name} style={{ borderBottom: "1px solid var(--line)" }}>
            {/* Header row */}
            <div style={{
              display: "grid", gridTemplateColumns: "minmax(0,1fr) minmax(150px,1fr) 100px 110px",
              padding: "12px 18px", alignItems: "center", cursor: "pointer", transition: "background .1s",
            }}
              onClick={() => setExpandedName(expandedName === t.name ? null : t.name)}
              onMouseEnter={e => (e.currentTarget as HTMLElement).style.background = "var(--card)"}
              onMouseLeave={e => (e.currentTarget as HTMLElement).style.background = ""}
            >
              <div>
                <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                  <span style={{ fontSize: 11, color: "var(--faint)", userSelect: "none" }}>{expandedName === t.name ? "▾" : "▸"}</span>
                  <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12.5, fontWeight: 500, color: "var(--text)" }}>{t.display_name}</div>
                </div>
                <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11, color: "var(--faint)", marginTop: 2, paddingLeft: 16 }}>{t.name}</div>
              </div>
              <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11.5, color: "var(--faint)" }}>
                {t.conditional || <span style={{ color: "var(--faint)", fontStyle: "italic" }}>Always</span>}
              </div>
              <div style={{ fontSize: 12, color: "var(--faint)" }}>
                {new Date(t.updated_at).toLocaleDateString()}
              </div>
              <div onClick={e => e.stopPropagation()}>
                <button className="btn btn-secondary btn-sm" onClick={() => setEditTemplate(t)}>Edit</button>
              </div>
            </div>
            {/* Expanded content */}
            {expandedName === t.name && (
              <div style={{ background: "var(--bg)", borderTop: "1px solid var(--line2)", padding: "0 18px 14px" }}>
                <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "8px 0 6px" }}>
                  <span style={{ fontSize: 11, fontWeight: 600, color: "var(--faint)", textTransform: "uppercase", letterSpacing: "0.06em" }}>YAML / Go template</span>
                </div>
                <pre style={{
                  margin: 0, padding: "14px 16px", borderRadius: 8,
                  background: "var(--panel)", border: "1px solid var(--line)",
                  fontSize: 12, lineHeight: 1.6, fontFamily: "JetBrains Mono,monospace",
                  color: "var(--muted)", overflowX: "auto", maxHeight: 400, overflowY: "auto",
                  whiteSpace: "pre",
                }}>
                  {t.content || <span style={{ color: "var(--faint)", fontStyle: "italic" }}>No content</span>}
                </pre>
              </div>
            )}
          </div>
        ))}
      </div>
      {editTemplate && <ManifestTemplateModal template={editTemplate} onClose={() => setEditTemplate(null)} />}
    </div>
  );
}

// ── Main page ─────────────────────────────────────────────────────────────────
const TABS = ["Clusters", "Environment Profiles", "Language Profiles", "Manifest Templates"] as const;
type Tab = typeof TABS[number];

const TAB_SLUG: Record<Tab, string> = {
  "Clusters": "clusters",
  "Environment Profiles": "environment-profiles",
  "Language Profiles": "language-profiles",
  "Manifest Templates": "manifest-templates",
};
const SLUG_TAB: Record<string, Tab> = Object.fromEntries(
  Object.entries(TAB_SLUG).map(([t, slug]) => [slug, t as Tab])
);

export function PlatformPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const tab = SLUG_TAB[searchParams.get("tab") ?? ""] ?? "Clusters";
  const setTab = (t: Tab) => setSearchParams(t === "Clusters" ? {} : { tab: TAB_SLUG[t] }, { replace: true });
  const [showRegisterCluster, setShowRegisterCluster] = useState(false);
  const [showNewLangProfile, setShowNewLangProfile] = useState(false);
  const [showNewManifestTemplate, setShowNewManifestTemplate] = useState(false);

  function handleAction() {
    if (tab === "Clusters") setShowRegisterCluster(true);
    else if (tab === "Language Profiles") setShowNewLangProfile(true);
    else if (tab === "Manifest Templates") setShowNewManifestTemplate(true);
  }

  const ACTION_LABELS: Record<Tab, string | null> = {
    "Clusters":             "Register cluster",
    "Environment Profiles": null,
    "Language Profiles":    "New language profile",
    "Manifest Templates":   "New template",
  };

  return (
    <div style={{ padding: 28, maxWidth: 1320, display: "flex", flexDirection: "column", gap: 22 }}>
      <PageHeader
        title="Platform administration"
        subtitle="Defaults every team inherits. Changes apply to services provisioned from now on."
        actions={ACTION_LABELS[tab] && (
          <button className="btn btn-primary" onClick={handleAction}>{ACTION_LABELS[tab]}</button>
        )}
      />

      <div className="tab-rail">
        {TABS.map(t => (
          <button key={t} className={`tab-btn${tab === t ? " active" : ""}`} onClick={() => setTab(t)}>{t}</button>
        ))}
      </div>

      {tab === "Clusters"              && <ClustersTab onRegister={() => setShowRegisterCluster(true)} />}
      {tab === "Environment Profiles"  && <EnvProfilesTab />}
      {tab === "Language Profiles"     && <LangProfilesTab onNew={() => setShowNewLangProfile(true)} />}
      {tab === "Manifest Templates"    && <ManifestTemplatesTab onNew={() => setShowNewManifestTemplate(true)} />}

      {showRegisterCluster      && <RegisterClusterModal onClose={() => setShowRegisterCluster(false)} />}
      {showNewLangProfile       && <LangProfileModal onClose={() => setShowNewLangProfile(false)} />}
      {showNewManifestTemplate  && <ManifestTemplateModal onClose={() => setShowNewManifestTemplate(false)} />}
    </div>
  );
}
