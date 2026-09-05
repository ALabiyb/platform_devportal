// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { useApplications, useTeams, useProjects, useDashboard, DashboardActivityEvent } from "@/lib/api";
import { PageHeader, Skeleton } from "@/components/kit";

// ── Status badge ──────────────────────────────────────────────────────────
function StatusBadge({ status }: { status: string }) {
  const map: Record<string, { cls: string; dot: string; label: string }> = {
    active:       { cls: "badge-success", dot: "var(--ok)",   label: "Healthy"   },
    provisioning: { cls: "badge-running", dot: "var(--accent)", label: "Deploying" },
    failed:       { cls: "badge-failed",  dot: "var(--bad)",  label: "Failed"    },
    archived:     { cls: "badge-pending", dot: "var(--faint)", label: "Archived"  },
  };
  const s = map[status] ?? map.archived;
  return (
    <span className={`badge ${s.cls}`}>
      <span style={{ width: 6, height: 6, borderRadius: "50%", background: s.dot, flexShrink: 0 }} />
      {s.label}
    </span>
  );
}

// ── Sparkline for pipeline success ────────────────────────────────────────
const BARS = [70, 88, 60, 95, 76, 100, 82, 45, 91, 68];
function Sparkline() {
  return (
    <div style={{ display: "flex", alignItems: "flex-end", gap: 2, height: 20 }}>
      {BARS.map((h, i) => (
        <div key={i} style={{
          flex: 1, borderRadius: 2,
          height: `${h}%`,
          background: h === 100 ? "var(--accent)" : h === 45 ? "var(--bad-soft)" : "var(--accent-soft)",
        }} />
      ))}
    </div>
  );
}

// ── KPI Card ──────────────────────────────────────────────────────────────
function KpiCard({ label, value, delta, deltaGood, sub, extra }: {
  label: string; value: string | number;
  delta?: string; deltaGood?: boolean;
  sub?: string; extra?: React.ReactNode;
}) {
  return (
    <div className="card" style={{ display: "flex", flexDirection: "column", gap: 10, height: "100%" }}>
      <p className="overline" style={{ margin: 0 }}>{label}</p>
      <div style={{ display: "flex", alignItems: "baseline", gap: 8 }}>
        <span style={{ fontSize: 30, fontWeight: 600, letterSpacing: "-0.03em", color: "var(--text)", lineHeight: 1, fontVariantNumeric: "tabular-nums" }}>
          {value}
        </span>
        {delta && (
          <span style={{ fontSize: 12, fontWeight: 600, color: deltaGood ? "var(--ok)" : "var(--bad)" }}>
            {delta}
          </span>
        )}
      </div>
      {extra}
      {sub && <p style={{ margin: 0, fontSize: 12, color: "var(--faint)" }}>{sub}</p>}
    </div>
  );
}

function KpiCardSkeleton() {
  return (
    <div className="card" style={{ display: "flex", flexDirection: "column", gap: 10, height: "100%" }}>
      <Skeleton width={70} height={10} />
      <Skeleton width={56} height={28} />
      <Skeleton width={120} height={12} />
    </div>
  );
}

// ── Recent activity ────────────────────────────────────────────────────────
type EventStatus = "failed" | "running" | "success" | "warn" | "pending";

function ActivityDot({ status }: { status: EventStatus }) {
  const map: Record<EventStatus, { color: string; cls?: string; shape: "circle" | "square" | "diamond" }> = {
    success: { color: "var(--ok)",     shape: "circle" },
    running: { color: "var(--accent)", shape: "circle" },
    failed:  { color: "var(--bad)",    shape: "diamond" },
    warn:    { color: "var(--warn)",   shape: "square" },
    pending: { color: "var(--faint)",  shape: "circle" },
  };
  const s = map[status];
  return (
    <span style={{
      width: 7, height: 7, background: s.color, flexShrink: 0,
      borderRadius: s.shape === "circle" ? "50%" : s.shape === "square" ? 2 : 2,
      transform: s.shape === "diamond" ? "rotate(45deg)" : "none",
      animation: status === "running" ? "dcpulse 1.2s ease-in-out infinite" : "none",
    }} />
  );
}

// ── Cluster status helper ─────────────────────────────────────────────────
function clusterOk(status: string) { return status === "active" || status === "healthy"; }

function activityStatus(action: string): "failed" | "running" | "success" | "warn" | "pending" {
  if (/fail|error|denied|reject/i.test(action)) return "failed";
  if (/provision|creat|deploy|sync/i.test(action)) return "success";
  if (/warn|finding/i.test(action)) return "warn";
  return "success";
}

