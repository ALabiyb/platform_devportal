// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState, useEffect } from "react";
import { useNavigate, useParams, Link } from "react-router-dom";
import { useTheme } from "@/components/Layout";
import { LANG_COLOR } from "@/lib/langColor";
import { FormField, Button } from "@/components/kit";
import {
  useApplications,
  useApplicationServices,
  useUsers,
  useCreateService,
  Application,
  Project,
  User,
} from "@/lib/api";

// ── Types ──────────────────────────────────────────────────────────────────────
interface WizardState {
  name: string; appId: string; assigneeId: string; description: string;
  serviceKind: string;
  buildTool: string; port: string; liveness: string; readiness: string; tier: string;
  infra: string[]; deps: string[]; // deps stores service IDs
}

const SERVICE_KINDS = [
  { id: "backend",  label: "Backend",  desc: "HTTP/gRPC API, worker with a health endpoint" },
  { id: "frontend", label: "Frontend", desc: "Static SPA served behind nginx" },
  { id: "worker",   label: "Worker",   desc: "Background/queue consumer, no inbound traffic" },
] as const;

const BUILD_TOOLS = [
  { id: "maven",  lang: "Java 21",       subtitle: "Spring Boot",       image: "temurin:21-jre",     cmd: "mvn -B package",  color: LANG_COLOR.maven },
  { id: "gradle", lang: "Kotlin 2.0",    subtitle: "Ktor",              image: "temurin:21-jre",     cmd: "./gradlew build", color: LANG_COLOR.gradle },
  { id: "go",     lang: "Go 1.23",       subtitle: "net/http",          image: "distroless:static",  cmd: "go build ./...",  color: LANG_COLOR.go },
  { id: "node",   lang: "Node 22",       subtitle: "Fastify",           image: "node:22-alpine",     cmd: "pnpm build",      color: LANG_COLOR.node },
  { id: "python", lang: "Python 3.12",   subtitle: "FastAPI",           image: "python:3.12-slim",   cmd: "uv sync",         color: LANG_COLOR.python },
];

const INFRA_OPTIONS = [
  { id: "postgres",  name: "PostgreSQL",  op: "CloudNativePG",        blurb: "Managed Postgres cluster with automatic failover" },
  { id: "kafka",     name: "Kafka",       op: "Strimzi",              blurb: "Topic set from the language profile defaults" },
  { id: "redis",     name: "Redis",       op: "Redis Operator",       blurb: "Single-node for caching and ephemeral state" },
  { id: "rabbitmq",  name: "RabbitMQ",    op: "RabbitMQ Cluster",     blurb: "Durable message queues and exchange routing" },
  { id: "minio",     name: "MinIO",       op: "MinIO Operator",       blurb: "S3-compatible object storage for large payloads" },
];

const TIERS = [
  { id: "small",  label: "Small",  spec: "250m / 512Mi · 2 pods",  desc: "Batch jobs, internal tools" },
  { id: "medium", label: "Medium", spec: "500m / 1Gi · 3 pods",    desc: "Default for tier-2 services" },
  { id: "large",  label: "Large",  spec: "1000m / 2Gi · 5 pods",   desc: "Tier-1, customer-facing" },
];

// ── Shared: a card section on the form ─────────────────────────────────────────
function FormSection({ title, description, children }: { title: string; description: string; children: React.ReactNode }) {
  return (
    <div className="card" style={{ display: "flex", flexDirection: "column", gap: 20 }}>
      <div>
        <h3 style={{ margin: "0 0 4px", fontSize: 15, fontWeight: 600, color: "var(--text)" }}>{title}</h3>
        <p style={{ margin: 0, fontSize: 12.5, color: "var(--faint)", lineHeight: 1.5, maxWidth: "60ch" }}>{description}</p>
      </div>
      {children}
    </div>
  );
}

