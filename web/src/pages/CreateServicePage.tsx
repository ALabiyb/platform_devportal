// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState, useEffect } from "react";
import { useNavigate, useParams, Link } from "react-router-dom";
import { useTheme } from "@/components/Layout";
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

const STEPS = ["Identity","Build tool","Runtime","Infra","Deps","Review","Provision"] as const;

const SERVICE_KINDS = [
  { id: "backend",  label: "Backend",  desc: "HTTP/gRPC API, worker with a health endpoint" },
  { id: "frontend", label: "Frontend", desc: "Static SPA served behind nginx" },
  { id: "worker",   label: "Worker",   desc: "Background/queue consumer, no inbound traffic" },
] as const;

const BUILD_TOOLS = [
  { id: "maven",  lang: "Java 21",       subtitle: "Spring Boot",       image: "temurin:21-jre",     cmd: "mvn -B package",  color: "#f87171" },
  { id: "gradle", lang: "Kotlin 2.0",    subtitle: "Ktor",              image: "temurin:21-jre",     cmd: "./gradlew build", color: "#4ade80" },
  { id: "go",     lang: "Go 1.23",       subtitle: "net/http",          image: "distroless:static",  cmd: "go build ./...",  color: "#38bdf8" },
  { id: "node",   lang: "Node 22",       subtitle: "Fastify",           image: "node:22-alpine",     cmd: "pnpm build",      color: "#fbbf24" },
  { id: "python", lang: "Python 3.12",   subtitle: "FastAPI",           image: "python:3.12-slim",   cmd: "uv sync",         color: "#c084fc" },
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


// ── Progress rail ─────────────────────────────────────────────────────────────
function ProgressRail({ step, maxReached, onJump }: { step: number; maxReached: number; onJump: (i: number) => void }) {
  const connector = (filled: boolean, half: boolean, hidden: boolean) => (
    <div style={{ flex: 1, height: 2, background: "var(--line)", position: "relative", overflow: "hidden", visibility: hidden ? "hidden" : "visible" }}>
      <div style={{
        position: "absolute", top: 0, left: 0, height: "100%",
        background: "var(--accent)",
        width: filled ? "100%" : half ? "50%" : "0%",
        transition: "width 0.45s cubic-bezier(0.4,0,0.2,1)",
      }} />
    </div>
  );

  return (
    <div style={{ padding: "22px 40px 0", maxWidth: 1040, margin: "0 auto", width: "100%" }}>
      <div style={{ display: "flex", alignItems: "center" }}>
        {STEPS.map((label, i) => {
          const isFirst = i === 0, isLast = i === STEPS.length - 1;
          const done = i < step, current = i === step, reachable = i <= maxReached;
          // left half of this step's connector fills when we've reached or passed step i
          const leftFilled = step >= i;
          // right half fills only when step i is done
          const rightFilled = done;

          return (
            <div key={i} style={{ display: "flex", alignItems: "center", flex: 1 }}>
              {/* Left connector half — hidden for first step */}
              {connector(leftFilled, false, isFirst)}

              {/* Node: circle above, label below */}
              <div style={{ flexShrink: 0, display: "flex", flexDirection: "column", alignItems: "center", gap: 8 }}>
                <button
                  onClick={() => reachable && i !== step && onJump(i)}
                  disabled={!reachable || i === step || i === 6}
                  title={label}
                  style={{
                    width: 32, height: 32, borderRadius: "50%",
                    cursor: reachable && i < step ? "pointer" : "default",
                    display: "flex", alignItems: "center", justifyContent: "center",
                    fontSize: 12, fontWeight: 700, fontFamily: "JetBrains Mono,monospace",
                    border: done ? "none" : current ? "2px solid var(--accent)" : "1.5px solid var(--line2)",
                    background: done ? "var(--accent)" : current ? "transparent" : "var(--panel)",
                    color: done ? "var(--accent-ink)" : current ? "var(--accent)" : "var(--faint)",
                    transition: "background 0.3s, border-color 0.3s, color 0.3s",
                    animation: current ? "dcring 1.6s ease-out infinite" : "none",
                    padding: 0, outline: "none",
                  }}
                >{done ? "✓" : i + 1}</button>
                <div style={{
                  fontSize: 11, fontWeight: current ? 600 : 400, whiteSpace: "nowrap",
                  color: current ? "var(--accent)" : done ? "var(--text)" : reachable ? "var(--muted)" : "var(--faint)",
                  transition: "color 0.3s",
                  animation: current ? "dcrise 0.25s ease both" : "none",
                }}>{label}</div>
              </div>

              {/* Right connector half — hidden for last step */}
              {connector(rightFilled, current, isLast)}
            </div>
          );
        })}
      </div>
    </div>
  );
}

// ── Step 1: Identity ──────────────────────────────────────────────────────────
function StepIdentity({ state, onChange, apps, users }: {
  state: WizardState;
  onChange: (p: Partial<WizardState>) => void;
  apps: Application[];
  users: User[];
}) {
  const selectedApp = apps.find((a: Application) => a.id === state.appId);

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 24 }}>
      <div>
        <label style={{ display: "block", fontSize: 12.5, fontWeight: 600, color: "var(--muted)", marginBottom: 6 }}>Service name</label>
        <input className="field field-lg field-mono" placeholder="ledger-reconciler" value={state.name}
          onChange={e => onChange({ name: e.target.value })} style={{ maxWidth: 360 }} />
        <div style={{ fontSize: 11, color: "var(--faint)", fontFamily: "JetBrains Mono,monospace", marginTop: 5 }}>lowercase, hyphens, ≤ 40 chars</div>
      </div>
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
        <div>
          <label style={{ display: "block", fontSize: 12.5, fontWeight: 600, color: "var(--muted)", marginBottom: 6 }}>Application</label>
          <select className="field" value={state.appId} onChange={e => onChange({ appId: e.target.value })}>
            <option value="">Select application…</option>
            {apps.map((a: Application) => (
              <option key={a.id} value={a.id}>{a.name}</option>
            ))}
          </select>
        </div>
        <div>
          <label style={{ display: "block", fontSize: 12.5, fontWeight: 600, color: "var(--muted)", marginBottom: 6 }}>Assignee</label>
          <select className="field" value={state.assigneeId} onChange={e => onChange({ assigneeId: e.target.value })}>
            <option value="">Select assignee…</option>
            {users.map((u: User) => (
              <option key={u.id} value={u.id}>{u.display_name || u.email}</option>
            ))}
          </select>
        </div>
      </div>
      <div style={{ maxWidth: 480 }}>
        <label style={{ display: "block", fontSize: 12.5, fontWeight: 600, color: "var(--muted)", marginBottom: 6 }}>Description</label>
        <textarea className="field" rows={3}
          placeholder="One line a stranger on call would understand."
          value={state.description} onChange={e => onChange({ description: e.target.value })}
          style={{ padding: "11px 13px", lineHeight: 1.5, resize: "vertical", height: "auto" }} />
      </div>
      {state.name && selectedApp && (
        <div style={{ background: "var(--panel)", borderRadius: 8, padding: "12px 14px", borderLeft: "6px solid var(--accent)", maxWidth: 480 }}>
          <div style={{ fontSize: 12, color: "var(--faint)" }}>Repository will be created at</div>
          <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12.5, color: "var(--accent)", marginTop: 4 }}>
            git.nexbridge.io/{selectedApp.git_namespace || selectedApp.slug}/{state.name}.git
          </div>
        </div>
      )}
    </div>
  );
}

