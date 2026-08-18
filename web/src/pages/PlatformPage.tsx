// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState } from "react";
import {
  useClusters, useCreateCluster, useUpdateCluster,
  useClusterServices, useUpsertClusterService,
  useManifestTemplates, useUpsertManifestTemplate,
  useEnvironmentProfiles, useUpdateEnvironmentProfile,
  useLanguageProfiles, useUpsertLanguageProfile,
  type Cluster, type ManifestTemplate, type EnvironmentProfile, type LanguageProfile,
} from "@/lib/api";
import { ApiError } from "@/lib/queryClient";

// ── colour helpers ─────────────────────────────────────────────────────────────

const ENV_COLOUR: Record<string, string> = {
  dev:  "#22d3ee",
  uat:  "#fb923c",
  prod: "#4ade80",
};

const SVC_LABELS: Record<string, { label: string; icon: string }> = {
  cnpg:     { label: "CloudNativePG (Postgres)", icon: "🐘" },
  kafka:    { label: "Kafka",                    icon: "⚡" },
  minio:    { label: "MinIO (Object Storage)",   icon: "🪣" },
  redis:    { label: "Redis",                    icon: "🔴" },
  rabbitmq: { label: "RabbitMQ",                 icon: "🐇" },
  vault:    { label: "HashiCorp Vault",           icon: "🔐" },
  gateway:  { label: "Gateway API",              icon: "🌐" },
};

const ALL_SERVICE_TYPES = Object.keys(SVC_LABELS);


const CONDITIONAL_LABELS: Record<string, string> = {
  "":        "Always",
  cnpg:      "PostgreSQL only",
  kafka:     "Kafka only",
  minio:     "MinIO only",
  redis:     "Redis only",
  rabbitmq:  "RabbitMQ only",
  prod:      "Production only",
};

// ── shared sub-components ─────────────────────────────────────────────────────

function SectionHeader({ title, sub }: { title: string; sub: string }) {
  return (
    <div className="mb-6">
      <h1 className="text-[20px] font-bold text-[#f8fafc] m-0">{title}</h1>
      <p className="text-[13px] text-[#64748b] mt-1 m-0">{sub}</p>
    </div>
  );
}

function Badge({ label, color }: { label: string; color: string }) {
  return (
    <span
      className="inline-block px-2 py-0.5 rounded text-[11px] font-semibold uppercase tracking-wide"
      style={{ background: color + "22", color }}
    >
      {label}
    </span>
  );
}

function ErrorMsg({ msg }: { msg: string }) {
  return msg ? <p className="text-[12px] text-[#f87171] mt-1 m-0">{msg}</p> : null;
}

function SaveOk({ msg }: { msg: string }) {
  return msg ? <p className="text-[12px] text-[#4ade80] mt-1 m-0">{msg}</p> : null;
}

function FieldRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-1">
      <label className="text-[11px] font-semibold text-[#64748b] uppercase tracking-wide">{label}</label>
      {children}
    </div>
  );
}

function TextInput({
  value, onChange, placeholder, disabled,
}: {
  value: string; onChange: (v: string) => void; placeholder?: string; disabled?: boolean;
}) {
  return (
    <input
      value={value}
      onChange={(e) => onChange(e.target.value)}
      placeholder={placeholder}
      disabled={disabled}
      className="h-8 px-3 rounded-md bg-[#0f172a] border border-[#334155] text-[#e2e8f0] text-[13px] focus:outline-none focus:border-primary/60 disabled:opacity-40"
    />
  );
}

// ── TAB 1: Clusters ───────────────────────────────────────────────────────────

function ClusterCard({
  cluster,
  onEdit,
  onConfigure,
}: {
  cluster: Cluster;
  onEdit: (c: Cluster) => void;
  onConfigure: (c: Cluster) => void;
}) {
  const color = ENV_COLOUR[cluster.environment] ?? "#94a3b8";
  return (
    <div className="bg-[#1e293b] border border-[#334155] rounded-xl p-5 flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Badge label={cluster.environment} color={color} />
          <span className="text-[15px] font-bold text-[#f8fafc]">{cluster.display_name}</span>
        </div>
        <Badge
          label={cluster.status}
          color={cluster.status === "active" ? "#4ade80" : "#94a3b8"}
        />
      </div>
      <div className="grid grid-cols-2 gap-x-6 gap-y-1.5">
        <div>
          <p className="text-[10px] text-[#475569] font-semibold uppercase tracking-wide m-0">API Endpoint</p>
          <p className="text-[12px] text-[#cbd5e1] font-mono m-0 truncate">{cluster.api_endpoint}</p>
        </div>
        <div>
          <p className="text-[10px] text-[#475569] font-semibold uppercase tracking-wide m-0">ArgoCD</p>
          <p className="text-[12px] text-[#cbd5e1] font-mono m-0 truncate">{cluster.argocd_url || "—"}</p>
        </div>
        <div>
          <p className="text-[10px] text-[#475569] font-semibold uppercase tracking-wide m-0">Name</p>
          <p className="text-[12px] text-[#cbd5e1] m-0">{cluster.name}</p>
        </div>
        <div>
          <p className="text-[10px] text-[#475569] font-semibold uppercase tracking-wide m-0">Registered</p>
          <p className="text-[12px] text-[#cbd5e1] m-0">{new Date(cluster.created_at).toLocaleDateString()}</p>
        </div>
      </div>
      <div className="flex gap-2 pt-1">
        <button
          onClick={() => onConfigure(cluster)}
          className="flex-1 h-8 rounded-lg bg-primary/20 border border-primary/40 text-primary text-[12px] font-medium cursor-pointer hover:bg-primary/30 transition-colors"
        >
          Configure Services
        </button>
        <button
          onClick={() => onEdit(cluster)}
          className="px-4 h-8 rounded-lg bg-[#0f172a] border border-[#334155] text-[#94a3b8] text-[12px] cursor-pointer hover:bg-[#334155] hover:text-[#f8fafc] transition-colors"
        >
          Edit
        </button>
      </div>
    </div>
  );
}