// Keyboard-operable selection card — role="radio" for single-select groups,
// role="checkbox" for multi-select. Plain onClick divs weren't reachable by
// keyboard at all before this.
function SelCard({ selected, onSelect, role, children }: {
  selected: boolean; onSelect: () => void; role: "radio" | "checkbox"; children: React.ReactNode;
}) {
  return (
    <div
      className={`sel-card${selected ? " selected" : ""}`}
      role={role}
      aria-checked={selected}
      tabIndex={0}
      onClick={onSelect}
      onKeyDown={e => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); onSelect(); } }}
    >
      {children}
    </div>
  );
}

// ── Identity ────────────────────────────────────────────────────────────────────
function SectionIdentity({ state, onChange, apps, users }: {
  state: WizardState;
  onChange: (p: Partial<WizardState>) => void;
  apps: Application[];
  users: User[];
}) {
  const selectedApp = apps.find((a: Application) => a.id === state.appId);

  return (
    <FormSection title="Identity" description="Choose a unique slug that becomes the repo name, image tag and ArgoCD app name.">
      <FormField label="Service name" hint="lowercase, hyphens, ≤ 40 chars">
        <input className="field field-lg field-mono" placeholder="ledger-reconciler" value={state.name}
          onChange={e => onChange({ name: e.target.value })} style={{ maxWidth: 360 }} autoFocus />
      </FormField>

      <div>
        <label style={{ display: "block", fontSize: 12.5, fontWeight: 600, color: "var(--muted)", marginBottom: 8 }}>Service kind</label>
        <div className="segmented" style={{ width: "fit-content" }}>
          {SERVICE_KINDS.map(k => (
            <button key={k.id} type="button" className={state.serviceKind === k.id ? "active" : ""} onClick={() => onChange({ serviceKind: k.id })} title={k.desc}>
              {k.label}
            </button>
          ))}
        </div>
        <div style={{ fontSize: 11.5, color: "var(--faint)", marginTop: 6 }}>
          {SERVICE_KINDS.find(k => k.id === state.serviceKind)?.desc}
        </div>
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16, maxWidth: 480 }}>
        <FormField label="Application">
          <select className="field" value={state.appId} onChange={e => onChange({ appId: e.target.value })}>
            <option value="">Select application…</option>
            {apps.map((a: Application) => (
              <option key={a.id} value={a.id}>{a.name}</option>
            ))}
          </select>
        </FormField>
        <FormField label="Assignee">
          <select className="field" value={state.assigneeId} onChange={e => onChange({ assigneeId: e.target.value })}>
            <option value="">Select assignee…</option>
            {users.map((u: User) => (
              <option key={u.id} value={u.id}>{u.display_name || u.email}</option>
            ))}
          </select>
        </FormField>
      </div>

      <FormField label="Description">
        <textarea className="field" rows={3}
          placeholder="One line a stranger on call would understand."
          value={state.description} onChange={e => onChange({ description: e.target.value })}
          style={{ padding: "11px 13px", lineHeight: 1.5, resize: "vertical", height: "auto", maxWidth: 480 }} />
      </FormField>

      {state.name && selectedApp && (
        <div style={{ background: "var(--bg)", borderRadius: 8, padding: "12px 14px", borderLeft: "3px solid var(--accent)", maxWidth: 480 }}>
          <div style={{ fontSize: 12, color: "var(--faint)" }}>Repository will be created at</div>
          <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12.5, color: "var(--accent)", marginTop: 4 }}>
            git.nexbridge.io/{selectedApp.git_namespace || selectedApp.slug}/{state.name}.git
          </div>
        </div>
      )}
    </FormSection>
  );
}