// ── Step 2: Build tool ────────────────────────────────────────────────────────
function StepBuildTool({ state, onChange }: { state: WizardState; onChange: (p: Partial<WizardState>) => void }) {
  return (
    <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(228px,1fr))", gap: 12 }}>
      {BUILD_TOOLS.map(t => {
        const selected = state.buildTool === t.id;
        return (
          <div key={t.id} className={`sel-card${selected ? " selected" : ""}`} onClick={() => onChange({ buildTool: t.id })}>
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
          </div>
        );
      })}
    </div>
  );
}

// ── Step 3: Runtime ───────────────────────────────────────────────────────────
function StepRuntime({ state, onChange }: { state: WizardState; onChange: (p: Partial<WizardState>) => void }) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 22 }}>
      <div style={{ display: "grid", gridTemplateColumns: "repeat(3, 1fr)", gap: 14, maxWidth: 560 }}>
        {[["Container port","port","8080"],["Liveness path","liveness","/actuator/health/liveness"],["Readiness path","readiness","/actuator/health/readiness"]].map(([label, key, placeholder]) => (
          <div key={key}>
            <label style={{ display: "block", fontSize: 12.5, fontWeight: 600, color: "var(--muted)", marginBottom: 6 }}>{label}</label>
            <input className="field field-mono" placeholder={placeholder}
              value={(state as unknown as Record<string, string>)[key!]}
              onChange={e => onChange({ [key as keyof WizardState]: e.target.value } as Partial<WizardState>)} />
          </div>
        ))}
      </div>
      <div>
        <label style={{ display: "block", fontSize: 12.5, fontWeight: 600, color: "var(--muted)", marginBottom: 10 }}>Resource tier</label>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(3,1fr)", gap: 12, maxWidth: 560 }}>
          {TIERS.map(t => {
            const selected = state.tier === t.id;
            return (
              <div key={t.id} className={`sel-card${selected ? " selected" : ""}`} onClick={() => onChange({ tier: t.id })}>
                <div style={{ display: "flex", justifyContent: "space-between" }}>
                  <span style={{ fontSize: 13, fontWeight: 600, color: "var(--text)" }}>{t.label}</span>
                  <span style={{ width: 14, height: 14, borderRadius: "50%", border: `1.5px solid ${selected ? "var(--accent)" : "var(--line2)"}`, background: selected ? "var(--accent)" : "transparent", flexShrink: 0 }} />
                </div>
                <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11.5, color: selected ? "var(--accent)" : "var(--muted)" }}>{t.spec}</div>
                <div style={{ fontSize: 12, color: "var(--faint)" }}>{t.desc}</div>
              </div>
            );
          })}
        </div>
      </div>
      <div style={{ background: "var(--panel)", borderRadius: 8, padding: "12px 14px", maxWidth: 480, fontSize: 12, color: "var(--faint)", lineHeight: 1.6 }}>
        Probe timings inherited from language profile: Initial delay 30s · period 10s · failure threshold 3.{" "}
        <Link to="/platform" style={{ color: "var(--accent)", textDecoration: "none" }}>Override →</Link>
      </div>
    </div>
  );
}

