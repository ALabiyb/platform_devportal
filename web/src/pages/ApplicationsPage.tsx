// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState, useEffect, useRef } from "react";
import { Link } from "react-router-dom";
import { useApplications, useApplicationServices, useProvisioningSteps, Application, Project, ProvisioningStep } from "@/lib/api";

const LANG_COLOR: Record<string, string> = {
  maven: "#f87171", gradle: "#f87171", java: "#f87171",
  go: "#38bdf8",
  node: "#fbbf24", npm: "#fbbf24",
  "nodejs-express": "#fbbf24", nextjs: "#f8fafc",
  python: "#c084fc", pip: "#c084fc", "python-fastapi": "#c084fc",
  docker: "#60a5fa",
  dotnet: "#a78bfa",
  auto: "var(--faint)",
};

const APP_PALETTE = [
  "#f87171","#fbbf24","#c084fc","#94a3b8","#38bdf8","#4ade80",
  "#fb923c","#a78bfa","#34d399","#f472b6",
];
function appColor(idx: number) { return APP_PALETTE[idx % APP_PALETTE.length]; }

type DisplayStatus = "healthy" | "deploying" | "failed" | "archived";

function mapStatus(s: string): DisplayStatus {
  if (s === "active") return "healthy";
  if (s === "provisioning") return "deploying";
  if (s === "failed") return "failed";
  return "archived";
}

function statusBadge(s: DisplayStatus) {
  const m: Record<DisplayStatus, { cls: string; label: string; dot: string; shape: string }> = {
    healthy:  { cls: "badge-success", label: "Healthy",   dot: "var(--ok)",     shape: "50%" },
    deploying:{ cls: "badge-running", label: "Deploying", dot: "var(--accent)", shape: "50%" },
    failed:   { cls: "badge-failed",  label: "Failed",    dot: "var(--bad)",    shape: "2px" },
    archived: { cls: "badge-pending", label: "Archived",  dot: "var(--faint)",  shape: "50%" },
  };
  const d = m[s] ?? m.archived;
  return (
    <span className={`badge ${d.cls}`}>
      <span style={{
        width: 6, height: 6, flexShrink: 0,
        borderRadius: d.shape,
        transform: s === "failed" ? "rotate(45deg)" : "none",
        background: d.dot,
        animation: s === "deploying" ? "dcpulse 1.2s ease-in-out infinite" : "none",
      }} />
      {d.label}
    </span>
  );
}

function InitialTile({ name, color }: { name: string; color: string }) {
  return (
    <span style={{
      width: 22, height: 22, borderRadius: 6, background: color + "22",
      border: `1px solid ${color}44`, color, flexShrink: 0,
      display: "inline-flex", alignItems: "center", justifyContent: "center",
      fontSize: 11, fontWeight: 700, fontFamily: "JetBrains Mono,monospace",
    }}>{(name[0] ?? "?").toUpperCase()}</span>
  );
}