// ── Build ───────────────────────────────────────────────────────────────────────
function SectionBuild({ state, onChange }: { state: WizardState; onChange: (p: Partial<WizardState>) => void }) {
  return (
    <FormSection title="Build" description="The language profile sets the Jenkinsfile template, Dockerfile base image and probe timings.">
      <div role="radiogroup" aria-label="Build tool" style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(228px,1fr))", gap: 12 }}>
        {BUILD_TOOLS.map(t => {
          const selected = state.buildTool === t.id;
          return (
            <SelCard key={t.id} role="radio" selected={selected} onSelect={() => onChange({ buildTool: t.id })}>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start" }}>
                <span style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11, fontWeight: 600, color: t.color }}>{t.id}</span>
                <span style={{ width: 14, height: 14, borderRadius: "50%", border: `1.5px solid ${selected ? "var(--accent)" : "var(--line2)"}`, background: selected ? "var(--accent)" : "transparent", flexShrink: 0, display: "flex", alignItems: "center", justifyContent: "center" }}>
                  {selected && <span style={{ width: 6, height: 6, borderRadius: "50%", background: "var(--accent-ink)" }} />}
                </span>
              </div>
              <div style={{ fontSize: 14, fontWeight: 600, color: "var(--text)" }}>{t.lang}</div>
              <div style={{ fontSize: 12, color: "var(--muted)" }}>{t.subtitle}</div>
              <div style={{ display: "flex", gap: 6, marginTop: 4, flexWrap: "wrap" }}>
                {[t.image, t.cmd].map(v => (
                  <span key={v} style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11.5, padding: "2px 8px", borderRadius: 6, background: selected ? "var(--accent-soft)" : "var(--card)", border: `1px solid ${selected ? "var(--accent)" : "var(--line2)"}`, color: selected ? "var(--accent)" : "var(--muted)" }}>{v}</span>
                ))}
              </div>
            </SelCard>
          );
        })}
      </div>
    </FormSection>
  );
}

// ── Runtime ─────────────────────────────────────────────────────────────────────
function SectionRuntime({ state, onChange }: { state: WizardState; onChange: (p: Partial<WizardState>) => void }) {
  return (
    <FormSection title="Runtime" description="These values go into the Kustomize base and the liveness/readiness probes.">
      <div style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 14, maxWidth: 560 }}>
        {[["Container port","port","8080"],["Liveness path","liveness","/actuator/health/liveness"],["Readiness path","readiness","/actuator/health/readiness"]].map(([label, key, placeholder]) => (
          <FormField key={key} label={label!}>
            <input className="field field-mono" placeholder={placeholder}
              value={(state as unknown as Record<string, string>)[key!]}
              onChange={e => onChange({ [key as keyof WizardState]: e.target.value } as Partial<WizardState>)} />
          </FormField>
        ))}
      </div>
      <div>
        <label style={{ display: "block", fontSize: 12.5, fontWeight: 600, color: "var(--muted)", marginBottom: 10 }}>Resource tier</label>
        <div role="radiogroup" aria-label="Resource tier" style={{ display: "grid", gridTemplateColumns: "repeat(3,1fr)", gap: 12, maxWidth: 560 }}>
          {TIERS.map(t => {
            const selected = state.tier === t.id;
            return (
              <SelCard key={t.id} role="radio" selected={selected} onSelect={() => onChange({ tier: t.id })}>
                <div style={{ display: "flex", justifyContent: "space-between" }}>
                  <span style={{ fontSize: 13, fontWeight: 600, color: "var(--text)" }}>{t.label}</span>
                  <span style={{ width: 14, height: 14, borderRadius: "50%", border: `1.5px solid ${selected ? "var(--accent)" : "var(--line2)"}`, background: selected ? "var(--accent)" : "transparent", flexShrink: 0 }} />
                </div>
                <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11.5, color: selected ? "var(--accent)" : "var(--muted)" }}>{t.spec}</div>
                <div style={{ fontSize: 12, color: "var(--faint)" }}>{t.desc}</div>
              </SelCard>
            );
          })}
        </div>
      </div>
      <div style={{ background: "var(--bg)", borderRadius: 8, padding: "12px 14px", maxWidth: 480, fontSize: 12, color: "var(--faint)", lineHeight: 1.6 }}>
        Probe timings inherited from language profile: Initial delay 30s · period 10s · failure threshold 3.{" "}
        <Link to="/platform" style={{ color: "var(--accent)", textDecoration: "none" }}>Override →</Link>
      </div>
    </FormSection>
  );
}