// ── Step 4: Infra ─────────────────────────────────────────────────────────────
function StepInfra({ state, onChange }: { state: WizardState; onChange: (p: Partial<WizardState>) => void }) {
  const toggle = (id: string) => {
    const set = new Set(state.infra);
    set.has(id) ? set.delete(id) : set.add(id);
    onChange({ infra: [...set] });
  };
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(230px,1fr))", gap: 12 }}>
        {INFRA_OPTIONS.map(inf => {
          const checked = state.infra.includes(inf.id);
          return (
            <div key={inf.id} className={`sel-card${checked ? " selected" : ""}`} onClick={() => toggle(inf.id)}>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                <span style={{ fontSize: 14, fontWeight: 600, color: "var(--text)" }}>{inf.name}</span>
                <span style={{ width: 17, height: 17, borderRadius: 5, border: `1.5px solid ${checked ? "var(--accent)" : "var(--line2)"}`, background: checked ? "var(--accent)" : "transparent", flexShrink: 0, display: "flex", alignItems: "center", justifyContent: "center", fontSize: 11, fontWeight: 700, color: "var(--accent-ink)" }}>
                  {checked ? "✓" : ""}
                </span>
              </div>
              <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11.5, color: "var(--accent)", marginTop: 2 }}>{inf.op}</div>
              <div style={{ fontSize: 12, color: "var(--muted)" }}>{inf.blurb}</div>
            </div>
          );
        })}
      </div>
      <div style={{ fontSize: 12.5, color: "var(--faint)", fontFamily: "JetBrains Mono,monospace" }}>
        {state.infra.length === 0
          ? "No infrastructure attached — the service will run stateless."
          : `${state.infra.join(", ")} · ~${state.infra.length * 2} vCPU reserved`}
      </div>
    </div>
  );
}