// ── Service steps drawer ──────────────────────────────────────────────────────
function ServiceStepsDrawer({ svc }: { svc: Project }) {
  const { data: steps = [], isLoading } = useProvisioningSteps(svc.id);

  const statusIcon = (s: ProvisioningStep) => {
    if (s.status === "done") return (
      <span style={{ width: 14, height: 14, borderRadius: "50%", background: "var(--ok-soft)", border: "1px solid var(--ok)", display: "flex", alignItems: "center", justifyContent: "center", fontSize: 8, color: "var(--ok)", fontWeight: 700, flexShrink: 0 }}>✓</span>
    );
    if (s.status === "failed") return (
      <span style={{ width: 14, height: 14, borderRadius: 3, border: "1.5px solid var(--bad)", background: "var(--bad-soft)", display: "flex", alignItems: "center", justifyContent: "center", fontSize: 8, color: "var(--bad)", fontWeight: 700, flexShrink: 0 }}>✕</span>
    );
    if (s.status === "running") return (
      <span style={{ width: 12, height: 12, borderRadius: "50%", border: "2px solid var(--accent)", borderTopColor: "transparent", animation: "dcspin .8s linear infinite", display: "block", flexShrink: 0 }} />
    );
    return <span style={{ width: 6, height: 6, borderRadius: "50%", background: "var(--line2)", display: "block", flexShrink: 0, margin: "0 4px" }} />;
  };

  return (
    <div style={{ background: "var(--bg)", borderTop: "1px solid var(--line2)", padding: "14px 18px 14px 72px" }}>
      <div style={{ fontSize: 11.5, fontWeight: 600, color: "var(--muted)", letterSpacing: "0.08em", marginBottom: 10 }}>PROVISIONING STEPS</div>
      {isLoading ? (
        <div style={{ fontSize: 12, color: "var(--faint)" }}>Loading steps…</div>
      ) : steps.length === 0 ? (
        <div style={{ fontSize: 12, color: "var(--faint)" }}>No provisioning steps recorded for this service.</div>
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: 0 }}>
          {steps.map((s, i) => (
            <div key={s.id} style={{ display: "flex", alignItems: "flex-start", gap: 10, padding: "6px 0", borderTop: i > 0 ? "1px solid var(--line)" : "none" }}>
              <div style={{ paddingTop: 2 }}>{statusIcon(s)}</div>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                  <span style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 10.5, color: "var(--faint)", minWidth: 18 }}>{String(s.step_index).padStart(2, "0")}</span>
                  <span style={{ fontSize: 12.5, color: s.status === "pending" ? "var(--faint)" : "var(--text)", fontWeight: s.status === "running" ? 500 : 400 }}>{s.label}</span>
                </div>
                {s.detail && (
                  <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11, color: s.status === "failed" ? "var(--bad)" : "var(--faint)", marginTop: 2, paddingLeft: 26, lineHeight: 1.5, wordBreak: "break-word" }}>{s.detail}</div>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function AppGroup({ app, color, open, onToggle }: {
  app: Application; color: string; open: boolean; onToggle: () => void;
}) {
  const { data: services = [], isLoading } = useApplicationServices(app.id);
  const [expandedSvcId, setExpandedSvcId] = useState<string | null>(null);
  const appStatus = mapStatus(app.status);

  const derivedStatus: DisplayStatus = (() => {
    if (!services.length) return appStatus;
    if (services.some(s => s.status === "failed")) return "failed";
    if (services.some(s => s.status === "provisioning")) return "deploying";
    if (services.every(s => s.status === "active")) return "healthy";
    return appStatus;
  })();

  return (
    <div className="card" style={{ padding: 0, overflow: "hidden" }}>
      <div
        onClick={onToggle}
        style={{
          display: "grid",
          gridTemplateColumns: "22px minmax(200px,1fr) 130px 90px 120px 20px",
          gap: 16, padding: "16px 18px", minWidth: 720,
          alignItems: "center", cursor: "pointer", transition: "background .1s",
        }}
        onMouseEnter={e => (e.currentTarget as HTMLElement).style.background = "var(--card)"}
        onMouseLeave={e => (e.currentTarget as HTMLElement).style.background = ""}
      >
        <InitialTile name={app.name} color={color} />
        <div>
          <div style={{ fontSize: 14, fontWeight: 600, whiteSpace: "nowrap" }}>{app.name}</div>
          <div style={{ fontSize: 12, color: "var(--muted)", marginTop: 2 }}>{app.description || app.git_namespace}</div>
        </div>
        <div>
          <div className="overline" style={{ marginBottom: 3 }}>Namespace</div>
          <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12, color: "var(--text)" }}>{app.git_namespace}</div>
        </div>
        <div>
          <div className="overline" style={{ marginBottom: 3 }}>Services</div>
          <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12, color: "var(--text)" }}>
            {isLoading ? "…" : services.length}
          </div>
        </div>
        <div>{statusBadge(derivedStatus)}</div>
        <span style={{ color: "var(--faint)", fontSize: 11, userSelect: "none" }}>{open ? "▾" : "▸"}</span>
      </div>

      {open && (
        <div style={{ background: "var(--bg)", borderTop: "1px solid var(--line)" }}>
          <div style={{
            display: "grid", gap: 14, paddingLeft: 56, paddingRight: 18,
            paddingTop: 10, paddingBottom: 10,
            gridTemplateColumns: "minmax(160px,1fr) 110px 200px 90px",
            minWidth: 720, borderBottom: "1px solid var(--line)",
          }}>
            {["Service","Language","Repository","Status"].map(h => (
              <div key={h} className="overline">{h}</div>
            ))}
          </div>
          {isLoading ? (
            <div style={{ padding: "20px 56px", fontSize: 13, color: "var(--faint)" }}>Loading services…</div>
          ) : services.length === 0 ? (
            <div style={{ padding: "20px 56px", fontSize: 13, color: "var(--faint)" }}>No services yet.</div>
          ) : services.map((svc: Project) => {
            const svcExpanded = expandedSvcId === svc.id;
            const showStepsHint = svc.status === "failed" || svc.status === "provisioning";
            return (
              <div key={svc.id}>
                <div style={{
                  display: "grid", gap: 14, paddingLeft: 56, paddingRight: 18,
                  paddingTop: 11, paddingBottom: 11, minWidth: 720,
                  gridTemplateColumns: "minmax(160px,1fr) 110px 200px 90px 20px",
                  borderBottom: svcExpanded ? "none" : "1px solid var(--line)", alignItems: "center",
                  transition: "background .1s", cursor: showStepsHint ? "pointer" : "default",
                  background: svcExpanded ? "var(--panel)" : "",
                }}
                  onClick={() => showStepsHint && setExpandedSvcId(svcExpanded ? null : svc.id)}
                  onMouseEnter={e => (e.currentTarget as HTMLElement).style.background = "var(--panel)"}
                  onMouseLeave={e => { if (!svcExpanded) (e.currentTarget as HTMLElement).style.background = ""; }}
                >
                  <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12.5, fontWeight: 500 }}>{svc.name}</div>
                  <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                    <span style={{ width: 7, height: 7, borderRadius: 2, background: LANG_COLOR[svc.build_tool] ?? "var(--faint)", flexShrink: 0 }} />
                    <span style={{ fontSize: 12, color: "var(--muted)" }}>{svc.build_tool}</span>
                  </div>
                  <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11.5, color: "var(--faint)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                    {svc.git_repo_url ? svc.git_repo_url.replace(/^https?:\/\//, "") : `${app.git_namespace}/${svc.slug}`}
                  </div>
                  <div>{statusBadge(mapStatus(svc.status))}</div>
                  <span style={{ fontSize: 11, color: "var(--faint)", userSelect: "none" }}>
                    {showStepsHint ? (svcExpanded ? "▾" : "▸") : ""}
                  </span>
                </div>
                {svcExpanded && (
                  <div style={{ borderBottom: "1px solid var(--line)" }}>
                    <ServiceStepsDrawer svc={svc} />
                  </div>
                )}
              </div>
            );
          })}
          <div style={{ padding: "12px 18px 12px 56px", display: "flex", alignItems: "center", gap: 16 }}>
            <Link to={`/applications/${app.id}/services/new`}
              className="btn btn-secondary btn-sm" style={{ textDecoration: "none" }}>
              + Add service
            </Link>
            <button className="btn btn-ghost btn-sm">Settings</button>
          </div>
        </div>
      )}
    </div>
  );
}