// ── Infrastructure ────────────────────────────────────────────────────────────
function SectionInfra({ state, onChange }: { state: WizardState; onChange: (p: Partial<WizardState>) => void }) {
  const toggle = (id: string) => {
    const set = new Set(state.infra);
    set.has(id) ? set.delete(id) : set.add(id);
    onChange({ infra: [...set] });
  };
  return (
    <FormSection title="Infrastructure" description="Infrastructure operator CRs will be committed alongside the Kustomize manifests.">
      <div role="group" aria-label="Infrastructure" style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(230px,1fr))", gap: 12 }}>
        {INFRA_OPTIONS.map(inf => {
          const checked = state.infra.includes(inf.id);
          return (
            <SelCard key={inf.id} role="checkbox" selected={checked} onSelect={() => toggle(inf.id)}>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                <span style={{ fontSize: 14, fontWeight: 600, color: "var(--text)" }}>{inf.name}</span>
                <span style={{ width: 17, height: 17, borderRadius: 5, border: `1.5px solid ${checked ? "var(--accent)" : "var(--line2)"}`, background: checked ? "var(--accent)" : "transparent", flexShrink: 0, display: "flex", alignItems: "center", justifyContent: "center", fontSize: 11, fontWeight: 700, color: "var(--accent-ink)" }}>
                  {checked ? "✓" : ""}
                </span>
              </div>
              <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11.5, color: "var(--accent)", marginTop: 2 }}>{inf.op}</div>
              <div style={{ fontSize: 12, color: "var(--muted)" }}>{inf.blurb}</div>
            </SelCard>
          );
        })}
      </div>
      <div style={{ fontSize: 12.5, color: "var(--faint)", fontFamily: "JetBrains Mono,monospace" }}>
        {state.infra.length === 0
          ? "No infrastructure attached — the service will run stateless."
          : `${state.infra.join(", ")} · ~${state.infra.length * 2} vCPU reserved`}
      </div>
    </FormSection>
  );
}

// ── Dependencies ──────────────────────────────────────────────────────────────
function SectionDeps({ state, onChange, services }: {
  state: WizardState;
  onChange: (p: Partial<WizardState>) => void;
  services: Project[];
}) {
  const [search, setSearch] = useState("");

  const toggle = (id: string) => {
    const set = new Set(state.deps);
    set.has(id) ? set.delete(id) : set.add(id);
    onChange({ deps: [...set] });
  };

  // Exclude the service being built (matched by name) and apply search filter
  const visible = services.filter(s =>
    s.name !== state.name &&
    (s.name.toLowerCase().includes(search.toLowerCase()) ||
     s.slug.toLowerCase().includes(search.toLowerCase()))
  );

  return (
    <FormSection title="Dependencies" description="Declared dependencies are added to the NetworkPolicy and service mesh allow-list.">
      <input className="field" style={{ maxWidth: 300 }} placeholder="Search services…" value={search} onChange={e => setSearch(e.target.value)} />
      <div style={{ background: "var(--bg)", border: "1px solid var(--line)", borderRadius: 10, maxHeight: 300, overflowY: "auto" }}>
        {visible.length === 0 ? (
          <div style={{ padding: "32px 16px", textAlign: "center", color: "var(--faint)", fontSize: 13 }}>
            {services.length === 0
              ? "No other services exist in this application yet."
              : "No services match your search."}
          </div>
        ) : visible.map((s, i) => {
          const selected = state.deps.includes(s.id);
          return (
            <div key={s.id} onClick={() => toggle(s.id)} role="checkbox" aria-checked={selected} tabIndex={0}
              onKeyDown={e => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); toggle(s.id); } }}
              style={{
                display: "flex", alignItems: "center", gap: 12, padding: "12px 16px",
                borderTop: i > 0 ? "1px solid var(--line)" : "none",
                background: selected ? "var(--accent-soft)" : "transparent", cursor: "pointer",
              }}>
              <span style={{ width: 17, height: 17, borderRadius: 5, border: `1.5px solid ${selected ? "var(--accent)" : "var(--line2)"}`, background: selected ? "var(--accent)" : "transparent", flexShrink: 0, display: "flex", alignItems: "center", justifyContent: "center", fontSize: 11, fontWeight: 700, color: "var(--accent-ink)" }}>
                {selected ? "✓" : ""}
              </span>
              <div style={{ flex: 1 }}>
                <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12.5, color: "var(--text)" }}>{s.name}</div>
                <div style={{ fontSize: 11.5, color: "var(--faint)", marginTop: 1 }}>{s.build_tool}</div>
              </div>
              <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12, color: "var(--muted)" }}>:{s.port || 8080}</div>
            </div>
          );
        })}
      </div>
      <div style={{ fontSize: 12.5, color: "var(--faint)" }}>
        {state.deps.length === 0
          ? "No dependencies declared — the service will run independently."
          : `${state.deps.length} service${state.deps.length > 1 ? "s" : ""} declared as dependencies`}
      </div>
    </FormSection>
  );
}