// ── Step 5: Deps ──────────────────────────────────────────────────────────────
function StepDeps({ state, onChange, services }: {
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
    <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
      <input className="field" style={{ maxWidth: 300 }} placeholder="Search services…" value={search} onChange={e => setSearch(e.target.value)} />
      <div style={{ background: "var(--panel)", border: "1px solid var(--line)", borderRadius: 10, maxHeight: 360, overflowY: "auto" }}>
        {visible.length === 0 ? (
          <div style={{ padding: "32px 16px", textAlign: "center", color: "var(--faint)", fontSize: 13 }}>
            {services.length === 0
              ? "No other services exist in this application yet."
              : "No services match your search."}
          </div>
        ) : visible.map((s, i) => {
          const selected = state.deps.includes(s.id);
          return (
            <div key={s.id} onClick={() => toggle(s.id)} style={{
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
    </div>
  );
}

// ── Step 6: Review ────────────────────────────────────────────────────────────
function ReviewSection({ title, onEdit, children }: { title: string; step?: number; onEdit: () => void; children: React.ReactNode }) {
  return (
    <div className="card" style={{ padding: 0 }}>
      <div style={{ padding: "11px 16px", borderBottom: "1px solid var(--line)", display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <span className="overline">{title}</span>
        <button className="btn btn-ghost btn-sm" onClick={onEdit}>Edit ↩</button>
      </div>
      <div style={{ padding: "14px 16px", display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(180px,1fr))", gap: 12 }}>
        {children}
      </div>
    </div>
  );
}

function KV({ k, v }: { k: string; v: string }) {
  return (
    <div>
      <div style={{ fontSize: 11.5, color: "var(--faint)", marginBottom: 3 }}>{k}</div>
      <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 13, color: "var(--text)" }}>{v || "—"}</div>
    </div>
  );
}

function StepReview({ state, onJump, services, apps, errorMsg }: {
  state: WizardState;
  onJump: (i: number) => void;
  services: Project[];
  apps: Application[];
  errorMsg: string | null;
}) {
  const bt = BUILD_TOOLS.find(b => b.id === state.buildTool);
  const ti = TIERS.find(t => t.id === state.tier);
  const appName = apps.find(a => a.id === state.appId)?.name ?? state.appId;
  const depNames = state.deps.map(id => {
    const s = services.find(s => s.id === id);
    return s ? `${s.name}:${s.port || 8080}` : id;
  }).join(", ");
  const kindLabel = SERVICE_KINDS.find(k => k.id === state.serviceKind)?.label ?? state.serviceKind;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
      <ReviewSection title="Identity" step={0} onEdit={() => onJump(0)}>
        <KV k="Name"        v={state.name || "—"} />
        <KV k="Kind"        v={kindLabel} />
        <KV k="Application" v={appName} />
        <KV k="Assignee"    v={state.assigneeId || "—"} />
      </ReviewSection>
      <ReviewSection title="Build" step={1} onEdit={() => onJump(1)}>
        <KV k="Profile"    v={state.buildTool || "—"} />
        <KV k="Runtime"    v={bt?.lang ?? "—"} />
        <KV k="Base image" v={bt?.image ?? "—"} />
      </ReviewSection>
      <ReviewSection title="Runtime" step={2} onEdit={() => onJump(2)}>
        <KV k="Port"     v={state.port || "8080"} />
        <KV k="Liveness" v={state.liveness || "/actuator/health/liveness"} />
        <KV k="Tier"     v={ti ? `${ti.label} · ${ti.spec}` : "medium"} />
      </ReviewSection>
      <ReviewSection title="Infrastructure" step={3} onEdit={() => onJump(3)}>
        <KV k="Attached"   v={state.infra.length ? state.infra.join(", ") : "None (stateless)"} />
        <KV k="Resources"  v={state.infra.length ? `~${state.infra.length * 2} vCPU reserved` : "—"} />
      </ReviewSection>
      <ReviewSection title="Dependencies" step={4} onEdit={() => onJump(4)}>
        <KV k="Services" v={depNames || "None"} />
      </ReviewSection>
      {errorMsg && (
        <div style={{ background: "var(--bad-soft,#7f1d1d22)", borderRadius: 8, padding: "12px 14px", borderLeft: "6px solid var(--bad,#ef4444)", fontSize: 13, color: "var(--bad,#ef4444)", lineHeight: 1.5 }}>
          {errorMsg}
        </div>
      )}
      <div style={{ background: "var(--warn-soft)", borderRadius: 8, padding: "12px 14px", borderLeft: "6px solid var(--warn)", fontSize: 12, color: "var(--muted)", lineHeight: 1.6 }}>
        Provisioning creates real resources. The repo, Harbor project and infra CRs are not automatically removed if you cancel mid-run.
      </div>
    </div>
  );
}

// ── Step 7: Provision ─────────────────────────────────────────────────────────
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
                <span style={{ width: 14, height: 14, borderRadius: 3, border: "1.5px solid var(--bad)", background: "var(--bad-soft,#7f1d1d22)", display: "flex", alignItems: "center", justifyContent: "center", fontSize: 9, color: "var(--bad)", fontWeight: 700, marginTop: 2 }}>✕</span>
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

// ── Wizard footer ─────────────────────────────────────────────────────────────
function WizardFooter({ step, canContinue, onBack, onNext, provisioning, allDone }: {
  step: number; canContinue: boolean; onBack: () => void; onNext: () => void; provisioning: boolean; allDone: boolean;
}) {
  const nextLabel = step === 5 ? (provisioning ? "Provisioning…" : "Provision service") : step === 6 ? (allDone ? "Done" : "Provisioning…") : "Continue";
  return (
    <div style={{ position: "fixed", bottom: 0, left: 0, right: 0, background: "var(--panel)", borderTop: "1px solid var(--line)", padding: "14px 24px", zIndex: 10 }}>
      <div style={{ maxWidth: 760, margin: "0 auto", display: "flex", alignItems: "center" }}>
        <button className="btn btn-secondary" onClick={onBack} disabled={step === 0 || provisioning} style={{ opacity: step === 0 ? 0.4 : 1 }}>← Back</button>
        <div style={{ flex: 1, textAlign: "center", fontFamily: "JetBrains Mono,monospace", fontSize: 12, color: "var(--faint)" }}>
          {step === 6 ? (allDone ? "all steps complete" : "provisioning…") : (!canContinue ? "complete this step to continue" : "")}
        </div>
        <button className="btn btn-primary" onClick={onNext} disabled={!canContinue || (step === 6 && !allDone) || provisioning}>
          {nextLabel}
        </button>
      </div>
    </div>
  );
}

// ── Main wizard ───────────────────────────────────────────────────────────────
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
  const [step, setStep] = useState(0);
  const [maxReached, setMaxReached] = useState(0);
  const [state, setState] = useState<WizardState>({ ...DEFAULT_STATE, appId: urlAppId || "" });
  const [isProvisioning, setIsProvisioning] = useState(false);
  const [streamUrl, setStreamUrl] = useState<string>("");
  const [createError, setCreateError] = useState<string | null>(null);

  const { data: apps = [] } = useApplications();
  const { data: users = [] } = useUsers();
  // Load services for the currently selected application (for deps step)
  const { data: appServices = [] } = useApplicationServices(state.appId);
  const createService = useCreateService(state.appId);

  const update = (p: Partial<WizardState>) => setState(prev => ({ ...prev, ...p }));

  const canContinue = (() => {
    if (step === 0) return state.name.length > 1 && !!state.appId;
    if (step === 1) return !!state.buildTool;
    if (step === 6) return false;
    return true;
  })();

  const goNext = async () => {
    if (step === 5) {
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
        setStep(6);
        setMaxReached(6);
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
      return;
    }
    if (step === 6) { navigate("/applications"); return; }
    const next = step + 1;
    setStep(next);
    setMaxReached(m => Math.max(m, next));
  };

  const goBack = () => step > 0 && setStep(s => s - 1);
  const jumpTo = (i: number) => { if (i <= maxReached) { setStep(i); } };

  const currentAppName = apps.find((a: Application) => a.id === state.appId)?.name ?? state.appId ?? "—";

  return (
    <div style={{ minHeight: "100vh", background: "var(--bg)", display: "flex", flexDirection: "column" }}>
      {/* Top bar */}
      <div style={{ height: 56, background: "var(--panel)", borderBottom: "1px solid var(--line)", display: "flex", alignItems: "center", padding: "0 24px", gap: 16, flexShrink: 0 }}>
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

      {/* Progress rail */}
      <ProgressRail step={step} maxReached={maxReached} onJump={jumpTo} />

      {/* Content */}
      <div style={{ flex: 1, padding: "36px 24px 120px", maxWidth: 760, margin: "0 auto", width: "100%" }}>
        <p className="overline accent" style={{ marginBottom: 6 }}>Step {step + 1} of 7 · {STEPS[step]}</p>
        <h2 style={{ fontSize: 26, fontWeight: 600, letterSpacing: "-0.03em", color: "var(--text)", marginBottom: 8, marginTop: 0 }}>
          {["Name the service","Pick a language profile","Port and health checks","Attach infrastructure","Declare service dependencies","Review before provisioning","Provisioning"][step]}
        </h2>
        <p style={{ fontSize: 14, color: "var(--muted)", maxWidth: "60ch", marginBottom: 28, marginTop: 0, lineHeight: 1.6 }}>
          {[
            "Choose a unique slug that becomes the repo name, image tag and ArgoCD app name.",
            "The language profile sets the Jenkinsfile template, Dockerfile base image and probe timings.",
            "These values go into the Kustomize base and the liveness/readiness probes.",
            "Infrastructure operator CRs will be committed alongside the Kustomize manifests.",
            "Declared dependencies are added to the NetworkPolicy and service mesh allow-list.",
            "Review every field before provisioning. Resources are created immediately.",
            "Sit back — the platform is wiring everything up.",
          ][step]}
        </p>
        {step === 0 && <StepIdentity state={state} onChange={update} apps={apps} users={users} />}
        {step === 1 && <StepBuildTool state={state} onChange={update} />}
        {step === 2 && <StepRuntime state={state} onChange={update} />}
        {step === 3 && <StepInfra state={state} onChange={update} />}
        {step === 4 && <StepDeps state={state} onChange={update} services={appServices} />}
        {step === 5 && <StepReview state={state} onJump={jumpTo} services={appServices} apps={apps} errorMsg={createError} />}
        {step === 6 && <ProvisionStep svcName={state.name} streamUrl={streamUrl} />}
      </div>

      {/* Footer */}
      {step < 6 && (
        <WizardFooter
          step={step} canContinue={canContinue}
          onBack={goBack} onNext={goNext}
          provisioning={isProvisioning} allDone={false}
        />
      )}
    </div>
  );
}
