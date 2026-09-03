// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
import { useState } from "react";
import { useAuditEvents, AuditEvent } from "@/lib/api";

type Outcome = "Allowed" | "Denied";

function inferOutcome(action: string): Outcome {
  if (/deny|denied|reject|forbidden|fail/i.test(action)) return "Denied";
  return "Allowed";
}

function inferCategory(resourceType: string): string {
  const m: Record<string, string> = {
    project:      "Provisioning",
    service:      "Provisioning",
    application:  "Applications",
    user:         "Users",
    credential:   "Credentials",
    team:         "Teams",
    template:     "Templates",
    cluster:      "Platform",
    session:      "Auth",
    auth:         "Auth",
  };
  return m[resourceType?.toLowerCase()] ?? resourceType ?? "General";
}

function formatAction(action: string): string {
  return action.replace(/\./g, " › ").replace(/_/g, " ");
}

function relativeTs(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const secs = Math.floor(diff / 1000);
  if (secs < 60) return `${secs}s ago`;
  const mins = Math.floor(secs / 60);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  return new Date(iso).toLocaleDateString();
}

function OutcomeBadge({ outcome }: { outcome: Outcome }) {
  if (outcome === "Denied")
    return <span className="badge badge-failed">Denied</span>;
  return <span className="badge badge-success">Allowed</span>;
}

function DetailPanel({ event }: { event: AuditEvent }) {
  const detail = event.detail ?? {};
  const keys = Object.keys(detail);
  return (
    <div style={{
      display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16,
      padding: "16px 18px", borderTop: "1px solid var(--line)",
      background: "var(--bg)",
    }}>
      <div>
        <div className="overline" style={{ marginBottom: 8 }}>Context</div>
        <div style={{ display: "grid", gridTemplateColumns: "auto 1fr", gap: "4px 12px", alignItems: "start" }}>
          <span style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11.5, color: "var(--faint)" }}>id</span>
          <span style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11.5, color: "var(--muted)" }}>{event.id}</span>
          {event.actor_email && <>
            <span style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11.5, color: "var(--faint)" }}>actor</span>
            <span style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11.5, color: "var(--muted)" }}>{event.actor_email}</span>
          </>}
          {event.resource_id && <>
            <span style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11.5, color: "var(--faint)" }}>resource_id</span>
            <span style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11.5, color: "var(--muted)" }}>{event.resource_id}</span>
          </>}
          {keys.map(k => (
            <>
              <span key={k + "_k"} style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11.5, color: "var(--faint)" }}>{k}</span>
              <span key={k + "_v"} style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11.5, color: "var(--muted)", wordBreak: "break-all" }}>
                {String((detail as Record<string, unknown>)[k])}
              </span>
            </>
          ))}
        </div>
      </div>
      <div>
        <div className="overline" style={{ marginBottom: 8 }}>Payload</div>
        <pre style={{
          margin: 0, fontFamily: "JetBrains Mono,monospace", fontSize: 11,
          color: "var(--muted)", background: "var(--panel)", padding: 12,
          borderRadius: 7, overflowX: "auto", whiteSpace: "pre-wrap", wordBreak: "break-all",
          maxHeight: 180, overflowY: "auto",
        }}>{keys.length > 0 ? JSON.stringify(detail, null, 2) : "—"}</pre>
      </div>
    </div>
  );
}

const ALL_CATEGORIES = ["Auth","Provisioning","Applications","Users","Credentials","Teams","Templates","Platform","General"];