// ── Live summary sidebar ──────────────────────────────────────────────────────
function KV({ k, v }: { k: string; v: string }) {
  return (
    <div style={{ display: "flex", justifyContent: "space-between", gap: 12 }}>
      <span style={{ fontSize: 12, color: "var(--faint)" }}>{k}</span>
      <span style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12, color: "var(--text)", textAlign: "right" }}>{v || "—"}</span>
    </div>
  );
}

function SummarySidebar({ state, apps, services, canProvision, isProvisioning, errorMsg, onProvision }: {
  state: WizardState;
  apps: Application[];
  services: Project[];
  canProvision: boolean;
  isProvisioning: boolean;
  errorMsg: string | null;
  onProvision: () => void;
}) {
  const bt = BUILD_TOOLS.find(b => b.id === state.buildTool);
  const ti = TIERS.find(t => t.id === state.tier);
  const app = apps.find(a => a.id === state.appId);
  const kindLabel = SERVICE_KINDS.find(k => k.id === state.serviceKind)?.label ?? state.serviceKind;
  const depNames = state.deps.map(id => services.find(s => s.id === id)?.name ?? id).join(", ");

  return (
    <div className="card" style={{ position: "sticky", top: 108, display: "flex", flexDirection: "column", gap: 16 }}>
      <div>
        <p className="overline accent" style={{ margin: "0 0 4px" }}>What gets created</p>
        <p style={{ margin: 0, fontFamily: "JetBrains Mono,monospace", fontSize: 15, fontWeight: 600, color: "var(--text)", wordBreak: "break-word" }}>
          {state.name || "unnamed-service"}
        </p>
        {app && (
          <p style={{ margin: "2px 0 0", fontSize: 11.5, color: "var(--faint)" }}>in {app.name}</p>
        )}
      </div>

      <div style={{ display: "flex", flexDirection: "column", gap: 8, borderTop: "1px solid var(--line)", paddingTop: 14 }}>
        <KV k="Kind" v={kindLabel} />
        <KV k="Build" v={bt ? `${bt.id} · ${bt.lang}` : "—"} />
        <KV k="Port" v={state.port || "8080"} />
        <KV k="Tier" v={ti?.label ?? "medium"} />
        <KV k="Infra" v={state.infra.length ? state.infra.join(", ") : "None"} />
        <KV k="Depends on" v={depNames || "None"} />
      </div>

      {errorMsg && (
        <div style={{ background: "var(--bad-soft)", borderRadius: 8, padding: "10px 12px", fontSize: 12.5, color: "var(--bad)", lineHeight: 1.5 }}>
          {errorMsg}
        </div>
      )}

      <Button onClick={onProvision} disabled={!canProvision} loading={isProvisioning} size="lg" style={{ justifyContent: "center", width: "100%" }}>
        {isProvisioning ? "Provisioning…" : "Provision service"}
      </Button>

      {!canProvision && !isProvisioning && (
        <p style={{ margin: 0, fontSize: 11.5, color: "var(--faint)", textAlign: "center" }}>
          {!state.appId ? "Select an application to continue." : "Name the service to continue."}
        </p>
      )}

      <p style={{ margin: 0, fontSize: 11, color: "var(--faint)", lineHeight: 1.5, borderTop: "1px solid var(--line)", paddingTop: 12 }}>
        Provisioning creates real resources immediately — the repo, Harbor project and infra CRs aren't automatically removed if it fails partway.
      </p>
    </div>
  );
}