function ClusterForm({
  initial,
  onClose,
}: {
  initial?: Cluster;
  onClose: () => void;
}) {
  const [name, setName] = useState(initial?.name ?? "");
  const [displayName, setDisplayName] = useState(initial?.display_name ?? "");
  const [environment, setEnvironment] = useState<string>(initial?.environment ?? "dev");
  const [apiEndpoint, setApiEndpoint] = useState(initial?.api_endpoint ?? "");
  const [argocdUrl, setArgocdUrl] = useState(initial?.argocd_url ?? "");
  const [status, setStatus] = useState(initial?.status ?? "active");
  const [err, setErr] = useState("");
  const [ok, setOk] = useState("");

  const create = useCreateCluster();
  const update = useUpdateCluster(initial?.id ?? "");

  async function save() {
    setErr(""); setOk("");
    const body = { name, display_name: displayName, environment: environment as Cluster["environment"], api_endpoint: apiEndpoint, argocd_url: argocdUrl, status };
    try {
      if (initial) {
        await update.mutateAsync(body);
      } else {
        await create.mutateAsync(body);
      }
      setOk("Saved.");
      setTimeout(onClose, 800);
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Save failed.");
    }
  }

  const isPending = create.isPending || update.isPending;

  return (
    <div className="bg-[#1e293b] border border-[#334155] rounded-xl p-6 flex flex-col gap-4 w-full max-w-lg">
      <div className="flex items-center justify-between">
        <h3 className="text-[15px] font-bold m-0">{initial ? "Edit Cluster" : "Register Cluster"}</h3>
        <button onClick={onClose} className="text-[#64748b] bg-transparent border-none cursor-pointer text-lg">✕</button>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <FieldRow label="Environment">
          {initial ? (
            <div className="h-8 px-3 flex items-center rounded-md bg-[#0f172a] border border-[#334155] text-[#64748b] text-[13px]">
              {environment}
            </div>
          ) : (
            <select
              value={environment}
              onChange={(e) => setEnvironment(e.target.value)}
              className="h-8 px-2 rounded-md bg-[#0f172a] border border-[#334155] text-[#e2e8f0] text-[13px] focus:outline-none"
            >
              <option value="dev">dev</option>
              <option value="uat">uat</option>
              <option value="prod">prod</option>
            </select>
          )}
        </FieldRow>
        <FieldRow label="Status">
          <select
            value={status}
            onChange={(e) => setStatus(e.target.value)}
            className="h-8 px-2 rounded-md bg-[#0f172a] border border-[#334155] text-[#e2e8f0] text-[13px] focus:outline-none"
          >
            <option value="active">active</option>
            <option value="inactive">inactive</option>
          </select>
        </FieldRow>
        <FieldRow label="Slug name">
          <TextInput value={name} onChange={setName} placeholder="dev-cluster" />
        </FieldRow>
        <FieldRow label="Display name">
          <TextInput value={displayName} onChange={setDisplayName} placeholder="Development Cluster" />
        </FieldRow>
      </div>
      <FieldRow label="K8s API endpoint">
        <TextInput value={apiEndpoint} onChange={setApiEndpoint} placeholder="https://192.168.200.10:6443" />
      </FieldRow>
      <FieldRow label="ArgoCD URL">
        <TextInput value={argocdUrl} onChange={setArgocdUrl} placeholder="https://argocd.dev.example.com" />
      </FieldRow>

      <ErrorMsg msg={err} />
      <SaveOk msg={ok} />

      <div className="flex gap-2 justify-end pt-1">
        <button
          onClick={onClose}
          className="h-8 px-4 rounded border border-[#334155] bg-transparent text-[#94a3b8] text-[12px] cursor-pointer"
        >
          Cancel
        </button>
        <button
          onClick={save}
          disabled={isPending}
          className="h-8 px-5 rounded bg-primary border-none text-white text-[12px] font-medium cursor-pointer disabled:opacity-50"
        >
          {isPending ? "Saving…" : "Save"}
        </button>
      </div>
    </div>
  );
}