function activityTitle(ev: DashboardActivityEvent): string {
  const svcName = (ev.detail?.name as string) ?? "";
  const m: Record<string, string> = {
    "service.created":               `Service created${svcName ? ` · ${svcName}` : ""}`,
    "service.provisioning.complete": `Provisioning complete${svcName ? ` · ${svcName}` : ""}`,
    "service.provisioning.failed":   `Provisioning failed${svcName ? ` · ${svcName}` : ""}`,
    "application.created":           `Application created${svcName ? ` · ${svcName}` : ""}`,
  };
  return m[ev.action] ?? ev.action.replace(/\./g, " ").replace(/_/g, " ")
    .split(" ").map(w => w.charAt(0).toUpperCase() + w.slice(1)).join(" ");
}

function activitySub(ev: DashboardActivityEvent): string {
  const parts: string[] = [];
  if (ev.actor_email) parts.push(ev.actor_email.split("@")[0]);
  if (ev.detail?.build_tool) parts.push(ev.detail.build_tool as string);
  if (ev.detail?.app) parts.push(ev.detail.app as string);
  if (ev.detail?.err) parts.push((ev.detail.err as string).slice(0, 60));
  return parts.join(" · ") || ev.resource_type;
}

// ── Dashboard ─────────────────────────────────────────────────────────────
export function DashboardPage() {
  const navigate = useNavigate();
  const { data: dashboard }                    = useDashboard();
  const { data: applications, isLoading: appsLoading } = useApplications();
  const { data: teams }    = useTeams();
  const { data: projects } = useProjects();
  const [range, setRange]  = useState<"24h"|"7d"|"30d">("24h");
  const [actFilter, setActFilter] = useState<"all"|"fail">("all");

  const stats        = dashboard?.stats;
  const totalApps    = stats?.app_count          ?? applications?.length ?? 0;
  const activeApps   = applications?.filter(a => a.status === "active").length ?? totalApps;
  const totalSvc     = stats?.service_count      ?? projects?.length    ?? 0;
  const activeSvc    = projects?.filter(p => p.status === "active").length   ?? 0;
  const failedSvc    = stats?.failed_service_count ?? projects?.filter(p => p.status === "failed").length ?? 0;
  const provisionSvc = projects?.filter(p => p.status === "provisioning").length ?? 0;
  const clusterCnt   = stats?.cluster_count ?? 0;
  const pipelineOk   = stats?.pipeline_success_rate ?? (failedSvc ? Math.round((1 - failedSvc / Math.max(totalSvc, 1)) * 100) : 96.4);
  const _teams = teams; void _teams;

  const realClusters  = dashboard?.clusters ?? [];
  const realActivity  = dashboard?.activity ?? [];
  const visibleEvents = actFilter === "fail"
    ? realActivity.filter(e => /fail|error|denied|reject/i.test(e.action))
    : realActivity;

  if (appsLoading && !dashboard) {
    return (
      <div style={{ padding: 28, maxWidth: 1400, display: "flex", flexDirection: "column", gap: 22 }}>
        <div>
          <Skeleton width={200} height={22} style={{ marginBottom: 8 }} />
          <Skeleton width={320} height={13} />
        </div>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))", gap: 12 }}>
          {Array.from({ length: 5 }).map((_, i) => <KpiCardSkeleton key={i} />)}
        </div>
        <div style={{ display: "grid", gridTemplateColumns: "minmax(0,1.7fr) minmax(280px,1fr)", gap: 16 }}>
          <div className="card" style={{ display: "flex", flexDirection: "column", gap: 14 }}>
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} style={{ display: "flex", alignItems: "center", gap: 12 }}>
                <Skeleton width={16} height={16} radius={8} />
                <Skeleton height={13} />
              </div>
            ))}
          </div>
          <div className="card" style={{ display: "flex", flexDirection: "column", gap: 14 }}>
            {Array.from({ length: 3 }).map((_, i) => <Skeleton key={i} height={13} />)}
          </div>
        </div>
      </div>
    );
  }

  return (
    <div style={{ padding: 28, maxWidth: 1400, display: "flex", flexDirection: "column", gap: 22 }}>
      <PageHeader
        title="Platform overview"
        subtitle={`Everything NexBridge runs, as of ${new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })} local.`}
        actions={
          <div className="segmented">
            {(["24h","7d","30d"] as const).map(r => (
              <button key={r} className={range === r ? "active" : ""} onClick={() => setRange(r)}>{r}</button>
            ))}
          </div>
        }
      />

      {/* KPI row */}
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))", gap: 12, alignItems: "stretch" }}>
        {/* Applications */}
        <div
          onClick={() => navigate("/applications")}
          role="link"
          tabIndex={0}
          onKeyDown={e => { if (e.key === "Enter") navigate("/applications"); }}
          style={{ cursor: "pointer", height: "100%" }}
        >
          <KpiCard
            label="Applications"
            value={totalApps}
            extra={
              <div style={{ display: "flex", gap: 12, fontSize: 12 }}>
                <span style={{ display: "flex", alignItems: "center", gap: 5 }}>
                  <span style={{ width: 7, height: 7, borderRadius: "50%", background: "var(--ok)" }} />
                  <span style={{ color: "var(--muted)" }}>{activeApps} active</span>
                </span>
                <span style={{ display: "flex", alignItems: "center", gap: 5 }}>
                  <span style={{ width: 7, height: 7, borderRadius: 2, background: "var(--faint)" }} />
                  <span style={{ color: "var(--muted)" }}>{totalApps - activeApps} archived</span>
                </span>
              </div>
            }
          />
        </div>

        {/* Services */}
        <KpiCard
          label="Services"
          value={totalSvc}
          extra={
            <div style={{ display: "flex", gap: 12, fontSize: 12, flexWrap: "wrap" }}>
              <span style={{ display: "flex", alignItems: "center", gap: 5 }}>
                <span style={{ width: 7, height: 7, borderRadius: "50%", background: "var(--ok)" }} />
                <span style={{ color: "var(--muted)" }}>{activeSvc} healthy</span>
              </span>
              {failedSvc > 0 && (
                <span style={{ display: "flex", alignItems: "center", gap: 5 }}>
                  <span style={{ width: 7, height: 7, borderRadius: 2, background: "var(--bad)", transform: "rotate(45deg)" }} />
                  <span style={{ color: "var(--bad)", fontWeight: 600 }}>{failedSvc} failed</span>
                </span>
              )}
              {provisionSvc > 0 && (
                <span style={{ display: "flex", alignItems: "center", gap: 5 }}>
                  <span style={{ width: 7, height: 7, borderRadius: "50%", background: "var(--accent)", animation: "dcpulse 1.2s ease-in-out infinite" }} />
                  <span style={{ color: "var(--muted)" }}>{provisionSvc} deploying</span>
                </span>
              )}
            </div>
          }
        />

        {/* Pipeline success */}
        <KpiCard
          label="Pipeline success"
          value={`${typeof pipelineOk === "number" ? pipelineOk.toFixed(1) : pipelineOk}%`}
          extra={<Sparkline />}
        />

        {/* Clusters */}
        <KpiCard
          label="Clusters"
          value={clusterCnt}
          extra={
            <div style={{ display: "flex", gap: 12, fontSize: 12 }}>
              <span style={{ display: "flex", alignItems: "center", gap: 5 }}>
                <span style={{ width: 7, height: 7, borderRadius: "50%", background: "var(--ok)" }} />
                <span style={{ color: "var(--muted)" }}>{realClusters.filter(c => clusterOk(c.status)).length} healthy</span>
              </span>
              <span style={{ display: "flex", alignItems: "center", gap: 5 }}>
                <span style={{ width: 7, height: 7, borderRadius: 2, background: "var(--warn)" }} />
                <span style={{ color: "var(--muted)" }}>{realClusters.filter(c => !clusterOk(c.status)).length} degraded</span>
              </span>
            </div>
          }
        />

        {/* Open findings */}
        <KpiCard
          label="Open findings"
          value={<span style={{ color: "var(--bad)" }}>23</span> as any}
          sub="Security vulnerabilities not yet fixed"
          extra={<span style={{ fontSize: 12, color: "var(--bad)", fontWeight: 600 }}>4 critical · from DefectDojo</span>}
        />
      </div>

      {/* Body grid */}
      <div style={{ display: "grid", gridTemplateColumns: "minmax(0,1.7fr) minmax(280px,1fr)", gap: 16, alignItems: "start" }}>
        {/* Left — Recent activity */}
        <div className="card" style={{ padding: 0 }}>
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "14px 18px 10px" }}>
            <h2 style={{ margin: 0, fontSize: 14, fontWeight: 600, color: "var(--text)" }}>Recent activity</h2>
            <div className="segmented">
              <button className={actFilter === "all" ? "active" : ""} onClick={() => setActFilter("all")}>All</button>
              <button className={actFilter === "fail" ? "active" : ""} onClick={() => setActFilter("fail")}>Failures</button>
            </div>
          </div>
          <div>
            {visibleEvents.length === 0 ? (
              <div style={{ padding: "24px 18px", fontSize: 13, color: "var(--faint)" }}>No activity recorded yet.</div>
            ) : visibleEvents.map((ev) => {
              const evStatus = activityStatus(ev.action);
              return (
                <div key={ev.id} style={{
                  display: "grid", gridTemplateColumns: "16px minmax(0,1fr) auto",
                  gap: 12, padding: "13px 18px",
                  borderTop: "1px solid var(--line)",
                  alignItems: "center",
                }}>
                  <ActivityDot status={evStatus} />
                  <div>
                    <div style={{ fontSize: 13, fontWeight: 500, color: "var(--text)" }}>{activityTitle(ev)}</div>
                    <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12, color: "var(--muted)" }}>
                      {activitySub(ev)}
                    </div>
                  </div>
                  <div style={{ textAlign: "right" }}>
                    <StatusBadge status={evStatus === "success" ? "active" : evStatus === "failed" ? "failed" : "provisioning"} />
                    <div style={{ fontSize: 11, color: "var(--faint)", marginTop: 3 }}>
                      {new Date(ev.created_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
          <div style={{ padding: "12px 18px", borderTop: "1px solid var(--line)", textAlign: "center" }}>
            <Link to="/audit" style={{ fontSize: 12, color: "var(--accent)", textDecoration: "none" }}>
              View full audit log →
            </Link>
          </div>
        </div>

        {/* Right column */}
        <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
          {/* Onboard CTA */}
          <Link to="/applications/new" style={{ textDecoration: "none" }}>
            <div style={{
              background: "var(--accent-soft)", border: "1px solid var(--accent)",
              borderRadius: 10, padding: 18,
              display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 12,
              transition: "filter .15s",
              cursor: "pointer",
            }}
              onMouseEnter={e => (e.currentTarget as HTMLElement).style.filter = "brightness(1.08)"}
              onMouseLeave={e => (e.currentTarget as HTMLElement).style.filter = "none"}
            >
              <div>
                <div style={{ fontSize: 15, fontWeight: 600, color: "var(--text)", marginBottom: 6 }}>Onboard new service</div>
                <div style={{ fontSize: 12, color: "var(--muted)", lineHeight: 1.5 }}>
                  Repo, pipeline, registry, manifests and GitOps app in about four minutes.
                </div>
              </div>
              <span style={{ fontSize: 18, color: "var(--accent)", flexShrink: 0 }}>→</span>
            </div>
          </Link>

          {/* Cluster health */}
          <div className="card" style={{ padding: 0 }}>
            <div style={{ padding: "12px 16px 8px", borderBottom: "1px solid var(--line)" }}>
              <h3 style={{ margin: 0, fontSize: 13, fontWeight: 600, color: "var(--text)" }}>Cluster health</h3>
            </div>
            {realClusters.length === 0 ? (
              <div style={{ padding: "16px", fontSize: 12, color: "var(--faint)" }}>No clusters registered.</div>
            ) : realClusters.map((c, i) => {
              const ok = clusterOk(c.status);
              return (
                <div key={c.id} style={{
                  display: "flex", alignItems: "center", gap: 10,
                  padding: "10px 16px", borderTop: i > 0 ? "1px solid var(--line)" : "none",
                }}>
                  <span style={{
                    width: 7, height: 7, flexShrink: 0,
                    borderRadius: ok ? "50%" : 2,
                    background: ok ? "var(--ok)" : "var(--warn)",
                  }} />
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12, color: "var(--text)" }}>{c.name}</div>
                    <div style={{ fontSize: 11, color: "var(--faint)" }}>{c.environment} · {c.display_name}</div>
                  </div>
                  <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12, fontWeight: 500, color: "var(--muted)" }}>
                    {c.status}
                  </div>
                </div>
              );
            })}
          </div>

          {/* Quick actions */}
          <div className="card" style={{ padding: 0 }}>
            <div style={{ padding: "12px 16px 8px", borderBottom: "1px solid var(--line)" }}>
              <h3 style={{ margin: 0, fontSize: 13, fontWeight: 600, color: "var(--text)" }}>Quick actions</h3>
            </div>
            {[
              { label: "Edit pipeline templates", to: "/templates" },
              { label: "Manage environment profiles", to: "/platform" },
              { label: "Browse applications", to: "/applications" },
            ].map((a, i) => (
              <Link key={i} to={a.to} style={{
                display: "flex", justifyContent: "space-between", alignItems: "center",
                padding: "10px 16px", borderTop: i > 0 ? "1px solid var(--line)" : "none",
                textDecoration: "none", color: "var(--muted)", fontSize: 13,
                transition: "color .12s",
              }}
                onMouseEnter={e => (e.currentTarget as HTMLElement).style.color = "var(--text)"}
                onMouseLeave={e => (e.currentTarget as HTMLElement).style.color = "var(--muted)"}
              >
                {a.label}
                <span style={{ color: "var(--faint)" }}>→</span>
              </Link>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