// ── Provisioning — live SSE step stream ───────────────────────────────────────
interface StepRow { index: number; label: string; status: string; detail: string }

function ProvisionStep({ svcName, streamUrl }: { svcName: string; streamUrl: string }) {
  const [steps, setSteps] = useState<StepRow[]>([]);
  const [done, setDone] = useState(false);
  const navigate = useNavigate();

  // Connect to real SSE stream from the backend provisioner
  useEffect(() => {
    if (!streamUrl) return;
    const es = new EventSource(streamUrl);
    es.onmessage = (e) => {
      try {
        const ev = JSON.parse(e.data) as { step_index: number; label: string; status: string; detail?: string; done?: boolean };
        if (ev.done) {
          setDone(true);
          es.close();
          return;
        }
        setSteps(prev => {
          const next = [...prev];
          const idx = next.findIndex(s => s.index === ev.step_index);
          const row: StepRow = { index: ev.step_index, label: ev.label, status: ev.status, detail: ev.detail ?? "" };
          if (idx >= 0) next[idx] = row; else next.push(row);
          return next.sort((a, b) => a.index - b.index);
        });
      } catch { /* ignore malformed frames */ }
    };
    es.onerror = () => es.close();
    return () => es.close();
  }, [streamUrl]);

  const completedCount = steps.filter(s => s.status === "done" || s.status === "failed").length;
  const totalCount = steps.length || 15; // provisioner always creates 15 steps
  const anyFailed = steps.some(s => s.status === "failed");
  const allDone = done;
  const progress = (completedCount / totalCount) * 100;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 18 }}>
      {/* Header card */}
      <div className="card">
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 12 }}>
          <div>
            <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 14, fontWeight: 600, color: "var(--text)" }}>{svcName}</div>
            <div style={{ fontSize: 12, color: "var(--faint)", marginTop: 2 }}>
              {allDone && anyFailed ? "provisioning finished with errors" : allDone ? "provisioning complete" : "provisioning in progress"}
            </div>
          </div>
          <span className={allDone && !anyFailed ? "badge badge-success" : anyFailed ? "badge badge-failed" : "badge badge-running"}>
            <span style={{ width: 6, height: 6, borderRadius: anyFailed ? "2px" : "50%", background: allDone && !anyFailed ? "var(--ok)" : anyFailed ? "var(--bad)" : "var(--accent)", animation: !allDone && !anyFailed ? "dcpulse 1.2s ease-in-out infinite" : "none", transform: anyFailed ? "rotate(45deg)" : "none" }} />
            {allDone && !anyFailed ? "Success" : anyFailed ? "Failed" : "Running"}
          </span>
        </div>
        <div style={{ height: 3, background: "var(--card)", borderRadius: 2, overflow: "hidden" }}>
          <div style={{ height: "100%", background: anyFailed ? "var(--bad)" : "var(--accent)", borderRadius: 2, width: `${progress}%`, transition: "width .5s ease" }} />
        </div>
      </div>

      {/* Steps */}
      {steps.length === 0 ? (
        <div style={{ display: "flex", flexDirection: "column", gap: 18, alignItems: "center", justifyContent: "center", minHeight: 260 }}>
          <div style={{ width: 36, height: 36, borderRadius: "50%", border: "3px solid var(--line2)", borderTopColor: "var(--accent)", animation: "dcspin 1s linear infinite" }} />
          <div style={{ fontSize: 13, color: "var(--muted)" }}>Connecting to provisioning stream…</div>
        </div>
      ) : (
        <div className="card" style={{ padding: 0 }}>
          {steps.map((s, i) => (
            <div key={s.index} style={{ display: "grid", gridTemplateColumns: "18px minmax(0,1fr)", gap: 14, padding: "12px 18px", borderTop: i > 0 ? "1px solid var(--line)" : "none", alignItems: "flex-start" }}>
              {/* Status icon */}
              {s.status === "done" ? (
                <span style={{ width: 16, height: 16, borderRadius: "50%", background: "var(--ok-soft)", border: "1px solid var(--ok)", display: "flex", alignItems: "center", justifyContent: "center", fontSize: 9, color: "var(--ok)", fontWeight: 700, marginTop: 1 }}>✓</span>
              ) : s.status === "failed" ? (
                <span style={{ width: 14, height: 14, borderRadius: 3, border: "1.5px solid var(--bad)", background: "var(--bad-soft)", display: "flex", alignItems: "center", justifyContent: "center", fontSize: 9, color: "var(--bad)", fontWeight: 700, marginTop: 2 }}>✕</span>
              ) : s.status === "running" ? (
                <span style={{ width: 14, height: 14, borderRadius: "50%", border: "2px solid var(--accent)", borderTopColor: "transparent", animation: "dcspin .8s linear infinite", display: "block", marginTop: 2 }} />
              ) : (
                <span style={{ width: 8, height: 8, borderRadius: "50%", background: "var(--line2)", display: "block", margin: "4px auto 0" }} />
              )}
              {/* Text */}
              <div>
                <div style={{ fontSize: 13, color: s.status === "pending" ? "var(--faint)" : "var(--text)", fontWeight: s.status === "running" ? 500 : 400 }}>
                  <span style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11, color: "var(--faint)", marginRight: 8 }}>{String(s.index).padStart(2, "0")}</span>
                  {s.label}
                </div>
                {s.detail && (
                  <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11.5, color: s.status === "failed" ? "var(--bad)" : "var(--faint)", marginTop: 3, lineHeight: 1.5 }}>{s.detail}</div>
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Done actions */}
      {allDone && (
        <div style={{ display: "flex", gap: 10 }}>
          <button className="btn btn-primary" onClick={() => navigate("/applications")}>View in Applications</button>
        </div>
      )}
    </div>
  );
}