export function ApplicationsPage() {
  const { data: apps = [], isLoading } = useApplications();
  const [openId, setOpenId] = useState<string>("");
  const [search, setSearch] = useState("");
  const [seg, setSeg] = useState<"all" | "my" | "attention">("all");
  const autoExpanded = useRef(false);

  useEffect(() => {
    if (!autoExpanded.current && apps.length > 0) {
      autoExpanded.current = true;
      setOpenId(apps[0].id);
    }
  }, [apps]);

  const filtered = apps.filter(a => {
    const q = search.toLowerCase();
    const matchSearch = !q || a.name.toLowerCase().includes(q) || a.description?.toLowerCase().includes(q) || a.git_namespace.toLowerCase().includes(q);
    const matchSeg = seg !== "attention" || a.status === "failed";
    return matchSearch && matchSeg;
  });

  return (
    <div style={{ padding: 28, maxWidth: 1240, display: "flex", flexDirection: "column", gap: 22 }}>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 16, flexWrap: "wrap" }}>
        <div>
          <h1 className="page-h1">Applications</h1>
          <p className="page-sub">{apps.length} applications · click a row to expand services</p>
        </div>
        <Link to="/applications/new" className="btn btn-primary" style={{ textDecoration: "none" }}>+ New application</Link>
      </div>

      <div style={{ display: "flex", gap: 10, alignItems: "center", flexWrap: "wrap" }}>
        <input className="field" style={{ maxWidth: 230 }} placeholder="Filter by name"
          value={search} onChange={e => setSearch(e.target.value)} />
        <div className="segmented">
          {(["all","my","attention"] as const).map(s => (
            <button key={s} className={seg === s ? "active" : ""} onClick={() => setSeg(s)}>
              {s === "all" ? "All" : s === "my" ? "My teams" : "Needs attention"}
            </button>
          ))}
        </div>
      </div>

      {isLoading ? (
        <div style={{ padding: "48px 0", textAlign: "center", color: "var(--faint)", fontSize: 13 }}>Loading applications…</div>
      ) : filtered.length === 0 ? (
        <div style={{ border: "1px dashed var(--line2)", borderRadius: 10, padding: 48, textAlign: "center" }}>
          <div style={{ fontSize: 14, fontWeight: 600, color: "var(--text)", marginBottom: 6 }}>
            {apps.length === 0 ? "No applications yet" : "No applications match that filter"}
          </div>
          <div style={{ fontSize: 13, color: "var(--muted)" }}>
            {apps.length === 0 ? "Create your first application to get started." : "Try clearing the search field."}
          </div>
          {apps.length === 0 && (
            <Link to="/applications/new" className="btn btn-primary" style={{ textDecoration: "none", display: "inline-block", marginTop: 18 }}>
              + New application
            </Link>
          )}
        </div>
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
          {filtered.map((app, idx) => (
            <AppGroup
              key={app.id}
              app={app}
              color={appColor(idx)}
              open={openId === app.id}
              onToggle={() => setOpenId(openId === app.id ? "" : app.id)}
            />
          ))}
        </div>
      )}
    </div>
  );
}