// Service config field map — which text fields to show per service type
const SVC_FIELDS: Record<string, string[]> = {
  cnpg:     ["cluster_name", "namespace", "superuser_secret"],
  kafka:    ["brokers", "admin_secret_ref", "admin_secret_namespace"],
  minio:    ["endpoint", "admin_secret_ref", "admin_secret_namespace"],
  redis:    ["host", "port", "secret_ref", "secret_namespace"],
  rabbitmq: ["host", "port", "secret_ref", "secret_namespace"],
  vault:    ["addr", "mount", "auth_mount", "namespace"],
  gateway:  ["name", "namespace", "section_name", "tls_secret", "domain"],
};

function ServiceConfigPanel({
  cluster,
  onClose,
}: {
  cluster: Cluster;
  onClose: () => void;
}) {
  const { data: svcs, isLoading } = useClusterServices(cluster.id);
  const [selected, setSelected] = useState<string>(ALL_SERVICE_TYPES[0]);
  const [enabled, setEnabled] = useState(false);
  const [fields, setFields] = useState<Record<string, string>>({});
  const [err, setErr] = useState("");
  const [ok, setOk] = useState("");
  const upsert = useUpsertClusterService(cluster.id, selected);

  // When selected service changes or data loads, populate form from existing config
  function loadService(type: string) {
    setSelected(type);
    setErr(""); setOk("");
    const existing = (svcs ?? []).find((s) => s.service_type === type);
    if (existing) {
      setEnabled(existing.enabled);
      setFields(existing.config as Record<string, string>);
    } else {
      setEnabled(false);
      setFields({});
    }
  }

  async function save() {
    setErr(""); setOk("");
    try {
      await upsert.mutateAsync({ enabled, config: fields });
      setOk("Saved.");
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Save failed.");
    }
  }

  const fieldNames = SVC_FIELDS[selected] ?? [];
  const color = ENV_COLOUR[cluster.environment] ?? "#94a3b8";

  return (
    <div className="bg-[#1e293b] border border-[#334155] rounded-xl p-6 flex flex-col gap-4 w-full max-w-2xl">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Badge label={cluster.environment} color={color} />
          <h3 className="text-[15px] font-bold m-0">Platform Services — {cluster.display_name}</h3>
        </div>
        <button onClick={onClose} className="text-[#64748b] bg-transparent border-none cursor-pointer text-lg">✕</button>
      </div>

      {isLoading ? (
        <p className="text-[13px] text-[#64748b]">Loading…</p>
      ) : (
        <div className="flex gap-4 flex-1">
          {/* Service list */}
          <div className="w-48 flex flex-col gap-0.5">
            {ALL_SERVICE_TYPES.map((type) => {
              const existing = (svcs ?? []).find((s) => s.service_type === type);
              const { icon } = SVC_LABELS[type];
              return (
                <button
                  key={type}
                  onClick={() => loadService(type)}
                  className={`flex items-center gap-2 px-3 py-2 rounded-lg text-[12px] border-none cursor-pointer transition-colors text-left ${
                    selected === type
                      ? "bg-primary/20 text-primary"
                      : "bg-transparent text-[#94a3b8] hover:bg-[#0f172a] hover:text-[#f8fafc]"
                  }`}
                >
                  <span>{icon}</span>
                  <span className="flex-1 truncate">{type}</span>
                  {existing?.enabled && (
                    <span className="w-1.5 h-1.5 rounded-full bg-[#4ade80] shrink-0" />
                  )}
                </button>
              );
            })}
          </div>

          {/* Config form */}
          <div className="flex-1 flex flex-col gap-3">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-[14px] font-semibold m-0">
                  {SVC_LABELS[selected].icon} {SVC_LABELS[selected].label}
                </p>
                <p className="text-[11px] text-[#475569] m-0">
                  Configure the connection details for this service on {cluster.display_name}.
                </p>
              </div>
              {/* Toggle */}
              <button
                onClick={() => setEnabled((v) => !v)}
                className={`relative w-10 h-5 rounded-full border-none cursor-pointer transition-colors ${
                  enabled ? "bg-primary" : "bg-[#334155]"
                }`}
              >
                <span
                  className={`absolute top-0.5 w-4 h-4 rounded-full bg-white transition-all ${
                    enabled ? "left-5" : "left-0.5"
                  }`}
                />
              </button>
            </div>

            {fieldNames.map((field) => (
              <FieldRow key={field} label={field.replace(/_/g, " ")}>
                <TextInput
                  value={fields[field] ?? ""}
                  onChange={(v) => setFields((prev) => ({ ...prev, [field]: v }))}
                  placeholder={field}
                />
              </FieldRow>
            ))}

            <ErrorMsg msg={err} />
            <SaveOk msg={ok} />

            <div className="flex justify-end pt-1">
              <button
                onClick={save}
                disabled={upsert.isPending}
                className="h-8 px-5 rounded bg-primary border-none text-white text-[12px] font-medium cursor-pointer disabled:opacity-50"
              >
                {upsert.isPending ? "Saving…" : "Save service config"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function ClustersTab() {
  const { data: clusters, isLoading } = useClusters();
  const [showForm, setShowForm] = useState(false);
  const [editCluster, setEditCluster] = useState<Cluster | null>(null);
  const [configCluster, setConfigCluster] = useState<Cluster | null>(null);

  const envOrder: Array<"dev" | "uat" | "prod"> = ["dev", "uat", "prod"];
  const sorted = [...(clusters ?? [])].sort(
    (a, b) => envOrder.indexOf(a.environment) - envOrder.indexOf(b.environment),
  );
  const registeredEnvs = new Set(sorted.map((c) => c.environment));
  const missing = envOrder.filter((e) => !registeredEnvs.has(e));

  if (configCluster) {
    return (
      <div>
        <button
          onClick={() => setConfigCluster(null)}
          className="mb-4 text-[12px] text-[#64748b] bg-transparent border-none cursor-pointer hover:text-[#f8fafc] flex items-center gap-1"
        >
          ← Back to clusters
        </button>
        <ServiceConfigPanel cluster={configCluster} onClose={() => setConfigCluster(null)} />
      </div>
    );
  }

  if (editCluster || showForm) {
    return (
      <div>
        <button
          onClick={() => { setShowForm(false); setEditCluster(null); }}
          className="mb-4 text-[12px] text-[#64748b] bg-transparent border-none cursor-pointer hover:text-[#f8fafc] flex items-center gap-1"
        >
          ← Back to clusters
        </button>
        <ClusterForm
          initial={editCluster ?? undefined}
          onClose={() => { setShowForm(false); setEditCluster(null); }}
        />
      </div>
    );
  }

  return (
    <div>
      <div className="flex items-start justify-between mb-6">
        <SectionHeader
          title="Cluster Registry"
          sub="Register one Kubernetes cluster per environment. DevPortal reads these at provisioning time to know where to create ArgoCD Applications."
        />
        {missing.length > 0 && (
          <button
            onClick={() => setShowForm(true)}
            className="h-9 px-4 rounded-lg bg-primary border-none text-white text-[13px] font-medium cursor-pointer hover:opacity-90 shrink-0"
          >
            + Register Cluster
          </button>
        )}
      </div>

      {isLoading ? (
        <p className="text-[13px] text-[#64748b]">Loading clusters…</p>
      ) : (
        <>
          <div className="grid grid-cols-3 gap-4">
            {sorted.map((c) => (
              <ClusterCard
                key={c.id}
                cluster={c}
                onEdit={(cl) => { setEditCluster(cl); setShowForm(false); }}
                onConfigure={(cl) => setConfigCluster(cl)}
              />
            ))}
            {/* Empty slots for unregistered environments */}
            {missing.map((env) => (
              <div
                key={env}
                className="bg-[#0f172a] border border-dashed border-[#334155] rounded-xl p-5 flex flex-col items-center justify-center gap-2 min-h-[180px] cursor-pointer hover:border-primary/50 transition-colors"
                onClick={() => setShowForm(true)}
              >
                <Badge label={env} color={ENV_COLOUR[env]} />
                <p className="text-[13px] text-[#475569] m-0">No cluster registered</p>
                <p className="text-[12px] text-primary m-0">+ Register</p>
              </div>
            ))}
          </div>

          {missing.length === 0 && (
            <div className="mt-4">
              <button
                onClick={() => setShowForm(true)}
                className="text-[12px] text-[#64748b] bg-transparent border-none cursor-pointer hover:text-[#f8fafc]"
              >
                + Register additional cluster
              </button>
            </div>
          )}
        </>
      )}
    </div>
  );
}

// ── TAB 2: Manifest Templates ─────────────────────────────────────────────────

function ManifestTemplatesTab() {
  const { data: templates, isLoading } = useManifestTemplates();
  const [selected, setSelected] = useState<ManifestTemplate | null>(null);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState("");
  const [draftDisplayName, setDraftDisplayName] = useState("");
  const [draftConditional, setDraftConditional] = useState("");
  const [isNew, setIsNew] = useState(false);
  const [newName, setNewName] = useState("");
  const [err, setErr] = useState("");
  const [ok, setOk] = useState("");

  const upsert = useUpsertManifestTemplate(isNew ? newName : selected?.name ?? "");

  function openTemplate(t: ManifestTemplate) {
    setSelected(t); setEditing(false); setDraft(""); setIsNew(false);
    setErr(""); setOk("");
  }

  function startEdit() {
    setDraft(selected?.content ?? "");
    setDraftDisplayName(selected?.display_name ?? "");
    setDraftConditional(selected?.conditional ?? "");
    setEditing(true); setErr(""); setOk("");
  }

  function startNew() {
    setSelected(null); setEditing(true); setIsNew(true);
    setDraft(""); setDraftDisplayName(""); setDraftConditional("");
    setNewName(""); setErr(""); setOk("");
  }

  async function save() {
    setErr(""); setOk("");
    try {
      const saved = await upsert.mutateAsync({
        display_name: draftDisplayName || (isNew ? newName : selected?.display_name),
        conditional: draftConditional,
        content: draft,
      });
      setOk("Template saved.");
      setEditing(false); setIsNew(false);
      setSelected(saved);
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Save failed.");
    }
  }

  return (
    <div className="flex gap-6 h-[calc(100vh-220px)]">
      {/* Template list */}
      <aside className="w-52 flex-shrink-0 flex flex-col gap-1 overflow-y-auto">
        <p className="text-[11px] text-[#475569] font-semibold uppercase tracking-widest mb-2">Templates</p>
        {isLoading ? (
          <p className="text-[12px] text-[#64748b]">Loading…</p>
        ) : (
          (templates ?? []).map((t) => (
            <button
              key={t.name}
              onClick={() => openTemplate(t)}
              className={`text-left px-3 py-2 rounded-lg text-[12px] border-none cursor-pointer transition-colors ${
                selected?.name === t.name && !isNew
                  ? "bg-primary/20 text-primary"
                  : "bg-transparent text-[#94a3b8] hover:bg-[#1e293b] hover:text-[#f8fafc]"
              }`}
            >
              <span className="font-mono">{t.name}</span>
              {t.conditional && (
                <span className="ml-1 text-[10px] text-[#475569]">({t.conditional})</span>
              )}
            </button>
          ))
        )}
        <button
          onClick={startNew}
          className={`mt-1 text-left px-3 py-2 rounded-lg text-[12px] border-none cursor-pointer transition-colors ${
            isNew ? "bg-primary/20 text-primary" : "bg-transparent text-[#4ade80]/70 hover:bg-[#1e293b]"
          }`}
        >
          + New template
        </button>
      </aside>

      {/* Editor */}
      {(selected || isNew) ? (
        <div className="flex-1 flex flex-col min-w-0">
          <div className="flex items-center gap-4 mb-3">
            {isNew ? (
              <div className="flex items-center gap-2">
                <span className="text-[12px] text-[#64748b]">Name:</span>
                <input
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  placeholder="14-cnpg-database"
                  className="h-7 px-2 rounded bg-[#0f172a] border border-[#334155] text-[#e2e8f0] text-[12px] font-mono focus:outline-none focus:border-primary/60 w-44"
                />
              </div>
            ) : (
              <h2 className="text-[15px] font-bold m-0 font-mono">{selected?.name}</h2>
            )}

            {editing && (
              <div className="flex items-center gap-2">
                <span className="text-[11px] text-[#64748b]">Condition:</span>
                <select
                  value={draftConditional}
                  onChange={(e) => setDraftConditional(e.target.value)}
                  className="h-7 px-2 rounded bg-[#0f172a] border border-[#334155] text-[#e2e8f0] text-[12px] focus:outline-none"
                >
                  {Object.entries(CONDITIONAL_LABELS).map(([val, label]) => (
                    <option key={val} value={val}>{label}</option>
                  ))}
                </select>
              </div>
            )}

            {!isNew && !editing && selected?.conditional && (
              <span className="text-[11px] bg-[#0f172a] border border-[#334155] px-2 py-0.5 rounded text-[#64748b]">
                {CONDITIONAL_LABELS[selected.conditional] ?? selected.conditional}
              </span>
            )}

            <div className="ml-auto flex items-center gap-2">
              {!isNew && !editing && selected?.updated_at && (
                <span className="text-[11px] text-[#475569]">
                  Updated {new Date(selected.updated_at).toLocaleDateString()}
                </span>
              )}
              {editing ? (
                <>
                  <button
                    onClick={() => { setEditing(false); setIsNew(false); setErr(""); }}
                    className="h-8 px-3 rounded border border-[#334155] bg-transparent text-[#94a3b8] text-[12px] cursor-pointer"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={save}
                    disabled={upsert.isPending}
                    className="h-8 px-4 rounded bg-primary border-none text-white text-[12px] font-medium cursor-pointer disabled:opacity-50"
                  >
                    {upsert.isPending ? "Saving…" : "Save"}
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

          <ErrorMsg msg={err} />
          <SaveOk msg={ok} />

          <textarea
            readOnly={!editing}
            value={editing ? draft : (selected?.content ?? "")}
            onChange={(e) => setDraft(e.target.value)}
            spellCheck={false}
            placeholder={editing ? "# Paste your Kubernetes YAML template here\n# Use %%%APP_NAME%%%, %%%NAMESPACE%%%, %%%IMAGE%%%, etc." : ""}
            className="flex-1 font-mono text-[12px] text-[#e2e8f0] bg-[#0f172a] border border-[#334155] rounded-lg p-4 resize-none focus:outline-none focus:border-primary/60 mt-2"
            style={{ lineHeight: "1.6" }}
          />
        </div>
      ) : (
        <div className="flex-1 flex flex-col items-center justify-center gap-3 text-center">
          <p className="text-[13px] text-[#475569] m-0">Select a template to view or edit its YAML content.</p>
          <p className="text-[12px] text-[#334155] m-0 max-w-xs">
            Templates use <code className="bg-[#1e293b] px-1 rounded text-[#93c5fd]">%%%MARKER%%%</code> tokens that DevPortal replaces at provision time.
          </p>
        </div>
      )}
    </div>
  );
}

// ── TAB 3: Environment Profiles ───────────────────────────────────────────────

function ProfileCard({
  profile,
  onSave,
}: {
  profile: EnvironmentProfile;
  onSave: (p: EnvironmentProfile) => Promise<void>;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(profile);
  const [err, setErr] = useState("");
  const [ok, setOk] = useState("");
  const color = ENV_COLOUR[profile.name] ?? "#94a3b8";

  async function save() {
    setErr(""); setOk("");
    try {
      await onSave(draft);
      setOk("Saved."); setEditing(false);
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "Save failed.");
    }
  }

  const s = (field: keyof EnvironmentProfile) => (val: string) =>
    setDraft((prev) => ({ ...prev, [field]: val }));
  const n = (field: keyof EnvironmentProfile) => (val: string) =>
    setDraft((prev) => ({ ...prev, [field]: parseInt(val) || 0 }));

  return (
    <div className="bg-[#1e293b] border border-[#334155] rounded-xl p-5 flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <Badge label={profile.name} color={color} />
        {!editing ? (
          <button
            onClick={() => { setDraft(profile); setEditing(true); setErr(""); setOk(""); }}
            className="h-7 px-3 rounded bg-[#0f172a] border border-[#334155] text-[#94a3b8] text-[11px] cursor-pointer hover:bg-[#334155]"
          >
            Edit
          </button>
        ) : (
          <div className="flex gap-2">
            <button
              onClick={() => setEditing(false)}
              className="h-7 px-3 rounded border border-[#334155] bg-transparent text-[#94a3b8] text-[11px] cursor-pointer"
            >
              Cancel
            </button>
            <button
              onClick={save}
              className="h-7 px-3 rounded bg-primary border-none text-white text-[11px] cursor-pointer"
            >
              Save
            </button>
          </div>
        )}
      </div>

      <div className="grid grid-cols-2 gap-2 text-[12px]">
        {editing ? (
          <>
            <FieldRow label="CPU Request"><TextInput value={draft.cpu_request} onChange={s("cpu_request")} /></FieldRow>
            <FieldRow label="CPU Limit"><TextInput value={draft.cpu_limit} onChange={s("cpu_limit")} /></FieldRow>
            <FieldRow label="Mem Request"><TextInput value={draft.mem_request} onChange={s("mem_request")} /></FieldRow>
            <FieldRow label="Mem Limit"><TextInput value={draft.mem_limit} onChange={s("mem_limit")} /></FieldRow>
            <FieldRow label="Replicas"><TextInput value={String(draft.replicas)} onChange={n("replicas")} /></FieldRow>
            <FieldRow label="Storage Class"><TextInput value={draft.storage_class} onChange={s("storage_class")} /></FieldRow>
          </>
        ) : (
          <>
            <div>
              <p className="text-[10px] text-[#475569] font-semibold uppercase tracking-wide m-0">CPU</p>
              <p className="text-[#cbd5e1] font-mono m-0">{profile.cpu_request} → {profile.cpu_limit}</p>
            </div>
            <div>
              <p className="text-[10px] text-[#475569] font-semibold uppercase tracking-wide m-0">Memory</p>
              <p className="text-[#cbd5e1] font-mono m-0">{profile.mem_request} → {profile.mem_limit}</p>
            </div>
            <div>
              <p className="text-[10px] text-[#475569] font-semibold uppercase tracking-wide m-0">Replicas</p>
              <p className="text-[#cbd5e1] m-0">{profile.replicas}</p>
            </div>
            <div>
              <p className="text-[10px] text-[#475569] font-semibold uppercase tracking-wide m-0">Storage Class</p>
              <p className="text-[#cbd5e1] font-mono m-0">{profile.storage_class || "—"}</p>
            </div>
          </>
        )}
      </div>

      {/* HPA section */}
      <div className="border-t border-[#334155] pt-3">
        <div className="flex items-center justify-between mb-2">
          <p className="text-[11px] text-[#475569] font-semibold uppercase tracking-wide m-0">HPA</p>
          {editing ? (
            <button
              onClick={() => setDraft((p) => ({ ...p, hpa_enabled: !p.hpa_enabled }))}
              className={`relative w-9 h-4.5 rounded-full border-none cursor-pointer transition-colors ${
                draft.hpa_enabled ? "bg-primary" : "bg-[#334155]"
              }`}
              style={{ height: 18 }}
            >
              <span
                className={`absolute top-0.5 w-3.5 h-3.5 rounded-full bg-white transition-all ${
                  draft.hpa_enabled ? "left-[18px]" : "left-0.5"
                }`}
              />
            </button>
          ) : (
            <Badge
              label={profile.hpa_enabled ? "Enabled" : "Disabled"}
              color={profile.hpa_enabled ? "#4ade80" : "#64748b"}
            />
          )}
        </div>
        {(editing ? draft.hpa_enabled : profile.hpa_enabled) && (
          <div className="grid grid-cols-3 gap-2">
            {editing ? (
              <>
                <FieldRow label="Min"><TextInput value={String(draft.hpa_min)} onChange={n("hpa_min")} /></FieldRow>
                <FieldRow label="Max"><TextInput value={String(draft.hpa_max)} onChange={n("hpa_max")} /></FieldRow>
                <FieldRow label="CPU %"><TextInput value={String(draft.cpu_threshold)} onChange={n("cpu_threshold")} /></FieldRow>
              </>
            ) : (
              <>
                <div>
                  <p className="text-[10px] text-[#475569] m-0">Min/Max</p>
                  <p className="text-[#cbd5e1] m-0 font-mono">{profile.hpa_min} – {profile.hpa_max}</p>
                </div>
                <div>
                  <p className="text-[10px] text-[#475569] m-0">CPU target</p>
                  <p className="text-[#cbd5e1] m-0">{profile.cpu_threshold}%</p>
                </div>
                <div>
                  <p className="text-[10px] text-[#475569] m-0">Mem target</p>
                  <p className="text-[#cbd5e1] m-0">{profile.mem_threshold}%</p>
                </div>
              </>
            )}
          </div>
        )}
      </div>

      <ErrorMsg msg={err} />
      <SaveOk msg={ok} />
    </div>
  );
}

// ── Language Profiles Tab ─────────────────────────────────────────────────────

function LanguageProfileCard({ profile, onSave }: { profile: LanguageProfile; onSave: (p: LanguageProfile) => Promise<void> }) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState<LanguageProfile>(profile);
  const [envKey, setEnvKey] = useState("");
  const [envVal, setEnvVal] = useState("");
  const [err, setErr] = useState("");
  const [ok, setOk] = useState("");

  function startEdit() {
    setDraft({ ...profile });
    setEnvKey(""); setEnvVal(""); setErr(""); setOk("");
    setEditing(true);
  }

  async function save() {
    setErr(""); setOk("");
    try {
      await onSave(draft);
      setOk("Saved."); setEditing(false);
    } catch (e: any) {
      setErr(e?.message ?? "Save failed.");
    }
  }

  function addEnv() {
    if (!envKey.trim()) return;
    setDraft((d) => ({ ...d, extra_env: { ...d.extra_env, [envKey.trim()]: envVal } }));
    setEnvKey(""); setEnvVal("");
  }

  function removeEnv(k: string) {
    setDraft((d) => {
      const next = { ...d.extra_env };
      delete next[k];
      return { ...d, extra_env: next };
    });
  }

  const p = editing ? draft : profile;

  return (
    <div className="bg-[#1e293b] border border-[#334155] rounded-xl p-4 flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <span className="text-[13px] font-semibold text-[#f8fafc]">{profile.display_name}</span>
        <span className="text-[11px] font-mono text-[#475569] bg-[#0f172a] px-2 py-0.5 rounded">{profile.build_tool}</span>
      </div>

      {/* Probe delays */}
      <div className="grid grid-cols-2 gap-2 text-[12px]">
        <div>
          <p className="text-[10px] text-[#475569] font-semibold uppercase tracking-wide m-0">Liveness delay</p>
          {editing ? (
            <input
              type="number" value={draft.liveness_delay}
              onChange={(e) => setDraft((d) => ({ ...d, liveness_delay: Number(e.target.value) }))}
              className="w-full bg-[#0f172a] border border-[#334155] rounded px-2 py-1 text-[12px] text-[#f8fafc] font-mono"
            />
          ) : (
            <p className="text-[#cbd5e1] font-mono m-0">{p.liveness_delay}s</p>
          )}
        </div>
        <div>
          <p className="text-[10px] text-[#475569] font-semibold uppercase tracking-wide m-0">Readiness delay</p>
          {editing ? (
            <input
              type="number" value={draft.readiness_delay}
              onChange={(e) => setDraft((d) => ({ ...d, readiness_delay: Number(e.target.value) }))}
              className="w-full bg-[#0f172a] border border-[#334155] rounded px-2 py-1 text-[12px] text-[#f8fafc] font-mono"
            />
          ) : (
            <p className="text-[#cbd5e1] font-mono m-0">{p.readiness_delay}s</p>
          )}
        </div>
      </div>

      {/* Extra env vars */}
      <div>
        <p className="text-[10px] text-[#475569] font-semibold uppercase tracking-wide mb-1 m-0">Injected env vars</p>
        {Object.entries(p.extra_env ?? {}).length === 0 ? (
          <p className="text-[11px] text-[#475569] m-0">—</p>
        ) : (
          <div className="flex flex-col gap-1">
            {Object.entries(p.extra_env ?? {}).map(([k, v]) => (
              <div key={k} className="flex items-center gap-1 font-mono text-[11px]">
                <span className="text-[#93c5fd]">{k}</span>
                <span className="text-[#475569]">=</span>
                <span className="text-[#86efac] flex-1 truncate">{v}</span>
                {editing && (
                  <button onClick={() => removeEnv(k)}
                    className="text-[#f87171] text-[10px] border-none bg-transparent cursor-pointer px-1">✕</button>
                )}
              </div>
            ))}
          </div>
        )}
        {editing && (
          <div className="flex gap-1 mt-2">
            <input placeholder="KEY" value={envKey} onChange={(e) => setEnvKey(e.target.value)}
              className="flex-1 bg-[#0f172a] border border-[#334155] rounded px-2 py-1 text-[11px] text-[#f8fafc] font-mono" />
            <input placeholder="value" value={envVal} onChange={(e) => setEnvVal(e.target.value)}
              className="flex-1 bg-[#0f172a] border border-[#334155] rounded px-2 py-1 text-[11px] text-[#f8fafc] font-mono" />
            <button onClick={addEnv}
              className="px-2 py-1 bg-[#1e3a5f] border border-[#334155] rounded text-[#93c5fd] text-[11px] cursor-pointer">Add</button>
          </div>
        )}
      </div>

      <ErrorMsg msg={err} />
      <SaveOk msg={ok} />

      <div className="flex gap-2 mt-auto">
        {editing ? (
          <>
            <button onClick={() => setEditing(false)}
              className="flex-1 h-7 rounded border border-[#334155] bg-transparent text-[#94a3b8] text-[11px] cursor-pointer">Cancel</button>
            <button onClick={save}
              className="flex-1 h-7 rounded bg-primary border-none text-white text-[11px] cursor-pointer">Save</button>
          </>
        ) : (
          <button onClick={startEdit}
            className="w-full h-7 rounded bg-[#1e3a5f] border border-[#334155] text-[#93c5fd] text-[11px] cursor-pointer hover:bg-[#1e293b]">
            Edit
          </button>
        )}
      </div>
    </div>
  );
}

function LanguageProfilesTab() {
  const { data: profiles, isLoading } = useLanguageProfiles();

  const upsertHooks: Record<string, ReturnType<typeof useUpsertLanguageProfile>> = {
    maven: useUpsertLanguageProfile("maven"),
    gradle: useUpsertLanguageProfile("gradle"),
    go: useUpsertLanguageProfile("go"),
    "nodejs-express": useUpsertLanguageProfile("nodejs-express"),
    nextjs: useUpsertLanguageProfile("nextjs"),
    "python-fastapi": useUpsertLanguageProfile("python-fastapi"),
    dotnet: useUpsertLanguageProfile("dotnet"),
    "flutter-web": useUpsertLanguageProfile("flutter-web"),
    auto: useUpsertLanguageProfile("auto"),
  };

  async function handleSave(p: LanguageProfile) {
    await upsertHooks[p.build_tool]?.mutateAsync(p);
  }

  return (
    <div>
      <SectionHeader
        title="Language Profiles"
        sub="Probe timing and injected env vars per build tool. Baked into base/deployment.yaml at provision time — edit here, applies to all future services."
      />
      {isLoading ? (
        <p className="text-[13px] text-[#64748b]">Loading profiles…</p>
      ) : (
        <div className="grid grid-cols-3 gap-4">
          {(profiles ?? []).map((p) => (
            <LanguageProfileCard key={p.build_tool} profile={p} onSave={handleSave} />
          ))}
        </div>
      )}
    </div>
  );
}

function EnvironmentProfilesTab() {
  const { data: profiles, isLoading } = useEnvironmentProfiles();
  const devUpdate = useUpdateEnvironmentProfile("dev");
  const uatUpdate = useUpdateEnvironmentProfile("uat");
  const prodUpdate = useUpdateEnvironmentProfile("prod");

  const updaters: Record<string, ReturnType<typeof useUpdateEnvironmentProfile>> = {
    dev: devUpdate, uat: uatUpdate, prod: prodUpdate,
  };

  async function handleSave(p: EnvironmentProfile) {
    await updaters[p.name].mutateAsync(p);
  }

  return (
    <div>
      <SectionHeader
        title="Environment Profiles"
        sub="Resource limits, replica counts, and HPA settings per environment tier. All services inherit these automatically."
      />
      {isLoading ? (
        <p className="text-[13px] text-[#64748b]">Loading profiles…</p>
      ) : (
        <div className="grid grid-cols-3 gap-4">
          {(profiles ?? []).map((p) => (
            <ProfileCard key={p.name} profile={p} onSave={handleSave} />
          ))}
        </div>
      )}
    </div>
  );
}

// ── Root Page ─────────────────────────────────────────────────────────────────

type Tab = "clusters" | "manifest-templates" | "environment-profiles" | "language-profiles";

const TABS: { id: Tab; label: string; icon: string }[] = [
  { id: "clusters",             label: "Cluster Registry",       icon: "🏗️" },
  { id: "manifest-templates",   label: "Manifest Templates",     icon: "📄" },
  { id: "environment-profiles", label: "Environment Profiles",   icon: "⚖️" },
  { id: "language-profiles",    label: "Language Profiles",      icon: "🔧" },
];

export function PlatformPage() {
  const [tab, setTab] = useState<Tab>("clusters");

  return (
    <div className="p-8">
      {/* Page title */}
      <div className="mb-6">
        <h1 className="text-[22px] font-bold text-[#f8fafc] m-0">Platform Engineering</h1>
        <p className="text-[13px] text-[#64748b] mt-1 m-0">
          Configure the shared infrastructure that all services inherit automatically.
        </p>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 bg-[#0f172a] rounded-lg p-1 border border-[#334155] w-fit mb-8">
        {TABS.map((t) => (
          <button
            key={t.id}
            onClick={() => setTab(t.id)}
            className={`flex items-center gap-2 px-4 py-2 rounded-md text-[13px] font-medium border-none cursor-pointer transition-colors ${
              tab === t.id
                ? "bg-[#1e293b] text-[#f8fafc]"
                : "bg-transparent text-[#64748b] hover:text-[#94a3b8]"
            }`}
          >
            <span>{t.icon}</span>
            {t.label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      {tab === "clusters"             && <ClustersTab />}
      {tab === "manifest-templates"   && <ManifestTemplatesTab />}
      {tab === "environment-profiles" && <EnvironmentProfilesTab />}
      {tab === "language-profiles"    && <LanguageProfilesTab />}
    </div>
  );
}