export function AuditLogPage() {
  const { data: events = [], isLoading } = useAuditEvents();
  const [search, setSearch] = useState("");
  const [cat, setCat] = useState("All");
  const [outcome, setOutcome] = useState<"All" | "Allowed" | "Denied">("All");
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const filtered = events.filter((e: AuditEvent) => {
    const q = search.toLowerCase();
    const matchSearch = !q
      || e.action.toLowerCase().includes(q)
      || (e.actor_email ?? "").toLowerCase().includes(q)
      || e.resource_type.toLowerCase().includes(q);
    const category = inferCategory(e.resource_type);
    const matchCat = cat === "All" || category === cat;
    const ev_outcome = inferOutcome(e.action);
    const matchOutcome = outcome === "All" || ev_outcome === outcome;
    return matchSearch && matchCat && matchOutcome;
  });

  return (
    <div style={{ padding: 28, maxWidth: 1400, display: "flex", flexDirection: "column", gap: 22 }}>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between" }}>
        <div>
          <h1 className="page-h1">Audit log</h1>
          <p className="page-sub">Immutable record of every significant platform action.</p>
        </div>
      </div>

      {/* Filters */}
      <div style={{ display: "flex", gap: 10, alignItems: "center", flexWrap: "wrap" }}>
        <input className="field" style={{ maxWidth: 260 }} placeholder="Search action, actor or resource"
          value={search} onChange={e => setSearch(e.target.value)} />
        <select className="field" style={{ width: "auto", padding: "0 12px" }}
          value={cat} onChange={e => setCat(e.target.value)}>
          <option>All</option>
          {ALL_CATEGORIES.map(c => <option key={c}>{c}</option>)}
        </select>
        <div className="segmented" style={{ marginLeft: "auto" }}>
          {(["All", "Allowed", "Denied"] as const).map(o => (
            <button key={o} className={outcome === o ? "active" : ""} onClick={() => setOutcome(o)}>{o}</button>
          ))}
        </div>
      </div>

      {/* Table */}
      <div className="tbl-wrap" style={{ overflowX: "auto" }}>
        <div className="tbl-head" style={{
          gridTemplateColumns: "140px 170px minmax(190px,1fr) minmax(160px,1fr) 100px 130px",
          minWidth: 900,
        }}>
          <div>Timestamp</div>
          <div>Actor</div>
          <div>Action</div>
          <div>Resource</div>
          <div>Outcome</div>
          <div>Category</div>
        </div>

        {isLoading ? (
          <div style={{ padding: "24px 18px", fontSize: 13, color: "var(--faint)" }}>Loading events…</div>
        ) : filtered.length === 0 ? (
          <div style={{ padding: "24px 18px", fontSize: 13, color: "var(--faint)" }}>
            {events.length === 0 ? "No audit events recorded yet." : "No events match these filters."}
          </div>
        ) : filtered.map((e: AuditEvent) => {
          const isExpanded = expandedId === e.id;
          const ev_outcome = inferOutcome(e.action);
          const category = inferCategory(e.resource_type);
          return (
            <div key={e.id} style={{ borderBottom: "1px solid var(--line)" }}>
              <div
                className="tbl-row"
                style={{
                  gridTemplateColumns: "140px 170px minmax(190px,1fr) minmax(160px,1fr) 100px 130px",
                  minWidth: 900, cursor: "pointer",
                  background: isExpanded ? "var(--card)" : "",
                }}
                onClick={() => setExpandedId(isExpanded ? null : e.id)}
              >
                <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11.5, color: "var(--faint)" }}>
                  {relativeTs(e.created_at)}
                </div>
                <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 12, color: "var(--muted)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                  {e.actor_email ? e.actor_email.split("@")[0] : e.user_id ? "service-account" : "system"}
                </div>
                <div style={{ fontSize: 13, fontWeight: 500, color: "var(--text)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                  {formatAction(e.action)}
                </div>
                <div style={{ fontFamily: "JetBrains Mono,monospace", fontSize: 11.5, color: "var(--faint)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                  {e.resource_type}{e.resource_id ? `/${e.resource_id.slice(0,8)}` : ""}
                </div>
                <div><OutcomeBadge outcome={ev_outcome} /></div>
                <div style={{ fontSize: 12, color: "var(--muted)" }}>{category}</div>
              </div>
              {isExpanded && <DetailPanel event={e} />}
            </div>
          );
        })}
      </div>

      {/* Footer */}
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", fontSize: 12, color: "var(--faint)" }}>
        <span>{filtered.length} of {events.length} events</span>
        <div style={{ display: "flex", gap: 8 }}>
          <button className="btn btn-ghost btn-sm" disabled>← Newer</button>
          <button className="btn btn-ghost btn-sm" disabled>Older →</button>
        </div>
      </div>
    </div>
  );
}