// ── Main page ─────────────────────────────────────────────────────────────────
const DEFAULT_STATE: WizardState = {
  name: "", appId: "", assigneeId: "", description: "",
  serviceKind: "backend",
  buildTool: "maven", port: "8080", liveness: "/actuator/health/liveness",
  readiness: "/actuator/health/readiness", tier: "medium",
  infra: [], deps: [],
};

export function CreateServicePage() {
  const { appId: urlAppId } = useParams<{ appId: string }>();
  const navigate = useNavigate();
  const { theme, toggle: toggleTheme } = useTheme();
  const [phase, setPhase] = useState<"form" | "provisioning">("form");
  const [state, setState] = useState<WizardState>({ ...DEFAULT_STATE, appId: urlAppId || "" });
  const [isProvisioning, setIsProvisioning] = useState(false);
  const [streamUrl, setStreamUrl] = useState<string>("");
  const [createError, setCreateError] = useState<string | null>(null);

  const { data: apps = [] } = useApplications();
  const { data: users = [] } = useUsers();
  // Load services for the currently selected application (for the deps section)
  const { data: appServices = [] } = useApplicationServices(state.appId);
  const createService = useCreateService(state.appId);

  const update = (p: Partial<WizardState>) => setState(prev => ({ ...prev, ...p }));

  const canProvision = state.name.trim().length > 1 && !!state.appId && !!state.buildTool;

  const handleProvision = async () => {
    if (!canProvision) return;
    setIsProvisioning(true);
    setCreateError(null);
    try {
      const result = await createService.mutateAsync({
        name: state.name,
        build_tool: state.buildTool,
        port: parseInt(state.port) || 8080,
        health_path: state.liveness || "/healthz",
        service_kind: state.serviceKind,
        notification_email: "",
        app_timezone: "Africa/Dar_es_Salaam",
        infra_requirements: state.infra.map(type => ({ service_type: type, config: {} })),
        talks_to: state.deps.map(id => ({
          project_id: id,
          port: appServices.find(s => s.id === id)?.port || 80,
        })),
        members: state.assigneeId ? [{ user_id: state.assigneeId, role: "developer" }] : [],
      });

      setStreamUrl(result.stream_url || "");
      setPhase("provisioning");
    } catch (error) {
      setIsProvisioning(false);
      let msg = "Failed to create service.";
      if (error instanceof Error) {
        msg = error.message.includes("already exists")
          ? `A service named "${state.name}" already exists in this application.`
          : error.message || msg;
      }
      setCreateError(msg);
    }
  };

  const currentAppName = apps.find((a: Application) => a.id === state.appId)?.name ?? state.appId ?? "—";

  return (
    <div style={{ minHeight: "100%", background: "var(--bg)", display: "flex", flexDirection: "column" }}>
      {/* Top bar */}
      <div style={{
        height: 56, background: "var(--panel)", borderBottom: "1px solid var(--line)",
        display: "flex", alignItems: "center", padding: "0 24px", gap: 16, flexShrink: 0,
        position: "sticky", top: 52, zIndex: 4,
      }}>
        <Link to="/" style={{ display: "flex", alignItems: "center", gap: 8, textDecoration: "none" }}>
          <span style={{ width: 26, height: 26, borderRadius: 7, background: "var(--accent)", display: "flex", alignItems: "center", justifyContent: "center", fontSize: 13, fontWeight: 700, color: "var(--accent-ink)" }}>N</span>
        </Link>
        <div style={{ fontSize: 13, color: "var(--faint)" }}>
          <span style={{ color: "var(--faint)" }}>{currentAppName}</span>
          <span style={{ color: "var(--line2)", margin: "0 8px" }}>/</span>
          <span style={{ fontWeight: 600, color: "var(--text)" }}>New service</span>
        </div>
        <div style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 10 }}>
          <button
            onClick={toggleTheme}
            className="btn btn-ghost btn-sm"
            style={{ width: 30, height: 30, padding: 0, display: "flex", alignItems: "center", justifyContent: "center" }}
          >{theme === "dark" ? "☀" : "☾"}</button>
          <button className="btn btn-secondary btn-sm"
            style={{ borderColor: "var(--line2)" }}
            onClick={() => navigate("/applications")}
            onMouseEnter={e => { (e.currentTarget as HTMLElement).style.borderColor = "var(--bad)"; (e.currentTarget as HTMLElement).style.color = "var(--bad)"; }}
            onMouseLeave={e => { (e.currentTarget as HTMLElement).style.borderColor = "var(--line2)"; (e.currentTarget as HTMLElement).style.color = "var(--text)"; }}
          >Cancel</button>
        </div>
      </div>

      {phase === "provisioning" ? (
        <div style={{ flex: 1, padding: "36px 24px 48px", maxWidth: 760, margin: "0 auto", width: "100%" }}>
          <ProvisionStep svcName={state.name} streamUrl={streamUrl} />
        </div>
      ) : (
        <div style={{
          flex: 1, padding: "32px 24px 64px", maxWidth: 1080, margin: "0 auto", width: "100%",
          display: "grid", gridTemplateColumns: "minmax(0,1fr) 300px", gap: 24, alignItems: "start",
        }}>
          <div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
            <div>
              <h2 style={{ fontSize: 24, fontWeight: 600, letterSpacing: "-0.03em", color: "var(--text)", margin: "0 0 6px" }}>New service</h2>
              <p style={{ fontSize: 13.5, color: "var(--muted)", margin: 0 }}>
                Fill in what you know — the summary on the right updates as you go, and provisions the moment you're ready.
              </p>
            </div>
            <SectionIdentity state={state} onChange={update} apps={apps} users={users} />
            <SectionBuild state={state} onChange={update} />
            <SectionRuntime state={state} onChange={update} />
            <SectionInfra state={state} onChange={update} />
            <SectionDeps state={state} onChange={update} services={appServices} />
          </div>
          <SummarySidebar
            state={state} apps={apps} services={appServices}
            canProvision={canProvision} isProvisioning={isProvisioning}
            errorMsg={createError} onProvision={handleProvision}
          />
        </div>
      )}
    </div>
  );
}
